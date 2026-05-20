package runner

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// NodeStats tracks per-node runtime statistics during test execution.
type NodeStats struct {
	mu sync.Mutex

	TotalReqs   atomic.Int64
	SuccessReqs atomic.Int64
	FailedReqs  atomic.Int64
	MinLatency  atomic.Int64
	TTFB         atomic.Int64

	errorCodes  map[string]*int64
	errorMu     sync.RWMutex
	latencies   []time.Duration
	maxSamples  int
}

// NodeSnapshot represents a point-in-time snapshot of node statistics.
type NodeSnapshot struct {
	NodeID      string            `json:"node_id"`
	NodeType    string            `json:"node_type"`
	NodeName    string            `json:"node_name"`
	TotalReqs   int64             `json:"total_requests"`
	SuccessReqs int64             `json:"success_requests"`
	FailedReqs  int64             `json:"failed_requests"`
	SuccessRate float64           `json:"success_rate"`
	AvgLatency  time.Duration     `json:"avg_latency"`
	P50Latency  time.Duration     `json:"p50_latency"`
	P90Latency  time.Duration     `json:"p90_latency"`
	P95Latency  time.Duration     `json:"p95_latency"`
	P99Latency  time.Duration     `json:"p99_latency"`
	MinLatency  time.Duration     `json:"min_latency"`
	TTFB        time.Duration     `json:"ttfb_ms"`
	ErrorCodes  map[string]int64  `json:"error_codes"`
}

// NewNodeStats creates a new NodeStats instance with the given maximum sample limit.
func NewNodeStats(maxSamples int) *NodeStats {
	if maxSamples < 0 {
		maxSamples = 0
	}
	return &NodeStats{
		maxSamples: maxSamples,
		latencies:  make([]time.Duration, 0, maxSamples),
	}
}

// RecordLatency records a latency measurement for this node.
func (s *NodeStats) RecordLatency(d time.Duration, success bool) {
	s.TotalReqs.Add(1)
	if success {
		s.SuccessReqs.Add(1)
	} else {
		s.FailedReqs.Add(1)
	}

	ns := d.Nanoseconds()

	if success && ns > 0 {
		for {
			old := s.MinLatency.Load()
			if old == 0 || ns < old {
				if s.MinLatency.CompareAndSwap(old, ns) {
					break
				}
				continue
			}
			break
		}

		if s.TTFB.Load() == 0 {
			s.TTFB.CompareAndSwap(0, ns)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.latencies = append(s.latencies, d)
	if s.maxSamples > 0 && len(s.latencies) > s.maxSamples {
		s.latencies = s.latencies[len(s.latencies)-s.maxSamples:]
	}
}

// RecordError records an error with an associated status code or reason string.
func (s *NodeStats) RecordError(code string) {
	s.errorMu.Lock()
	if s.errorCodes == nil {
		s.errorCodes = make(map[string]*int64)
	}
	if _, ok := s.errorCodes[code]; !ok {
		v := int64(0)
		s.errorCodes[code] = &v
	}
	ptr := s.errorCodes[code]
	s.errorMu.Unlock()
	atomic.AddInt64(ptr, 1)
}

// LatencyPercentiles calculates and returns avg, p50, p90, p95, p99 latencies.
func (s *NodeStats) LatencyPercentiles() (avg, p50, p90, p95, p99 time.Duration) {
	s.mu.Lock()
	list := make([]time.Duration, len(s.latencies))
	copy(list, s.latencies)
	s.mu.Unlock()

	if len(list) == 0 {
		return 0, 0, 0, 0, 0
	}

	var total time.Duration
	for _, l := range list {
		total += l
	}
	avg = total / time.Duration(len(list))

	sort.Slice(list, func(i, j int) bool { return list[i] < list[j] })
	p50 = percentile(list, 50)
	p90 = percentile(list, 90)
	p95 = percentile(list, 95)
	p99 = percentile(list, 99)
	return
}

// Snapshot returns a current snapshot of all statistics.
func (s *NodeStats) Snapshot() *NodeSnapshot {
	avg, p50, p90, p95, p99 := s.LatencyPercentiles()

	total := s.TotalReqs.Load()
	succ := s.SuccessReqs.Load()
	fail := s.FailedReqs.Load()

	rate := float64(0)
	if total > 0 {
		rate = float64(succ) / float64(total) * 100
	}

	return &NodeSnapshot{
		TotalReqs:   total,
		SuccessReqs: succ,
		FailedReqs:  fail,
		SuccessRate: rate,
		AvgLatency:  avg,
		P50Latency:  p50,
		P90Latency:  p90,
		P95Latency:  p95,
		P99Latency:  p99,
		MinLatency:  time.Duration(s.MinLatency.Load()),
		TTFB:        time.Duration(s.TTFB.Load()),
		ErrorCodes:  s.getErrorCodes(),
	}
}

func (s *NodeStats) getErrorCodes() map[string]int64 {
	s.errorMu.RLock()
	defer s.errorMu.RUnlock()
	result := make(map[string]int64, len(s.errorCodes))
	for code, ptr := range s.errorCodes {
		result[code] = atomic.LoadInt64(ptr)
	}
	return result
}
