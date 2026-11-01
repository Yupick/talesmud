package mudserver

import (
	"net/http/httptest"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/service"
)

// countWriteLoops reports how many (*wsTransport).writeLoop goroutines are
// alive right now. Every websocket connection owns exactly one, and it must
// exit when the connection disconnects (QA B21.5).
func countWriteLoops() int {
	buf := make([]byte, 2<<20)
	n := runtime.Stack(buf, true)
	return strings.Count(string(buf[:n]), "(*wsTransport).writeLoop")
}

// writeLoopStacks returns only the stack blocks of live writeLoops, so a
// failure log stays readable instead of dumping the whole process.
func writeLoopStacks() string {
	buf := make([]byte, 2<<20)
	n := runtime.Stack(buf, true)
	blocks := strings.Split(string(buf[:n]), "\n\n")
	kept := make([]string, 0, 8)
	for _, b := range blocks {
		if strings.Contains(b, "(*wsTransport).writeLoop") {
			kept = append(kept, b)
		}
	}
	return strings.Join(kept, "\n\n")
}

// trackingFacade/trackingUsers count UsersService.Update calls *after* the
// underlying write returned. A test that waits for the exact count therefore
// knows the previous session's presence write is finished; the atomic
// counter hands the happens-before edge to the next cycle. (attachSession
// and handleConnectionClosed mutate the shared *entities.User unsynchronised,
// so overlapping a reconnect with the previous session's persist would race —
// a pre-existing production concern outside this P2's scope.)
type trackingUsers struct {
	service.UsersService
	doneUpdates atomic.Int64
}

func (u *trackingUsers) Update(refID string, user *entities.User) error {
	err := u.UsersService.Update(refID, user)
	u.doneUpdates.Add(1)
	return err
}

type trackingFacade struct {
	service.Facade
	us *trackingUsers
}

func (f *trackingFacade) UsersService() service.UsersService { return f.us }

