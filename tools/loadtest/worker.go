package main

// worker.go drives one concurrent client: a WebSocket session that sends
// application-level "hello" probes and measures round-trip latency, or an HTTP
// worker that hammers an authenticated JSON endpoint.

import (
	"io"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type workerConfig struct {
	mode     string // "ws" or "http"
	index    int
	wsURL    string
	httpPath string
	rate     float64 // probes per second (0 = connection hold only)
	duration time.Duration
	timeout  time.Duration
	auth     *authConfig
}

type incoming struct {
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
	Key     string `json:"key,omitempty"`
	Proto   int    `json:"proto,omitempty"`
	Client  string `json:"client,omitempty"`
}

func runWorker(cfg workerConfig, st *stats) {
	switch cfg.mode {
	case "http":
		runHTTPWorker(cfg, st)
	default:
		runWSWorker(cfg, st)
	}
}

func runWSWorker(cfg workerConfig, st *stats) {
	target, header, err := cfg.auth.target(cfg.index, cfg.wsURL)
	if err != nil {
		st.addInitErr()
		return
	}

	dialer := &websocket.Dialer{HandshakeTimeout: cfg.timeout}
	start := time.Now()
	conn, _, err := dialer.Dial(target, header)
	if err != nil {
		st.addConnect(false, 0)
		return
	}
	defer conn.Close()

	// Wait for the server's opening message; that marks a usable session.
	_ = conn.SetReadDeadline(time.Now().Add(cfg.timeout))
	if _, _, err := conn.ReadMessage(); err != nil {
		st.addConnect(false, 0)
		return
	}
	st.addConnect(true, time.Since(start))

	deadline := time.Now().Add(cfg.duration)
	interval := time.Duration(0)
	if cfg.rate > 0 {
		interval = time.Duration(float64(time.Second) / cfg.rate)
	}

	// Always send one probe so we have at least one RTT sample.
	probe(cfg, conn, st)
	if interval <= 0 {
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for time.Now().Before(deadline) {
		<-ticker.C
		if !probe(cfg, conn, st) {
			return
		}
	}
}

// probe sends one application message and waits for a reply. It returns false
// when the connection is no longer usable.
func probe(cfg workerConfig, conn *websocket.Conn, st *stats) bool {
	out := incoming{Type: "hello", Proto: 1, Client: "loadtest/1.0", Message: ""}
	start := time.Now()
	if err := conn.WriteJSON(out); err != nil {
		st.addMsgErr()
		return false
	}
	st.addSent()

	_ = conn.SetReadDeadline(time.Now().Add(cfg.timeout))
	if _, data, err := conn.ReadMessage(); err != nil {
		st.addMsgErr()
		return false
	} else {
		_ = data
	}
	st.addRecv(time.Since(start))
	return true
}

func runHTTPWorker(cfg workerConfig, st *stats) {
	token := cfg.auth.tokenFor(cfg.index)
	if token == "" {
		var err error
		token, err = cfg.auth.createUser(cfg.index)
		if err != nil {
			st.addInitErr()
			return
		}
	}

	client := &http.Client{Timeout: cfg.timeout}
	target := cfg.auth.httpBase + cfg.httpPath
	deadline := time.Now().Add(cfg.duration)
	interval := time.Duration(0)
	if cfg.rate > 0 {
		interval = time.Duration(float64(time.Second) / cfg.rate)
	}

	for {
		start := time.Now()
		req, err := http.NewRequest(http.MethodGet, target, nil)
		if err != nil {
			st.addInitErr()
			return
		}
		req.Header.Set("Authorization", "Bearer "+token)
		if cfg.auth.origin != "" {
			req.Header.Set("Origin", cfg.auth.origin)
		}
		resp, err := client.Do(req)
		if err != nil {
			st.addMsgErr()
		} else {
			io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				st.addSent()
				st.addRecv(time.Since(start))
			} else {
				st.addMsgErr()
			}
		}
		if interval <= 0 {
			return
		}
		if time.Now().Add(interval).After(deadline) {
			return
		}
		time.Sleep(interval)
	}
}
