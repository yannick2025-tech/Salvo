package ratelimiter

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLeakyBucketName(t *testing.T) {
	lb := NewLeakyBucket(10)
	assert.Equal(t, "leaky-bucket", lb.Name())
}

func TestLeakyBucketRate(t *testing.T) {
	lb := NewLeakyBucket(100)
	assert.Equal(t, 100, lb.Rate())
}

func TestLeakyBucketPerRequest(t *testing.T) {
	lb := NewLeakyBucket(100)
	assert.Equal(t, 10*time.Millisecond, lb.PerRequest())
}

func TestLeakyBucketMaxSlackDefault(t *testing.T) {
	lb := NewLeakyBucket(100)
	assert.Equal(t, 10*10*time.Millisecond, lb.MaxSlack())
}

func TestLeakyBucketMaxSlackCustom(t *testing.T) {
	lb := NewLeakyBucket(100, WithSlack(5))
	assert.Equal(t, 5*10*time.Millisecond, lb.MaxSlack())
}

func TestLeakyBucketMaxSlackZero(t *testing.T) {
	lb := NewLeakyBucket(100, WithSlack(0))
	assert.Equal(t, time.Duration(0), lb.MaxSlack())
}

func TestLeakyBucketFirstRequestAllowed(t *testing.T) {
	lb := NewLeakyBucket(10)
	assert.True(t, lb.Allow())
}

func TestLeakyBucketRateLimiting(t *testing.T) {
	original := nowFunc
	defer func() { nowFunc = original }()

	base := time.Now()
	nowFunc = func() time.Time { return base }

	lb := NewLeakyBucket(10)

	assert.True(t, lb.Allow(), "first request should be allowed")

	assert.False(t, lb.Allow(), "second request too soon should be rejected")

	nowFunc = func() time.Time { return base.Add(100 * time.Millisecond) }
	assert.True(t, lb.Allow(), "request after perRequest interval should be allowed")
}

func TestLeakyBucketSlackAllowsBurst(t *testing.T) {
	original := nowFunc
	defer func() { nowFunc = original }()

	base := time.Now()
	nowFunc = func() time.Time { return base }

	lb := NewLeakyBucket(10, WithSlack(3))

	assert.True(t, lb.Allow(), "first request allowed")

	nowFunc = func() time.Time { return base.Add(500 * time.Millisecond) }

	burstCount := 0
	for i := 0; i < 10; i++ {
		if lb.Allow() {
			burstCount++
		}
	}
	assert.Equal(t, 4, burstCount, "slack=3 should allow 4 burst requests (1 + slack) after idle")
}

func TestLeakyBucketWithoutSlack(t *testing.T) {
	original := nowFunc
	defer func() { nowFunc = original }()

	base := time.Now()
	nowFunc = func() time.Time { return base }

	lb := NewLeakyBucket(10, WithSlack(0))

	assert.True(t, lb.Allow(), "first request allowed")

	nowFunc = func() time.Time { return base.Add(500 * time.Millisecond) }

	assert.True(t, lb.Allow(), "request after idle allowed (no slack, resets to now)")
	assert.False(t, lb.Allow(), "next request too soon, no slack to borrow from")
}

func TestLeakyBucketWaitBlocks(t *testing.T) {
	lb := NewLeakyBucket(1000)

	_, err := lb.Wait(context.Background())
	require.NoError(t, err)

	start := time.Now()
	_, err = lb.Wait(context.Background())
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, elapsed, 800*time.Microsecond, "Wait should block for ~1ms")
}

func TestLeakyBucketWaitContextCancel(t *testing.T) {
	lb := NewLeakyBucket(10)
	_, err := lb.Wait(context.Background())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		_, err := lb.Wait(ctx)
		done <- err
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	case <-time.After(time.Second):
		t.Fatal("Wait should have returned after context cancel")
	}
}

func TestLeakyBucketBlockingLimiterInterface(t *testing.T) {
	lb := NewLeakyBucket(10)
	var _ BlockingLimiter = lb
	var _ Limiter = lb
}

func TestLeakyBucketSlackAfterLongIdle(t *testing.T) {
	original := nowFunc
	defer func() { nowFunc = original }()

	base := time.Now()
	nowFunc = func() time.Time { return base }

	lb := NewLeakyBucket(10, WithSlack(2))

	assert.True(t, lb.Allow())

	nowFunc = func() time.Time { return base.Add(10 * time.Second) }

	assert.True(t, lb.Allow())
	assert.True(t, lb.Allow())
	assert.True(t, lb.Allow())
	assert.False(t, lb.Allow(), "only slack+1=3 burst allowed after long idle")
}

func TestLeakyBucketSteadyRate(t *testing.T) {
	original := nowFunc
	defer func() { nowFunc = original }()

	base := time.Now()
	nowFunc = func() time.Time { return base }

	lb := NewLeakyBucket(100, WithSlack(0))

	assert.True(t, lb.Allow())

	for i := 0; i < 5; i++ {
		nowFunc = func() time.Time { return base.Add(time.Duration(i+1) * 10 * time.Millisecond) }
		assert.True(t, lb.Allow(), "request at steady rate should be allowed")
	}
}

func TestLeakyBucketConcurrentAllow(t *testing.T) {
	lb := NewLeakyBucket(1000, WithSlack(0))

	var wg sync.WaitGroup
	allowed := atomic.Int64{}
	rejected := atomic.Int64{}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if lb.Allow() {
				allowed.Add(1)
			} else {
				rejected.Add(1)
			}
		}()
	}

	wg.Wait()

	total := allowed.Load() + rejected.Load()
	assert.Equal(t, int64(100), total, "all requests should be accounted for")
	assert.Greater(t, allowed.Load(), int64(0), "some requests should be allowed")
}
