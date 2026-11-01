package mudserver

import (
	"errors"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

// Transport is how one player session sends and closes.
// WebSocket and SSH both implement it. Send must not be assumed to run on
// the game loop's goroutine; SSH implementations enqueue instead of writing.
type Transport interface {
	Send(v any) error
	Close(code int, reason string)
	RemoteIP() string
	Kind() string
}

const (
	// defaultWSWriteTimeout bounds one frame write on the WebSocket. A peer
	// that stops reading stalls only its own writer goroutine for at most
	// this long; the write deadline then fails the frame and the connection
	// is torn down instead of hanging a broadcast or ping round forever
	// (QA B19.6-2).
	defaultWSWriteTimeout = 10 * time.Second

	// wsOutboundQueue is the per-connection send backlog between Send and the
	// writer goroutine. It absorbs normal bursts (welcome + room snapshots +
	// chat) while the peer's TCP window is momentarily closed; anything
	// beyond it means a genuinely stalled reader.
	wsOutboundQueue = 128
)

type wsTransport struct {
	conn *websocket.Conn
	ip   string

	// out is drained by writeLoop, the only goroutine that writes data
	// frames. Send enqueues, so no caller ever blocks on a slow peer.
	out       chan any
	writeWait time.Duration

	// mu guards closed/err only; the socket itself is owned by writeLoop.
	mu     sync.Mutex
	closed bool
	err    error

	once sync.Once
	done chan struct{}
}

// newWSTransport wraps conn with the default write timeout.
func newWSTransport(conn *websocket.Conn, ip string) *wsTransport {
	return newWSTransportTimeout(conn, ip, defaultWSWriteTimeout)
}

// newWSTransportTimeout is newWSTransport with a custom frame write timeout.
// Tests use a short deadline to exercise the slow-consumer path quickly.
func newWSTransportTimeout(conn *websocket.Conn, ip string, writeWait time.Duration) *wsTransport {
	if writeWait <= 0 {
		writeWait = defaultWSWriteTimeout
	}
	w := &wsTransport{
		conn:      conn,
		ip:        ip,
		out:       make(chan any, wsOutboundQueue),
		writeWait: writeWait,
		done:      make(chan struct{}),
	}
	go w.writeLoop()
	return w
}

// writeLoop is the sole writer of data frames on this connection.
func (w *wsTransport) writeLoop() {
	for {
		select {
		case v := <-w.out:
			if err := w.writeJSON(v); err != nil {
				w.shutdown(err)
				return
			}
		case <-w.done:
			return
		}
	}
}

func (w *wsTransport) writeJSON(v any) error {
	// A fresh deadline per frame: an expired one would poison every later
	// write on this connection.
	if err := w.conn.SetWriteDeadline(time.Now().Add(w.writeWait)); err != nil {
		return err
	}
	return w.conn.WriteJSON(v)
}

// Send queues v for the write loop and returns immediately. It never writes
// on the caller's goroutine, so a client that stops reading cannot stall
// pings, broadcasts or room sends to other players. A peer that leaves the
// queue full is closed (its session reconnects cleanly) rather than allowed
// to block the server.
func (w *wsTransport) Send(v any) error {
	if w == nil || w.conn == nil {
		return errors.New("websocket closed")
	}
	if err := w.status(); err != nil {
		return err
	}
	if w.enqueue(v) {
		return nil
	}
	if err := w.status(); err != nil {
		return err
	}
	cause := errors.New("websocket: slow consumer (outbound queue full)")
	log.WithFields(log.Fields{"ip": w.ip, "queued": len(w.out)}).Warn("WS slow consumer: outbound queue full, closing connection")
	w.shutdown(cause)
	return cause
}

// enqueue pushes v, retrying once because a full queue usually means the
// writer is mid-frame and frees a slot momentarily. Returns false when the
// queue stayed full or the transport shut down.
func (w *wsTransport) enqueue(v any) bool {
	for try := 0; try < 2; try++ {
		select {
		case w.out <- v:
			return true
		default:
		}
		select {
		case <-w.done:
			return false
		default:
		}
	}
	return false
}

// status reports a transport that already failed or was closed.
func (w *wsTransport) status() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.closed {
		return nil
	}
	if w.err != nil {
		return w.err
	}
	return errors.New("websocket closed")
}

// shutdown tears the connection down exactly once and wakes every sender.
func (w *wsTransport) shutdown(cause error) {
	w.once.Do(func() {
		w.mu.Lock()
		w.closed = true
		w.err = cause
		w.mu.Unlock()
		close(w.done)
		_ = w.conn.Close()
	})
}

func (w *wsTransport) Close(code int, reason string) {
	if w == nil || w.conn == nil {
		return
	}
	w.once.Do(func() {
		w.mu.Lock()
		w.closed = true
		w.mu.Unlock()
		close(w.done)
		if code != 0 {
			// WriteControl is documented as safe to call concurrently with
			// the data writer, which may still be mid-frame here.
			_ = w.conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(code, reason),
				time.Now().Add(time.Second),
			)
		}
		_ = w.conn.Close()
	})
}

func (w *wsTransport) RemoteIP() string {
	if w == nil {
		return ""
	}
	return w.ip
}

func (w *wsTransport) Kind() string { return "ws" }
