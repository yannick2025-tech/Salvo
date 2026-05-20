package ratelimiter

import (
	"sync"
	"time"
)

// FixedWindow implements a fixed-window rate limiter. It counts
// requests within a fixed time interval and resets the counter at
// each interval boundary.
type FixedWindow struct {
	mu       sync.Mutex
	rate     int
	window   time.Duration
	count    int
	winStart time.Time
}

// NewFixedWindow creates a fixed-window limiter. rate is the maximum
// number of requests per window; window is the window duration.
func NewFixedWindow(rate int, window time.Duration) *FixedWindow {
	return &FixedWindow{
		rate:     rate,
		window:   window,
		winStart: nowFunc(),
	}
}

// Allow implements Limiter.
func (fw *FixedWindow) Allow() bool {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	fw.advance()

	if fw.count < fw.rate {
		fw.count++
		return true
	}
	return false
}

// Name implements Limiter.
func (fw *FixedWindow) Name() string { return "fixed-window" }

// Rate returns the configured max requests per window.
func (fw *FixedWindow) Rate() int { return fw.rate }

// Window returns the configured window duration.
func (fw *FixedWindow) Window() time.Duration { return fw.window }

// Count returns the current window request count.
func (fw *FixedWindow) Count() int {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	fw.advance()
	return fw.count
}

func (fw *FixedWindow) advance() {
	now := nowFunc()
	if now.Sub(fw.winStart) >= fw.window {
		fw.count = 0
		fw.winStart = now
	}
}
