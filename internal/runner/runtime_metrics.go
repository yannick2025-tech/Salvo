// Package runner implements the test execution engine for Salvo scenarios.
//
// This file provides the RuntimeMetricsCollector that samples Go runtime
// and system-level performance metrics during test execution.
package runner

import (
	"runtime"
	"sync"
	"time"
)

// RuntimeMetricsSnapshot holds a point-in-time sample of runtime and system
// performance metrics. It is produced by RuntimeMetricsCollector at regular
// intervals during a test run.
type RuntimeMetricsSnapshot struct {
	Timestamp time.Time `json:"timestamp"`

	// Go Runtime
	GoroutineCount int64   `json:"goroutine_count"`
	HeapAllocMB    float64 `json:"heap_alloc_mb"`
	HeapSysMB      float64 `json:"heap_sys_mb"`
	HeapIdleMB     float64 `json:"heap_idle_mb"`
	GCPauseTotalNs uint64  `json:"gc_pause_total_ns"`
	GCPauseLastNs  uint64  `json:"gc_pause_last_ns"`
	GCCount        uint32  `json:"gc_count"`
	NextGC         uint64  `json:"next_gc"`

	// System (Linux/Mac)
	CPUUsagePercent float64 `json:"cpu_percent"`
	RSSMemoryMB     float64 `json:"rss_mb"`
	ThreadCount     int     `json:"thread_count"`

	// Runner Internal
	ActiveWorkers   int `json:"active_workers"`
	PendingQueueLen int `json:"pending_queue_len"`

	// Task Wait Time (P50/P95/P99)
	TaskWaitAvgMs       float64 `json:"task_wait_avg_ms"`
	TaskWaitP50Ms       float64 `json:"task_wait_p50_ms"`
	TaskWaitP95Ms       float64 `json:"task_wait_p95_ms"`
	TaskWaitP99Ms       float64 `json:"task_wait_p99_ms"`
	TaskWaitMaxMs       float64 `json:"task_wait_max_ms"`
	TaskWaitSampleCount int64   `json:"task_wait_samples"`
}

// RuntimeMetricsCollector periodically samples Go runtime and system
// performance metrics during a test run. It is safe for concurrent use.
type RuntimeMetricsCollector struct {
	interval time.Duration
	enabled  bool

	mu        sync.Mutex
	snapshots []RuntimeMetricsSnapshot
	done      chan struct{}
	stopOnce  sync.Once

	waitStatsProvider WaitTimeStatsProvider
	runnerStateProvider RunnerStateProvider
}

// RunnerStateProvider is an interface for components that can provide
// runner internal state metrics such as active worker count and
// pending queue length.
type RunnerStateProvider interface {
	ActiveWorkers() int
	PendingQueueLen() int
}

// SetRunnerStateProvider sets the provider for runner internal state.
func (c *RuntimeMetricsCollector) SetRunnerStateProvider(p RunnerStateProvider) {
	c.runnerStateProvider = p
}

// WaitTimeStatsProvider is an interface for components that can provide
// task wait time statistics. The Pool type satisfies this interface.
type WaitTimeStatsProvider interface {
	TaskWaitStats() poolWaitTimeStats
}

// poolWaitTimeStats mirrors pool.WaitTimeStats to avoid importing the
// pool package (which would create a circular dependency). The Pool
// type must wrap its return value to satisfy this interface.
type poolWaitTimeStats struct {
	Avg         time.Duration
	P50         time.Duration
	P95         time.Duration
	P99         time.Duration
	Max         time.Duration
	SampleCount int64
}

// SetWaitTimeStatsProvider sets the provider for task wait time
// statistics. Typically this is called by the Runner with a wrapper
// around its Pool instance.
func (c *RuntimeMetricsCollector) SetWaitTimeStatsProvider(p WaitTimeStatsProvider) {
	c.waitStatsProvider = p
}

// PoolWaitTimeStats is a convenience type that wraps a function to
// satisfy the WaitTimeStatsProvider interface. This avoids a direct
// import of the pool package.
type PoolWaitTimeStats struct {
	Avg         time.Duration
	P50         time.Duration
	P95         time.Duration
	P99         time.Duration
	Max         time.Duration
	SampleCount int64
}

// WaitTimeStatsFunc is a function type that returns task wait time
// statistics. It implements WaitTimeStatsProvider.
type WaitTimeStatsFunc func() PoolWaitTimeStats

