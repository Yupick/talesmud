package main

// Tests for the pure harness logic in loadtest: percentile math, latency
// summaries, the WS->HTTP base URL derivation and ticket URL rewrites. The
// server handshake itself is covered by pkg/server and pkg/mudserver tests.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPercentileNearestRank(t *testing.T) {
	// 1..100 ms: nearest-rank p-th value is the ceil(p*n)-th element.
	sorted := make([]time.Duration, 100)
	for i := range sorted {
		sorted[i] = time.Duration(i+1) * time.Millisecond
	}
	cases := []struct {
		p    float64
		want time.Duration
	}{
		{0.50, 50 * time.Millisecond},
		{0.90, 90 * time.Millisecond},
		{0.95, 95 * time.Millisecond},
		{0.99, 99 * time.Millisecond},
		{1.00, 100 * time.Millisecond},
	}
	for _, c := range cases {
		if got := percentile(sorted, c.p); got != c.want {
			t.Errorf("percentile(%.2f) = %v, want %v", c.p, got, c.want)
		}
	}

	if got := percentile(nil, 0.5); got != 0 {
		t.Errorf("percentile(nil) = %v, want 0", got)
	}
	// p below 1/n clamps to the first element, not a negative index.
	if got := percentile([]time.Duration{7 * time.Millisecond}, 0.01); got != 7*time.Millisecond {
		t.Errorf("percentile clamp low = %v, want 7ms", got)
	}
}

func TestSummarizeKnownSamples(t *testing.T) {
	samples := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
	}
	sum := summarize(samples)
	if sum.Count != 4 {
		t.Fatalf("Count = %d, want 4", sum.Count)
	}
	if sum.MinMs != 10 || sum.MaxMs != 40 {
		t.Errorf("Min/Max = %v/%v, want 10/40", sum.MinMs, sum.MaxMs)
	}
	if sum.AvgMs != 25 {
		t.Errorf("Avg = %v, want 25", sum.AvgMs)
	}
	if sum.P50Ms != 20 || sum.P95Ms != 40 || sum.P99Ms != 40 {
		t.Errorf("p50/p95/p99 = %v/%v/%v, want 20/40/40", sum.P50Ms, sum.P95Ms, sum.P99Ms)
	}
}

func TestSummarizeEmpty(t *testing.T) {
	sum := summarize(nil)
	if sum.Count != 0 || sum.MinMs != 0 || sum.P50Ms != 0 || sum.MaxMs != 0 || sum.AvgMs != 0 {
		t.Fatalf("empty summarize should be all zero: %+v", sum)
	}
}

func TestSummarizeDoesNotMutateInput(t *testing.T) {
	in := []time.Duration{30 * time.Millisecond, 10 * time.Millisecond, 20 * time.Millisecond}
	summarize(in)
	if in[0] != 30*time.Millisecond || in[1] != 10*time.Millisecond {
		t.Fatalf("summarize sorted the caller's slice: %v", in)
	}
}

func TestMSConversion(t *testing.T) {
	if got := ms(1500 * time.Microsecond); got != 1.5 {
		t.Errorf("ms(1.5ms) = %v, want 1.5", got)
	}
	if got := ms(2 * time.Second); got != 2000 {
		t.Errorf("ms(2s) = %v, want 2000", got)
	}
}

func TestStatsCountersAndSnapshot(t *testing.T) {
	st := &stats{}
	st.addInitErr()
	st.addConnect(true, 5*time.Millisecond)
	st.addConnect(false, 0)
	st.addSent()
	st.addSent()
	st.addRecv(12 * time.Millisecond)
	st.addMsgErr()

	snap := st.snapshot()
	if snap.Attempted != 3 {
		t.Errorf("Attempted = %d, want 3", snap.Attempted)
	}
	if snap.Connected != 1 || snap.ConnErrors != 1 || snap.InitErrors != 1 {
		t.Errorf("connect counters = %d/%d/%d, want 1/1/1", snap.Connected, snap.ConnErrors, snap.InitErrors)
	}
	if snap.MsgsSent != 2 || snap.MsgsRecv != 1 || snap.MsgErrors != 1 {
		t.Errorf("msg counters = %d/%d/%d, want 2/1/1", snap.MsgsSent, snap.MsgsRecv, snap.MsgErrors)
	}
	if snap.Connect.Count != 1 || snap.Connect.P50Ms != 5 {
		t.Errorf("connect summary = %+v, want 1 sample @5ms", snap.Connect)
	}
	if snap.RTT.Count != 1 || snap.RTT.P50Ms != 12 {
		t.Errorf("rtt summary = %+v, want 1 sample @12ms", snap.RTT)
	}
	if !strings.Contains(snap.JSON(), `"p95Ms"`) {
		t.Errorf("JSON output missing p95Ms: %s", snap.JSON())
	}
}

func TestSnapshotIsACopy(t *testing.T) {
	st := &stats{}
	st.addConnect(true, time.Millisecond)
	first := st.snapshot()
	st.addConnect(true, 9*time.Millisecond)
	if first.Connect.Count != 1 {
		t.Fatalf("snapshot leaked later writes: %+v", first)
	}
	if got := st.snapshot(); got.Connect.Count != 2 {
		t.Fatalf("second snapshot = %+v, want 2 connects", got)
	}
}

