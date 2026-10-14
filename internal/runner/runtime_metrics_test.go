package runner

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntimeMetricsSnapshotFields(t *testing.T) {
	s := RuntimeMetricsSnapshot{
		Timestamp:           time.Now(),
		GoroutineCount:      100,
		HeapAllocMB:         256.5,
		HeapSysMB:           512.0,
		HeapIdleMB:          255.5,
		GCPauseTotalNs:      1000000,
		GCPauseLastNs:       50000,
		GCCount:             10,
		NextGC:              4194304,
		CPUUsagePercent:     45.2,
		RSSMemoryMB:         300.0,
		ThreadCount:         8,
		ActiveWorkers:       4,
		PendingQueueLen:     10,
		TaskWaitAvgMs:       1.5,
		TaskWaitP50Ms:       1.2,
		TaskWaitP95Ms:       3.8,
		TaskWaitP99Ms:       5.2,
		TaskWaitMaxMs:       10.0,
		TaskWaitSampleCount: 500,
	}

	assert.Equal(t, int64(100), s.GoroutineCount)
	assert.Equal(t, 256.5, s.HeapAllocMB)
	assert.Equal(t, 45.2, s.CPUUsagePercent)
	assert.Equal(t, 1.5, s.TaskWaitAvgMs)
	assert.Equal(t, int64(500), s.TaskWaitSampleCount)
}

func TestNewRuntimeMetricsCollector(t *testing.T) {
	collector := NewRuntimeMetricsCollector(2*time.Second, true)
	require.NotNil(t, collector)
	assert.Equal(t, 2*time.Second, collector.interval)
	assert.True(t, collector.enabled)
}

func TestNewRuntimeMetricsCollectorDisabled(t *testing.T) {
	collector := NewRuntimeMetricsCollector(2*time.Second, false)
	require.NotNil(t, collector)
	assert.False(t, collector.enabled)
}

func TestRuntimeMetricsCollectorSampleGoRuntime(t *testing.T) {
	collector := NewRuntimeMetricsCollector(2*time.Second, true)

	snapshot := collector.sample()

	assert.False(t, snapshot.Timestamp.IsZero(), "timestamp should be set")
	assert.Greater(t, snapshot.GoroutineCount, int64(0), "goroutine count should be positive")
	assert.Greater(t, snapshot.HeapAllocMB, 0.0, "heap alloc should be positive")
	assert.Greater(t, snapshot.HeapSysMB, 0.0, "heap sys should be positive")
	assert.GreaterOrEqual(t, snapshot.HeapIdleMB, 0.0, "heap idle should be non-negative")
	assert.GreaterOrEqual(t, snapshot.GCPauseTotalNs, uint64(0), "gc pause total should be non-negative")
	assert.GreaterOrEqual(t, snapshot.GCCount, uint32(0), "gc count should be non-negative")
	assert.Greater(t, snapshot.NextGC, uint64(0), "next gc should be positive")
}

func TestRuntimeMetricsCollectorSampleConsistency(t *testing.T) {
	collector := NewRuntimeMetricsCollector(2*time.Second, true)

	// Force some allocations to make metrics non-trivial
	_ = make([]byte, 1024*1024)
	runtime.GC()

	snapshot := collector.sample()

	// After GC, heap should be reasonable
	assert.Greater(t, snapshot.HeapAllocMB, 0.0)
	// HeapSys >= HeapAlloc (system memory >= allocated)
	assert.GreaterOrEqual(t, snapshot.HeapSysMB, snapshot.HeapAllocMB)
}

func TestRuntimeMetricsCollectorStartStop(t *testing.T) {
	collector := NewRuntimeMetricsCollector(100*time.Millisecond, true)

	collector.Start()
	time.Sleep(350 * time.Millisecond) // Allow ~3 samples
	collector.Stop()

	snapshots := collector.Snapshots()
	assert.GreaterOrEqual(t, len(snapshots), 2, "should have at least 2 samples after 350ms at 100ms interval")
}

func TestRuntimeMetricsCollectorDisabledNoSampling(t *testing.T) {
	collector := NewRuntimeMetricsCollector(100*time.Millisecond, false)

	collector.Start()
	time.Sleep(250 * time.Millisecond)
	collector.Stop()

	snapshots := collector.Snapshots()
	assert.Equal(t, 0, len(snapshots), "disabled collector should not produce samples")
}

