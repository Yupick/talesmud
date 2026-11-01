// Package qa_b22 holds the independent QA harnesses for Batch 22.
//
// Nothing in here is production code: it exists so QA can verify the B21.5
// writeLoop leak fix with its own connect/disconnect churn, written from the
// outside of pkg/mudserver (the only sanctioned place for new QA tests is
// tools/**).
package qa_b22

import (
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/talesmud/talesmud/pkg/db/sqlite"
	"github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/mudserver"
	"github.com/talesmud/talesmud/pkg/mudserver/game"
	"github.com/talesmud/talesmud/pkg/repository"
	"github.com/talesmud/talesmud/pkg/service"
)

// countWriteLoops reports how many (*wsTransport).writeLoop goroutines are
// alive. Every websocket connection owns exactly one; a disconnected client
// must never leave it parked (QA B21.5/B22).
func countWriteLoops() int {
	buf := make([]byte, 2<<20)
	n := runtime.Stack(buf, true)
	return strings.Count(string(buf[:n]), "(*wsTransport).writeLoop")
}

// writeLoopStacks dumps only the live writeLoop stack blocks so a failure
// log stays readable.
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

// topGoroutines summarises the 8 most frequent function names on live
// stacks, to show *what* grew when a run fails.
func topGoroutines() string {
	buf := make([]byte, 2<<20)
	n := runtime.Stack(buf, true)
	counts := map[string]int{}
	for _, block := range strings.Split(string(buf[:n]), "\n\n") {
		lines := strings.Split(block, "\n")
		if len(lines) < 2 {
			continue
		}
		frame := strings.TrimSpace(lines[1]) // lines[0] is the "goroutine N" header
		if i := strings.LastIndexByte(frame, '('); i > 0 { // drop only the "(0x…)" args
			frame = frame[:i]
		}
		if i := strings.LastIndexByte(frame, '/'); i >= 0 {
			frame = frame[i+1:]
		}
		if frame == "" {
			continue
		}
		counts[frame]++
	}
	type kv struct {
		k string
		v int
	}
	all := make([]kv, 0, len(counts))
	for k, v := range counts {
		all = append(all, kv{k, v})
	}
	// insertion-sort the top 8: the map is small (dozens of entries)
	for i := 1; i < len(all); i++ {
		for j := i; j > 0 && all[j].v > all[j-1].v; j-- {
			all[j], all[j-1] = all[j-1], all[j]
		}
	}
	if len(all) > 8 {
		all = all[:8]
	}
	out := make([]string, 0, len(all))
	for _, e := range all {
		out = append(out, fmt.Sprintf("%4d  %s", e.v, e.k))
	}
	return strings.Join(out, "\n")
}

// TestChurnConnectDisconnectReturnsToBaseline is QA's own B22 verification:
// it does NOT reuse the in-package transport_leak_test.go. It drives
// HandleConnections through a real websocket for N connect/disconnect cycles
// (plain disconnects and session takeovers) and requires both the total
// goroutine count and the number of live writeLoop goroutines to come back
// to the pre-churn baseline. It repeats the whole measurement for several
// rounds, so a slow leak cannot hide inside one warm-up.
func TestChurnConnectDisconnectReturnsToBaseline(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := sqlite.Open(filepath.Join(t.TempDir(), "qa_b22.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	facade := service.NewFacade(repository.NewSQLiteFactory(db), nil)

	user := &entities.User{
		Entity:   &entities.Entity{ID: "user-churn"},
		RefID:    "ref-churn",
		Nickname: "Churner",
	}
	if _, err := facade.UsersService().Import(user); err != nil {
		t.Fatalf("import user: %v", err)
	}

	srv := mudserver.New(facade, nil)
	srv.SetSessionHook(nil)

	g, ok := srv.GameCtrl().(*game.Game)
	if !ok {
		t.Fatalf("GameCtrl() is %T, want *game.Game", srv.GameCtrl())
	}

	// Game.Run is never started in this harness: drain the channels the
	// disconnect path writes to so handleConnectionClosed cannot wedge.
	stop := make(chan struct{})
	quits := make(chan struct{}, 8192)
	t.Cleanup(func() { close(stop) })
	go func() {
		for {
			select {
			case <-g.OnUserQuit:
				select {
				case quits <- struct{}{}:
				default: // never block the drain on a full observation buffer
				}
			case <-g.OnUserJoined:
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

	// waitQuit waits for the server's live-session disconnect signal
	// (Game.OnUserQuit), i.e. detachSession ran for a session it owns.
	waitQuit := func(what string) {
		t.Helper()
		select {
		case <-quits:
		case <-time.After(5 * time.Second):
			t.Fatalf("no OnUserQuit after %s: server never finished the disconnect path", what)
		}
	}

	// plainCycle: client drops the socket; the read loop detaches the session.
	plainCycle := func() {
		t.Helper()
		conn := dial()
		readWelcome(conn)
		_ = conn.Close()
		waitQuit("plain disconnect")
	}

	// replaceCycle: a newer socket takes the same user over; the old one is
	// closed with 4001 and detaches as stale (no quit), the new one quits.
	replaceCycle := func() {
		t.Helper()
		c1 := dial()
		readWelcome(c1)
		c2 := dial()
		readWelcome(c2)
		_ = c1.SetReadDeadline(time.Now().Add(5 * time.Second))
		for {
			if _, _, err := c1.ReadMessage(); err != nil {
				break // server closed the replaced socket
			}
		}
		_ = c1.Close()
		_ = c2.Close()
		waitQuit("replace disconnect")
	}

	// Warm-up: httptest/gin/first SQLite writes spawn housekeeping goroutines
	// that belong to the baseline, not to the measured churn.
	plainCycle()
	time.Sleep(200 * time.Millisecond)

	const margin = 5
	const rounds = 3
	const plainCycles = 30
	const replaceCycles = 10

	for round := 1; round <= rounds; round++ {
		baseline := runtime.NumGoroutine()
		baselineLoops := countWriteLoops()

		for i := 0; i < plainCycles; i++ {
			plainCycle()
		}
		for i := 0; i < replaceCycles; i++ {
			replaceCycle()
		}

		// A leaking build parks exactly one writeLoop per cycle and never
		// converges; a fixed build winds down within a few scheduler ticks.
		deadline := time.Now().Add(10 * time.Second)
		converged := false
		var got, loops int
		for {
			got, loops = runtime.NumGoroutine(), countWriteLoops()
			if got <= baseline+margin && loops <= baselineLoops {
				converged = true
				break
			}
			if time.Now().After(deadline) {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}

		t.Logf("round %d: baseline=%d goroutines/%d writeLoops -> after %d cycles: %d goroutines/%d writeLoops (converged=%v)",
			round, baseline, baselineLoops, plainCycles+replaceCycles, got, loops, converged)

		if !converged {
			t.Fatalf("round %d: goroutines did NOT return to baseline after %d connect/disconnect cycles:\n"+
				"got %d goroutines (baseline %d, allowed %d), %d writeLoop(s) parked (baseline %d)\n"+
				"=== live writeLoop stacks ===\n%s\n=== top goroutines ===\n%s",
				round, plainCycles+replaceCycles, got, baseline, baseline+margin,
				loops, baselineLoops, writeLoopStacks(), topGoroutines())
		}
	}
}
