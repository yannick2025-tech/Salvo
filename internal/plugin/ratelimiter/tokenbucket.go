package ratelimiter

import (
	"sync"
	"time"
)

// TokenBucket implements the token-bucket algorithm. Tokens are added
// at a steady rate up to the burst capacity. Each request consumes one
// token.
type TokenBucket struct {
	mu         sync.Mutex
	rate       float64
	burst      int
	tokens     float64
	maxTokens  float64
	lastRefill time.Time
}

// NewTokenBucket creates a token-bucket limiter. rate is tokens per
// second; burst is the maximum token count.
func NewTokenBucket(rate float64, burst int) *TokenBucket {
	return &TokenBucket{
		rate:       rate,
		burst:      burst,
		tokens:     float64(burst),
		maxTokens:  float64(burst),
		lastRefill: nowFunc(),
	}
}

// Allow implements Limiter.
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}
	return false
}

// Name implements Limiter.
func (tb *TokenBucket) Name() string { return "token-bucket" }

// Rate returns the configured refill rate.
func (tb *TokenBucket) Rate() float64 { return tb.rate }

// Burst returns the configured burst size.
func (tb *TokenBucket) Burst() int { return tb.burst }

// Tokens returns the current number of available tokens.
func (tb *TokenBucket) Tokens() float64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.refill()
	return tb.tokens
}

func (tb *TokenBucket) refill() {
	now := nowFunc()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += elapsed * tb.rate
	if tb.tokens > tb.maxTokens {
		tb.tokens = tb.maxTokens
	}
	tb.lastRefill = now
}
