package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/core/dag"
)

// mockSubFlowRunner creates a mock sub-flow runner that returns a simple output.
// If failSceneID is set, that scene ID returns an error.
func mockSubFlowRunner(t *testing.T, failSceneID string) func(ctx context.Context, sceneID string, variables map[string]any) (*dag.Output, error) {
	t.Helper()
	return func(ctx context.Context, sceneID string, variables map[string]any) (*dag.Output, error) {
		if sceneID == failSceneID {
			return nil, fmt.Errorf("scene not found: %s", sceneID)
		}

		// Check context cancellation.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Return variables merged with a sub-flow specific key.
		merged := make(map[string]any)
		for k, v := range variables {
			merged[k] = v
		}
		merged["subToken"] = "sub-value-" + sceneID
		merged["scene_id"] = sceneID

		return &dag.Output{
			Response: map[string]any{
				"node_id":     "sub-flow-node",
				"type":        "sub_flow",
				"merged_vars": merged,
			},
		}, nil
	}
}

func TestExecuteSubFlow_Sync(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := subFlowConfig{SceneID: "scene-123", Async: false}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)
	node.subFlowRunner = mockSubFlowRunner(t, "")

	output, err := node.executeSubFlow(ctx, &dag.Input{}, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "sub-flow-node", resp["node_id"])
	assert.Equal(t, "sub_flow", resp["type"])
}

func TestExecuteSubFlow_Async(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := subFlowConfig{SceneID: "scene-456", Async: true}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)
	node.subFlowRunner = mockSubFlowRunner(t, "")

	output, err := node.executeSubFlow(ctx, &dag.Input{}, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "sub_flow", resp["type"])
	assert.Equal(t, "scene-456", resp["scene_id"])
	assert.Equal(t, true, resp["async"])
}

func TestExecuteSubFlow_SyncVariableMerge(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := subFlowConfig{SceneID: "scene-789", Async: false}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)
	node.subFlowRunner = mockSubFlowRunner(t, "")

	input := &dag.Input{
		Variables: map[string]any{
			"parentVar": "parent-value",
		},
	}

	output, err := node.executeSubFlow(ctx, input, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)

	merged, hasVars := resp["merged_vars"].(map[string]any)
	require.True(t, hasVars, "merged_vars should be present")
	assert.Equal(t, "parent-value", merged["parentVar"], "parent variable should pass through")
	assert.Equal(t, "sub-value-scene-789", merged["subToken"], "sub-flow variable should be merged")
}

func TestExecuteSubFlow_SceneNotFound(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := subFlowConfig{SceneID: "nonexistent-scene", Async: false}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)
	node.subFlowRunner = mockSubFlowRunner(t, "nonexistent-scene")

	_, err = node.executeSubFlow(ctx, &dag.Input{}, node.log)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scene not found")
}

func TestExecuteSubFlow_EmptySceneID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := subFlowConfig{SceneID: "", Async: false}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)
	node.subFlowRunner = mockSubFlowRunner(t, "")

	_, err = node.executeSubFlow(ctx, &dag.Input{}, node.log)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scene_id is required")
}

func TestExecuteSubFlow_NilRunner(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := subFlowConfig{SceneID: "scene-111", Async: false}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)
	// subFlowRunner is nil (not set)

	_, err = node.executeSubFlow(ctx, &dag.Input{}, node.log)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sub-flow runner not available")
}

func TestExecuteSubFlow_DepthLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set depth to 4 (5 levels total) - should work.
	ctx4 := context.WithValue(ctx, subFlowDepthKey, 4)
	cfg := subFlowConfig{SceneID: "scene-depth-5", Async: false}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)
	node.subFlowRunner = mockSubFlowRunner(t, "")

	output, err := node.executeSubFlow(ctx4, &dag.Input{}, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	// Set depth to 5 (6 levels total - exceeds max) - should error.
	ctx5 := context.WithValue(ctx, subFlowDepthKey, 5)
	_, err = node.executeSubFlow(ctx5, &dag.Input{}, node.log)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "depth limit exceeded")
}

func TestExecuteSubFlow_CycleDetection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Visited list already contains the scene we're trying to call.
	visited := []string{"scene-A", "scene-B", "scene-A"}
	ctxWithVisited := context.WithValue(ctx, subFlowVisitedKey, visited)

	cfg := subFlowConfig{SceneID: "scene-A", Async: false}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)
	node.subFlowRunner = mockSubFlowRunner(t, "")

	_, err = node.executeSubFlow(ctxWithVisited, &dag.Input{}, node.log)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular sub-flow reference")
	assert.Contains(t, err.Error(), "scene-A")
}

func TestExecuteSubFlow_DepthAndVisitedCarryForward(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var callDepth int32
	var callVisited []string

	runner := func(ctx context.Context, sceneID string, variables map[string]any) (*dag.Output, error) {
		d, _ := ctx.Value(subFlowDepthKey).(int)
		v, _ := ctx.Value(subFlowVisitedKey).([]string)
		atomic.StoreInt32(&callDepth, int32(d))
		callVisited = v
		return &dag.Output{Response: map[string]any{"scene_id": sceneID}}, nil
	}

	// Start with depth=2 and already visited "scene-A"
	ctxSetup := context.WithValue(ctx, subFlowDepthKey, 2)
	ctxSetup = context.WithValue(ctxSetup, subFlowVisitedKey, []string{"scene-A"})

	cfg := subFlowConfig{SceneID: "scene-B", Async: false}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)
	node.subFlowRunner = runner

	_, err = node.executeSubFlow(ctxSetup, &dag.Input{}, node.log)
	require.NoError(t, err)

	assert.Equal(t, int32(3), atomic.LoadInt32(&callDepth), "depth should be incremented to 3")
	assert.Contains(t, callVisited, "scene-A", "visited should contain original scene")
	assert.Contains(t, callVisited, "scene-B", "visited should contain new scene")
	assert.Len(t, callVisited, 2, "visited should have exactly 2 entries")
}

func TestExecuteSubFlow_ContextCancelled(t *testing.T) {
	// Use already-cancelled context.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := subFlowConfig{SceneID: "scene-cancel", Async: false}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)
	node.subFlowRunner = mockSubFlowRunner(t, "")

	_, err = node.executeSubFlow(ctx, &dag.Input{}, node.log)
	require.Error(t, err)
}

func TestExecuteSubFlow_NilInput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := subFlowConfig{SceneID: "scene-nil-input", Async: false}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)
	node.subFlowRunner = mockSubFlowRunner(t, "")

	output, err := node.executeSubFlow(ctx, nil, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)
}

func TestExecuteSubFlow_AsyncDoesNotBlock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	slowRunner := func(ctx context.Context, sceneID string, variables map[string]any) (*dag.Output, error) {
		// Simulate slow sub-scene execution.
		time.Sleep(200 * time.Millisecond)
		return &dag.Output{Response: map[string]any{"scene_id": sceneID}}, nil
	}

	cfg := subFlowConfig{SceneID: "scene-slow", Async: true}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)
	node.subFlowRunner = slowRunner

	start := time.Now()
	output, err := node.executeSubFlow(ctx, &dag.Input{}, node.log)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, output)
	assert.True(t, elapsed < 100*time.Millisecond, "async sub-flow should return immediately, took %v", elapsed)

	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, resp["async"])
}
