package timer

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThinkTimeWait(t *testing.T) {
	tt := NewThinkTime(50 * time.Millisecond)
	start := time.Now()
	err := tt.Wait(context.Background())
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond)
	assert.Less(t, elapsed, 100*time.Millisecond)
}

func TestThinkTimeContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	tt := NewThinkTime(200 * time.Millisecond)
	err := tt.Wait(ctx)
	assert.Error(t, err)
}

func TestThinkTimeZero(t *testing.T) {
	tt := NewThinkTime(0)
	start := time.Now()
	err := tt.Wait(context.Background())
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Less(t, elapsed, 10*time.Millisecond)
}

func TestTickerFires(t *testing.T) {
	var count atomic.Int32
	tk := NewTicker(50*time.Millisecond, func(ctx context.Context) error {
		count.Add(1)
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 175*time.Millisecond)
	defer cancel()

	err := tk.Run(ctx)
	require.NoError(t, err)

	fired := count.Load()
	assert.GreaterOrEqual(t, fired, int32(2), "ticker should fire at least twice")
	assert.LessOrEqual(t, fired, int32(4), "ticker should not fire too many times")
}

func TestTickerStopsOnError(t *testing.T) {
	var count atomic.Int32
	tk := NewTicker(50*time.Millisecond, func(ctx context.Context) error {
		count.Add(1)
		if count.Load() >= 2 {
			return assert.AnError
		}
		return nil
	})

	err := tk.Run(context.Background())
	assert.Error(t, err)
	assert.Equal(t, int32(2), count.Load())
}

func TestTickerContextCancellation(t *testing.T) {
	var count atomic.Int32
	tk := NewTicker(50*time.Millisecond, func(ctx context.Context) error {
		count.Add(1)
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(75 * time.Millisecond)
		cancel()
	}()

	_ = tk.Run(ctx)
	assert.LessOrEqual(t, count.Load(), int32(2))
}

func TestTickerZeroInterval(t *testing.T) {
	tk := NewTicker(0, func(ctx context.Context) error { return nil })
	err := tk.Run(context.Background())
	assert.Error(t, err)
}

func TestTickerInterval(t *testing.T) {
	tk := NewTicker(100 * time.Millisecond, func(ctx context.Context) error { return nil })
	assert.Equal(t, 100*time.Millisecond, tk.Interval())
}

func TestThinkTimeDuration(t *testing.T) {
	tt := NewThinkTime(100 * time.Millisecond)
	assert.Equal(t, 100*time.Millisecond, tt.Duration())
}
