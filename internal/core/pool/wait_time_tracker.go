package pool

import (
	"sort"
	"sync"
	"time"
)

// WaitTimeStats holds aggregated statistics for task queue wait times.
type WaitTimeStats struct {
	Avg         time.Duration `json:"avg"`
	P50         time.Duration `json:"p50"`
	P95         time.Duration `json:"p95"`
	P99         time.Duration `json:"p99"`
	Max         time.Duration `json:"max"`
	SampleCount int64         `json:"sample_count"`
}

// WaitTimeTracker records task wait durations in a fixed-size circular
// buffer and computes percentile statistics. It is safe for concurrent
// use from multiple goroutines.
type WaitTimeTracker struct {
	mu      sync.Mutex
	samples []time.Duration
	pos     int
	count   int64
	totalNs int64
}

// NewWaitTimeTracker creates a tracker with the given buffer capacity.
func NewWaitTimeTracker(capacity int) *WaitTimeTracker {
	if capacity <= 0 {
		capacity = 1000
	}
	return &WaitTimeTracker{
		samples: make([]time.Duration, capacity),
	}
}

// Record adds a wait duration sample to the circular buffer.
func (t *WaitTimeTracker) Record(d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.samples[t.pos] = d
	t.pos = (t.pos + 1) % len(t.samples)
	t.count++
	t.totalNs += int64(d)
}

// Stats returns aggregated percentile statistics from the current
// buffer contents. For buffers that have not wrapped, only the
// filled portion is considered.
func (t *WaitTimeTracker) Stats() WaitTimeStats {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.count == 0 {
		return WaitTimeStats{}
	}

	n := int(t.count)
	if n > len(t.samples) {
		n = len(t.samples)
	}

	sorted := make([]time.Duration, n)
	copy(sorted, t.samples[:n])
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	return WaitTimeStats{
		Avg:         time.Duration(t.totalNs / t.count),
		P50:         percentile(sorted, 50),
		P95:         percentile(sorted, 95),
		P99:         percentile(sorted, 99),
		Max:         sorted[n-1],
		SampleCount: t.count,
	}
}

// Reset clears all recorded samples.
func (t *WaitTimeTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	for i := range t.samples {
		t.samples[i] = 0
	}
	t.pos = 0
	t.count = 0
	t.totalNs = 0
}

// percentile computes the p-th percentile (0-100) from a sorted slice
// of durations using linear interpolation. Returns 0 for empty slices.
func percentile(sorted []time.Duration, p float64) time.Duration {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}

	// Nearest-rank method with linear interpolation
	idx := float64(n-1) * p / 100.0
	lower := int(idx)
	upper := lower + 1
	if upper >= n {
		return sorted[n-1]
	}

	frac := idx - float64(lower)
	return sorted[lower] + time.Duration(float64(sorted[upper]-sorted[lower])*frac)
}