// TaskWaitStats implements WaitTimeStatsProvider.
func (f WaitTimeStatsFunc) TaskWaitStats() poolWaitTimeStats {
	s := f()
	return poolWaitTimeStats{
		Avg:         s.Avg,
		P50:         s.P50,
		P95:         s.P95,
		P99:         s.P99,
		Max:         s.Max,
		SampleCount: s.SampleCount,
	}
}

// NewRuntimeMetricsCollector creates a collector with the given sampling
// interval and enabled flag. When enabled is false, Start is a no-op.
func NewRuntimeMetricsCollector(interval time.Duration, enabled bool) *RuntimeMetricsCollector {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	return &RuntimeMetricsCollector{
		interval: interval,
		enabled:  enabled,
	}
}

// Start begins the sampling loop in a background goroutine. If the
// collector is disabled, Start returns immediately without spawning a
// goroutine.
func (c *RuntimeMetricsCollector) Start() {
	if !c.enabled {
		return
	}
	c.done = make(chan struct{})
	go c.run()
}

// Stop signals the sampling goroutine to stop, takes one final sample,
// and waits for the goroutine to exit. It is safe to call Stop multiple
// times.
func (c *RuntimeMetricsCollector) Stop() {
	if !c.enabled || c.done == nil {
		return
	}
	c.stopOnce.Do(func() {
		close(c.done)
	})
	// Take a final sample after the loop exits
	c.mu.Lock()
	c.snapshots = append(c.snapshots, c.sample())
	c.mu.Unlock()
}

// Snapshots returns a copy of all collected snapshots.
func (c *RuntimeMetricsCollector) Snapshots() []RuntimeMetricsSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()

	out := make([]RuntimeMetricsSnapshot, len(c.snapshots))
	copy(out, c.snapshots)
	return out
}

// run is the main sampling loop.
func (c *RuntimeMetricsCollector) run() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			s := c.sample()
			c.mu.Lock()
			c.snapshots = append(c.snapshots, s)
			c.mu.Unlock()
		}
	}
}

// sample collects a single RuntimeMetricsSnapshot from the Go runtime.
func (c *RuntimeMetricsCollector) sample() RuntimeMetricsSnapshot {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	s := RuntimeMetricsSnapshot{
		Timestamp:      time.Now().UTC(),
		GoroutineCount: int64(runtime.NumGoroutine()),
		HeapAllocMB:    bytesToMB(mem.HeapAlloc),
		HeapSysMB:      bytesToMB(mem.HeapSys),
		HeapIdleMB:     bytesToMB(mem.HeapIdle),
		GCPauseTotalNs: mem.PauseTotalNs,
		GCCount:        mem.NumGC,
		NextGC:         mem.NextGC,
	}

	// Last GC pause: the most recent entry in the circular PauseNs buffer.
	if mem.NumGC > 0 {
		idx := (mem.NumGC + 255) % 256 // last written slot
		s.GCPauseLastNs = mem.PauseNs[idx]
	}

	// System metrics: platform-specific implementations fill these in.
	c.sampleSystem(&s)

	// Task wait time statistics from the pool (if available).
	if c.waitStatsProvider != nil {
		ws := c.waitStatsProvider.TaskWaitStats()
		s.TaskWaitAvgMs = ws.Avg.Seconds() * 1000
		s.TaskWaitP50Ms = ws.P50.Seconds() * 1000
		s.TaskWaitP95Ms = ws.P95.Seconds() * 1000
		s.TaskWaitP99Ms = ws.P99.Seconds() * 1000
		s.TaskWaitMaxMs = ws.Max.Seconds() * 1000
		s.TaskWaitSampleCount = ws.SampleCount
	}

	// Runner internal state (if available).
	if c.runnerStateProvider != nil {
		s.ActiveWorkers = c.runnerStateProvider.ActiveWorkers()
		s.PendingQueueLen = c.runnerStateProvider.PendingQueueLen()
	}

	return s
}

// bytesToMB converts a byte count to megabytes.
func bytesToMB(bytes uint64) float64 {
	return float64(bytes) / 1024.0 / 1024.0
}

