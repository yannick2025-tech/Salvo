package ratelimiter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFixedWindowName(t *testing.T) {
	fw := NewFixedWindow(10, time.Second)
	assert.Equal(t, "fixed-window", fw.Name())
}

func TestFixedWindowRate(t *testing.T) {
	fw := NewFixedWindow(10, time.Second)
	assert.Equal(t, 10, fw.Rate())
}

func TestFixedWindowWindow(t *testing.T) {
	fw := NewFixedWindow(10, time.Second)
	assert.Equal(t, time.Second, fw.Window())
}

func TestFixedWindowAllowsUpToRate(t *testing.T) {
	fw := NewFixedWindow(3, time.Second)
	for i := 0; i < 3; i++ {
		assert.True(t, fw.Allow())
	}
	assert.False(t, fw.Allow())
}

func TestFixedWindowResetsAfterWindow(t *testing.T) {
	original := nowFunc
	defer func() { nowFunc = original }()

	base := time.Now()
	nowFunc = func() time.Time { return base }

	fw := NewFixedWindow(2, 100*time.Millisecond)
	assert.True(t, fw.Allow())
	assert.True(t, fw.Allow())
	assert.False(t, fw.Allow())

	nowFunc = func() time.Time { return base.Add(150 * time.Millisecond) }
	assert.True(t, fw.Allow())
}

func TestFixedWindowCount(t *testing.T) {
	fw := NewFixedWindow(10, time.Second)
	assert.Equal(t, 0, fw.Count())
	fw.Allow()
	assert.Equal(t, 1, fw.Count())
}

func TestFixedWindowZeroRate(t *testing.T) {
	fw := NewFixedWindow(0, time.Second)
	assert.False(t, fw.Allow())
}
