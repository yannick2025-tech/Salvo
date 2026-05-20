// Package cascade implements a hierarchical context system for Salvo
// that propagates timeouts from Global → Scene → Chain → API.
//
// Each level can set its own timeout, but the effective deadline is
// always the minimum of the parent's deadline and the current level's
// timeout. This ensures that a child context can never outlive its
// parent.
package cascade

import (
	"context"
	"fmt"
	"time"
)

type contextKey string

const (
	sceneIDKey contextKey = "salvo:scene_id"
	apiIDKey   contextKey = "salvo:api_id"
	chainKey   contextKey = "salvo:chain_key"
)

// NewContext creates a child context with a scene identifier and an
// optional timeout. If timeout > 0, the context carries a deadline
// that is the earlier of the parent's deadline (if any) and
// now+timeout.
func NewContext(parent context.Context, sceneID string, timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx := context.WithValue(parent, sceneIDKey, sceneID)
	ctx = context.WithValue(ctx, chainKey, sceneID)

	if timeout <= 0 {
		return context.WithCancel(ctx)
	}

	if parentDeadline, ok := parent.Deadline(); ok {
		childDeadline := time.Now().Add(timeout)
		if childDeadline.After(parentDeadline) {
			// Cap at parent deadline.
			dur := time.Until(parentDeadline)
			if dur <= 0 {
				// Parent already expired.
				ctx, cancel := context.WithCancel(ctx)
				cancel()
				return ctx, cancel
			}
			return context.WithTimeout(ctx, dur)
		}
	}

	return context.WithTimeout(ctx, timeout)
}

// WithAPITimeout creates a child context with an API identifier and
// timeout. The effective deadline is the earlier of the parent's
// deadline and now+timeout.
func WithAPITimeout(parent context.Context, apiID string, timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx := context.WithValue(parent, apiIDKey, apiID)

	if timeout <= 0 {
		return context.WithCancel(ctx)
	}

	if parentDeadline, ok := parent.Deadline(); ok {
		childDeadline := time.Now().Add(timeout)
		if childDeadline.After(parentDeadline) {
			dur := time.Until(parentDeadline)
			if dur <= 0 {
				ctx, cancel := context.WithCancel(ctx)
				cancel()
				return ctx, cancel
			}
			return context.WithTimeout(ctx, dur)
		}
	}

	return context.WithTimeout(ctx, timeout)
}

// SceneID extracts the scene identifier from the context.
func SceneID(ctx context.Context) (string, bool) {
	val := ctx.Value(sceneIDKey)
	if val == nil {
		return "", false
	}
	id, ok := val.(string)
	return id, ok
}

// APIID extracts the API identifier from the context.
func APIID(ctx context.Context) (string, bool) {
	val := ctx.Value(apiIDKey)
	if val == nil {
		return "", false
	}
	id, ok := val.(string)
	return id, ok
}

// ChainKey extracts the chain/trace key from the context.
func ChainKey(ctx context.Context) (string, bool) {
	val := ctx.Value(chainKey)
	if val == nil {
		return "", false
	}
	key, ok := val.(string)
	return key, ok
}

// WithChainKey sets a chain/trace key on the context for correlation.
func WithChainKey(parent context.Context, key string) context.Context {
	return context.WithValue(parent, chainKey, key)
}

// RemainingTimeout returns the time remaining before the context
// deadline. If the context has no deadline, it returns the default
// duration. If the context is already expired, it returns 0.
func RemainingTimeout(ctx context.Context, defaultDur time.Duration) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return defaultDur
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return 0
	}
	return remaining
}

// FormatTimeoutError creates a formatted timeout error with context
// information.
func FormatTimeoutError(ctx context.Context, operation string) error {
	sceneID, _ := SceneID(ctx)
	apiID, _ := APIID(ctx)
	return fmt.Errorf("%s timed out (scene=%s, api=%s)", operation, sceneID, apiID)
}
