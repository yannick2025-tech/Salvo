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
