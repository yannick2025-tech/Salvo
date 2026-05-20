package ratelimiter

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// LeakyBucket implements the leaky-bucket algorithm using a reservation
// timestamp approach inspired by go.uber.org/ratelimit.
//
// Instead of tracking a water level, it computes the next time a
// request is allowed. If the current time is before that reservation,
// the request is either rejected (Allow) or delayed (Wait).
//
// A maxSlack parameter allows previously "unspent" time to accumulate
// for future bursts, up to a configurable bound. This prevents a
// long idle period from allowing an unbounded burst.
type LeakyBucket struct {
	mu sync.Mutex

	perRequest time.Duration
	maxSlack   time.Duration
	slack      int

	last atomic.Int64
}

// LeakyBucketOption configures a LeakyBucket.
type LeakyBucketOption func(*LeakyBucket)

// WithSlack sets the maximum number of requests that can be
// "pre-accumulated" during idle periods. A slack of N means up to N
// requests can burst immediately after an idle period. Default is 10.
// Set to 0 for strict (no slack) behavior.
func WithSlack(slack int) LeakyBucketOption {
	return func(lb *LeakyBucket) { lb.slack = slack }
}

// NewLeakyBucket creates a leaky-bucket limiter. rate is the number of
// requests per second. The bucket enforces a steady output rate of
// 1/rate seconds between requests.
func NewLeakyBucket(rate int, opts ...LeakyBucketOption) *LeakyBucket {
	lb := &LeakyBucket{
		perRequest: time.Second / time.Duration(rate),
		maxSlack:   10 * (time.Second / time.Duration(rate)),
		slack:      10,
	}
	for _, opt := range opts {
		opt(lb)
	}
	if lb.slack <= 0 {
		lb.maxSlack = 0
	} else {
		lb.maxSlack = time.Duration(lb.slack) * lb.perRequest
	}
	lb.last.Store(0)
	return lb
}

// Allow implements Limiter. It returns true if the request can be
// made immediately without waiting. This is the non-blocking mode.
func (lb *LeakyBucket) Allow() bool {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	now := nowFunc().UnixNano()
	last := lb.last.Load()

	next := lb.nextAllowedTime(now, last)

	if now >= next {
		lb.last.Store(next)
		return true
	}
	return false
}

// Wait implements BlockingLimiter. It blocks until the request is
// permitted or the context is cancelled.
func (lb *LeakyBucket) Wait(ctx context.Context) (time.Time, error) {
	lb.mu.Lock()

	now := nowFunc().UnixNano()
	last := lb.last.Load()

	next := lb.nextAllowedTime(now, last)
	lb.last.Store(next)
	lb.mu.Unlock()

	sleepDuration := time.Duration(next - now)
	if sleepDuration > 0 {
		timer := time.NewTimer(sleepDuration)
		defer timer.Stop()

		select {
		case <-timer.C:
			return time.Unix(0, next), nil
		case <-ctx.Done():
			return time.Time{}, ctx.Err()
		}
	}

	return time.Unix(0, now), nil
}

// Name implements Limiter.
func (lb *LeakyBucket) Name() string { return "leaky-bucket" }

// Rate returns the configured rate (requests per second).
func (lb *LeakyBucket) Rate() int {
	return int(time.Second / lb.perRequest)
}

// PerRequest returns the time between each request.
func (lb *LeakyBucket) PerRequest() time.Duration {
	return lb.perRequest
}

// MaxSlack returns the configured maximum slack duration.
func (lb *LeakyBucket) MaxSlack() time.Duration {
	return lb.maxSlack
}

func (lb *LeakyBucket) nextAllowedTime(now, last int64) int64 {
	switch {
	case last == 0 || (lb.maxSlack == 0 && now-last > int64(lb.perRequest)):
		return now
	case lb.maxSlack > 0 && now-last > int64(lb.maxSlack)+int64(lb.perRequest):
		return now - int64(lb.maxSlack)
	default:
		return last + int64(lb.perRequest)
	}
}
