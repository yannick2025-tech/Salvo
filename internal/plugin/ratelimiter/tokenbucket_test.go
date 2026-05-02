package ratelimiter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTokenBucketName(t *testing.T) {
	tb := NewTokenBucket(10, 5)
	assert.Equal(t, "token-bucket", tb.Name())
}

func TestTokenBucketRate(t *testing.T) {
	tb := NewTokenBucket(100, 10)
	assert.Equal(t, 100.0, tb.Rate())
}

func TestTokenBucketBurst(t *testing.T) {
	tb := NewTokenBucket(100, 10)
	assert.Equal(t, 10, tb.Burst())
}

func TestTokenBucketAllowsUpToBurst(t *testing.T) {
	tb := NewTokenBucket(0, 3)
	for i := 0; i < 3; i++ {
		assert.True(t, tb.Allow())
	}
	assert.False(t, tb.Allow())
}

func TestTokenBucketRefillsOverTime(t *testing.T) {
	original := nowFunc
	defer func() { nowFunc = original }()

	base := time.Now()
	nowFunc = func() time.Time { return base }

	tb := NewTokenBucket(1000, 1)
	assert.True(t, tb.Allow())
	assert.False(t, tb.Allow())

	nowFunc = func() time.Time { return base.Add(20 * time.Millisecond) }
	assert.True(t, tb.Allow())
}

func TestTokenBucketTokens(t *testing.T) {
	tb := NewTokenBucket(0, 5)
	assert.Equal(t, 5.0, tb.Tokens())
}

func TestTokenBucketTokensAfterAcquire(t *testing.T) {
	tb := NewTokenBucket(0, 5)
	tb.Allow()
	assert.Equal(t, 4.0, tb.Tokens())
}

func TestTokenBucketTokensCapped(t *testing.T) {
	original := nowFunc
	defer func() { nowFunc = original }()

	base := time.Now()
	nowFunc = func() time.Time { return base }

	tb := NewTokenBucket(1e6, 3)
	nowFunc = func() time.Time { return base.Add(10 * time.Millisecond) }
	assert.LessOrEqual(t, tb.Tokens(), 3.0)
}

func TestTokenBucketZeroRateZeroBurst(t *testing.T) {
	tb := NewTokenBucket(0, 0)
	assert.False(t, tb.Allow())
}
