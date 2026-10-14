package pool

import (
	"math"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPercentileEmpty(t *testing.T) {
	result := percentile(nil, 50)
	assert.Equal(t, time.Duration(0), result)
}

func TestPercentileSingleElement(t *testing.T) {
	result := percentile([]time.Duration{100 * time.Millisecond}, 50)
	assert.Equal(t, 100*time.Millisecond, result)
}

func TestPercentileP50(t *testing.T) {
	data := []time.Duration{
		1 * time.Millisecond,
		2 * time.Millisecond,
		3 * time.Millisecond,
		4 * time.Millisecond,
		5 * time.Millisecond,
		6 * time.Millisecond,
		7 * time.Millisecond,
		8 * time.Millisecond,
		9 * time.Millisecond,
		10 * time.Millisecond,
	}
	// idx = (10-1) * 50/100 = 4.5 -> interpolate between 5ms and 6ms = 5.5ms
	result := percentile(data, 50)
	assert.Equal(t, 5500*time.Microsecond, result)
}

func TestPercentileP95(t *testing.T) {
	data := make([]time.Duration, 100)
	for i := 0; i < 100; i++ {
		data[i] = time.Duration(i+1) * time.Millisecond
	}
	// idx = 99 * 0.95 = 94.05 -> interpolate between 95ms and 96ms
	// Due to float64 precision, exact value is 95049999ns, not 95050000ns
	result := percentile(data, 95)
	assert.Equal(t, 95049999*time.Nanosecond, result)
}

func TestPercentileP99(t *testing.T) {
	data := make([]time.Duration, 100)
	for i := 0; i < 100; i++ {
		data[i] = time.Duration(i+1) * time.Millisecond
	}
	// idx = 99 * 0.99 = 98.01 -> interpolate between 99ms and 100ms
	result := percentile(data, 99)
	assert.Equal(t, 99010*time.Microsecond, result)
}

func TestPercentileInterpolation(t *testing.T) {
	data := []time.Duration{1 * time.Millisecond, 2 * time.Millisecond}
	// idx = 1 * 0.5 = 0.5 -> interpolate between 1ms and 2ms = 1.5ms
	result := percentile(data, 50)
	assert.Equal(t, 1500*time.Microsecond, result)
}

func TestWaitTimeTrackerRecordAndStats(t *testing.T) {
	tracker := NewWaitTimeTracker(1000)

	tracker.Record(10 * time.Millisecond)
	tracker.Record(20 * time.Millisecond)
	tracker.Record(30 * time.Millisecond)

	stats := tracker.Stats()
	assert.Equal(t, int64(3), stats.SampleCount)
	assert.Equal(t, 20*time.Millisecond, stats.Avg)
	assert.Equal(t, 30*time.Millisecond, stats.Max)
}

func TestWaitTimeTrackerEmpty(t *testing.T) {
	tracker := NewWaitTimeTracker(1000)

	stats := tracker.Stats()
	assert.Equal(t, int64(0), stats.SampleCount)
	assert.Equal(t, time.Duration(0), stats.Avg)
	assert.Equal(t, time.Duration(0), stats.P50)
	assert.Equal(t, time.Duration(0), stats.P95)
	assert.Equal(t, time.Duration(0), stats.P99)
	assert.Equal(t, time.Duration(0), stats.Max)
}

func TestWaitTimeTrackerCircularBuffer(t *testing.T) {
	capacity := 5
	tracker := NewWaitTimeTracker(capacity)

	for i := 0; i < capacity+3; i++ {
		tracker.Record(time.Duration(i+1) * time.Millisecond)
	}

	stats := tracker.Stats()
	assert.Equal(t, int64(capacity+3), stats.SampleCount)

	// The circular buffer should only hold the last `capacity` samples
	// Samples 4,5,6,7,8 (0-indexed: 3,4,5,6,7) -> values 4ms,5ms,6ms,7ms,8ms
	assert.Equal(t, 8*time.Millisecond, stats.Max)
	// P50 of [4,5,6,7,8]ms with interpolation: idx=4*0.5=2 -> 6ms
	assert.Equal(t, 6*time.Millisecond, stats.P50)
}

func TestWaitTimeTrackerConcurrent(t *testing.T) {
	tracker := NewWaitTimeTracker(10000)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				tracker.Record(time.Duration(j+1) * time.Microsecond)
			}
		}()
	}

	wg.Wait()

	stats := tracker.Stats()
	assert.Equal(t, int64(10000), stats.SampleCount)
	assert.Greater(t, stats.Avg, time.Duration(0))
	assert.Greater(t, stats.P99, time.Duration(0))
}

func TestWaitTimeTrackerReset(t *testing.T) {
	tracker := NewWaitTimeTracker(100)

	tracker.Record(10 * time.Millisecond)
	tracker.Record(20 * time.Millisecond)

	stats := tracker.Stats()
	assert.Equal(t, int64(2), stats.SampleCount)

	tracker.Reset()

	stats = tracker.Stats()
	assert.Equal(t, int64(0), stats.SampleCount)
	assert.Equal(t, time.Duration(0), stats.Avg)
}

func TestWaitTimeStatsPercentileAccuracy(t *testing.T) {
	tracker := NewWaitTimeTracker(10000)

	for i := 0; i < 1000; i++ {
		tracker.Record(time.Duration(i+1) * time.Millisecond)
	}

	stats := tracker.Stats()
	assert.Equal(t, int64(1000), stats.SampleCount)

	// P50: idx = 999 * 0.5 = 499.5 -> interpolate between 500ms and 501ms = 500.5ms
	assert.Equal(t, 500500*time.Microsecond, stats.P50)

	// P95: idx = 999 * 0.95 = 949.05 -> interpolate between 950ms and 951ms
	// Float64 precision: actual is 950049999ns
	assert.Equal(t, 950049999*time.Nanosecond, stats.P95)

	// P99: idx = 999 * 0.99 = 989.01 -> interpolate between 990ms and 991ms
	// Float64 precision: actual is 990009999ns
	assert.Equal(t, 990009999*time.Nanosecond, stats.P99)

	// Max should be 1000ms
	assert.Equal(t, 1000*time.Millisecond, stats.Max)

	// Avg should be ~500.5ms
	expectedAvg := time.Duration(math.Round(float64(500500) * float64(time.Microsecond)))
	assert.Equal(t, expectedAvg, stats.Avg)
}

func BenchmarkWaitTimeTrackerRecord(b *testing.B) {
	tracker := NewWaitTimeTracker(10000)
	d := 100 * time.Microsecond

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracker.Record(d)
	}
}

func BenchmarkWaitTimeTrackerStats(b *testing.B) {
	tracker := NewWaitTimeTracker(1000)
	for i := 0; i < 1000; i++ {
		tracker.Record(time.Duration(i+1) * time.Microsecond)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracker.Stats()
	}
}
