package main

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestMintInterval(t *testing.T) {
	tests := []struct {
		name    string
		ipCount int
		perMin  int
		wantMin time.Duration
		wantMax time.Duration
	}{
		// 1 ip * 12/min -> 1s / (12/60) = 5s.
		{"single ip full rate", 1, 12, 5 * time.Second, 5 * time.Second},
		// 1 ip * 6/min -> 1s / (6/60) = 10s.
		{"single ip low rate", 1, 6, 10 * time.Second, 10 * time.Second},
		// 2 ips * 12/min -> 1s / (24/60) = 2.5s.
		{"two ips full rate", 2, 12, 2500 * time.Millisecond, 2500 * time.Millisecond},
		// 200 ips * 12/min -> 1s / 40 = 25ms (above the 20ms floor).
		{"200 ips full rate", 200, 12, 20 * time.Millisecond, 30 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mintInterval(tt.ipCount, tt.perMin)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("mintInterval(%d, %d) = %v, want between %v and %v",
					tt.ipCount, tt.perMin, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestMintIntervalClamps(t *testing.T) {
	// perMin < 1 falls back to the server default of 12/min (5s interval),
	// never a zero/negative (busy-loop) interval.
	if got := mintInterval(1, 0); got != 5*time.Second {
		t.Errorf("mintInterval(1, 0) = %v, want 5s (perMin clamped to 12)", got)
	}
	// ipCount < 1 falls back to a single IP.
	if got := mintInterval(0, 12); got != 5*time.Second {
		t.Errorf("mintInterval(0, 12) = %v, want 5s (ipCount clamped to 1)", got)
	}
	// Huge budget must still respect the 20ms floor.
	if got := mintInterval(100000, 12); got < mintMinInterval {
		t.Errorf("mintInterval(100000, 12) = %v, want >= %v", got, mintMinInterval)
	}
}

func TestXffIP(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		ip := xffIP(i)
		if !strings.HasPrefix(ip, "10.") {
			t.Errorf("xffIP(%d) = %q, want prefix 10.", i, ip)
		}
		if parsed := net.ParseIP(ip); parsed == nil {
			t.Errorf("xffIP(%d) = %q is not a valid IP", i, ip)
		}
		seen[ip] = true
	}
	if len(seen) != 50 {
		t.Errorf("xffIP produced %d unique IPs for 50 indices, want 50", len(seen))
	}
	// xffIP(0) must not end in .0.
	if strings.HasSuffix(xffIP(0), ".0") {
		t.Errorf("xffIP(0) = %q, must not end in .0", xffIP(0))
	}
}

func TestMintRegisterResponse(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		body          string
		wantToken     string
		wantRateLimit bool
		wantErr       bool
	}{
		{
			name:      "created with token",
			status:    http.StatusCreated,
			body:      `{"token":"jwt-abc"}`,
			wantToken: "jwt-abc",
		},
		{
			name:      "ok with token",
			status:    http.StatusOK,
			body:      `{"token":"jwt-xyz"}`,
			wantToken: "jwt-xyz",
		},
		{
			name:          "rate limited",
			status:        http.StatusTooManyRequests,
			body:          `{"error":"too many"}`,
			wantRateLimit: true,
		},
		{
			name:    "server error",
			status:  http.StatusInternalServerError,
			body:    `boom`,
			wantErr: true,
		},
		{
			name:    "bad json",
			status:  http.StatusCreated,
			body:    `not-json`,
			wantErr: true,
		},
		{
			name:    "empty token",
			status:  http.StatusCreated,
			body:    `{"token":""}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, rateLimited, err := mintRegisterResponse(tt.status, []byte(tt.body))
			if rateLimited != tt.wantRateLimit {
				t.Errorf("rateLimited = %v, want %v", rateLimited, tt.wantRateLimit)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if token != tt.wantToken {
				t.Errorf("token = %q, want %q", token, tt.wantToken)
			}
		})
	}
}

func TestMintBody(t *testing.T) {
	body := mintBody(42, 1729000000000000000)
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("mintBody is not JSON-marshalable: %v", err)
	}
	var payload map[string]string
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("mintBody produced invalid JSON: %v", err)
	}

	uname := payload["username"]
	if !strings.HasPrefix(uname, "lt_") {
		t.Errorf("username = %q, want lt_ prefix", uname)
	}
	if len(uname) < 3 || len(uname) > 20 {
		t.Errorf("username %q length %d outside server validation 3-20", uname, len(uname))
	}
	for _, r := range uname {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_'
		if !ok {
			t.Errorf("username %q contains invalid rune %q (server allows [a-z0-9_])", uname, r)
		}
	}
	if payload["password"] == "" {
		t.Error("password must not be empty")
	}
	if payload["email"] != uname+"@example.test" {
		t.Errorf("email = %q, want %q", payload["email"], uname+"@example.test")
	}
}

func TestMintBodyUnique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		body := mintBody(i, 1729000000000000000)
		if seen[body["username"]] {
			t.Fatalf("duplicate username %q at index %d", body["username"], i)
		}
		seen[body["username"]] = true
	}
}

func TestMintBodyTruncatesLongSeed(t *testing.T) {
	// A huge nano must still yield a <=20-char username.
	body := mintBody(0, 0x7fffffffffffffff)
	if got := len(body["username"]); got > 20 {
		t.Errorf("username length %d exceeds 20: %q", got, body["username"])
	}
}


