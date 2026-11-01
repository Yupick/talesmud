package main

import (
	"errors"
	"net/http"
	"net/http/pprof"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// startPprof serves net/http/pprof on a dedicated listener when enabled, so a
// wedged process can be profiled without a SIGQUIT dump (QA B19.6-3).
//
//	ENABLE_PPROF=true   enable the endpoint (1/true/yes/on, case-insensitive)
//	PPROF_ADDR=host:port  bind address (default 127.0.0.1:6060)
//
// Disabled by default: pprof exposes process internals (command line, heap,
// goroutines) to anyone who can reach the port. It binds to loopback unless
// PPROF_ADDR says otherwise.
func startPprof() {
	if !envTruthy(os.Getenv("ENABLE_PPROF")) {
		return
	}
	addr := os.Getenv("PPROF_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6060"
	}

	mux := http.NewServeMux()
	// Index also serves the named profiles (heap, goroutine, ...) under
	// /debug/pprof/<name>.
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.WithError(err).WithField("addr", addr).Warn("pprof server failed")
		}
	}()
	log.WithField("addr", addr).Info("pprof endpoint enabled (ENABLE_PPROF)")
}

// envTruthy reports whether an environment variable opts in.
func envTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