// SystemMetricsSummary holds aggregated statistics computed from a
// slice of RuntimeMetricsSnapshot after a test run completes.
type SystemMetricsSummary struct {
	SampleCount int `json:"sample_count"`

	// Goroutine
	GoroutineMin   int64   `json:"goroutine_min"`
	GoroutineMax   int64   `json:"goroutine_max"`
	GoroutineAvg   float64 `json:"goroutine_avg"`
	GoroutinePeakAt string `json:"goroutine_peak_at,omitempty"`

	// Heap
	HeapAllocMinMB float64 `json:"heap_alloc_min_mb"`
	HeapAllocMaxMB float64 `json:"heap_alloc_max_mb"`
	HeapAllocAvgMB float64 `json:"heap_alloc_avg_mb"`

	// CPU
	CPUMin   float64 `json:"cpu_min"`
	CPUMax   float64 `json:"cpu_max"`
	CPUAvg   float64 `json:"cpu_avg"`
	CPUPeakAt string `json:"cpu_peak_at,omitempty"`

	// GC
	GCPauseTotalMs float64 `json:"gc_pause_total_ms"`
	GCCount        uint32  `json:"gc_count"`

	// Task Wait
	TaskWaitAvgMs   float64 `json:"task_wait_avg_ms"`
	TaskWaitP99MaxMs float64 `json:"task_wait_p99_max_ms"`

	// Time range
	FirstSampleAt time.Time `json:"first_sample_at"`
	LastSampleAt  time.Time `json:"last_sample_at"`
}

// ComputeSummary aggregates all collected snapshots into a summary.
// Returns an empty summary if no snapshots are available.
func (c *RuntimeMetricsCollector) ComputeSummary() SystemMetricsSummary {
	c.mu.Lock()
	snapshots := make([]RuntimeMetricsSnapshot, len(c.snapshots))
	copy(snapshots, c.snapshots)
	c.mu.Unlock()

	if len(snapshots) == 0 {
		return SystemMetricsSummary{}
	}

	summary := SystemMetricsSummary{
		SampleCount:  len(snapshots),
		FirstSampleAt: snapshots[0].Timestamp,
		LastSampleAt:  snapshots[len(snapshots)-1].Timestamp,
	}

	// Initialize min/max with first sample
	s0 := snapshots[0]
	summary.GoroutineMin = s0.GoroutineCount
	summary.GoroutineMax = s0.GoroutineCount
	summary.HeapAllocMinMB = s0.HeapAllocMB
	summary.HeapAllocMaxMB = s0.HeapAllocMB
	summary.CPUMin = s0.CPUUsagePercent
	summary.CPUMax = s0.CPUUsagePercent

	var goroutineTotal int64
	var heapAllocTotal float64
	var cpuTotal float64
	var taskWaitAvgTotal float64
	var taskWaitP99Max float64
	var taskWaitSamples int64

	for _, s := range snapshots {
		// Goroutine
		if s.GoroutineCount < summary.GoroutineMin {
			summary.GoroutineMin = s.GoroutineCount
		}
		if s.GoroutineCount > summary.GoroutineMax {
			summary.GoroutineMax = s.GoroutineCount
			summary.GoroutinePeakAt = s.Timestamp.Format(time.RFC3339)
		}
		goroutineTotal += s.GoroutineCount

		// Heap
		if s.HeapAllocMB < summary.HeapAllocMinMB {
			summary.HeapAllocMinMB = s.HeapAllocMB
		}
		if s.HeapAllocMB > summary.HeapAllocMaxMB {
			summary.HeapAllocMaxMB = s.HeapAllocMB
		}
		heapAllocTotal += s.HeapAllocMB

		// CPU
		if s.CPUUsagePercent < summary.CPUMin {
			summary.CPUMin = s.CPUUsagePercent
		}
		if s.CPUUsagePercent > summary.CPUMax {
			summary.CPUMax = s.CPUUsagePercent
			summary.CPUPeakAt = s.Timestamp.Format(time.RFC3339)
		}
		cpuTotal += s.CPUUsagePercent

		// GC
		if s.GCPauseTotalNs > 0 {
			summary.GCPauseTotalMs = float64(s.GCPauseTotalNs) / 1e6
		}
		if s.GCCount > summary.GCCount {
			summary.GCCount = s.GCCount
		}

		// Task Wait
		if s.TaskWaitAvgMs > 0 {
			taskWaitAvgTotal += s.TaskWaitAvgMs
			taskWaitSamples++
		}
		if s.TaskWaitP99Ms > taskWaitP99Max {
			taskWaitP99Max = s.TaskWaitP99Ms
		}
	}

	n := float64(len(snapshots))
	summary.GoroutineAvg = float64(goroutineTotal) / n
	summary.HeapAllocAvgMB = heapAllocTotal / n
	summary.CPUAvg = cpuTotal / n

	if taskWaitSamples > 0 {
		summary.TaskWaitAvgMs = taskWaitAvgTotal / float64(taskWaitSamples)
	}
	summary.TaskWaitP99MaxMs = taskWaitP99Max

	return summary
}
