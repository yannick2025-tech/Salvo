package ratelimiter

import (
	"sync"
	"time"
)

// LeakyBucket implements the leaky-bucket algorithm. Requests enter the
// bucket (up to capacity) and drain at a fixed rate. If the bucket is
// full, new requests are rejected.
type LeakyBucket struct {
	mu       sync.Mutex
	rate     float64
	capacity int
	water    int
	lastLeak time.Time
}

// NewLeakyBucket creates a leaky-bucket limiter. rate is the drain
// rate (requests per second); capacity is the bucket size.
func NewLeakyBucket(rate float64, capacity int) *LeakyBucket {
	return &LeakyBucket{
		rate:     rate,
		capacity: capacity,
		water:    0,
		lastLeak: nowFunc(),
	}
}

// Allow implements Limiter.
func (lb *LeakyBucket) Allow() bool {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.leak()

	if lb.water < lb.capacity {
		lb.water++
		return true
	}
	return false
}

// Name implements Limiter.
func (lb *LeakyBucket) Name() string { return "leaky-bucket" }

// Rate returns the configured drain rate.
func (lb *LeakyBucket) Rate() float64 { return lb.rate }

// Capacity returns the configured bucket capacity.
func (lb *LeakyBucket) Capacity() int { return lb.capacity }

// Water returns the current water level.
func (lb *LeakyBucket) Water() int {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.leak()
	return lb.water
}

func (lb *LeakyBucket) leak() {
	now := nowFunc()
	elapsed := now.Sub(lb.lastLeak).Seconds()
	drained := int(elapsed * lb.rate)
	if drained > 0 {
		lb.water -= drained
		if lb.water < 0 {
			lb.water = 0
		}
		lb.lastLeak = now
	}
}
