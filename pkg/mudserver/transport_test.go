package mudserver

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// dialWS opens a client socket against a local httptest server.
func dialWS(t *testing.T, httpURL string) *websocket.Conn {
	t.Helper()
	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(httpURL, "http"), nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	return ws
}

// TestWSTransportSendDeliversInOrder covers the happy path of the queued
// writer: Send must still be synchronous-free but never reorder or lose
// frames, Close must be idempotent, and sends after Close must fail fast.
func TestWSTransportSendDeliversInOrder(t *testing.T) {
	got := make(chan string, 16)
	upgraded := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return // the test fails on the upgrade timeout below
		}
		upgraded <- peer
		for {
			var s string
			if err := peer.ReadJSON(&s); err != nil {
				return
			}
			got <- s
		}
	}))
	t.Cleanup(srv.Close)

	ws := dialWS(t, srv.URL)
	select {
	case <-upgraded:
	case <-time.After(5 * time.Second):
		t.Fatal("peer never upgraded")
	}

	tr := newWSTransport(ws, "test-peer")
	t.Cleanup(func() { tr.Close(0, "") })

	want := []string{"one", "two", "three", "four", "five"}
	for i, s := range want {
		if err := tr.Send(s); err != nil {
			t.Fatalf("Send(%q) #%d: %v", s, i, err)
		}
	}
	for i, w := range want {
		select {
		case g := <-got:
			if g != w {
				t.Fatalf("message %d = %q, want %q (order must be preserved)", i, g, w)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("message %d (%q) never arrived", i, w)
		}
	}

	// Close is idempotent; sends afterwards fail instead of hanging.
	tr.Close(4001, "done")
	tr.Close(4001, "done")
	start := time.Now()
	if err := tr.Send("after-close"); err == nil {
		t.Fatal("Send after Close must report an error")
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("Send after Close blocked for %v", d)
	}
}

// TestWSTransportSlowConsumerNeverBlocksSend is the P1 hardening test for
// QA B19.6-2: a peer that stops reading (tiny socket buffers, no reads at
// all) must not stall Send. The frame write deadline bounds the writer, the
// outbound queue absorbs the burst, and the transport then closes instead of
// blocking pings/broadcasts for everyone else.
func TestWSTransportSlowConsumerNeverBlocksSend(t *testing.T) {
	release := make(chan struct{})
	upgraded := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		if tcp, ok := peer.UnderlyingConn().(*net.TCPConn); ok {
			// Shrink the receive window so not reading really stalls the writer.
			_ = tcp.SetReadBuffer(2048)
		}
		upgraded <- struct{}{}
		<-release // never reads a single frame
		_ = peer.Close()
	}))
	t.Cleanup(srv.Close)

	ws := dialWS(t, srv.URL)
	if tcp, ok := ws.UnderlyingConn().(*net.TCPConn); ok {
		_ = tcp.SetWriteBuffer(2048)
	}
	select {
	case <-upgraded:
	case <-time.After(5 * time.Second):
		t.Fatal("peer never upgraded")
	}

	tr := newWSTransportTimeout(ws, "slow-peer", 200*time.Millisecond)
	t.Cleanup(func() {
		tr.Close(0, "")
		close(release)
	})

	// 1 MiB per frame, far beyond the ~2 KiB socket buffers on both ends.
	payload := strings.Repeat("x", 1<<20)

	var sendErr error
	slowest := time.Duration(0)
	calls := 0
	for i := 0; i < 200 && sendErr == nil; i++ {
		start := time.Now()
		sendErr = tr.Send(payload)
		calls = i + 1
		if d := time.Since(start); d > slowest {
			slowest = d
		}
		if d := time.Since(start); d > 2*time.Second {
			t.Fatalf("Send blocked %v on a stalled peer (call %d)", d, i)
		}
	}
	if sendErr == nil {
		t.Fatal("expected a stalled peer to fail Send via write deadline or full queue")
	}

	// Once failed, every send keeps failing immediately.
	start := time.Now()
	if err := tr.Send(payload); err == nil {
		t.Fatal("Send must keep failing after the transport shut down")
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("Send after failure blocked for %v", d)
	}
	t.Logf("slow consumer surfaced after %d sends (slowest Send %v, error: %v)", calls, slowest, sendErr)
}
