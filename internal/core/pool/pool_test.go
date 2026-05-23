package pool

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPoolSubmitAndExecute(t *testing.T) {
	var executed atomic.Int32

	p, err := New(4, Config{RunMode: RunModeCount, Count: 5})
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		p.Submit(func(ctx context.Context) error {
			executed.Add(1)
			return nil
		})
	}

	err = p.Wait()
	require.NoError(t, err)
	assert.Equal(t, int32(5), executed.Load())
}

func TestPoolWorkerCount(t *testing.T) {
	p, err := New(8, Config{RunMode: RunModeCount, Count: 10})
	require.NoError(t, err)
	assert.Equal(t, 8, p.WorkerCount())
}

func TestPoolDurationMode(t *testing.T) {
	var executed atomic.Int32

	p, err := New(2, Config{
		RunMode:  RunModeDuration,
		Duration: 200 * time.Millisecond,
	})
	require.NoError(t, err)

	go func() {
		for i := 0; i < 100; i++ {
			p.Submit(func(ctx context.Context) error {
				executed.Add(1)
				time.Sleep(10 * time.Millisecond)
				return nil
			})
		}
	}()

	err = p.Wait()
	require.NoError(t, err)
	assert.Greater(t, executed.Load(), int32(0))
}

func TestPoolCountMode(t *testing.T) {
	var executed atomic.Int32

	p, err := New(4, Config{RunMode: RunModeCount, Count: 20})
	require.NoError(t, err)

	for i := 0; i < 20; i++ {
		p.Submit(func(ctx context.Context) error {
			executed.Add(1)
			return nil
		})
	}

	err = p.Wait()
	require.NoError(t, err)
	assert.Equal(t, int32(20), executed.Load())
}

func TestPoolContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	p, err := NewWithContext(ctx, 2, Config{RunMode: RunModeDuration, Duration: 30 * time.Second})
	require.NoError(t, err)

	var executed atomic.Int32
	for i := 0; i < 20; i++ {
		p.Submit(func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(200 * time.Millisecond):
				executed.Add(1)
				return nil
			}
		})
	}

	_ = p.Wait()
	assert.Less(t, executed.Load(), int32(20), "should not execute all tasks after cancellation")
}

func TestPoolTaskError(t *testing.T) {
	p, err := New(2, Config{RunMode: RunModeCount, Count: 5})
	require.NoError(t, err)

	errTask := errors.New("task failed")
	p.Submit(func(ctx context.Context) error {
		return errTask
	})

	err = p.Wait()
	assert.ErrorIs(t, err, errTask)
}

func TestPoolZeroWorkerCount(t *testing.T) {
	_, err := New(0, Config{RunMode: RunModeCount, Count: 10})
	assert.Error(t, err)
}

func TestPoolInvalidConfig(t *testing.T) {
	_, err := New(4, Config{RunMode: RunModeCount, Count: 0})
	assert.Error(t, err)

	_, err = New(4, Config{RunMode: RunModeDuration, Duration: 0})
	assert.Error(t, err)
}

func TestPoolShutdown(t *testing.T) {
	p, err := New(2, Config{RunMode: RunModeCount, Count: 100})
	require.NoError(t, err)

	var executed atomic.Int32
	for i := 0; i < 100; i++ {
		p.Submit(func(ctx context.Context) error {
			executed.Add(1)
			time.Sleep(5 * time.Millisecond)
			return nil
		})
	}

	p.Shutdown()
	assert.Panics(t, func() {
		p.Submit(func(ctx context.Context) error { return nil })
	})
}

func TestPoolSubmittedCount(t *testing.T) {
	p, err := New(2, Config{RunMode: RunModeCount, Count: 10})
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		p.Submit(func(ctx context.Context) error { return nil })
	}

	assert.Equal(t, int64(10), p.Submitted())
	_ = p.Wait()
}

func TestPoolCompletedCount(t *testing.T) {
	p, err := New(4, Config{RunMode: RunModeCount, Count: 10})
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		p.Submit(func(ctx context.Context) error { return nil })
	}

	_ = p.Wait()
	assert.Equal(t, int64(10), p.Completed())
}

func TestPoolTaskWaitStats(t *testing.T) {
	p, err := New(2, Config{RunMode: RunModeCount, Count: 10})
	require.NoError(t, err)

	// Submit tasks with a small sleep to create measurable wait times
	for i := 0; i < 10; i++ {
		p.Submit(func(ctx context.Context) error {
			time.Sleep(5 * time.Millisecond)
			return nil
		})
	}

	_ = p.Wait()

	stats := p.TaskWaitStats()
	assert.Greater(t, stats.SampleCount, int64(0), "should have recorded wait time samples")
	assert.Greater(t, stats.Avg, time.Duration(0), "average wait time should be positive")
	assert.GreaterOrEqual(t, stats.Max, stats.Avg, "max should be >= avg")
	assert.GreaterOrEqual(t, stats.P99, stats.P50, "P99 should be >= P50")
}

func TestPoolTaskWaitStatsEmpty(t *testing.T) {
	p, err := New(2, Config{RunMode: RunModeCount, Count: 1})
	require.NoError(t, err)

	// No tasks submitted yet
	stats := p.TaskWaitStats()
	assert.Equal(t, int64(0), stats.SampleCount)
}

func BenchmarkPoolSubmit(b *testing.B) {
	p, err := New(4, Config{RunMode: RunModeDuration, Duration: 30 * time.Second})
	if err != nil {
		b.Fatal(err)
	}
	defer p.Shutdown()

	task := func(ctx context.Context) error { return nil }

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Submit(task)
	}
}
