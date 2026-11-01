package llmgen

import (
	"sync"
	"time"
)

// StatsSnapshot is a point-in-time view of pipeline activity.
type StatsSnapshot struct {
	Requests      int64            `json:"requests"`
	Cached        int64            `json:"cached"`
	Fallbacks     int64            `json:"fallbacks"`
	Errors        int64            `json:"errors"`
	RateLimited   int64            `json:"rateLimited"`
	InputTokens   int64            `json:"inputTokens"`
	OutputTokens  int64            `json:"outputTokens"`
	ByTask        map[string]int64 `json:"byTask"`
	BySource      map[string]int64 `json:"bySource"`
	LastLatencyMS int64            `json:"lastLatencyMs"`
}

// Stats accumulates counters for the admin status endpoint.
type Stats struct {
	mu            sync.Mutex
	requests      int64
	cached        int64
	fallbacks     int64
	errors        int64
	rateLimited   int64
	inputTokens   int64
	outputTokens  int64
	byTask        map[string]int64
	bySource      map[string]int64
	lastLatencyMS int64
}

// NewStats builds an empty stats collector.
func NewStats() *Stats {
	return &Stats{byTask: map[string]int64{}, bySource: map[string]int64{}}
}

func (s *Stats) record(resp Response, latency time.Duration) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests++
	s.byTask[string(resp.Task)]++
	s.bySource[string(resp.Source)]++
	s.inputTokens += int64(resp.InputTokens)
	s.outputTokens += int64(resp.OutputTokens)
	s.lastLatencyMS = latency.Milliseconds()
	if resp.Cached {
		s.cached++
	}
	if resp.Source == SourceFallback || resp.Source == SourceDeterministic {
		s.fallbacks++
	}
}

func (s *Stats) recordError(task Task) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests++
	s.errors++
	s.byTask[string(task)]++
}

func (s *Stats) recordRateLimited(task Task) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests++
	s.rateLimited++
	s.byTask[string(task)]++
}

// Snapshot returns a copy of the counters safe to serialize.
func (s *Stats) Snapshot() StatsSnapshot {
	if s == nil {
		return StatsSnapshot{ByTask: map[string]int64{}, BySource: map[string]int64{}}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := StatsSnapshot{
		Requests:      s.requests,
		Cached:        s.cached,
		Fallbacks:     s.fallbacks,
		Errors:        s.errors,
		RateLimited:   s.rateLimited,
		InputTokens:   s.inputTokens,
		OutputTokens:  s.outputTokens,
		ByTask:        make(map[string]int64, len(s.byTask)),
		BySource:      make(map[string]int64, len(s.bySource)),
		LastLatencyMS: s.lastLatencyMS,
	}
	for k, v := range s.byTask {
		snap.ByTask[k] = v
	}
	for k, v := range s.bySource {
		snap.BySource[k] = v
	}
	return snap
}