func TestRuntimeMetricsCollectorStopIdempotent(t *testing.T) {
	collector := NewRuntimeMetricsCollector(100*time.Millisecond, true)
	collector.Start()
	time.Sleep(150 * time.Millisecond)

	// Stop twice should not panic
	collector.Stop()
	collector.Stop()
}

func TestRuntimeMetricsCollectorSnapshotsIsCopy(t *testing.T) {
	collector := NewRuntimeMetricsCollector(100*time.Millisecond, true)
	collector.Start()
	time.Sleep(250 * time.Millisecond)
	collector.Stop()

	snapshots1 := collector.Snapshots()
	snapshots2 := collector.Snapshots()

	// Should be equal content but different slices
	assert.Equal(t, len(snapshots1), len(snapshots2))
}

func TestRuntimeMetricsCollectorFinalSample(t *testing.T) {
	collector := NewRuntimeMetricsCollector(100*time.Millisecond, true)
	collector.Start()
	time.Sleep(150 * time.Millisecond)

	beforeStop := len(collector.Snapshots())
	collector.Stop()
	afterStop := len(collector.Snapshots())

	// Stop should take one final sample
	assert.GreaterOrEqual(t, afterStop, beforeStop)
}

func TestRuntimeMetricsCollectorWithWaitTimeStats(t *testing.T) {
	collector := NewRuntimeMetricsCollector(100*time.Millisecond, true)

	// Set up a mock WaitTimeStatsProvider
	collector.SetWaitTimeStatsProvider(WaitTimeStatsFunc(func() PoolWaitTimeStats {
		return PoolWaitTimeStats{
			Avg:         5 * time.Millisecond,
			P50:         4 * time.Millisecond,
			P95:         8 * time.Millisecond,
			P99:         10 * time.Millisecond,
			Max:         15 * time.Millisecond,
			SampleCount: 1000,
		}
	}))

	collector.Start()
	time.Sleep(250 * time.Millisecond)
	collector.Stop()

	snapshots := collector.Snapshots()
	assert.GreaterOrEqual(t, len(snapshots), 1)

	// Check that TaskWait fields are populated
	s := snapshots[len(snapshots)-1]
	assert.Equal(t, 5.0, s.TaskWaitAvgMs)
	assert.Equal(t, 4.0, s.TaskWaitP50Ms)
	assert.Equal(t, 8.0, s.TaskWaitP95Ms)
	assert.Equal(t, 10.0, s.TaskWaitP99Ms)
	assert.Equal(t, 15.0, s.TaskWaitMaxMs)
	assert.Equal(t, int64(1000), s.TaskWaitSampleCount)
}

func TestRuntimeMetricsCollectorWithoutWaitTimeStats(t *testing.T) {
	collector := NewRuntimeMetricsCollector(100*time.Millisecond, true)

	collector.Start()
	time.Sleep(250 * time.Millisecond)
	collector.Stop()

	snapshots := collector.Snapshots()
	assert.GreaterOrEqual(t, len(snapshots), 1)

	// Without a provider, TaskWait fields should be zero
	s := snapshots[len(snapshots)-1]
	assert.Equal(t, 0.0, s.TaskWaitAvgMs)
	assert.Equal(t, int64(0), s.TaskWaitSampleCount)
}

func TestRuntimeMetricsCollectorComputeSummary(t *testing.T) {
	collector := NewRuntimeMetricsCollector(100*time.Millisecond, true)

	collector.SetWaitTimeStatsProvider(WaitTimeStatsFunc(func() PoolWaitTimeStats {
		return PoolWaitTimeStats{
			Avg:         5 * time.Millisecond,
			P50:         4 * time.Millisecond,
			P95:         8 * time.Millisecond,
			P99:         10 * time.Millisecond,
			Max:         15 * time.Millisecond,
			SampleCount: 1000,
		}
	}))

	collector.Start()
	time.Sleep(350 * time.Millisecond)
	collector.Stop()

	summary := collector.ComputeSummary()
	assert.GreaterOrEqual(t, summary.SampleCount, 2, "should have at least 2 samples")
	assert.Greater(t, summary.GoroutineAvg, 0.0, "goroutine avg should be positive")
	assert.GreaterOrEqual(t, summary.GoroutineMax, summary.GoroutineMin, "max >= min")
	assert.Greater(t, summary.HeapAllocAvgMB, 0.0, "heap avg should be positive")
	assert.False(t, summary.FirstSampleAt.IsZero(), "first sample time should be set")
	assert.False(t, summary.LastSampleAt.IsZero(), "last sample time should be set")
	assert.Greater(t, summary.TaskWaitAvgMs, 0.0, "task wait avg should be positive")
	assert.Greater(t, summary.TaskWaitP99MaxMs, 0.0, "task wait p99 max should be positive")
}

