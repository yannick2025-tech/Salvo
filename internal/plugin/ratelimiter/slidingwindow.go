package ratelimiter

import (
	"sync"
	"time"
)

// SlidingWindow implements a sliding-window rate limiter. It maintains
// weighted counters for the current and previous time windows to
// provide a smooth transition at window boundaries.
type SlidingWindow struct {
	mu       sync.Mutex
	rate     int
	window   time.Duration
	current  int
	previous int
	currStart time.Time
}

// NewSlidingWindow creates a sliding-window limiter. rate is the
// maximum number of requests per window; window is the window duration.
func NewSlidingWindow(rate int, window time.Duration) *SlidingWindow {
	return &SlidingWindow{
		rate:      rate,
		window:    window,
		currStart: nowFunc(),
	}
}

// Allow implements Limiter.
func (sw *SlidingWindow) Allow() bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	sw.advance()

	elapsed := nowFunc().Sub(sw.currStart)
	ratio := float64(sw.window-elapsed) / float64(sw.window)
	weighted := float64(sw.previous)*ratio + float64(sw.current)

	if int(weighted) < sw.rate {
		sw.current++
		return true
	}
	return false
}

// Name implements Limiter.
func (sw *SlidingWindow) Name() string { return "sliding-window" }

// Rate returns the configured max requests per window.
func (sw *SlidingWindow) Rate() int { return sw.rate }

// Window returns the configured window duration.
func (sw *SlidingWindow) Window() time.Duration { return sw.window }

// Current returns the current window counter.
func (sw *SlidingWindow) Current() int {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.advance()
	return sw.current
}

func (sw *SlidingWindow) advance() {
	now := nowFunc()
	elapsed := now.Sub(sw.currStart)

	if elapsed >= sw.window {
		windowsPassed := int(elapsed / sw.window)
		if windowsPassed >= 2 {
			sw.previous = 0
			sw.current = 0
		} else {
			sw.previous = sw.current
			sw.current = 0
		}
		sw.currStart = sw.currStart.Add(time.Duration(windowsPassed) * sw.window)
	}
}
