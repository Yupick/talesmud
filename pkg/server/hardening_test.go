package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// resetJWKSCache clears the process-wide JWKS cache between tests.
func resetJWKSCache(t *testing.T) {
	t.Helper()
	jwksCache.Lock()
	jwksCache.keys = nil
	jwksCache.fetchedAt = time.Time{}
	jwksCache.Unlock()
	t.Cleanup(func() {
		jwksCache.Lock()
		jwksCache.keys = nil
		jwksCache.fetchedAt = time.Time{}
		jwksCache.Unlock()
	})
}

func clearHTTPTimeoutEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"HTTP_READ_HEADER_TIMEOUT", "HTTP_READ_TIMEOUT",
		"HTTP_WRITE_TIMEOUT", "HTTP_IDLE_TIMEOUT",
	} {
		t.Setenv(key, "")
	}
}

func TestHTTPServerTimeoutsDefault(t *testing.T) {
	clearHTTPTimeoutEnv(t)

	srv := newHTTPServer("127.0.0.1:0", http.NewServeMux())

	if srv.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("ReadHeaderTimeout = %v, want 5s", srv.ReadHeaderTimeout)
	}
	if srv.ReadTimeout != 15*time.Second {
		t.Errorf("ReadTimeout = %v, want 15s", srv.ReadTimeout)
	}
	if srv.WriteTimeout != 30*time.Second {
		t.Errorf("WriteTimeout = %v, want 30s", srv.WriteTimeout)
	}
	if srv.IdleTimeout != 60*time.Second {
		t.Errorf("IdleTimeout = %v, want 60s", srv.IdleTimeout)
	}
}

func TestHTTPServerTimeoutsFromEnv(t *testing.T) {
	t.Setenv("HTTP_READ_HEADER_TIMEOUT", "2s")
	t.Setenv("HTTP_READ_TIMEOUT", "7") // bare seconds
	t.Setenv("HTTP_WRITE_TIMEOUT", "45s")
	t.Setenv("HTTP_IDLE_TIMEOUT", "90s")

	srv := newHTTPServer("127.0.0.1:0", http.NewServeMux())

	if srv.ReadHeaderTimeout != 2*time.Second {
		t.Errorf("ReadHeaderTimeout = %v, want 2s", srv.ReadHeaderTimeout)
	}
	if srv.ReadTimeout != 7*time.Second {
		t.Errorf("ReadTimeout = %v, want 7s", srv.ReadTimeout)
	}
	if srv.WriteTimeout != 45*time.Second {
		t.Errorf("WriteTimeout = %v, want 45s", srv.WriteTimeout)
	}
	if srv.IdleTimeout != 90*time.Second {
		t.Errorf("IdleTimeout = %v, want 90s", srv.IdleTimeout)
	}
}

func TestEnvDurationFallsBackOnInvalid(t *testing.T) {
	t.Setenv("B17_BAD_DURATION", "not-a-duration")
	if got := envDuration("B17_BAD_DURATION", 3*time.Second); got != 3*time.Second {
		t.Fatalf("envDuration fallback = %v, want 3s", got)
	}
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeadersMiddleware())
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ping", nil))

	cases := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
	}
	for header, want := range cases {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	// The global middleware is deliberately conservative: no CSP here so the
	// SPA / play client / WS upgrade keep working.
	if csp := rec.Header().Get("Content-Security-Policy"); csp != "" {
		t.Errorf("global CSP unexpectedly set: %q", csp)
	}
}

func TestFetchJWKSFromURLValid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[{"kty":"RSA","kid":"k1","use":"sig","n":"abc","e":"AQAB"}]}`))
	}))
	defer srv.Close()

	keys, err := fetchJWKSFromURL(srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("fetchJWKSFromURL: %v", err)
	}
	if len(keys) != 1 || keys[0].Kid != "k1" {
		t.Fatalf("unexpected keys: %+v", keys)
	}
}

func TestFetchJWKSFromURLRejectsOversizedBody(t *testing.T) {
	oversized := `{"keys":[{"kty":"RSA","kid":"` + strings.Repeat("x", maxJWKSBytes) + `"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(oversized))
	}))
	defer srv.Close()

	keys, err := fetchJWKSFromURL(srv.Client(), srv.URL)
	if err == nil {
		t.Fatalf("expected size-limit error, got keys %+v", keys)
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected size-limit error, got %v", err)
	}
}

func TestFetchJWKSFromURLRespectsTimeout(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-block:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(block)

	client := &http.Client{Timeout: 200 * time.Millisecond}
	start := time.Now()
	keys, err := fetchJWKSFromURL(client, srv.URL)
	if err == nil {
		t.Fatalf("expected timeout error, got keys %+v", keys)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("fetch took %v, expected to time out quickly", elapsed)
	}
}

func TestFetchJWKSFromURLRejectsEmptyURL(t *testing.T) {
	if _, err := fetchJWKSFromURL(http.DefaultClient, "  "); err == nil {
		t.Fatal("expected error for empty JWKS URL")
	}
}

func TestFetchJWKSFromURLRejectsErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if _, err := fetchJWKSFromURL(srv.Client(), srv.URL); err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}

func TestFetchJWKSUsesCacheAndStaleFallback(t *testing.T) {
	resetJWKSCache(t)

	var requests int32
	var fail int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		if atomic.LoadInt32(&fail) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		payload, _ := json.Marshal(jwks{Keys: []webKeys{{Kty: "RSA", Kid: "k1"}}})
		_, _ = w.Write(payload)
	}))
	defer srv.Close()
	t.Setenv("AUTH0_WK_JWKS", srv.URL)

	keys, err := fetchJWKS()
	if err != nil || len(keys) != 1 {
		t.Fatalf("first fetch: keys=%+v err=%v", keys, err)
	}

	// Fresh cache: no second request.
	if _, err := fetchJWKS(); err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("expected 1 upstream request, got %d", got)
	}

	// Expire the cache and make the upstream fail: stale keys are returned.
	jwksCache.Lock()
	jwksCache.fetchedAt = time.Now().Add(-2 * jwksCacheTTL)
	jwksCache.Unlock()
	atomic.StoreInt32(&fail, 1)
	keys, err = fetchJWKS()
	if err != nil {
		t.Fatalf("stale fallback returned error: %v", err)
	}
	if len(keys) != 1 || keys[0].Kid != "k1" {
		t.Fatalf("stale fallback keys=%+v", keys)
	}
}

func TestFetchJWKSCoalescesConcurrentRefreshes(t *testing.T) {
	resetJWKSCache(t)

	var requests int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[{"kty":"RSA","kid":"k1"}]}`))
	}))
	defer srv.Close()
	t.Setenv("AUTH0_WK_JWKS", srv.URL)

	var wg sync.WaitGroup
	errs := make([]error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = fetchJWKS()
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: %v", i, err)
		}
	}
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("expected coalesced single upstream request, got %d", got)
	}
}
