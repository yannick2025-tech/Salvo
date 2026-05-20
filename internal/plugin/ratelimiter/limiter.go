// Package ratelimiter provides rate limiting algorithms for Salvo.
// It defines the Limiter interface that all algorithms implement and
// a LimiterPlugin adapter that wraps any Limiter into a plugin.Plugin.
//
// Supported algorithms:
//   - TokenBucket: steady rate with burst allowance
//   - LeakyBucket: reservation-based with max slack (inspired by
//     go.uber.org/ratelimit), supports both blocking and non-blocking
//   - SlidingWindow: weighted counter over a sliding time window
//   - FixedWindow: simple counter reset at fixed intervals
package ratelimiter

import (
	"context"
	"fmt"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/plugin"
)

// Limiter is the core interface that every rate-limiting algorithm
// must implement. It is a pure algorithm with no knowledge of the
// plugin system.
type Limiter interface {
	// Allow returns true if the request is permitted, false if the
	// rate limit has been exceeded. This is the non-blocking mode.
	Allow() bool
	// Name returns the algorithm identifier (e.g. "token-bucket").
	Name() string
}

// BlockingLimiter is an optional interface that limiters may implement
// to support blocking mode. In blocking mode, the caller waits until
// the rate limiter permits the request.
type BlockingLimiter interface {
	Limiter
	// Wait blocks until the request is permitted or the context is
	// cancelled. Returns the time at which the request was allowed.
	Wait(ctx context.Context) (time.Time, error)
}

// LimiterPlugin adapts a Limiter into a plugin.Plugin so it can be
// registered with the plugin.Registry.
type LimiterPlugin struct {
	limiter    Limiter
	priority   int
	pluginName string
}

// LimiterPluginOption configures a LimiterPlugin.
type LimiterPluginOption func(*LimiterPlugin)

// WithLimiterPriority sets the plugin priority.
func WithLimiterPriority(p int) LimiterPluginOption {
	return func(lp *LimiterPlugin) { lp.priority = p }
}

// WithLimiterPluginName sets a custom plugin name.
func WithLimiterPluginName(n string) LimiterPluginOption {
	return func(lp *LimiterPlugin) { lp.pluginName = n }
}

// NewLimiterPlugin wraps a Limiter as a plugin.Plugin.
func NewLimiterPlugin(l Limiter, opts ...LimiterPluginOption) *LimiterPlugin {
	lp := &LimiterPlugin{
		limiter:    l,
		priority:   1,
		pluginName: l.Name(),
	}
	for _, opt := range opts {
		opt(lp)
	}
	return lp
}

// Name implements plugin.Plugin.
func (lp *LimiterPlugin) Name() string { return lp.pluginName }

// Priority implements plugin.Plugin.
func (lp *LimiterPlugin) Priority() int { return lp.priority }

// Before checks if the limiter allows the request. If not, it aborts
// the pipeline with a rate-limit error.
func (lp *LimiterPlugin) Before(ctx *plugin.Context) error {
	if !lp.limiter.Allow() {
		ctx.Abort(fmt.Errorf("ratelimiter: rate limit exceeded (%s)", lp.limiter.Name()))
	}
	return nil
}

// After is a no-op for rate limiters.
func (lp *LimiterPlugin) After(_ *plugin.Context) error {
	return nil
}

// AlgorithmName returns the underlying algorithm name.
func (lp *LimiterPlugin) AlgorithmName() string {
	return lp.limiter.Name()
}

// Limiter returns the underlying Limiter for direct access.
func (lp *LimiterPlugin) Limiter() Limiter {
	return lp.limiter
}

// compile-time check
var _ plugin.Plugin = (*LimiterPlugin)(nil)

// nowFunc is overridable for testing.
var nowFunc = time.Now