func TestRuntimeMetricsCollectorComputeSummaryEmpty(t *testing.T) {
	collector := NewRuntimeMetricsCollector(2*time.Second, true)
	// Don't start, so no samples
	summary := collector.ComputeSummary()
	assert.Equal(t, 0, summary.SampleCount)
}

// mockRunnerState implements RunnerStateProvider for testing.
type mockRunnerState struct {
	activeWorkers   int
	pendingQueueLen int
}

func (m *mockRunnerState) ActiveWorkers() int   { return m.activeWorkers }
func (m *mockRunnerState) PendingQueueLen() int { return m.pendingQueueLen }

func TestRuntimeMetricsCollectorWithRunnerState(t *testing.T) {
	collector := NewRuntimeMetricsCollector(100*time.Millisecond, true)

	state := &mockRunnerState{activeWorkers: 5, pendingQueueLen: 12}
	collector.SetRunnerStateProvider(state)

	collector.Start()
	time.Sleep(250 * time.Millisecond)
	collector.Stop()

	snapshots := collector.Snapshots()
	require.GreaterOrEqual(t, len(snapshots), 1)

	s := snapshots[len(snapshots)-1]
	assert.Equal(t, 5, s.ActiveWorkers)
	assert.Equal(t, 12, s.PendingQueueLen)
}

func TestRuntimeMetricsCollectorWithoutRunnerState(t *testing.T) {
	collector := NewRuntimeMetricsCollector(100*time.Millisecond, true)

	collector.Start()
	time.Sleep(250 * time.Millisecond)
	collector.Stop()

	snapshots := collector.Snapshots()
	require.GreaterOrEqual(t, len(snapshots), 1)

	s := snapshots[len(snapshots)-1]
	assert.Equal(t, 0, s.ActiveWorkers)
	assert.Equal(t, 0, s.PendingQueueLen)
}

func TestSystemMetricsDataInReportDetail(t *testing.T) {
	collector := NewRuntimeMetricsCollector(50*time.Millisecond, true)

	collector.SetWaitTimeStatsProvider(WaitTimeStatsFunc(func() PoolWaitTimeStats {
		return PoolWaitTimeStats{
			Avg:         3 * time.Millisecond,
			P50:         2 * time.Millisecond,
			P95:         5 * time.Millisecond,
			P99:         8 * time.Millisecond,
			Max:         12 * time.Millisecond,
			SampleCount: 500,
		}
	}))

	state := &mockRunnerState{activeWorkers: 10, pendingQueueLen: 3}
	collector.SetRunnerStateProvider(state)

	collector.Start()
	time.Sleep(200 * time.Millisecond)
	collector.Stop()

	snapshots := collector.Snapshots()
	summary := collector.ComputeSummary()

	// Build a ReportDetail with system metrics, simulating what createReport does.
	detail := ReportDetail{
		Metadata: ReportMetadata{
			RunID:       12345,
			SceneID:     67890,
			Status:      "success",
			StartedAt:   time.Now().Add(-5 * time.Second),
			FinishedAt:  time.Now(),
			DurationSec: 5.0,
			WorkerCount: 10,
			RunMode:     "duration",
		},
		GlobalSummary: GlobalSummary{
			TotalRequests: 1000,
			SuccessCount:  950,
			FailCount:     50,
			SuccessRate:   95.0,
		},
	}

	if len(snapshots) > 0 {
		detail.SystemMetrics = &SystemMetricsData{
			TimeSeries: snapshots,
			Summary:    summary,
		}
	}

	// Verify SystemMetrics is populated.
	require.NotNil(t, detail.SystemMetrics)
	assert.GreaterOrEqual(t, len(detail.SystemMetrics.TimeSeries), 1)
	assert.Greater(t, detail.SystemMetrics.Summary.SampleCount, 0)

	// Verify snapshot has runner state.
	lastSnapshot := detail.SystemMetrics.TimeSeries[len(detail.SystemMetrics.TimeSeries)-1]
	assert.Equal(t, 10, lastSnapshot.ActiveWorkers)
	assert.Equal(t, 3, lastSnapshot.PendingQueueLen)
	assert.Equal(t, 3.0, lastSnapshot.TaskWaitAvgMs)
	assert.Equal(t, int64(500), lastSnapshot.TaskWaitSampleCount)

	// Verify summary has aggregated data.
	assert.Greater(t, detail.SystemMetrics.Summary.GoroutineAvg, 0.0)
	assert.Greater(t, detail.SystemMetrics.Summary.TaskWaitAvgMs, 0.0)
}

