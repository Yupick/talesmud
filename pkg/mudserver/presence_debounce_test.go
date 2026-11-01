package mudserver

import (
	"sync/atomic"
	"testing"

	"github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/service"
)

// countingFacade swaps in a UsersService that counts Update calls while
// delegating to the real service, so the DB still behaves normally.
type countingFacade struct {
	service.Facade
	us *countingUsers
}

func (c *countingFacade) UsersService() service.UsersService { return c.us }

type countingUsers struct {
	service.UsersService
	updates    atomic.Int64
	lastOnline atomic.Bool
	lastRef    atomic.Value
}

func (c *countingUsers) Update(refID string, user *entities.User) error {
	c.updates.Add(1)
	c.lastOnline.Store(user.IsOnline)
	c.lastRef.Store(refID)
	return c.UsersService.Update(refID, user)
}

// TestPresencePersistedOnConnectAndDisconnectOnly pins the P1 debounce
// (QA B19.6-1): LastSeen/IsOnline used to be written to SQLite twice per
// input line (ConnectUserSession + handleInput). Presence is now refreshed
// in memory on every message and persisted once on connect and once on
// disconnect — an offline session must still end up offline in the DB.
func TestPresencePersistedOnConnectAndDisconnectOnly(t *testing.T) {
	base := newTestFacade(t)
	counter := &countingUsers{UsersService: base.UsersService()}
	counter.lastRef.Store("")

	srv := New(&countingFacade{Facade: base, us: counter}, nil).(*server)
	srv.SetSessionHook(nil)

	user := &entities.User{
		Entity:   &entities.Entity{ID: "user-presence"},
		RefID:    "ref-presence",
		Nickname: "Presence",
	}
	if _, err := base.UsersService().Import(user); err != nil {
		t.Fatalf("import user: %v", err)
	}

	conn := srv.attachSession(user, &fakeTransport{})
	if got := counter.updates.Load(); got != 1 {
		t.Fatalf("presence writes after connect = %d, want 1", got)
	}
	if !counter.lastOnline.Load() {
		t.Fatal("connect must persist online=true")
	}

	// The game loop is not running here; drain the input queue like it does
	// so handleInput never blocks on its send.
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	go func() {
		for {
			select {
			case <-srv.Game.OnMessageReceived():
			case <-stop:
				return
			}
		}
	}()

	const inputs = 50
	for i := 0; i < inputs; i++ {
		srv.handleInput(user, "look", false)
	}
	if got := counter.updates.Load(); got != 1 {
		t.Fatalf("presence writes after %d inputs = %d, want 1 (debounced to connect/disconnect)", inputs, got)
	}
	if !user.IsOnline {
		t.Fatal("in-memory presence must stay online during the session")
	}

	// The guard is a real transition check, not a blanket skip: an
	// offline -> online flip still persists (callers that attach outside
	// attachSession rely on it).
	user.IsOnline = false
	srv.handleInput(user, "look", false)
	if got := counter.updates.Load(); got != 2 {
		t.Fatalf("presence writes after offline->online transition = %d, want 2", got)
	}
	if !counter.lastOnline.Load() {
		t.Fatal("the transition write must persist online=true")
	}

	if !srv.handleConnectionClosed(user, conn) {
		t.Fatal("disconnect was not processed")
	}
	if got := counter.updates.Load(); got != 3 {
		t.Fatalf("presence writes after disconnect = %d, want 3", got)
	}
	if counter.lastOnline.Load() {
		t.Fatal("disconnect must persist online=false as the final write")
	}
	if user.IsOnline {
		t.Fatal("in-memory user must be offline after disconnect")
	}
	if counter.lastRef.Load() != "ref-presence" {
		t.Fatalf("persisted ref = %v, want ref-presence", counter.lastRef.Load())
	}
}
