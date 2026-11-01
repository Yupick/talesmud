package main

// stats.go holds the latency collection and percentile math for the harness.

import (
	"encoding/json"
	"math"
	"sort"
	"sync"
	"time"
)

// stats collects latency samples and counters across all workers.
type stats struct {
	mu sync.Mutex

	attempted int
	connected int
	connErrs  int
	initErrs  int
	msgSent   int
	msgRecv   int
	msgErrs   int

	connectLat []time.Duration
	rttLat     []time.Duration
}

func (s *stats) addInitErr() {
	s.mu.Lock()
	s.attempted++
	s.initErrs++
	s.mu.Unlock()
}

func (s *stats) addConnect(ok bool, lat time.Duration) {
	s.mu.Lock()
	s.attempted++
	if ok {
		s.connected++
		s.connectLat = append(s.connectLat, lat)
	} else {
		s.connErrs++
	}
	s.mu.Unlock()
}

func (s *stats) addSent() {
	s.mu.Lock()
	s.msgSent++
	s.mu.Unlock()
}

func (s *stats) addRecv(lat time.Duration) {
	s.mu.Lock()
	s.msgRecv++
	s.rttLat = append(s.rttLat, lat)
	s.mu.Unlock()
}

func (s *stats) addMsgErr() {
	s.mu.Lock()
	s.msgErrs++
	s.mu.Unlock()
}

func (s *stats) snapshot() snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return snapshot{
		Attempted:  s.attempted,
		Connected:  s.connected,
		ConnErrors: s.connErrs,
		InitErrors: s.initErrs,
		MsgsSent:   s.msgSent,
		MsgsRecv:   s.msgRecv,
		MsgErrors:  s.msgErrs,
		Connect:    summarize(s.connectLat),
		RTT:        summarize(s.rttLat),
	}
}

type latencySummary struct {
	Count int     `json:"count"`
	MinMs float64 `json:"minMs"`
	P50Ms float64 `json:"p50Ms"`
	P95Ms float64 `json:"p95Ms"`
	P99Ms float64 `json:"p99Ms"`
	MaxMs float64 `json:"maxMs"`
	AvgMs float64 `json:"avgMs"`
}

type snapshot struct {
	Attempted  int            `json:"attempted"`
	Connected  int            `json:"connected"`
	ConnErrors int            `json:"connErrors"`
	InitErrors int            `json:"initErrors"`
	MsgsSent   int            `json:"msgsSent"`
	MsgsRecv   int            `json:"msgsRecv"`
	MsgErrors  int            `json:"msgErrors"`
	Connect    latencySummary `json:"connectLatency"`
	RTT        latencySummary `json:"rttLatency"`
}

func summarize(samples []time.Duration) latencySummary {
	sum := latencySummary{Count: len(samples)}
	if len(samples) == 0 {
		return sum
	}
	sorted := make([]time.Duration, len(samples))
	copy(sorted, samples)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	var total time.Duration
	for _, d := range sorted {
		total += d
	}
	sum.MinMs = ms(sorted[0])
	sum.MaxMs = ms(sorted[len(sorted)-1])
	sum.AvgMs = ms(total / time.Duration(len(sorted)))
	sum.P50Ms = ms(percentile(sorted, 0.50))
	sum.P95Ms = ms(percentile(sorted, 0.95))
	sum.P99Ms = ms(percentile(sorted, 0.99))
	return sum
}

// percentile uses the nearest-rank method on an already sorted slice.
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(math.Ceil(p*float64(len(sorted)))) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= len(sorted) {
		rank = len(sorted) - 1
	}
	return sorted[rank]
}

func ms(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000.0
}

func (s snapshot) JSON() string {
	b, _ := json.MarshalIndent(s, "", "  ")
	return string(b)
}
