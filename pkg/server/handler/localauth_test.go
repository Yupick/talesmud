package handler

import (
	"testing"
	"time"
)

func TestIPLimiterRejectsOverLimit(t *testing.T) {
	l := newIPLimiter(2, time.Minute)
	if !l.allow("1.2.3.4") || !l.allow("1.2.3.4") {
		t.Fatal("first two attempts should be allowed")
	}
	if l.allow("1.2.3.4") {
		t.Fatal("third attempt within the window should be rejected")
	}
}

func TestIPLimiterPurgesExpiredEntries(t *testing.T) {
	l := newIPLimiter(10, 20*time.Millisecond)
	for i := 0; i < 50; i++ {
		l.allow(string(rune('a'+i%26)) + ".example")
	}
	// Wait for the window to pass so a purge removes every entry.
	time.Sleep(40 * time.Millisecond)
	if !l.allow("fresh.example") {
		t.Fatal("fresh IP should be allowed")
	}
	l.mu.Lock()
	count := len(l.hits)
	l.mu.Unlock()
	if count != 1 {
		t.Fatalf("expected only the fresh entry after purge, got %d", count)
	}
}

func TestIPLimiterCapsEntries(t *testing.T) {
	l := newIPLimiter(10, time.Hour)
	l.maxEntries = 2
	if !l.allow("a") || !l.allow("b") {
		t.Fatal("first two IPs should be allowed")
	}
	if l.allow("c") {
		t.Fatal("new IP beyond the cap should be rejected")
	}
	l.mu.Lock()
	count := len(l.hits)
	l.mu.Unlock()
	if count != 2 {
		t.Fatalf("expected map to stay at cap 2, got %d", count)
	}
}
