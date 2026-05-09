package runner

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
)

func TestTimeSeriesCollector_StartAndStop(t *testing.T) {
	store := newTestTimeSeriesStore(t)
	defer store.Close()

	collector := NewTimeSeriesCollector(TimeSeriesConfig{
		SampleInterval:   50 * time.Millisecond,
		FlushInterval:    100 * time.Millisecond,
		MemoryWindowSec:  5,
		MaxNodes:         10,
	}, snowflake.ID(1), store, nil)

	startTime := time.Now()
	err := collector.Start(startTime)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	err = collector.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestTimeSeriesCollector_SampleCollection(t *testing.T) {
	store := newTestTimeSeriesStore(t)
	defer store.Close()

	runID := snowflake.ID(12345)
	collector := NewTimeSeriesCollector(TimeSeriesConfig{
		SampleInterval:   50 * time.Millisecond,
		FlushInterval:    100 * time.Millisecond,
		MemoryWindowSec:  10,
		MaxNodes:         10,
	}, runID, store, nil)

	statsProvider := &mockStatsProvider{
		globalSnapshots: []Sample{
			{
				Timestamp:     time.Now(),
				QPS:           100.0,
				TotalRequests: 100,
				SuccessCount:  95,
				FailCount:     5,
				AvgLatencyMs:  50.0,
				P50LatencyMs:  40.0,
			},
		},
		nodeSnapshots: map[string][]Sample{
			"node-1": {{
				Timestamp:     time.Now(),
				QPS:           80.0,
				TotalRequests: 80,
				SuccessCount:  78,
				FailCount:     2,
				AvgLatencyMs:  45.0,
			}},
		},
	}

	collector.SetStatsProvider(statsProvider)

	startTime := time.Now()
	collector.Start(startTime)
	time.Sleep(250 * time.Millisecond)
	collector.Stop()

	data := collector.GetCollectedData()
	if len(data.GlobalSamples) == 0 {
		t.Error("expected global samples to be collected")
	}

	results, _ := store.QueryByRunID(context.Background(), runID)
	if len(results) == 0 {
		t.Error("expected records to be flushed to database")
	}
}

func TestTimeSeriesCollector_MemoryWindowCleanup(t *testing.T) {
	store := newTestTimeSeriesStore(t)
	defer store.Close()

	collector := NewTimeSeriesCollector(TimeSeriesConfig{
		SampleInterval:   10 * time.Millisecond,
		FlushInterval:    500 * time.Millisecond,
		MemoryWindowSec:  1, // 1 second window
		MaxNodes:         5,
	}, snowflake.ID(999), store, nil)

	statsProvider := &mockStatsProvider{
		globalSnapshots: generateMockSamples(150, 10*time.Millisecond),
	}
	collector.SetStatsProvider(statsProvider)

	startTime := time.Now()
	collector.Start(startTime)
	time.Sleep(2 * time.Second)
	collector.Stop()

	data := collector.GetCollectedData()
	expectedMaxSamples := 100 // 1 second / 10ms = ~100 samples
	if len(data.GlobalSamples) > expectedMaxSamples+10 {
		t.Errorf("memory window cleanup failed: got %d samples, expected <= %d",
			len(data.GlobalSamples), expectedMaxSamples)
	}
}

func TestTimeSeriesCollector_ConcurrentAccess(t *testing.T) {
	store := newTestTimeSeriesStore(t)
	defer store.Close()

	collector := NewTimeSeriesCollector(TimeSeriesConfig{
		SampleInterval:   20 * time.Millisecond,
		FlushInterval:    50 * time.Millisecond,
		MemoryWindowSec:  5,
		MaxNodes:         10,
	}, snowflake.ID(555), store, nil)

	statsProvider := &mockStatsProvider{
		globalSnapshots: generateMockSamples(1000, 20*time.Millisecond),
	}
	collector.SetStatsProvider(statsProvider)

	collector.Start(time.Now())

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(time.Duration(i%5) * time.Millisecond)
			_ = collector.GetCollectedData()
		}()
	}

	wg.Wait()
	collector.Stop()
}

func TestTimeSeriesCollector_GetCollectedData(t *testing.T) {
	store := newTestTimeSeriesStore(t)
	defer store.Close()

	runID := snowflake.ID(777)
	collector := NewTimeSeriesCollector(TimeSeriesConfig{
		SampleInterval:   30 * time.Millisecond,
		FlushInterval:    60 * time.Millisecond,
		MemoryWindowSec:  30,
		MaxNodes:         5,
	}, runID, store, nil)

	statsProvider := &mockStatsProvider{
		globalSnapshots: []Sample{
			{Timestamp: time.Now().Add(-2 * time.Second), QPS: 100, TotalRequests: 100, SuccessCount: 90, FailCount: 10, AvgLatencyMs: 50, P50LatencyMs: 40, P95LatencyMs: 80, P99LatencyMs: 90},
			{Timestamp: time.Now().Add(-1 * time.Second), QPS: 120, TotalRequests: 120, SuccessCount: 115, FailCount: 5, AvgLatencyMs: 45, P50LatencyMs: 35, P95LatencyMs: 75, P99LatencyMs: 85},
			{Timestamp: time.Now(), QPS: 110, TotalRequests: 110, SuccessCount: 105, FailCount: 5, AvgLatencyMs: 48, P50LatencyMs: 38, P95LatencyMs: 78, P99LatencyMs: 88},
		},
	}
	collector.SetStatsProvider(statsProvider)

	collector.Start(time.Now())
	time.Sleep(150 * time.Millisecond)
	collector.Stop()

	data := collector.GetCollectedData()

	if data == nil {
		t.Fatal("GetCollectedData returned nil")
	}

	if len(data.GlobalSamples) == 0 {
		t.Error("expected non-empty GlobalSamples")
	}
}

type mockStatsProvider struct {
	globalSnapshots []Sample
	nodeSnapshots   map[string][]Sample
	index           int
	mu              sync.Mutex
}

func (m *mockStatsProvider) GlobalSnapshot() *Sample {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.globalSnapshots) == 0 {
		return &Sample{Timestamp: time.Now()}
	}

	snap := m.globalSnapshots[m.index%len(m.globalSnapshots)]
	m.index++
	return &snap
}

func (m *mockStatsProvider) NodeSnapshots() map[string]*Sample {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make(map[string]*Sample)
	for nodeID, snaps := range m.nodeSnapshots {
		if len(snaps) > 0 {
			idx := m.index % len(snaps)
			snap := snaps[idx]
			result[nodeID] = &snap
		}
	}
	return result
}

func generateMockSamples(count int, interval time.Duration) []Sample {
	samples := make([]Sample, count)
	now := time.Now()
	for i := 0; i < count; i++ {
		samples[i] = Sample{
			Timestamp:     now.Add(time.Duration(i) * interval),
			QPS:           float64(100 + i),
			TotalRequests: int64(100 + i),
			SuccessCount:  int64(95 + i),
			FailCount:     int64(5),
			AvgLatencyMs:  float64(50 + i/10),
			P50LatencyMs:  float64(40 + i/10),
			P95LatencyMs:  float64(80 + i/10),
			P99LatencyMs:  float64(90 + i/10),
		}
	}
	return samples
}