// TestDisconnectReleasesWriteLoopGoroutines is the QA B21.5 leak test.
//
// The Batch 20 transport runs one writeLoop goroutine per connection, parked
// in a select over out/done. The disconnect path used to close only the raw
// socket (`defer ws.Close()`), never the transport, so each clean disconnect
// leaked exactly one goroutine (~1 per connection; 494 still parked after a
// 2250-connection campaign). After N clean cycles — plain client disconnect
// and session replacement — the process must be back at its goroutine
// baseline, with zero writeLoops left behind.
func TestDisconnectReleasesWriteLoopGoroutines(t *testing.T) {
	gin.SetMode(gin.TestMode)

	base := newTestFacade(t)
	tracker := &trackingUsers{UsersService: base.UsersService()}
	srv := New(&trackingFacade{Facade: base, us: tracker}, nil).(*server)
	// Keep the classic path even if another test flipped the presentation.
	srv.SetSessionHook(nil)

	user := &entities.User{
		Entity:   &entities.Entity{ID: "user-leak"},
		RefID:    "ref-leak",
		Nickname: "Leaker",
	}
	if _, err := base.UsersService().Import(user); err != nil {
		t.Fatalf("import user: %v", err)
	}

	// Game.Run is not started in unit tests: drain the channels the
	// connect/disconnect path writes to so OnUserQuit never wedges the
	// read loop (which would itself look like a goroutine leak).
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	go func() {
		for {
			select {
			case <-srv.Game.OnUserQuit:
			case <-srv.Game.OnUserJoined:
			case <-stop:
				return
			}
		}
	}()

	r := gin.New()
	r.GET("/ws", func(c *gin.Context) {
		c.Set("user", user)
		srv.HandleConnections(c)
	})
	httpSrv := httptest.NewServer(r)
	t.Cleanup(httpSrv.Close)
	url := "ws" + strings.TrimPrefix(httpSrv.URL, "http") + "/ws"

	dial := func() *websocket.Conn {
		t.Helper()
		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			t.Fatalf("dial websocket: %v", err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		return conn
	}

	readWelcome := func(conn *websocket.Conn) {
		t.Helper()
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		if _, _, err := conn.ReadMessage(); err != nil {
			t.Fatalf("welcome frame: %v", err)
		}
	}

	// waitDetached blocks until the server has finished this user's
	// disconnect path (registry entry gone), i.e. detachSession ran.
	waitDetached := func() {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			if _, ok := srv.Clients.Get(user.ID); !ok {
				return
			}
			if time.Now().After(deadline) {
				t.Fatal("server never detached the session after client disconnect")
			}
			time.Sleep(5 * time.Millisecond)
		}
	}

	// waitUpdates blocks until target Update calls have fully completed.
	// Via the atomic this also orders the next cycle's attachSession after
	// the previous session's presence write (no -race noise from the shared
	// user object).
	waitUpdates := func(target int64) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for tracker.doneUpdates.Load() < target {
			if time.Now().After(deadline) {
				t.Fatalf("presence writes never reached %d (got %d): disconnect path did not finish",
					target, tracker.doneUpdates.Load())
			}
			time.Sleep(5 * time.Millisecond)
		}
	}

	// oneCycle is a plain client-initiated disconnect: the read loop sees
	// the socket error and runs detachSession -> handleConnectionClosed.
	// Expected Updates: attachSession + the live-disconnect presence write.
	oneCycle := func() {
		t.Helper()
		want := tracker.doneUpdates.Load() + 2
		conn := dial()
		readWelcome(conn)
		_ = conn.Close()
		waitDetached()
		waitUpdates(want)
	}

	// replaceCycle takes the session over with a newer socket: the old one
	// is closed with 4001 by attachSession, its read loop detaches as a
	// stale socket (no presence write, no UserQuit) and must not wedge the
	// new session. Expected Updates: c1 attach + c2 attach + c2 live detach.
	replaceCycle := func() {
		t.Helper()
		want := tracker.doneUpdates.Load() + 3
		c1 := dial()
		readWelcome(c1)
		c2 := dial()
		readWelcome(c2)
		_ = c1.SetReadDeadline(time.Now().Add(5 * time.Second))
		for {
			if _, _, err := c1.ReadMessage(); err != nil {
				break // closed by the server (4001) after c2 took over
			}
		}
		_ = c1.Close()
		_ = c2.Close()
		waitDetached()
		waitUpdates(want)
	}

	// Warm-up cycle first: httptest, gin and the first SQLite writes may
	// spawn background goroutines of their own, and those belong to the
	// baseline, not to the measured cycles.
	oneCycle()
	time.Sleep(150 * time.Millisecond)
	baseline := runtime.NumGoroutine()
	baselineLoops := countWriteLoops()

	const plainCycles = 30
	const replaceCycles = 10
	for i := 0; i < plainCycles; i++ {
		oneCycle()
	}
	for i := 0; i < replaceCycles; i++ {
		replaceCycle()
	}

	// With the bug, every cycle parks one writeLoop forever: the count stays
	// at baseline+40 and never converges, so this wait separates a real leak
	// from ordinary shutdown noise (margin covers stragglers still winding
	// down from the last cycle).
	const margin = 5
	deadline := time.Now().Add(10 * time.Second)
	for {
		got, loops := runtime.NumGoroutine(), countWriteLoops()
		if got <= baseline+margin && loops <= baselineLoops {
			t.Logf("goroutines back at baseline: %d (baseline %d, margin %d), writeLoops %d (baseline %d) after %d cycles",
				got, baseline, margin, loops, baselineLoops, plainCycles+replaceCycles)
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("goroutines did not return to baseline after %d connect/disconnect cycles:\n"+
				"got %d goroutines (baseline %d, allowed %d), %d writeLoop(s) still parked (baseline %d)\n"+
				"leaked one writeLoop per disconnect — QA B21.5\n%s",
				plainCycles+replaceCycles, got, baseline, baseline+margin, loops, baselineLoops, writeLoopStacks())
		}
		time.Sleep(50 * time.Millisecond)
	}
}
