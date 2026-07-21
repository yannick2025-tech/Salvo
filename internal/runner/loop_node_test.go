package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/core/dag"
)

func TestExecuteLoop_FixedCount(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var count atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	cfg := loopConfig{
		LoopCount: 3,
		Steps: []stepConfig{
			{Name: "step1", Request: &stepRequestConfig{Method: "GET", URL: server.URL}},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	output, err := node.executeLoop(ctx, &dag.Input{}, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	assert.Equal(t, int32(3), count.Load(), "step should execute 3 times")

	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "test-while-node", resp["node_id"])
	assert.Equal(t, "loop", resp["type"])
	assert.Equal(t, 3, resp["iterations"])
}

func TestExecuteLoop_CountZero(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := loopConfig{
		LoopCount: 0,
		Steps: []stepConfig{
			{Name: "step1", Request: &stepRequestConfig{Method: "GET", URL: server.URL}},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	output, err := node.executeLoop(ctx, &dag.Input{}, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 0, resp["iterations"], "zero iterations with loop_count=0")
}

func TestExecuteLoop_CountOne(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var count atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	cfg := loopConfig{
		LoopCount: 1,
		Steps: []stepConfig{
			{Name: "step1", Request: &stepRequestConfig{Method: "GET", URL: server.URL}},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	output, err := node.executeLoop(ctx, &dag.Input{}, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	assert.Equal(t, int32(1), count.Load())
	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 1, resp["iterations"])
}

func TestExecuteLoop_VariableAccumulation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Server returns increasing values based on URL path.
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		resp := map[string]int{"iteration": callCount}
		data, _ := json.Marshal(resp)
		_, _ = w.Write(data)
	}))
	defer server.Close()

	cfg := loopConfig{
		LoopCount: 3,
		Steps: []stepConfig{
			{
				Name:    "accumulate",
				Request: &stepRequestConfig{Method: "GET", URL: server.URL},
				Extract: []extractEntry{{Variable: "iterationNum", Path: "$.iteration"}},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	output, err := node.executeLoop(ctx, &dag.Input{}, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)

	// Final iterationNum should be 3 (last write wins).
	vars, hasVars := resp["merged_vars"].(map[string]any)
	require.True(t, hasVars, "merged_vars should be present")
	assert.Equal(t, float64(3), vars["iterationNum"], "final variable value should be 3")
}

func TestExecuteLoop_EmptySteps(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// loop_count=3 but no steps.
	cfg := loopConfig{
		LoopCount: 3,
		Steps:     []stepConfig{},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	output, err := node.executeLoop(ctx, &dag.Input{}, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 0, resp["iterations"], "no iterations with empty steps")
}

func TestExecuteLoop_ContextCancelled(t *testing.T) {
	// Use already-cancelled context.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := loopConfig{
		LoopCount: 5,
		Steps: []stepConfig{
			{Name: "step1", Request: &stepRequestConfig{Method: "GET", URL: "http://localhost:1"}},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	_, err = node.executeLoop(ctx, &dag.Input{}, node.log)
	require.Error(t, err)
}

func TestExecuteLoop_StepCondition(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var executed atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		executed.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	cfg := loopConfig{
		LoopCount: 3,
		Steps: []stepConfig{
			{
				Name:      "skipped",
				Condition: &stepConditionConfig{Variable: "nonExistent", Operator: "equals", Value: "1"},
				Request:   &stepRequestConfig{Method: "GET", URL: server.URL},
			},
			{
				Name:    "active",
				Request: &stepRequestConfig{Method: "GET", URL: server.URL},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	output, err := node.executeLoop(ctx, &dag.Input{}, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	// Only active step runs each iteration (3 times).
	assert.Equal(t, int32(3), executed.Load(), "only active step should execute 3 times")
}

func TestExecuteLoop_NilInput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := loopConfig{
		LoopCount: 2,
		Steps: []stepConfig{
			{Name: "noop"},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	output, err := node.executeLoop(ctx, nil, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	resps, ok := output.Response.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 2, resps["iterations"], "loop should still execute 2 iterations even with requestless steps")
}

func TestExecuteLoop_ThinkTime(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	cfg := loopConfig{
		LoopCount: 2,
		Steps: []stepConfig{
			{
				Name:    "slow",
				Request: &stepRequestConfig{Method: "GET", URL: server.URL},
				ThinkTime: &thinkTimeConfig{
					Min: 50,
					Max: 100,
				},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	start := time.Now()
	output, err := node.executeLoop(ctx, &dag.Input{}, node.log)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, output)

	// 2 iterations * at least 50ms think time = at least 100ms.
	assert.True(t, elapsed >= 50*time.Millisecond, "should take at least 50ms with think time, took %v", elapsed)

	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 2, resp["iterations"])
}

func TestExecuteLoop_SuccessRequestsCounted(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	cfg := loopConfig{
		LoopCount: 3,
		Steps: []stepConfig{
			{Name: "step1", Request: &stepRequestConfig{Method: "GET", URL: server.URL}},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := &sceneNode{
		id:            "test-loop-node",
		log:           newTestLogger(),
		stats:         &Stats{},
		httpOnlyStats: &Stats{},
		nodeStats:     NewNodeStats(10000),
	}
	node.config = string(cfgBytes)

	_, err = node.executeLoop(ctx, &dag.Input{}, node.log)
	require.NoError(t, err)

	// 3 successful HTTP requests should be counted in all 3 stats objects.
	assert.Equal(t, int64(3), node.stats.TotalReqs.Load(), "global stats should count 3 total requests")
	assert.Equal(t, int64(3), node.stats.SuccessReqs.Load(), "global stats should count 3 successes")
	assert.Equal(t, int64(0), node.stats.FailedReqs.Load(), "global stats should count 0 failures")

	assert.Equal(t, int64(3), node.httpOnlyStats.TotalReqs.Load(), "httpOnlyStats should count 3 total requests")
	assert.Equal(t, int64(3), node.httpOnlyStats.SuccessReqs.Load(), "httpOnlyStats should count 3 successes")

	nodeSnap := node.nodeStats.Snapshot()
	assert.Equal(t, int64(3), nodeSnap.TotalReqs, "nodeStats should count 3 total requests")
	assert.Equal(t, int64(3), nodeSnap.SuccessReqs, "nodeStats should count 3 successes")
}

func TestExecuteLoop_FailedRequestsCounted(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Server always returns 500.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := loopConfig{
		LoopCount: 2,
		Steps: []stepConfig{
			{Name: "fail-step", Request: &stepRequestConfig{Method: "GET", URL: server.URL}},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := &sceneNode{
		id:            "test-loop-node",
		log:           newTestLogger(),
		stats:         &Stats{},
		httpOnlyStats: &Stats{},
		nodeStats:     NewNodeStats(10000),
	}
	node.config = string(cfgBytes)

	// Loop does NOT stop on HTTP 500 (unlike WHILE), it continues.
	output, err := node.executeLoop(ctx, &dag.Input{}, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	// 2 failed HTTP requests (2 iterations * 1 step each) should be counted.
	assert.Equal(t, int64(2), node.stats.TotalReqs.Load(), "global stats should count 2 total requests")
	assert.Equal(t, int64(2), node.stats.FailedReqs.Load(), "global stats should count 2 failures")
	assert.Equal(t, int64(0), node.stats.SuccessReqs.Load(), "global stats should count 0 successes")

	assert.Equal(t, int64(2), node.httpOnlyStats.TotalReqs.Load(), "httpOnlyStats should count 2 total requests")
	assert.Equal(t, int64(2), node.httpOnlyStats.FailedReqs.Load(), "httpOnlyStats should count 2 failures")

	nodeSnap := node.nodeStats.Snapshot()
	assert.Equal(t, int64(2), nodeSnap.TotalReqs, "nodeStats should count 2 total requests")
	assert.Equal(t, int64(2), nodeSnap.FailedReqs, "nodeStats should count 2 failures")
}