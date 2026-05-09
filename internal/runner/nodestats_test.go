package runner

import (
	"sync"
	"testing"
	"time"
)

func TestNodeStats_RecordLatency(t *testing.T) {
	stats := NewNodeStats(1000)

	stats.RecordLatency(100*time.Millisecond, true)
	stats.RecordLatency(200*time.Millisecond, true)
	stats.RecordLatency(50*time.Millisecond, false)

	if total := stats.TotalReqs.Load(); total != 3 {
		t.Errorf("expected total requests 3, got %d", total)
	}

	if succ := stats.SuccessReqs.Load(); succ != 2 {
		t.Errorf("expected success requests 2, got %d", succ)
	}

	if fail := stats.FailedReqs.Load(); fail != 1 {
		t.Errorf("expected failed requests 1, got %d", fail)
	}
}

func TestNodeStats_LatencyPercentiles(t *testing.T) {
	stats := NewNodeStats(10000)

	latencies := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
		60 * time.Millisecond,
		70 * time.Millisecond,
		80 * time.Millisecond,
		90 * time.Millisecond,
		100 * time.Millisecond,
	}

	for _, lat := range latencies {
		stats.RecordLatency(lat, true)
	}

	avg, p50, p95, p99 := stats.LatencyPercentiles()

	expectedAvg := 55 * time.Millisecond
	if avg != expectedAvg {
		t.Errorf("expected avg latency %v, got %v", expectedAvg, avg)
	}

	if p50 != 60*time.Millisecond {
		t.Errorf("expected p50 latency 60ms, got %v", p50)
	}

	if p95 != 100*time.Millisecond {
		t.Errorf("expected p95 latency 100ms, got %v", p95)
	}

	if p99 != 100*time.Millisecond {
		t.Errorf("expected p99 latency 100ms, got %v", p99)
	}
}

func TestNodeStats_LatencyPercentiles_Empty(t *testing.T) {
	stats := NewNodeStats(1000)

	avg, p50, p95, p99 := stats.LatencyPercentiles()

	if avg != 0 || p50 != 0 || p95 != 0 || p99 != 0 {
		t.Error("expected all percentiles to be zero for empty stats")
	}
}

func TestNodeStats_Snapshot(t *testing.T) {
	stats := NewNodeStats(1000)

	stats.RecordLatency(100*time.Millisecond, true)
	stats.RecordLatency(150*time.Millisecond, true)
	stats.RecordLatency(80*time.Millisecond, false)

	snap := stats.Snapshot()

	if snap.TotalReqs != 3 {
		t.Errorf("expected total requests 3, got %d", snap.TotalReqs)
	}

	if snap.SuccessReqs != 2 {
		t.Errorf("expected success requests 2, got %d", snap.SuccessReqs)
	}

	if snap.FailedReqs != 1 {
		t.Errorf("expected failed requests 1, got %d", snap.FailedReqs)
	}

	expectedRate := float64(2) / float64(3) * 100
	if snap.SuccessRate < expectedRate-0.01 || snap.SuccessRate > expectedRate+0.01 {
		t.Errorf("expected success rate ~%.2f, got %.2f", expectedRate, snap.SuccessRate)
	}
}

func TestNodeStats_MaxSamplesLimit(t *testing.T) {
	maxSamples := 5
	stats := NewNodeStats(maxSamples)

	for i := 0; i < 10; i++ {
		stats.RecordLatency(time.Duration(i+1)*time.Millisecond, true)
	}

	snap := stats.Snapshot()
	if snap.TotalReqs != 10 {
		t.Errorf("total requests should count all records, not just samples: got %d", snap.TotalReqs)
	}

	avg, _, _, _ := stats.LatencyPercentiles()
	if avg == 0 {
		t.Error("avg latency should be calculated from retained samples")
	}
}

func TestNodeStats_ConcurrentAccess(t *testing.T) {
	stats := NewNodeStats(10000)

	var wg sync.WaitGroup
	const goroutines = 100
	const recordsPerGoroutine = 100

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < recordsPerGoroutine; j++ {
				stats.RecordLatency(time.Duration(j)*time.Microsecond, id%2 == 0)
			}
		}(i)
	}
	wg.Wait()

	total := stats.TotalReqs.Load()
	expectedTotal := int64(goroutines * recordsPerGoroutine)
	if total != expectedTotal {
		t.Errorf("concurrent access: expected %d total requests, got %d", expectedTotal, total)
	}
}

func TestNewNodeStats(t *testing.T) {
	tests := []struct {
		name        string
		maxSamples  int
		expectPanic bool
	}{
		{"valid max samples", 1000, false},
		{"zero max samples", 0, false},
		{"negative max samples", -1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tt.expectPanic && r == nil {
					t.Error("expected panic but did not occur")
				}
				if !tt.expectPanic && r != nil {
					t.Errorf("unexpected panic: %v", r)
				}
			}()

			stats := NewNodeStats(tt.maxSamples)
			if stats == nil {
				t.Error("NewNodeStats should not return nil")
			}
		})
	}
}

func BenchmarkNodeStats_RecordLatency(b *testing.B) {
	stats := NewNodeStats(100000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stats.RecordLatency(time.Duration(i%1000)*time.Microsecond, i%10 != 0)
	}
}

func BenchmarkNodeStats_LatencyPercentiles(b *testing.B) {
	stats := NewNodeStats(100000)

	for i := 0; i < 100000; i++ {
		stats.RecordLatency(time.Duration(i%1000)*time.Microsecond, true)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stats.LatencyPercentiles()
	}
}
