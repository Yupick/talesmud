package mudserver

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/talesmud/talesmud/pkg/db/sqlite"
	"github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/mudserver/game/messages"
	"github.com/talesmud/talesmud/pkg/repository"
	"github.com/talesmud/talesmud/pkg/service"
)

// newTestFacade builds a throwaway SQLite-backed facade for mudserver tests.
func newTestFacade(t *testing.T) service.Facade {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "mud.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return service.NewFacade(repository.NewSQLiteFactory(db), nil)
}

// newTestMud builds a server with an imported user. Neither Game.Run nor the
// join forwarder is started: nothing consumes Game.OnUserJoined, which is
// exactly the stalled-game-loop condition the join queue must survive.
func newTestMud(t *testing.T) (*server, service.Facade, *entities.User) {
	t.Helper()
	facade := newTestFacade(t)
	user := &entities.User{
		Entity:   &entities.Entity{ID: "user-test"},
		RefID:    "ref-test",
		Nickname: "Tester",
	}
	if _, err := facade.UsersService().Import(user); err != nil {
		t.Fatalf("import user: %v", err)
	}
	srv := New(facade, nil).(*server)
	// Keep the classic path even if another test flipped the presentation.
	srv.SetSessionHook(nil)
	return srv, facade, user
}

// fakeTransport records sends; it never blocks, like a healthy client.
type fakeTransport struct {
	mu         sync.Mutex
	sent       []any
	closeCalls int
}

func (f *fakeTransport) Send(v any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, v)
	return nil
}

func (f *fakeTransport) Close(code int, reason string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closeCalls++
}

func (f *fakeTransport) RemoteIP() string { return "127.0.0.1" }

func (f *fakeTransport) Kind() string { return "fake" }

// TestJoinQueueIsFIFOAndCloseReleasesPopper covers the queue primitive itself.
func TestJoinQueueIsFIFOAndCloseReleasesPopper(t *testing.T) {
	q := newJoinQueue(4)

	want := make([]string, 0, 5)
	for i := 0; i < 5; i++ {
		id := "user-" + string(rune('a'+i))
		want = append(want, id)
		q.push(&messages.UserJoined{User: &entities.User{Entity: &entities.Entity{ID: id}}})
	}
	if got := q.backlog(); got != 5 {
		t.Fatalf("backlog = %d, want 5", got)
	}
	for _, id := range want {
		got, ok := q.pop()
		if !ok {
			t.Fatal("pop reported a closed queue with items pending")
		}
		if got.User.ID != id {
			t.Fatalf("FIFO broken: got %q, want %q", got.User.ID, id)
		}
	}

	// A blocked popper must be released by close.
	popped := make(chan bool, 1)
	go func() {
		_, ok := q.pop()
		popped <- ok
	}()
	time.Sleep(20 * time.Millisecond)
	q.close()
	select {
	case ok := <-popped:
		if ok {
			t.Fatal("pop on a closed empty queue must report closed")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("close did not release a blocked popper")
	}
}

// TestAttachSessionJoinEnqueueNeverBlocksReadLoops is the P0 cascade test:
// attachSession used to send directly to Game.OnUserJoined (buffer 20) from
// every connection's read loop, parking all later connections when the game
// loop was not draining — 712 goroutines wedged that way in QA B19.3. The
// enqueue must return for every join regardless of the game loop's state.
func TestAttachSessionJoinEnqueueNeverBlocksReadLoops(t *testing.T) {
	srv, _, user := newTestMud(t)

	const joins = 100 // old OnUserJoined buffer holds 20
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < joins; i++ {
			srv.attachSession(user, &fakeTransport{})
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("attachSession blocked: read loops would stall (backlog %d, channel %d)",
			srv.joins.backlog(), len(srv.Game.OnUserJoined))
	}

	if got := srv.joins.backlog(); got != joins {
		t.Fatalf("queued joins = %d, want %d (joins must be queued, never dropped)", got, joins)
	}
	if got := len(srv.Game.OnUserJoined); got != 0 {
		t.Fatalf("game channel has %d joins with no consumer running, want 0", got)
	}

	// With the forwarder running, the first buffer-worth reaches the game
	// channel and the rest wait in the queue: no loss, no blocking.
	go srv.forwardJoins()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && len(srv.Game.OnUserJoined) < 20 {
		time.Sleep(10 * time.Millisecond)
	}
	if got := len(srv.Game.OnUserJoined); got != 20 {
		t.Fatalf("forwarded joins = %d, want the full channel buffer 20", got)
	}
	// Everything is either in the channel, in the queue, or in flight in the
	// forwarder — and the forwarder cannot proceed because nothing consumes.
	total := len(srv.Game.OnUserJoined) + srv.joins.backlog()
	if total < joins-1 || total > joins {
		t.Fatalf("accounted joins = %d, want %d or %d (channel + backlog + in-flight): joins were lost",
			total, joins, joins-1)
	}

	t.Cleanup(func() {
		// Unblock the forwarder, then let it observe the closed queue.
		srv.joins.close()
		go func() {
			for range srv.Game.OnUserJoined {
			}
		}()
	})
}
