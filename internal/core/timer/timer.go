// Package timer provides two timing primitives for Salvo test scenarios:
//
//   - ThinkTime: pauses execution for a fixed duration before proceeding
//     (similar to JMeter's "Think Time").
//   - Ticker:   repeatedly invokes a function at a fixed interval
//     (similar to Go's time.Ticker but with context and error handling).
package timer

import (
	"context"
	"fmt"
	"time"
)

// ThinkTime pauses execution for a configured duration. It is used to
// simulate user "think time" between API calls.
type ThinkTime struct {
	duration time.Duration
}

// NewThinkTime creates a ThinkTime with the given wait duration.
// A zero duration means no wait.
func NewThinkTime(d time.Duration) *ThinkTime {
	return &ThinkTime{duration: d}
}

// Duration returns the configured wait duration.
func (t *ThinkTime) Duration() time.Duration {
	return t.duration
}

// Wait blocks for the configured duration or until the context is
// cancelled. Returns the context error if cancelled.
func (t *ThinkTime) Wait(ctx context.Context) error {
	if t.duration <= 0 {
		return nil
	}

	select {
	case <-time.After(t.duration):
		return nil
	case <-ctx.Done():
		return fmt.Errorf("think time cancelled: %w", ctx.Err())
	}
}

// TickFunc is the function invoked by Ticker on each tick.
type TickFunc func(ctx context.Context) error

// Ticker repeatedly invokes a function at a fixed interval until the
// context is cancelled or the function returns an error.
type Ticker struct {
	interval time.Duration
	fn       TickFunc
}

// NewTicker creates a Ticker that calls fn every interval.
// The interval must be > 0; otherwise Run returns an error.
func NewTicker(interval time.Duration, fn TickFunc) *Ticker {
	return &Ticker{interval: interval, fn: fn}
}

// Interval returns the configured tick interval.
func (tk *Ticker) Interval() time.Duration {
	return tk.interval
}

// Run starts the ticker loop. It blocks until the context is cancelled
// or fn returns an error. The first invocation happens after the first
// interval elapses.
func (tk *Ticker) Run(ctx context.Context) error {
	if tk.interval <= 0 {
		return fmt.Errorf("ticker interval must be > 0, got %s", tk.interval)
	}

	ticker := time.NewTicker(tk.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := tk.fn(ctx); err != nil {
				return fmt.Errorf("ticker function error: %w", err)
			}
		}
	}
}
