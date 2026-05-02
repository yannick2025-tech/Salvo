// Package ratelimiter provides a token-bucket rate limiter plugin for
// Salvo. It controls the rate at which requests are dispatched to the
// target server, preventing overload and simulating realistic traffic
// patterns.
package ratelimiter

import (
	"fmt"
	"sync"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/plugin"
)

// Limiter implements a token-bucket rate limiter as a Plugin.
type Limiter struct {
	mu         sync.Mutex
	name       string
	priority   int
	rate       float64
	burst      int
	tokens     float64
	maxTokens  float64
	lastRefill time.Time
}

// Option configures a Limiter during construction.
type Option func(*Limiter)

// WithPriority sets the plugin priority.
func WithPriority(p int) Option {
	return func(l *Limiter) { l.priority = p }
}

// WithName sets a custom plugin name.
func WithName(n string) Option {
	return func(l *Limiter) { l.name = n }
}

// NewLimiter creates a token-bucket rate limiter. rate is the number of
// tokens added per second; burst is the maximum number of tokens that
// can accumulate.
func NewLimiter(rate float64, burst int, opts ...Option) *Limiter {
	l := &Limiter{
		name:       "ratelimiter",
		priority:   1,
		rate:       rate,
		burst:      burst,
		tokens:     float64(burst),
		maxTokens:  float64(burst),
		lastRefill: time.Now(),
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Name implements plugin.Plugin.
func (l *Limiter) Name() string { return l.name }

// Priority implements plugin.Plugin.
func (l *Limiter) Priority() int { return l.priority }

// Before checks if a token is available. If not, it aborts the
// pipeline with a rate-limit error.
func (l *Limiter) Before(ctx *plugin.Context) error {
	if !l.tryAcquire() {
		ctx.Abort(fmt.Errorf("ratelimiter: rate limit exceeded (%.1f req/s)", l.rate))
	}
	return nil
}

// After is a no-op for the rate limiter.
func (l *Limiter) After(_ *plugin.Context) error {
	return nil
}

// Rate returns the configured token refill rate.
func (l *Limiter) Rate() float64 { return l.rate }

// Burst returns the configured burst size.
func (l *Limiter) Burst() int { return l.burst }

// Tokens returns the current number of available tokens.
func (l *Limiter) Tokens() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refill()
	return l.tokens
}

func (l *Limiter) tryAcquire() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.refill()

	if l.tokens >= 1 {
		l.tokens--
		return true
	}
	return false
}

func (l *Limiter) refill() {
	now := time.Now()
	elapsed := now.Sub(l.lastRefill).Seconds()
	l.tokens += elapsed * l.rate
	if l.tokens > l.maxTokens {
		l.tokens = l.maxTokens
	}
	l.lastRefill = now
}
