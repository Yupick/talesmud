package qa_b22

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/talesmud/talesmud/pkg/db/sqlite"
	"github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/mudserver"
	"github.com/talesmud/talesmud/pkg/mudserver/game"
	"github.com/talesmud/talesmud/pkg/mudserver/game/messages"
	"github.com/talesmud/talesmud/pkg/repository"
	"github.com/talesmud/talesmud/pkg/service"
)

// TestSessionUserPresenceRace is the QA B22 reproduction of the PRE-EXISTENT
// data race Backend annotated while fixing the writeLoop leak:
//
//	one shared *entities.User is mutated by several connection goroutines
//	without any synchronisation:
//	  - attachSession (new socket's handler):      user.LastSeen/IsOnline = …
//	  - handleConnectionClosed (socket's handler): user.IsOnline/LastSeen = …
//	  - Game.ConnectUserSession (sessions.go):
//	        user.IsOnline = true; user.LastSeen = time.Now()
//	  - Game.SetUserSessionCharacter (sessions.go): same fields + LastCharacter
//	plus UsersService().Update(refID, user) on those goroutines, which
//	serialises the same struct for SQLite.
//
// This is NOT a B22 regression: it predates Batch 22 (it is why the
// transport_leak_test.go harness goes out of its way to order cycles with an
// atomic counter). It is opt-in because the fix belongs to game code, not to
// QA; run it under -race with:
//
//	QA_B22_REPRO_RACE=1 go test -race -count=1 -run TestSessionUserPresenceRace ./tools/qa_b22/
//
// Expected output with the race detector: WARNING: DATA RACE with
// attachSession/ConnectUserSession/handleConnectionClosed (or
// UsersService.Update) on the user struct.
func TestSessionUserPresenceRace(t *testing.T) {
	if os.Getenv("QA_B22_REPRO_RACE") == "" {
		t.Skip("opt-in reproduction of the pre-existing sessions race: set QA_B22_REPRO_RACE=1")
	}
	gin.SetMode(gin.TestMode)
	gin.DefaultWriter = io_Discard{}

	db, err := sqlite.Open(filepath.Join(t.TempDir(), "qa_b22_race.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	facade := service.NewFacade(repository.NewSQLiteFactory(db), nil)

	user := &entities.User{
		Entity:   &entities.Entity{ID: "user-race"},
		RefID:    "ref-race",
		Nickname: "Racer",
	}
	if _, err := facade.UsersService().Import(user); err != nil {
		t.Fatalf("import user: %v", err)
	}

	srv := mudserver.New(facade, nil)
	srv.SetSessionHook(nil)

	g := srv.GameCtrl().(*game.Game)
	var inputs atomic.Int64
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	go func() {
		for {
			select {
			case <-g.OnUserQuit:
			case <-g.OnUserJoined:
			case <-g.OnMessageReceived():
				inputs.Add(1)
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
		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		return conn
	}
	readWelcome := func(conn *websocket.Conn) {
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		if _, _, err := conn.ReadMessage(); err != nil {
			t.Fatalf("welcome: %v", err)
		}
	}

	// A fresh "typing" socket per round keeps handleInput alive in every
	// round (each takeover closes the previous socket with 4001).
	//
	// The 20 ms pause is NOT a synchronisation edge for the race detector
	// (the server goroutines are different goroutines with no shared lock
	// on the user struct): it only makes sure the input frame was actually
	// dispatched to handleInput -> Game.ConnectUserSession before the
	// takeover's attachSession writes the same fields.
	var previous *websocket.Conn
	const rounds = 40
	for i := 0; i < rounds; i++ {
		cTyping := dial()
		readWelcome(cTyping)
		if previous != nil {
			_ = previous.Close()
		}
		previous = cTyping

		if err := cTyping.WriteJSON(messages.IncomingMessage{Message: "look"}); err != nil {
			t.Fatalf("input write: %v", err)
		}
		// Let the read loop run handleInput (ConnectUserSession mutates the
		// shared user). No TSan ordering is implied by this sleep.
		time.Sleep(20 * time.Millisecond)

		// While the takeover attaches (attachSession), run the same presence
		// write the input path runs: handleInput -> Game.ConnectUserSession
		// (game/sessions.go:214-216). Calling it directly avoids relying on
		// frame scheduling, but it is the identical code path a client
		// message takes.
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			g.ConnectUserSession(user)
		}()
		cTakeover := dial()
		readWelcome(cTakeover) // attachSession mutates the same user fields
		wg.Wait()
		time.Sleep(5 * time.Millisecond)
		_ = cTakeover.Close() // detach path: handleConnectionClosed mutates the user
		time.Sleep(5 * time.Millisecond)
	}
	if previous != nil {
		_ = previous.Close()
	}

	// Give the last disconnect time to run its presence write.
	time.Sleep(200 * time.Millisecond)
	t.Logf("%d takeover/input rounds completed against one shared *entities.User (%d input lines dispatched; run under -race to see the report)", rounds, inputs.Load())
}

// io_Discard keeps the race report readable: gin logs every request otherwise.
type io_Discard struct{}

func (io_Discard) Write(p []byte) (int, error) { return len(p), nil }
