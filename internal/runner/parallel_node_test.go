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

func TestExecuteParallel_AllSuccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var count atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	cfg := struct {
		Steps []stepConfig `json:"steps"`
	}{
		Steps: []stepConfig{
			{Name: "step1", Request: &stepRequestConfig{Method: "GET", URL: server.URL + "/a"}},
			{Name: "step2", Request: &stepRequestConfig{Method: "GET", URL: server.URL + "/b"}},
			{Name: "step3", Request: &stepRequestConfig{Method: "GET", URL: server.URL + "/c"}},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	output, err := node.executeParallel(ctx, &dag.Input{}, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	assert.Equal(t, int32(3), count.Load(), "all 3 steps should execute")
}

func TestExecuteParallel_EmptySteps(t *testing.T) {
	cfg := struct {
		Steps []stepConfig `json:"steps"`
	}{
		Steps: []stepConfig{},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	output, err := node.executeParallel(context.Background(), &dag.Input{}, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "test-while-node", resp["node_id"])
	assert.Equal(t, "parallel", resp["type"])
}

func TestExecuteParallel_PartialFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var count atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := count.Add(1)
		if c == 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	cfg := struct {
		Steps []stepConfig `json:"steps"`
	}{
		Steps: []stepConfig{
			{Name: "step1", Request: &stepRequestConfig{Method: "GET", URL: server.URL + "/a"}},
			{Name: "step2", Request: &stepRequestConfig{Method: "GET", URL: server.URL + "/b"}},
			{Name: "step3", Request: &stepRequestConfig{Method: "GET", URL: server.URL + "/c"}},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	_, err = node.executeParallel(ctx, &dag.Input{}, node.log)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed", "error should indicate which step failed")

	// All steps should still execute even if one fails.
	assert.Equal(t, int32(3), count.Load(), "all 3 steps should execute even if one fails")
}

func TestExecuteParallel_VariableIsolation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"value":"step_result"}`))
	}))
	defer server.Close()

	cfg := struct {
		Steps []stepConfig `json:"steps"`
	}{
		Steps: []stepConfig{
			{
				Name: "stepA",
				Request: &stepRequestConfig{Method: "GET", URL: server.URL + "/a"},
				Extract: []extractEntry{{Variable: "varA", Path: "$.value"}},
			},
			{
				Name: "stepB",
				Request: &stepRequestConfig{Method: "GET", URL: server.URL + "/b"},
				Extract: []extractEntry{{Variable: "varB", Path: "$.value"}},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	output, err := node.executeParallel(ctx, &dag.Input{}, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)

	// Both variables should be available in merged output.
	merged, hasVars := resp["merged_vars"].(map[string]any)
	require.True(t, hasVars, "merged_vars should be present")
	assert.Equal(t, "step_result", merged["varA"])
	assert.Equal(t, "step_result", merged["varB"])
}

func TestExecuteParallel_VariableConflict(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"token":"token-A"}`))
	}))
	defer serverA.Close()

	serverB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"token":"token-B"}`))
	}))
	defer serverB.Close()

	cfg := struct {
		Steps []stepConfig `json:"steps"`
	}{
		Steps: []stepConfig{
			{
				Name: "stepA",
				Request: &stepRequestConfig{Method: "GET", URL: serverA.URL},
				Extract: []extractEntry{{Variable: "token", Path: "$.token"}},
			},
			{
				Name: "stepB",
				Request: &stepRequestConfig{Method: "GET", URL: serverB.URL},
				Extract: []extractEntry{{Variable: "token", Path: "$.token"}},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	output, err := node.executeParallel(ctx, &dag.Input{}, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)

	merged, hasVars := resp["merged_vars"].(map[string]any)
	require.True(t, hasVars, "merged_vars should be present")

	// Last-write-wins: token should be either token-A or token-B (non-deterministic).
	token, ok := merged["token"].(string)
	require.True(t, ok)
	assert.Contains(t, []string{"token-A", "token-B"}, token)
}

func TestExecuteParallel_StepCondition(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var executed atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		executed.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	cfg := struct {
		Steps []stepConfig `json:"steps"`
	}{
		Steps: []stepConfig{
			{
				Name:      "skipped step",
				Condition: &stepConditionConfig{Variable: "nonExistent", Operator: "equals", Value: "1"},
				Request:   &stepRequestConfig{Method: "GET", URL: server.URL},
			},
			{
				Name:    "active step",
				Request: &stepRequestConfig{Method: "GET", URL: server.URL},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	output, err := node.executeParallel(ctx, &dag.Input{}, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	// Only 1 step should execute (the conditionally skipped one is not executed).
	assert.Equal(t, int32(1), executed.Load(), "only 1 step should execute")
}

func TestExecuteParallel_ConcurrentSafety(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond) // Ensure concurrent execution
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	// Create 50 parallel steps to test concurrent safety.
	steps := make([]stepConfig, 50)
	for i := range steps {
		steps[i] = stepConfig{
			Name:    "step",
			Request: &stepRequestConfig{Method: "GET", URL: server.URL},
		}
	}

	cfg := struct {
		Steps []stepConfig `json:"steps"`
	}{Steps: steps}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	output, err := node.executeParallel(ctx, &dag.Input{}, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)
}

func TestExecuteParallel_NilInput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := struct {
		Steps []stepConfig `json:"steps"`
	}{
		Steps: []stepConfig{
			{Name: "noop"},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	output, err := node.executeParallel(ctx, nil, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)
}

func TestExecuteParallel_ContextCancelled(t *testing.T) {
	// Use already-cancelled context.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := struct {
		Steps []stepConfig `json:"steps"`
	}{
		Steps: []stepConfig{
			{Name: "step", Request: &stepRequestConfig{Method: "GET", URL: "http://localhost:1"}},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	_, err = node.executeParallel(ctx, &dag.Input{}, node.log)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

func TestExecuteParallel_VariableMergeWithInitialVars(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":"extracted"}`))
	}))
	defer server.Close()

	cfg := struct {
		Steps []stepConfig `json:"steps"`
	}{
		Steps: []stepConfig{
			{
				Name:    "extract step",
				Request: &stepRequestConfig{Method: "GET", URL: server.URL},
				Extract: []extractEntry{{Variable: "newVar", Path: "$.result"}},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	input := &dag.Input{
		Variables: map[string]any{
			"initialKey": "initialValue",
		},
	}

	output, err := node.executeParallel(ctx, input, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)

	merged, hasVars := resp["merged_vars"].(map[string]any)
	require.True(t, hasVars, "merged_vars should be present")
	assert.Equal(t, "initialValue", merged["initialKey"])
	assert.Equal(t, "extracted", merged["newVar"])
}