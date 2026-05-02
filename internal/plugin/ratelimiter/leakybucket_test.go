package ratelimiter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLeakyBucketName(t *testing.T) {
	lb := NewLeakyBucket(10, 5)
	assert.Equal(t, "leaky-bucket", lb.Name())
}

func TestLeakyBucketRate(t *testing.T) {
	lb := NewLeakyBucket(10, 5)
	assert.Equal(t, 10.0, lb.Rate())
}

func TestLeakyBucketCapacity(t *testing.T) {
	lb := NewLeakyBucket(10, 5)
	assert.Equal(t, 5, lb.Capacity())
}

func TestLeakyBucketAllowsUpToCapacity(t *testing.T) {
	lb := NewLeakyBucket(0, 3)
	for i := 0; i < 3; i++ {
		assert.True(t, lb.Allow())
	}
	assert.False(t, lb.Allow())
}

func TestLeakyBucketDrainsOverTime(t *testing.T) {
	original := nowFunc
	defer func() { nowFunc = original }()

	base := time.Now()
	nowFunc = func() time.Time { return base }

	lb := NewLeakyBucket(1000, 2)
	assert.True(t, lb.Allow())
	assert.True(t, lb.Allow())
	assert.False(t, lb.Allow())

	nowFunc = func() time.Time { return base.Add(2 * time.Millisecond) }
	assert.True(t, lb.Allow())
}

func TestLeakyBucketWater(t *testing.T) {
	lb := NewLeakyBucket(0, 5)
	assert.Equal(t, 0, lb.Water())
}

func TestLeakyBucketWaterAfterAllow(t *testing.T) {
	lb := NewLeakyBucket(0, 5)
	lb.Allow()
	assert.Equal(t, 1, lb.Water())
}

func TestLeakyBucketZeroRateZeroCapacity(t *testing.T) {
	lb := NewLeakyBucket(0, 0)
	assert.False(t, lb.Allow())
}
