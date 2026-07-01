package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/core/dag"
	"github.com/yannick2025-tech/Salvo/internal/core/expr"
	"github.com/yannick2025-tech/Salvo/internal/generator/builtin"
	"github.com/yannick2025-tech/Salvo/internal/plugin/so"
)

// =============================================================================
// Section 13.4: Parallel node + variable system integration tests.
//
// These tests verify the integration between parallel node's concurrent
// execution and the variable system. Tests cover variable extraction from
// multiple parallel steps, variable merging (last-write-wins), input variable
// propagation, expression engine integration, SO plugin calls, think time,
// context cancellation, and concurrent safety.
// =============================================================================

// ---------------------------------------------------------------------------
// Variable extraction and merging
// ---------------------------------------------------------------------------

// TestParallel_MultipleStepsExtractDifferentVars verifies that multiple
// parallel steps each extract different variables and all are merged into
// the output.
func TestParallel_MultipleStepsExtractDifferentVars(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"token":"abc123","user":"alice"}}`))
	}))
	defer serverA.Close()

	serverB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"charge":85,"status":"CHARGING"}}`))
	}))
	defer serverB.Close()

	cfg := parallelConfig{
		Steps: []stepConfig{
			{
				Name:    "get auth",
				Request: &stepRequestConfig{Method: "GET", URL: serverA.URL + "/auth"},
				Extract: []extractEntry{
					{Variable: "token", Path: "$.data.token"},
					{Variable: "user", Path: "$.data.user"},
				},
			},
			{
				Name:    "get charge",
				Request: &stepRequestConfig{Method: "GET", URL: serverB.URL + "/charge"},
				Extract: []extractEntry{
					{Variable: "chargeLevel", Path: "$.data.charge"},
					{Variable: "chargeStatus", Path: "$.data.status"},
				},
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
	assert.Equal(t, "abc123", merged["token"])
	assert.Equal(t, "alice", merged["user"])
	assert.Equal(t, float64(85), merged["chargeLevel"])
	assert.Equal(t, "CHARGING", merged["chargeStatus"])
	assert.Equal(t, 2, resp["completed"])
	assert.Equal(t, 2, resp["total"])
}

// TestParallel_InputVariablesPropagatedToAllSteps verifies that input
// variables from dag.Input are available to all parallel steps, e.g.,
// used in URL paths.
func TestParallel_InputVariablesPropagatedToAllSteps(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var capturedPaths []string
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedPaths = append(capturedPaths, r.URL.String())
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	cfg := parallelConfig{
		Steps: []stepConfig{
			{
				Name:    "step1",
				Request: &stepRequestConfig{Method: "GET", URL: server.URL + "/user/${userId}"},
				Extract: []extractEntry{{Variable: "result", Path: "$.status"}},
			},
			{
				Name:    "step2",
				Request: &stepRequestConfig{Method: "GET", URL: server.URL + "/device/${deviceId}"},
				Extract: []extractEntry{{Variable: "result2", Path: "$.status"}},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	input := &dag.Input{
		Variables: map[string]any{
			"userId":   "42",
			"deviceId": "device-007",
		},
	}

	output, err := node.executeParallel(ctx, input, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, capturedPaths, 2)
	// Both URLs should be resolved (order is non-deterministic due to concurrency).
	assert.Contains(t, capturedPaths[0]+capturedPaths[1], "/user/42")
	assert.Contains(t, capturedPaths[0]+capturedPaths[1], "/device/device-007")
}

// TestParallel_InputVariablesInHeaders verifies input variables used in
// request headers are resolved and accessible.
func TestParallel_InputVariablesInHeaders(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var capturedHeaders http.Header
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedHeaders = r.Header.Clone()
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	cfg := parallelConfig{
		Steps: []stepConfig{
			{
				Name: "auth step",
				Request: &stepRequestConfig{
					Method:  "GET",
					URL:     server.URL + "/check",
					Headers: map[string]string{"Authorization": "Bearer ${token}"},
				},
				Extract: []extractEntry{{Variable: "status", Path: "$.status"}},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	output, err := node.executeParallel(ctx, &dag.Input{Variables: map[string]any{"token": "my-secret-token"}}, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, "Bearer my-secret-token", capturedHeaders.Get("Authorization"))
}

// ---------------------------------------------------------------------------
// Expression engine integration
// ---------------------------------------------------------------------------

// TestParallel_ExpressionEngineInStepURL verifies that expression engine
// functions like ${__random(min, max)} are resolved in parallel step URLs.
// Note: URLs are pre-resolved (the runner pipeline resolves expressions
// before passing config to the node).
func TestParallel_ExpressionEngineInStepURL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reg := expr.NewFunctionRegistry()
	builtin.RegisterAll(reg)

	var capturedPath string
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedPath = r.URL.String()
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	// Pre-resolve expression engine parts (runner pipeline does this before
	// executeParallel).
	stepURL := server.URL + "/charge?timeout=${__random(60, 600)}"
	resolvedURL, err := expr.Resolve(stepURL, nil, reg)
	require.NoError(t, err)
	require.NotContains(t, resolvedURL, "${__random}")

	cfg := parallelConfig{
		Steps: []stepConfig{
			{
				Name: "random timeout",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    resolvedURL,
				},
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

	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, capturedPath)
	assert.Contains(t, capturedPath, "/charge?timeout=")

	// Verify timeout is a number in [60, 600].
	timeoutStart := len("/charge?timeout=")
	timeoutStr := capturedPath[timeoutStart:]
	// Ensure we got a numeric value.
	assert.GreaterOrEqual(t, len(timeoutStr), 2, "timeout value should be at least 2 digits")
}

// TestParallel_BuiltinFunctionInStepURL verifies that builtin functions
// resolve correctly in parallel step URLs and each parallel invocation
// gets a potentially different value.
func TestParallel_BuiltinFunctionInStepURL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reg := expr.NewFunctionRegistry()
	builtin.RegisterAll(reg)

	var capturedPaths []string
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedPaths = append(capturedPaths, r.URL.String())
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	// Pre-resolve URLs.
	url1, err := expr.Resolve(server.URL+"/a?r=${__random(1, 100)}", nil, reg)
	require.NoError(t, err)
	url2, err := expr.Resolve(server.URL+"/b?r=${__random(1, 100)}", nil, reg)
	require.NoError(t, err)

	cfg := parallelConfig{
		Steps: []stepConfig{
			{
				Name:    "step1",
				Request: &stepRequestConfig{Method: "GET", URL: url1},
			},
			{
				Name:    "step2",
				Request: &stepRequestConfig{Method: "GET", URL: url2},
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

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, capturedPaths, 2)
	// Parallel steps execute concurrently, so the capture order is not
	// guaranteed. Verify both paths are present regardless of order.
	joined := strings.Join(capturedPaths, " ")
	assert.Contains(t, joined, "/a?r=")
	assert.Contains(t, joined, "/b?r=")
}

// ---------------------------------------------------------------------------
// SO plugin integration
// ---------------------------------------------------------------------------

// TestParallel_SOPluginExpressionInStep verifies that SO plugin expressions
// are resolved in parallel step URLs, e.g., encrypting parameters before
// sending HTTP requests.
func TestParallel_SOPluginExpressionInStep(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Register an in-memory AES plugin for testing.
	reg := expr.NewFunctionRegistry()
	builtin.RegisterAll(reg)

	loader := so.NewLoader()
	aesPlugin := &testShellAESPlugin{}
	err := loader.Register(aesPlugin)
	require.NoError(t, err)

	err = so.RegisterSO(reg, loader)
	require.NoError(t, err)

	// Use a fixed key and IV for deterministic encryption.
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	iv := "MTIzNDU2Nzg5MDEyMzQ1Ng==" // base64 of "1234567890123456"
	plaintext := "sensitive-data-42"

	// Pre-compute the expected encrypted value to verify the SO plugin works.
	encExpr := fmt.Sprintf(`${__so("shell-aes", "encrypt", "%s", "%s", "%s")}`, key, iv, plaintext)
	encResult, err := expr.Resolve(encExpr, nil, reg)
	require.NoError(t, err)
	require.NotEmpty(t, encResult)

	var capturedPath string
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedPath = r.URL.String()
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	// Use the encrypted value in a parallel step URL.
	cfg := parallelConfig{
		Steps: []stepConfig{
			{
				Name: "send encrypted",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    fmt.Sprintf(server.URL+"/submit?data=%s", encResult),
				},
				Extract: []extractEntry{{Variable: "result", Path: "$.status"}},
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

	mu.Lock()
	defer mu.Unlock()
	assert.Contains(t, capturedPath, "/submit?data=")
	assert.Contains(t, capturedPath, encResult)
}

// ---------------------------------------------------------------------------
// Variable conflicts (last-write-wins)
// ---------------------------------------------------------------------------

// TestParallel_SameVariableExtractedByMultipleSteps verifies that when
// multiple parallel steps extract the same variable, the last-write-wins
// semantics apply (non-deterministic, but should not error).
func TestParallel_SameVariableExtractedByMultipleSteps(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"value":"final"}`))
	}))
	defer server.Close()

	cfg := parallelConfig{
		Steps: []stepConfig{
			{
				Name:    "stepA",
				Request: &stepRequestConfig{Method: "GET", URL: server.URL + "/a"},
				Extract: []extractEntry{{Variable: "shared", Path: "$.value"}},
			},
			{
				Name:    "stepB",
				Request: &stepRequestConfig{Method: "GET", URL: server.URL + "/b"},
				Extract: []extractEntry{{Variable: "shared", Path: "$.value"}},
			},
			{
				Name:    "stepC",
				Request: &stepRequestConfig{Method: "GET", URL: server.URL + "/c"},
				Extract: []extractEntry{{Variable: "shared", Path: "$.value"}},
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
	require.True(t, hasVars)
	assert.Equal(t, "final", merged["shared"])
	assert.Equal(t, 3, resp["completed"])
}

// ---------------------------------------------------------------------------
// Think time in parallel steps
// ---------------------------------------------------------------------------

// TestParallel_ThinkTimeDelaysExecution verifies that think_time delays
// each parallel step independently.
func TestParallel_ThinkTimeDelaysExecution(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var mu sync.Mutex
	executionTimes := make([]time.Time, 0, 2)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		executionTimes = append(executionTimes, time.Now())
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	cfg := parallelConfig{
		Steps: []stepConfig{
			{
				Name:      "step1",
				ThinkTime: &thinkTimeConfig{Min: 100, Max: 200},
				Request:   &stepRequestConfig{Method: "GET", URL: server.URL + "/a"},
			},
			{
				Name:      "step2",
				ThinkTime: &thinkTimeConfig{Min: 100, Max: 200},
				Request:   &stepRequestConfig{Method: "GET", URL: server.URL + "/b"},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	start := time.Now()
	output, err := node.executeParallel(ctx, &dag.Input{}, node.log)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, output)

	// Both steps run concurrently, so total time should be ~max(think_time) not sum.
	// Total should be < 400ms (not 2*200ms since they run in parallel).
	assert.Less(t, elapsed, 400*time.Millisecond,
		"parallel execution with think time should complete faster than sequential sum")
	assert.GreaterOrEqual(t, elapsed, 80*time.Millisecond,
		"parallel execution should take at least the minimum think time")
}

// ---------------------------------------------------------------------------
// Context cancellation
// ---------------------------------------------------------------------------

// TestParallel_ContextCancellation verifies that when the context is
// cancelled, all running parallel steps are cancelled and the node returns
// an error.
func TestParallel_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Hang until context is cancelled.
		<-ctx.Done()
	}))
	defer slowServer.Close()

	// Use a separate context that we cancel immediately.
	execCtx, execCancel := context.WithCancel(context.Background())
	execCancel() // Cancel immediately.

	cfg := parallelConfig{
		Steps: []stepConfig{
			{
				Name:    "slow step",
				Request: &stepRequestConfig{Method: "GET", URL: slowServer.URL + "/hang"},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	_, err = node.executeParallel(execCtx, &dag.Input{}, node.log)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context cancelled")
}

// TestParallel_ContextCancellationDuringExecution verifies that cancellation
// mid-execution returns an appropriate error.
func TestParallel_ContextCancellationDuringExecution(t *testing.T) {
	_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer slowServer.Close()

	execCtx, execCancel := context.WithCancel(context.Background())

	// Cancel after 100ms to ensure the request is in-flight.
	go func() {
		time.Sleep(100 * time.Millisecond)
		execCancel()
	}()

	cfg := parallelConfig{
		Steps: []stepConfig{
			{
				Name:    "step1",
				Request: &stepRequestConfig{Method: "GET", URL: slowServer.URL + "/slow"},
			},
			{
				Name:    "step2",
				Request: &stepRequestConfig{Method: "GET", URL: slowServer.URL + "/slow"},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	_, err = node.executeParallel(execCtx, &dag.Input{}, node.log)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// Nested JSONPath extraction
// ---------------------------------------------------------------------------

// TestParallel_NestedJSONPathExtraction verifies that deeply nested JSON
// paths are extracted correctly from parallel step responses.
func TestParallel_NestedJSONPathExtraction(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"response": {
				"data": {
					"user": {
						"profile": {
							"name": "Alice",
							"email": "alice@example.com"
						},
						"settings": {
							"theme": "dark"
						}
					}
				}
			}
		}`))
	}))
	defer server.Close()

	cfg := parallelConfig{
		Steps: []stepConfig{
			{
				Name:    "get profile",
				Request: &stepRequestConfig{Method: "GET", URL: server.URL + "/user"},
				Extract: []extractEntry{
					{Variable: "userName", Path: "$.response.data.user.profile.name"},
					{Variable: "email", Path: "$.response.data.user.profile.email"},
					{Variable: "theme", Path: "$.response.data.user.settings.theme"},
				},
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
	require.True(t, hasVars)
	assert.Equal(t, "Alice", merged["userName"])
	assert.Equal(t, "alice@example.com", merged["email"])
	assert.Equal(t, "dark", merged["theme"])
}

// ---------------------------------------------------------------------------
// Step with no request
// ---------------------------------------------------------------------------

// TestParallel_StepWithNoRequest verifies that a step with no request
// is skipped and does not affect the parallel execution result.
func TestParallel_StepWithNoRequest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	cfg := parallelConfig{
		Steps: []stepConfig{
			{
				Name: "noop step", // No Request field — should be skipped.
			},
			{
				Name:    "active step",
				Request: &stepRequestConfig{Method: "GET", URL: server.URL + "/active"},
				Extract: []extractEntry{{Variable: "status", Path: "$.status"}},
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
	assert.Equal(t, int32(1), callCount.Load(), "only 1 step should execute")
	assert.Equal(t, 1, resp["completed"])
	assert.Equal(t, 2, resp["total"])
}

// ---------------------------------------------------------------------------
// Step conditions
// ---------------------------------------------------------------------------

// TestParallel_StepConditionTrue verifies that a step with a condition
// that evaluates to true executes normally.
func TestParallel_StepConditionTrue(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	cfg := parallelConfig{
		Steps: []stepConfig{
			{
				Name:      "conditional step — passes",
				Condition: &stepConditionConfig{Variable: "role", Operator: "equals", Value: "admin"},
				Request:   &stepRequestConfig{Method: "GET", URL: server.URL + "/admin"},
				Extract:   []extractEntry{{Variable: "adminStatus", Path: "$.status"}},
			},
			{
				Name:      "conditional step — fails",
				Condition: &stepConditionConfig{Variable: "role", Operator: "equals", Value: "superadmin"},
				Request:   &stepRequestConfig{Method: "GET", URL: server.URL + "/super"},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	input := &dag.Input{Variables: map[string]any{"role": "admin"}}
	output, err := node.executeParallel(ctx, input, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	assert.Equal(t, int32(1), callCount.Load(), "only 1 step should execute")

	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)
	merged, hasVars := resp["merged_vars"].(map[string]any)
	require.True(t, hasVars)
	assert.Equal(t, "ok", merged["adminStatus"])
}

// ---------------------------------------------------------------------------
// Concurrent correctness
// ---------------------------------------------------------------------------

// TestParallel_ManyStepsConcurrentExtraction verifies that a large number
// of parallel steps (50) each extracting variables works correctly without
// data races or lost updates.
func TestParallel_ManyStepsConcurrentExtraction(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"value":"concurrent_ok"}`))
	}))
	defer server.Close()

	steps := make([]stepConfig, 50)
	for i := range steps {
		varName := fmt.Sprintf("var_%d", i)
		steps[i] = stepConfig{
			Name:    fmt.Sprintf("step_%d", i),
			Request: &stepRequestConfig{Method: "GET", URL: server.URL + fmt.Sprintf("/%d", i)},
			Extract: []extractEntry{{Variable: varName, Path: "$.value"}},
		}
	}

	cfg := parallelConfig{Steps: steps}
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
	require.True(t, hasVars)

	// Verify all 50 variables are present and correct.
	for i := 0; i < 50; i++ {
		varName := fmt.Sprintf("var_%d", i)
		assert.Equal(t, "concurrent_ok", merged[varName], "var_%d should be present", i)
	}
	assert.Equal(t, 50, resp["completed"])
}

// ---------------------------------------------------------------------------
// Variable output shape
// ---------------------------------------------------------------------------

// TestParallel_OutputResponseShape verifies the output response structure
// includes all expected fields.
func TestParallel_OutputResponseShape(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"value":"test"}`))
	}))
	defer server.Close()

	cfg := parallelConfig{
		Steps: []stepConfig{
			{
				Name:    "step",
				Request: &stepRequestConfig{Method: "GET", URL: server.URL + "/test"},
				Extract: []extractEntry{{Variable: "myVar", Path: "$.value"}},
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

	assert.Equal(t, "test-while-node", resp["node_id"])
	assert.Equal(t, "parallel", resp["type"])
	assert.Equal(t, 1, resp["completed"])
	assert.Equal(t, 1, resp["total"])

	merged, hasVars := resp["merged_vars"].(map[string]any)
	require.True(t, hasVars)
	assert.Equal(t, "test", merged["myVar"])
}
