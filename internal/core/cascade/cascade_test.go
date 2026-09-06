package cascade

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewContext(t *testing.T) {
	parent := context.Background()
	ctx, cancel := NewContext(parent, "scene-1", 30*time.Second)
	defer cancel()

	id, ok := SceneID(ctx)
	assert.True(t, ok)
	assert.Equal(t, "scene-1", id)

	deadline, ok := ctx.Deadline()
	assert.True(t, ok)
	assert.WithinDuration(t, time.Now().Add(30*time.Second), deadline, 100*time.Millisecond)
}

func TestChainTimeout(t *testing.T) {
	parent := context.Background()

	globalCtx, globalCancel := NewContext(parent, "global", 10*time.Second)
	defer globalCancel()

	sceneCtx, sceneCancel := NewContext(globalCtx, "scene-1", 5*time.Second)
	defer sceneCancel()

	// Scene timeout should be shorter than global.
	deadline, ok := sceneCtx.Deadline()
	assert.True(t, ok)
	assert.WithinDuration(t, time.Now().Add(5*time.Second), deadline, 100*time.Millisecond)
}

func TestChainTimeoutSceneExceedsGlobal(t *testing.T) {
	parent := context.Background()

	globalCtx, globalCancel := NewContext(parent, "global", 5*time.Second)
	defer globalCancel()

	// Scene timeout exceeds global; the effective deadline should be
	// capped by the global context.
	sceneCtx, sceneCancel := NewContext(globalCtx, "scene-1", 30*time.Second)
	defer sceneCancel()

	deadline, ok := sceneCtx.Deadline()
	assert.True(t, ok)
	// Should be closer to 5s (global) than 30s (scene).
	assert.WithinDuration(t, time.Now().Add(5*time.Second), deadline, 100*time.Millisecond)
}

func TestAPITimeout(t *testing.T) {
	parent := context.Background()

	sceneCtx, sceneCancel := NewContext(parent, "scene-1", 30*time.Second)
	defer sceneCancel()

	apiCtx, apiCancel := WithAPITimeout(sceneCtx, "api-1", 5*time.Second)
	defer apiCancel()

	id, ok := APIID(apiCtx)
	assert.True(t, ok)
	assert.Equal(t, "api-1", id)

	deadline, ok := apiCtx.Deadline()
	assert.True(t, ok)
	assert.WithinDuration(t, time.Now().Add(5*time.Second), deadline, 100*time.Millisecond)
}

func TestAPITimeoutExceedsScene(t *testing.T) {
	parent := context.Background()

	sceneCtx, sceneCancel := NewContext(parent, "scene-1", 5*time.Second)
	defer sceneCancel()

	// API timeout exceeds scene; effective deadline capped by scene.
	apiCtx, apiCancel := WithAPITimeout(sceneCtx, "api-1", 30*time.Second)
	defer apiCancel()

	deadline, ok := apiCtx.Deadline()
	assert.True(t, ok)
	assert.WithinDuration(t, time.Now().Add(5*time.Second), deadline, 100*time.Millisecond)
}

func TestNoTimeout(t *testing.T) {
	parent := context.Background()

	ctx, cancel := NewContext(parent, "scene-1", 0)
	defer cancel()

	_, ok := ctx.Deadline()
	assert.False(t, ok, "zero timeout should not set a deadline")
}

func TestSceneIDMissing(t *testing.T) {
	_, ok := SceneID(context.Background())
	assert.False(t, ok)
}

func TestAPIIDMissing(t *testing.T) {
	_, ok := APIID(context.Background())
	assert.False(t, ok)
}

func TestChainTimeoutExpired(t *testing.T) {
	parent := context.Background()

	sceneCtx, sceneCancel := NewContext(parent, "scene-1", 50*time.Millisecond)
	defer sceneCancel()

	apiCtx, apiCancel := WithAPITimeout(sceneCtx, "api-1", 30*time.Millisecond)
	defer apiCancel()

	<-apiCtx.Done()
	assert.ErrorIs(t, apiCtx.Err(), context.DeadlineExceeded)
}

func TestChainTimeoutSceneCancelsAPI(t *testing.T) {
	parent := context.Background()

	sceneCtx, sceneCancel := NewContext(parent, "scene-1", 50*time.Millisecond)
	defer sceneCancel()

	apiCtx, apiCancel := WithAPITimeout(sceneCtx, "api-1", 30*time.Second)
	defer apiCancel()

	<-apiCtx.Done()
	assert.ErrorIs(t, apiCtx.Err(), context.DeadlineExceeded)
}

