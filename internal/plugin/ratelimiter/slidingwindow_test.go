package ratelimiter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSlidingWindowName(t *testing.T) {
	sw := NewSlidingWindow(10, time.Second)
	assert.Equal(t, "sliding-window", sw.Name())
}

func TestSlidingWindowRate(t *testing.T) {
	sw := NewSlidingWindow(10, time.Second)
	assert.Equal(t, 10, sw.Rate())
}

func TestSlidingWindowWindow(t *testing.T) {
	sw := NewSlidingWindow(10, time.Second)
	assert.Equal(t, time.Second, sw.Window())
}

func TestSlidingWindowAllowsUpToRate(t *testing.T) {
	sw := NewSlidingWindow(3, time.Second)
	for i := 0; i < 3; i++ {
		assert.True(t, sw.Allow())
	}
	assert.False(t, sw.Allow())
}

func TestSlidingWindowResetsAfterWindow(t *testing.T) {
	original := nowFunc
	defer func() { nowFunc = original }()

	base := time.Now()
	nowFunc = func() time.Time { return base }

	sw := NewSlidingWindow(2, 100*time.Millisecond)
	assert.True(t, sw.Allow())
	assert.True(t, sw.Allow())
	assert.False(t, sw.Allow())

	nowFunc = func() time.Time { return base.Add(150 * time.Millisecond) }
	assert.True(t, sw.Allow())
}

func TestSlidingWindowCurrent(t *testing.T) {
	sw := NewSlidingWindow(10, time.Second)
	assert.Equal(t, 0, sw.Current())
	sw.Allow()
	assert.Equal(t, 1, sw.Current())
}

func TestSlidingWindowZeroRate(t *testing.T) {
	sw := NewSlidingWindow(0, time.Second)
	assert.False(t, sw.Allow())
}