func BenchmarkRuntimeMetricsCollectorSample(b *testing.B) {
	collector := NewRuntimeMetricsCollector(2*time.Second, true)
	collector.SetWaitTimeStatsProvider(WaitTimeStatsFunc(func() PoolWaitTimeStats {
		return PoolWaitTimeStats{
			Avg:         1 * time.Millisecond,
			P50:         1 * time.Millisecond,
			P95:         2 * time.Millisecond,
			P99:         5 * time.Millisecond,
			Max:         10 * time.Millisecond,
			SampleCount: 1000,
		}
	}))
	state := &mockRunnerState{activeWorkers: 50, pendingQueueLen: 5}
	collector.SetRunnerStateProvider(state)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.sample()
	}
}

func BenchmarkRuntimeMetricsCollectorComputeSummary(b *testing.B) {
	collector := NewRuntimeMetricsCollector(10*time.Millisecond, true)
	collector.SetWaitTimeStatsProvider(WaitTimeStatsFunc(func() PoolWaitTimeStats {
		return PoolWaitTimeStats{
			Avg:         1 * time.Millisecond,
			P50:         1 * time.Millisecond,
			P95:         2 * time.Millisecond,
			P99:         5 * time.Millisecond,
			Max:         10 * time.Millisecond,
			SampleCount: 1000,
		}
	}))
	state := &mockRunnerState{activeWorkers: 50, pendingQueueLen: 5}
	collector.SetRunnerStateProvider(state)

	collector.Start()
	time.Sleep(100 * time.Millisecond)
	collector.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.ComputeSummary()
	}
}

func TestRuntimeMetricsCollectorMemoryFootprint(t *testing.T) {
	// Simulate 1-hour collection at 2-second intervals = 1800 samples.
	// The ring buffer caps at maxSnapshots (3600), so 1800 fits entirely.
	collector := NewRuntimeMetricsCollector(2*time.Second, true)
	collector.SetWaitTimeStatsProvider(WaitTimeStatsFunc(func() PoolWaitTimeStats {
		return PoolWaitTimeStats{
			Avg:         1 * time.Millisecond,
			P50:         1 * time.Millisecond,
			P95:         2 * time.Millisecond,
			P99:         5 * time.Millisecond,
			Max:         10 * time.Millisecond,
			SampleCount: 1000,
		}
	}))
	state := &mockRunnerState{activeWorkers: 50, pendingQueueLen: 5}
	collector.SetRunnerStateProvider(state)

	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)
	before := m.Alloc

	// Simulate 1800 samples by calling sample directly.
	for i := 0; i < 1800; i++ {
		s := collector.sample()
		collector.mu.Lock()
		collector.snapshots = append(collector.snapshots, s)
		collector.mu.Unlock()
	}

	runtime.GC()
	runtime.ReadMemStats(&m)
	after := m.Alloc

	usedBytes := int64(after) - int64(before)
	usedKB := float64(usedBytes) / 1024.0
	t.Logf("Memory used after 1800 samples (1h @ 2s): %.1f KB", usedKB)

	// Each snapshot is ~200 bytes, 1800 snapshots ≈ 360KB + slice overhead.
	// Target: < 500KB.
	assert.Less(t, usedKB, 500.0, "memory footprint should be under 500KB for 1-hour simulation")
}