func TestChainKey(t *testing.T) {
	parent := context.Background()

	ctx, cancel := NewContext(parent, "scene-1", 10*time.Second)
	defer cancel()

	chain, ok := ChainKey(ctx)
	assert.True(t, ok)
	assert.Equal(t, "scene-1", chain)
}

func TestWithChainKey(t *testing.T) {
	parent := context.Background()

	ctx := WithChainKey(parent, "custom-chain")

	chain, ok := ChainKey(ctx)
	assert.True(t, ok)
	assert.Equal(t, "custom-chain", chain)
}

func TestChainKeyMissing(t *testing.T) {
	_, ok := ChainKey(context.Background())
	assert.False(t, ok)
}

func TestRemainingTimeout_WithDeadline(t *testing.T) {
	parent := context.Background()

	ctx, cancel := NewContext(parent, "scene-1", 10*time.Second)
	defer cancel()

	remaining := RemainingTimeout(ctx, 5*time.Second)
	assert.Greater(t, remaining, 9*time.Second)
	assert.LessOrEqual(t, remaining, 10*time.Second)
}

func TestRemainingTimeout_NoDeadline(t *testing.T) {
	parent := context.Background()

	ctx, cancel := NewContext(parent, "scene-1", 0)
	defer cancel()

	remaining := RemainingTimeout(ctx, 5*time.Second)
	assert.Equal(t, 5*time.Second, remaining)
}

func TestRemainingTimeout_Expired(t *testing.T) {
	parent := context.Background()

	ctx, cancel := NewContext(parent, "scene-1", 10*time.Millisecond)
	defer cancel()

	time.Sleep(20 * time.Millisecond)

	remaining := RemainingTimeout(ctx, 5*time.Second)
	assert.Equal(t, time.Duration(0), remaining)
}

func TestFormatTimeoutError(t *testing.T) {
	parent := context.Background()

	sceneCtx, sceneCancel := NewContext(parent, "scene-123", 10*time.Second)
	defer sceneCancel()

	apiCtx, apiCancel := WithAPITimeout(sceneCtx, "api-456", 5*time.Second)
	defer apiCancel()

	err := FormatTimeoutError(apiCtx, "HTTP request")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP request timed out")
	assert.Contains(t, err.Error(), "scene=scene-123")
	assert.Contains(t, err.Error(), "api=api-456")
}

func TestFormatTimeoutError_MissingContext(t *testing.T) {
	err := FormatTimeoutError(context.Background(), "operation")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "operation timed out")
	assert.Contains(t, err.Error(), "scene=")
	assert.Contains(t, err.Error(), "api=")
}

func TestNewContext_ParentAlreadyExpired(t *testing.T) {
	parent := context.Background()

	expiredCtx, expiredCancel := NewContext(parent, "expired", 10*time.Millisecond)
	defer expiredCancel()

	time.Sleep(20 * time.Millisecond)

	childCtx, childCancel := NewContext(expiredCtx, "child", 30*time.Second)
	defer childCancel()

	_, ok := childCtx.Deadline()
	assert.True(t, ok, "child context should have a deadline")

	select {
	case <-childCtx.Done():
		// Expected: child context should be done immediately
	case <-time.After(100 * time.Millisecond):
		t.Fatal("child context should have been cancelled immediately")
	}
}

func TestWithAPITimeout_ParentAlreadyExpired(t *testing.T) {
	parent := context.Background()

	expiredCtx, expiredCancel := NewContext(parent, "expired", 10*time.Millisecond)
	defer expiredCancel()

	time.Sleep(20 * time.Millisecond)

	apiCtx, apiCancel := WithAPITimeout(expiredCtx, "api-1", 30*time.Second)
	defer apiCancel()

	_, ok := apiCtx.Deadline()
	assert.True(t, ok, "api context should have a deadline")

	select {
	case <-apiCtx.Done():
		// Expected: api context should be done immediately
	case <-time.After(100 * time.Millisecond):
		t.Fatal("api context should have been cancelled immediately")
	}
}

func TestWithAPITimeout_ZeroTimeout(t *testing.T) {
	parent := context.Background()

	// 使用无 deadline 的父 context
	apiCtx, apiCancel := WithAPITimeout(parent, "api-1", 0)
	defer apiCancel()

	_, ok := apiCtx.Deadline()
	assert.False(t, ok, "zero timeout with no parent deadline should not set a deadline")

	id, ok := APIID(apiCtx)
	assert.True(t, ok)
	assert.Equal(t, "api-1", id)
}

func TestNewContext_NegativeTimeout(t *testing.T) {
	parent := context.Background()

	ctx, cancel := NewContext(parent, "scene-1", -5*time.Second)
	defer cancel()

	_, ok := ctx.Deadline()
	assert.False(t, ok, "negative timeout should not set a deadline")
}
