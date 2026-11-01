package llmgen

import (
	"sync"
	"time"
)

// Limiter is the minimal per-key sliding-window rate limiter the pipeline
// consults before calling the provider.
type Limiter interface {
	Allow(key string) bool
}

// WindowLimiter allows at most limit events per window for each key.
type WindowLimiter struct {
	limit  int
	window time.Duration

	mu      sync.Mutex
	buckets map[string][]time.Time
}

// NewWindowLimiter builds a limiter with the given limit and window.
func NewWindowLimiter(limit int, window time.Duration) *WindowLimiter {
	if window <= 0 {
		window = time.Minute
	}
	return &WindowLimiter{limit: limit, window: window, buckets: map[string][]time.Time{}}
}

// Allow records an event for key when the window has room.
func (l *WindowLimiter) Allow(key string) bool {
	if l == nil || l.limit <= 0 {
		return true
	}
	now := time.Now()
	cutoff := now.Add(-l.window)
	l.mu.Lock()
	defer l.mu.Unlock()
	events := l.buckets[key]
	kept := events[:0]
	for _, ts := range events {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= l.limit {
		l.buckets[key] = kept
		return false
	}
	l.buckets[key] = append(kept, now)
	return true
}