func TestHTTPBaseFromWS(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"ws://localhost:8080/ws", "http://localhost:8080"},
		{"wss://mud.example/ws", "https://mud.example"},
		{"http://localhost:8010", "http://localhost:8010"},
		{"https://mud.example/", "https://mud.example"},
	}
	for _, c := range cases {
		got, err := httpBaseFromWS(c.in)
		if err != nil {
			t.Errorf("httpBaseFromWS(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("httpBaseFromWS(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if _, err := httpBaseFromWS("ftp://mud.example/ws"); err == nil {
		t.Error("ftp scheme should be rejected")
	}
	if _, err := httpBaseFromWS("://nope"); err == nil {
		t.Error("unparsable url should be rejected")
	}
}

func TestWithTicket(t *testing.T) {
	got := withTicket("ws://localhost:8080/ws", "abc 123")
	if !strings.Contains(got, "ws://localhost:8080/ws?ticket=") {
		t.Fatalf("missing ticket query: %s", got)
	}
	if !strings.Contains(got, "abc+123") && !strings.Contains(got, "abc%20123") {
		t.Fatalf("ticket not escaped: %s", got)
	}

	// An existing query string survives and the ticket is merged in.
	got = withTicket("ws://host/ws?foo=bar", "t1")
	if !strings.Contains(got, "foo=bar") || !strings.Contains(got, "ticket=t1") {
		t.Fatalf("query merge failed: %s", got)
	}
}

func TestNewAuthConfigValidatesURL(t *testing.T) {
	if _, err := newAuthConfig("guest", "ftp://x/ws", "", "", "", false, time.Second, nil); err == nil {
		t.Error("expected error for unsupported scheme")
	}
	cfg, err := newAuthConfig("guest", "ws://localhost:8080/ws", "", "", "", false, time.Second, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.httpBase != "http://localhost:8080" {
		t.Errorf("httpBase = %q", cfg.httpBase)
	}
	if cfg.mode != authGuest {
		t.Errorf("mode = %q", cfg.mode)
	}
}

func TestAuthTargetErrors(t *testing.T) {
	base, err := newAuthConfig("ticket", "ws://localhost:8080/ws", "", "", "", false, time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := base.target(0, "ws://localhost:8080/ws"); err == nil {
		t.Error("auth=ticket without -ticket must fail")
	}

	tok, err := newAuthConfig("token", "ws://localhost:8080/ws", "", "", "", false, time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := tok.target(0, "ws://localhost:8080/ws"); err == nil {
		t.Error("auth=token without -token must fail")
	}

	// -bearer skips the ticket round-trip and dials with the JWT header.
	tokBearer, err := newAuthConfig("token", "ws://localhost:8080/ws", "", "jwt-value", "", true, time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	url, header, err := tokBearer.target(0, "ws://localhost:8080/ws")
	if err != nil {
		t.Fatal(err)
	}
	if url != "ws://localhost:8080/ws" {
		t.Errorf("bearer url = %q, want the raw ws url", url)
	}
	if got := header.Get("Authorization"); got != "Bearer jwt-value" {
		t.Errorf("Authorization = %q", got)
	}

	bad, err := newAuthConfig("password", "ws://localhost:8080/ws", "", "", "", false, time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := bad.target(0, "ws://localhost:8080/ws"); err == nil {
		t.Error("unknown auth mode must fail")
	}
}

func TestAuthTargetOriginHeader(t *testing.T) {
	cfg, err := newAuthConfig("ticket", "ws://localhost:8080/ws", "https://play.example", "", "tkt", false, time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	url, header, err := cfg.target(0, "ws://localhost:8080/ws")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(url, "ticket=tkt") {
		t.Errorf("url = %q, want ticket query", url)
	}
	if got := header.Get("Origin"); got != "https://play.example" {
		t.Errorf("Origin = %q", got)
	}
}

func TestTokenForRoundRobin(t *testing.T) {
	cfg := &authConfig{tokens: []string{"t0", "t1", "t2"}}
	for i, want := range []string{"t0", "t1", "t2", "t0"} {
		if got := cfg.tokenFor(i); got != want {
			t.Errorf("tokenFor(%d) = %q, want %q", i, got, want)
		}
	}
	// A single -token is shared when no list is loaded.
	single := &authConfig{token: "only"}
	if got := single.tokenFor(7); got != "only" {
		t.Errorf("tokenFor with single token = %q", got)
	}
	// The list wins over the single token.
	both := &authConfig{token: "only", tokens: []string{"a", "b"}}
	if got := both.tokenFor(1); got != "b" {
		t.Errorf("tokenFor list should win, got %q", got)
	}
}

func TestLoadTokensFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.txt")
	content := "# pre-minted sessions\ntok-a\n\n  tok-b  \n# trailing comment\ntok-c\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadTokensFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"tok-a", "tok-b", "tok-c"}
	if len(got) != len(want) {
		t.Fatalf("LoadTokensFile = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("tokens[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	if _, err := LoadTokensFile(filepath.Join(dir, "missing.txt")); err == nil {
		t.Error("missing file must fail")
	}
	empty := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(empty, []byte("# nothing\n\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTokensFile(empty); err == nil {
		t.Error("file without tokens must fail")
	}
}

func TestTokenModeAcceptsTokensFile(t *testing.T) {
	cfg, err := newAuthConfig("token", "ws://localhost:8080/ws", "", "", "", true, time.Second, []string{"t0", "t1", "t2"})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for i := 0; i < 3; i++ {
		url, header, err := cfg.target(i, "ws://localhost:8080/ws")
		if err != nil {
			t.Fatalf("worker %d: %v", i, err)
		}
		if url != "ws://localhost:8080/ws" {
			t.Errorf("bearer url = %q", url)
		}
		tok := strings.TrimPrefix(header.Get("Authorization"), "Bearer ")
		if tok == "" || seen[tok] {
			t.Errorf("worker %d got empty or duplicate token %q", i, tok)
		}
		seen[tok] = true
	}
}
