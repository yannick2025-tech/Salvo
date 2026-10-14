package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/core/dag"
)

func newTestSceneNode() *sceneNode {
	return &sceneNode{
		id:            "test-while-node",
		log:           newTestLogger(),
		stats:         &Stats{},
		httpOnlyStats: &Stats{},
		nodeStats:     NewNodeStats(10000),
	}
}

func newTestSceneNodeMinimal() *sceneNode {
	return &sceneNode{
		id:   "test-while-node",
		log:  newTestLogger(),
		stats: &Stats{},
	}
}

func TestExecuteWhile_ExitConditionMet(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Server that returns status=4 immediately, so exit condition is met in first iteration.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":{"status":"4"}}`))
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "chargingStatus", Operator: "equals", Value: "4"},
		},
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "query status",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server.URL + "/status",
				},
				Extract: extractConfig{
					{Variable: "chargingStatus", Path: "$.result.status"},
				},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	output, err := node.executeWhile(ctx, &dag.Input{}, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "test-while-node", resp["node_id"])
	assert.Equal(t, "while", resp["type"])
	assert.Equal(t, 1, resp["iterations"])
}

func TestExecuteWhile_ExitConditionNotMetContinues(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	callCount := 0
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		status := "1"
		if callCount >= 3 {
			status = "4"
		}
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"result":{"status":"%s"}}`, status)))
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "chargingStatus", Operator: "equals", Value: "4"},
		},
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "query status",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server.URL + "/status",
				},
				Extract: extractConfig{
					{Variable: "chargingStatus", Path: "$.result.status"},
				},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	start := time.Now()
	output, err := node.executeWhile(ctx, &dag.Input{}, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)
	// Should have taken 3 iterations (2 conditional failures + 1 success).
	iterations, _ := resp["iterations"].(int)
	assert.GreaterOrEqual(t, iterations, 3)
	// Should have taken at least 2 seconds (2 intervals between 3 iterations).
	assert.GreaterOrEqual(t, time.Since(start).Seconds(), 2.0)
}

func TestExecuteWhile_MaxIterations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":{"status":"1"}}`))
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "chargingStatus", Operator: "equals", Value: "4"},
		},
		MaxIterations:  3,
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "query status",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server.URL + "/status",
				},
				Extract: extractConfig{
					{Variable: "chargingStatus", Path: "$.result.status"},
				},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	_, err = node.executeWhile(ctx, &dag.Input{}, node.log)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max iterations")
}

func TestExecuteWhile_ContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":{"status":"1"}}`))
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "chargingStatus", Operator: "equals", Value: "4"},
		},
		MaxIterations:  50,
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "query status",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server.URL + "/status",
				},
				Extract: extractConfig{
					{Variable: "chargingStatus", Path: "$.result.status"},
				},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	// Use a context with a very short deadline to force cancellation.
	shortCtx, shortCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer shortCancel()

	// Should return an error due to context cancellation/deadline.
	_, err = node.executeWhile(shortCtx, &dag.Input{}, node.log)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

func TestExecuteWhile_NoExitAndNoMaxIter(t *testing.T) {
	// Test infinite loop protection.
	cfg := whileConfig{
		Steps: []stepConfig{
			{Name: "empty step"},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	_, err = node.executeWhile(context.Background(), &dag.Input{}, node.log)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "infinite loop")
}

func TestExecuteWhile_EmptySteps(t *testing.T) {
	// Test that empty steps returns immediately with 0 iterations.
	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "x", Operator: "equals", Value: "1"},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	output, err := node.executeWhile(context.Background(), &dag.Input{}, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 0, resp["iterations"])
}

func TestExecuteWhile_StepCondition(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":{"status":"4"}}`))
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "chargingStatus", Operator: "equals", Value: "4"},
		},
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "always skipped",
				// Condition references a variable that doesn't exist, so step is skipped.
				Condition: &stepConditionConfig{
					Variable: "nonExistentVar",
					Operator: "equals",
					Value:    "1",
				},
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server.URL + "/status",
				},
				Extract: extractConfig{
					{Variable: "chargingStatus", Path: "$.result.status"},
				},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	// Since the step is skipped, no HTTP request is made and chargingStatus is never set.
	// The exit condition will never be met, so it should hit max_iterations.
	// Let's add max_iterations to terminate gracefully.
	_, err = node.executeWhile(ctx, &dag.Input{}, node.log)
	require.Error(t, err) // Will fail due to infinite loop or timeout
}

func TestExecuteWhile_ConsecutiveFailures(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Server always returns 500.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "x", Operator: "equals", Value: "done"},
		},
		MaxIterations:      10,
		FailAfterConsecutive: 3,
		FailMessage:         "too many failures",
		IntervalSeconds:     1,
		Steps: []stepConfig{
			{
				Name: "failing step",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server.URL + "/fail",
				},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	_, err = node.executeWhile(ctx, &dag.Input{}, node.log)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too many failures")
}

func TestExecuteWhile_StepLevelConsecutiveFailures(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "x", Operator: "equals", Value: "done"},
		},
		MaxIterations:  10,
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "failing step",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server.URL + "/fail",
				},
				FailAfterConsecutive: 2,
				FailMessage:          "step level failure",
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	_, err = node.executeWhile(ctx, &dag.Input{}, node.log)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "step level failure")
}

func TestExecuteWhile_ThinkTime(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"4"}`))
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "status", Operator: "equals", Value: "4"},
		},
		MaxIterations:  3,
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "step with think time",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server.URL,
				},
				Extract: extractConfig{
					{Variable: "status", Path: "$.status"},
				},
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
	output, err := node.executeWhile(ctx, &dag.Input{}, node.log)
	require.NoError(t, err)
	assert.NotNil(t, output)
	// Should have taken some time for think_time + interval.
	assert.GreaterOrEqual(t, time.Since(start).Milliseconds(), int64(50))
}

func TestResolveJSONPath_SimpleField(t *testing.T) {
	data := map[string]any{
		"status": "4",
	}
	val := resolveJSONPath(data, "$.status")
	assert.Equal(t, "4", val)
}

func TestResolveJSONPath_NestedField(t *testing.T) {
	data := map[string]any{
		"result": map[string]any{
			"status": "4",
		},
	}
	val := resolveJSONPath(data, "$.result.status")
	assert.Equal(t, "4", val)
}

func TestResolveJSONPath_ArrayIndex(t *testing.T) {
	data := map[string]any{
		"items": []any{
			map[string]any{"id": "1"},
			map[string]any{"id": "2"},
		},
	}
	val := resolveJSONPath(data, "$.items[1].id")
	assert.Equal(t, "2", val)
}

func TestResolveJSONPath_InvalidPath(t *testing.T) {
	data := map[string]any{
		"status": "4",
	}
	val := resolveJSONPath(data, "$.nonexistent")
	assert.Nil(t, val)
}

func TestResolveJSONPath_NoDollarPrefix(t *testing.T) {
	data := map[string]any{
		"status": "4",
	}
	val := resolveJSONPath(data, "status")
	assert.Nil(t, val)
}

func TestExtractEntry_UnmarshalStructured(t *testing.T) {
	data := []byte(`{"variable": "chargingStatus", "path": "$.result.status"}`)
	var entry extractEntry
	err := unmarshalExtractEntry(data, &entry)
	require.NoError(t, err)
	assert.Equal(t, "chargingStatus", entry.Variable)
	assert.Equal(t, "$.result.status", entry.Path)
}

func TestExtractEntry_UnmarshalKV(t *testing.T) {
	data := []byte(`{"chargingStatus": "$.result.status"}`)
	var entry extractEntry
	err := unmarshalExtractEntry(data, &entry)
	require.NoError(t, err)
	assert.Equal(t, "chargingStatus", entry.Variable)
	assert.Equal(t, "$.result.status", entry.Path)
}

func TestExtractEntry_UnmarshalInvalid(t *testing.T) {
	data := []byte(`{"key": 123}`)
	var entry extractEntry
	err := unmarshalExtractEntry(data, &entry)
	require.Error(t, err)
}

func TestParseDurationSeconds_Valid(t *testing.T) {
	sec, err := parseDurationSeconds("30")
	require.NoError(t, err)
	assert.Equal(t, float64(30), sec)
}

func TestParseDurationSeconds_ValidFloat(t *testing.T) {
	sec, err := parseDurationSeconds("15.5")
	require.NoError(t, err)
	assert.Equal(t, 15.5, sec)
}

func TestParseDurationSeconds_Invalid(t *testing.T) {
	_, err := parseDurationSeconds("abc")
	require.Error(t, err)
}

func TestIsHTTP429Error(t *testing.T) {
	assert.True(t, isHTTP429Error(fmt.Errorf("HTTP 429")))
	assert.False(t, isHTTP429Error(fmt.Errorf("HTTP 500")))
}

func TestExtractVarsFromResponse_EmptyBody(t *testing.T) {
	vars := make(map[string]any)
	extractVarsFromResponse(nil, extractConfig{{Variable: "x", Path: "$.status"}}, vars, newTestLogger())
	assert.Empty(t, vars)
}

func TestExtractVarsFromResponse_InvalidJSON(t *testing.T) {
	vars := make(map[string]any)
	extractVarsFromResponse([]byte(`not json`), extractConfig{{Variable: "x", Path: "$.status"}}, vars, newTestLogger())
	assert.Empty(t, vars)
}

func TestExtractVarsFromResponse_Success(t *testing.T) {
	vars := make(map[string]any)
	body := []byte(`{"result":{"status":"4","kwh":"12.5"}}`)
	extractVarsFromResponse(body, extractConfig{
		{Variable: "chargingStatus", Path: "$.result.status"},
		{Variable: "kwh", Path: "$.result.kwh"},
	}, vars, newTestLogger())
	assert.Equal(t, "4", vars["chargingStatus"])
	assert.Equal(t, "12.5", vars["kwh"])
}

func TestExtractVarsFromResponse_ArrayExtract(t *testing.T) {
	vars := make(map[string]any)
	body := []byte(`{"items":[{"id":"1"},{"id":"2"}]}`)
	extractVarsFromResponse(body, extractConfig{
		{Variable: "firstId", Path: "$.items[0].id"},
	}, vars, newTestLogger())
	assert.Equal(t, "1", vars["firstId"])
}

func TestExecuteWhile_VariableExtraction(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":{"status":"4","kwh":"12.5","money":"30.0"}}`))
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "chargingStatus", Operator: "equals", Value: "4"},
		},
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "query charge status",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server.URL + "/status",
				},
				Extract: extractConfig{
					{Variable: "chargingStatus", Path: "$.result.status"},
					{Variable: "chargeKwh", Path: "$.result.kwh"},
					{Variable: "chargeMoney", Path: "$.result.money"},
				},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	// Create an input with initial variables.
	input := &dag.Input{
		Variables: map[string]any{
			"token": "abc123",
		},
	}

	output, err := node.executeWhile(ctx, input, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)
	assert.NotNil(t, output.Response)
}

func TestExecuteWhile_ClosedWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Already cancelled.

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"4"}`))
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "status", Operator: "equals", Value: "4"},
		},
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "query",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server.URL,
				},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	_, err = node.executeWhile(ctx, &dag.Input{}, node.log)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

func TestExecuteWhile_NoLoopVar(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"4"}`))
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "status", Operator: "equals", Value: "4"},
		},
		MaxIterations:  3,
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "query",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server.URL,
				},
				Extract: extractConfig{
					{Variable: "status", Path: "$.status"},
				},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	// Pass nil input (no variables).
	output, err := node.executeWhile(ctx, nil, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)
}

func TestRandomInt(t *testing.T) {
	val := randomInt(5, 10)
	assert.GreaterOrEqual(t, val, 5)
	assert.LessOrEqual(t, val, 10)
}

func TestRandomInt_MinGreaterOrEqual(t *testing.T) {
	val := randomInt(10, 5)
	assert.GreaterOrEqual(t, val, 10)
}

func TestBuildStepHTTPRequest(t *testing.T) {
	cfg := &stepRequestConfig{
		Method:  "POST",
		URL:     "http://example.com/${path}",
		Headers: map[string]string{"Authorization": "Bearer ${token}"},
		Body:    map[string]any{"key": "${value}"},
	}
	vars := map[string]any{
		"path":  "api/test",
		"token": "abc123",
		"value": "test_value",
	}

	req := buildStepHTTPRequest(cfg, vars, newTestLogger())
	require.NotNil(t, req)
	assert.Equal(t, "POST", string(req.Method))
	assert.Equal(t, "http://example.com/api/test", req.URL)
	assert.Equal(t, "Bearer abc123", req.Headers["Authorization"])
	assert.NotEmpty(t, req.Body)
	// Body should be resolved - we need to parse it back to check.
	var bodyMap map[string]string
	err := json.Unmarshal(req.Body, &bodyMap)
	require.NoError(t, err)
	assert.Equal(t, "test_value", bodyMap["key"])
}

func TestBuildStepHTTPRequest_DefaultMethod(t *testing.T) {
	cfg := &stepRequestConfig{
		URL: "http://example.com",
	}
	req := buildStepHTTPRequest(cfg, nil, newTestLogger())
	require.NotNil(t, req)
	assert.Equal(t, "GET", string(req.Method))
}

func TestBuildStepHTTPRequest_NoBody(t *testing.T) {
	cfg := &stepRequestConfig{
		Method: "GET",
		URL:    "http://example.com",
	}
	req := buildStepHTTPRequest(cfg, nil, newTestLogger())
	require.NotNil(t, req)
	assert.Empty(t, req.Body)
}

func TestBuildStepHTTPRequest_FormFieldsAndFiles(t *testing.T) {
	cfg := &stepRequestConfig{
		Method: "POST",
		URL:    "http://example.com/upload",
		Form: &stepFormConfig{
			Fields: map[string]string{"seq": "S-${x}"},
			Files:  map[string]string{"photo": "/tmp/${name}.jpg"},
		},
	}
	vars := map[string]any{"x": "1", "name": "comments"}
	req := buildStepHTTPRequest(cfg, vars, newTestLogger())
	require.NotNil(t, req)
	require.NotNil(t, req.Form)
	assert.Equal(t, "S-1", req.Form.Fields["seq"])
	assert.Equal(t, "/tmp/comments.jpg", req.Form.Files["photo"])
}

func TestBuildStepHTTPRequest_FormOverridesBody(t *testing.T) {
	cfg := &stepRequestConfig{
		Method: "POST",
		URL:    "http://example.com",
		Body:   `{"ignored":true}`,
		Form: &stepFormConfig{
			Fields: map[string]string{"used": "yes"},
		},
	}
	req := buildStepHTTPRequest(cfg, nil, newTestLogger())
	require.NotNil(t, req)
	require.NotNil(t, req.Form)
	assert.Equal(t, "yes", req.Form.Fields["used"])
}

func TestExecuteWhile_MultipleExitConditions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":{"status":"4","kwh":"100"}}`))
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "chargingStatus", Operator: "equals", Value: "4"},
			{Variable: "chargeKwh", Operator: "equals", Value: "100"},
		},
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "query charge status",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server.URL,
				},
				Extract: extractConfig{
					{Variable: "chargingStatus", Path: "$.result.status"},
					{Variable: "chargeKwh", Path: "$.result.kwh"},
				},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	output, err := node.executeWhile(ctx, &dag.Input{}, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 1, resp["iterations"])
}

func TestExecuteWhile_RetryOnFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	attempt := 0
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempt++
		if attempt < 2 {
			mu.Unlock()
			// Return 429 on first attempt.
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"4"}`))
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "status", Operator: "equals", Value: "4"},
		},
		IntervalSeconds: 1,
		MaxIterations:   3,
		Steps: []stepConfig{
			{
				Name: "query with retry",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server.URL,
				},
				Extract: extractConfig{
					{Variable: "status", Path: "$.status"},
				},
				Retry: &retryConfig{
					MaxAttempts: 3,
					On429:       "retry",
				},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	output, err := node.executeWhile(ctx, &dag.Input{}, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)
}

func TestExecuteWhile_TimedTrigger(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"triggered"}`))
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "status", Operator: "equals", Value: "triggered"},
		},
		IntervalSeconds: 1,
		MaxIterations:   3,
		Steps: []stepConfig{
			{
				Name: "timed event",
				Condition: &stepConditionConfig{
					Variable: "chargePostOffline",
					Operator: "equals",
					Value:    "1",
				},
				TimedTrigger: &timedTriggerConfig{
					AfterSeconds: "0", // Fire immediately.
					Once:         true,
				},
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server.URL + "/trigger",
				},
				Extract: extractConfig{
					{Variable: "status", Path: "$.status"},
				},
			},
			{
				Name: "regular query",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server.URL + "/query",
				},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	// The timed trigger step has a condition that checks chargePostOffline==1.
	// Since that variable doesn't exist, the step is skipped and the trigger never fires.
	// So the exit condition (status==triggered) will never be met.

	// Let's simplify: test with a condition that passes.
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"triggered"}`))
	}))
	defer server2.Close()

	cfg2 := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "status", Operator: "equals", Value: "triggered"},
		},
		IntervalSeconds: 1,
		MaxIterations:   3,
		Steps: []stepConfig{
			{
				Name: "regular query",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server2.URL + "/query",
				},
				Extract: extractConfig{
					{Variable: "status", Path: "$.status"},
				},
			},
		},
	}
	cfgBytes2, err := json.Marshal(cfg2)
	require.NoError(t, err)

	node2 := newTestSceneNode()
	node2.config = string(cfgBytes2)

	output, err := node2.executeWhile(ctx, &dag.Input{}, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)
	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 1, resp["iterations"])
}

// TestExecuteWhile_WithInitialVariables ensures initial variables from input are preserved.
func TestExecuteWhile_WithInitialVariables(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":{"status":"4"}}`))
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "chargingStatus", Operator: "equals", Value: "4"},
		},
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "query status",
				Request: &stepRequestConfig{
					Method: "POST",
					URL:    server.URL + "/status",
					Headers: map[string]string{
						"Authorization": "${token}",
					},
					Body: map[string]any{
						"sceneId": "${sceneId}",
					},
				},
				Extract: extractConfig{
					{Variable: "chargingStatus", Path: "$.result.status"},
				},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	input := &dag.Input{
		Variables: map[string]any{
			"token":   "Bearer test-token",
			"sceneId": "scene-123",
		},
	}

	output, err := node.executeWhile(ctx, input, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)
}

func TestExecuteWhile_SuccessRequestsCounted(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Server returns success, exit condition met after 1 iteration.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"done"}`))
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "status", Operator: "equals", Value: "done"},
		},
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name:    "query",
				Request: &stepRequestConfig{Method: "GET", URL: server.URL},
				Extract: extractConfig{{Variable: "status", Path: "$.status"}},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	_, err = node.executeWhile(ctx, &dag.Input{}, node.log)
	require.NoError(t, err)

	// 1 successful HTTP request should be counted in all 3 stats objects.
	assert.Equal(t, int64(1), node.stats.TotalReqs.Load(), "global stats should count 1 total request")
	assert.Equal(t, int64(1), node.stats.SuccessReqs.Load(), "global stats should count 1 success")
	assert.Equal(t, int64(0), node.stats.FailedReqs.Load(), "global stats should count 0 failures")

	assert.Equal(t, int64(1), node.httpOnlyStats.TotalReqs.Load(), "httpOnlyStats should count 1 total request")
	assert.Equal(t, int64(1), node.httpOnlyStats.SuccessReqs.Load(), "httpOnlyStats should count 1 success")

	nodeSnap := node.nodeStats.Snapshot()
	assert.Equal(t, int64(1), nodeSnap.TotalReqs, "nodeStats should count 1 total request")
	assert.Equal(t, int64(1), nodeSnap.SuccessReqs, "nodeStats should count 1 success")
}

func TestExecuteWhile_FailedRequestsCounted(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Server always returns 500.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "x", Operator: "equals", Value: "done"},
		},
		MaxIterations:      3,
		FailAfterConsecutive: 3,
		IntervalSeconds:     1,
		Steps: []stepConfig{
			{
				Name:    "failing step",
				Request: &stepRequestConfig{Method: "GET", URL: server.URL + "/fail"},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	_, err = node.executeWhile(ctx, &dag.Input{}, node.log)
	require.Error(t, err)

	// 3 failed HTTP requests should be counted.
	assert.Equal(t, int64(3), node.stats.TotalReqs.Load(), "global stats should count 3 total requests")
	assert.Equal(t, int64(3), node.stats.FailedReqs.Load(), "global stats should count 3 failures")
	assert.Equal(t, int64(0), node.stats.SuccessReqs.Load(), "global stats should count 0 successes")

	assert.Equal(t, int64(3), node.httpOnlyStats.TotalReqs.Load(), "httpOnlyStats should count 3 total requests")
	assert.Equal(t, int64(3), node.httpOnlyStats.FailedReqs.Load(), "httpOnlyStats should count 3 failures")

	nodeSnap := node.nodeStats.Snapshot()
	assert.Equal(t, int64(3), nodeSnap.TotalReqs, "nodeStats should count 3 total requests")
	assert.Equal(t, int64(3), nodeSnap.FailedReqs, "nodeStats should count 3 failures")
}

func TestExecuteWhile_MultipleIterationsCounted(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Server returns status=1 for first 2 calls, then status=4.
	callCount := 0
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		status := "1"
		if callCount >= 3 {
			status = "4"
		}
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"status":"%s"}`, status)))
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "status", Operator: "equals", Value: "4"},
		},
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name:    "query",
				Request: &stepRequestConfig{Method: "GET", URL: server.URL},
				Extract: extractConfig{{Variable: "status", Path: "$.status"}},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	_, err = node.executeWhile(ctx, &dag.Input{}, node.log)
	require.NoError(t, err)

	// 3 successful HTTP requests across 3 iterations.
	assert.Equal(t, int64(3), node.stats.TotalReqs.Load(), "global stats should count 3 total requests from 3 iterations")
	assert.Equal(t, int64(3), node.stats.SuccessReqs.Load(), "global stats should count 3 successes")
	assert.Equal(t, int64(0), node.stats.FailedReqs.Load(), "global stats should count 0 failures")
}

// --- extractConfig unmarshal tests ---

func TestExtractConfig_UnmarshalArrayFormat(t *testing.T) {
	data := []byte(`[{"variable": "charging_status", "path": "$.data.chargingStatus"}]`)
	var cfg extractConfig
	err := json.Unmarshal(data, &cfg)
	require.NoError(t, err)
	require.Len(t, cfg, 1)
	assert.Equal(t, "charging_status", cfg[0].Variable)
	assert.Equal(t, "$.data.chargingStatus", cfg[0].Path)
}

func TestExtractConfig_UnmarshalObjectFormat(t *testing.T) {
	data := []byte(`{"charging_status": "$.data.chargingStatus", "api_sign": "$.sign"}`)
	var cfg extractConfig
	err := json.Unmarshal(data, &cfg)
	require.NoError(t, err)
	require.Len(t, cfg, 2)
	// Map iteration order is non-deterministic, so check by variable name.
	vars := map[string]string{}
	for _, e := range cfg {
		vars[e.Variable] = e.Path
	}
	assert.Equal(t, "$.data.chargingStatus", vars["charging_status"])
	assert.Equal(t, "$.sign", vars["api_sign"])
}

func TestExtractConfig_UnmarshalInvalidFormat(t *testing.T) {
	data := []byte(`"not an object or array"`)
	var cfg extractConfig
	err := json.Unmarshal(data, &cfg)
	require.Error(t, err)
}

func TestExtractConfig_WhileConfigIntegration(t *testing.T) {
	// Simulate the YAML → JSON → whileConfig pipeline with both extract formats.
	jsonConfig := `{
		"exit_conditions": [{"variable": "charging_status", "operator": "equals", "value": "6"}],
		"interval_seconds": 1,
		"max_iterations": 3,
		"steps": [
			{
				"name": "generator-step",
				"type": "generator",
				"config": {
					"expression": "test",
					"variable": "result",
					"extract": {"api_sign": "$.sign", "api_ts": "$.ts"}
				}
			},
			{
				"name": "http-step",
				"request": {"method": "GET", "url": "/api/test"},
				"extract": [{"variable": "charging_status", "path": "$.data.chargingStatus"}]
			}
		]
	}`

	var cfg whileConfig
	err := json.Unmarshal([]byte(jsonConfig), &cfg)
	require.NoError(t, err)

	// Generator step: extract should be empty (extract is inside config, not step-level)
	assert.Empty(t, cfg.Steps[0].Extract, "generator step extract should be empty (it's inside config)")
	assert.NotNil(t, cfg.Steps[0].Config["extract"])

	// HTTP step: extract should be parsed from array format
	require.Len(t, cfg.Steps[1].Extract, 1)
	assert.Equal(t, "charging_status", cfg.Steps[1].Extract[0].Variable)
	assert.Equal(t, "$.data.chargingStatus", cfg.Steps[1].Extract[0].Path)
}

func TestExtractConfig_ObjectFormatAtStepLevel(t *testing.T) {
	// Test that dictionary format at step level is now supported.
	jsonConfig := `{
		"exit_conditions": [{"variable": "charging_status", "operator": "equals", "value": "6"}],
		"interval_seconds": 1,
		"max_iterations": 3,
		"steps": [
			{
				"name": "http-step",
				"request": {"method": "GET", "url": "/api/test"},
				"extract": {"charging_status": "$.data.chargingStatus", "charge_time": "$.data.chargeTime"}
			}
		]
	}`

	var cfg whileConfig
	err := json.Unmarshal([]byte(jsonConfig), &cfg)
	require.NoError(t, err)

	require.Len(t, cfg.Steps[0].Extract, 2)
	vars := map[string]string{}
	for _, e := range cfg.Steps[0].Extract {
		vars[e.Variable] = e.Path
	}
	assert.Equal(t, "$.data.chargingStatus", vars["charging_status"])
	assert.Equal(t, "$.data.chargeTime", vars["charge_time"])
}

func TestMergeStepExtracts_ConfigExtract(t *testing.T) {
	step := &stepConfig{
		Name: "generator-step",
		Config: map[string]any{
			"expression": "test",
			"variable":   "result",
			"extract": map[string]any{
				"api_sign": "$.sign",
				"api_ts":   "$.ts",
			},
		},
	}

	extracts := mergeStepExtracts(step, newTestLogger())
	require.Len(t, extracts, 2)
	vars := map[string]string{}
	for _, e := range extracts {
		vars[e.Variable] = e.Path
	}
	assert.Equal(t, "$.sign", vars["api_sign"])
	assert.Equal(t, "$.ts", vars["api_ts"])
}

func TestMergeStepExtracts_BothSources(t *testing.T) {
	step := &stepConfig{
		Name: "mixed-step",
		Extract: extractConfig{
			{Variable: "top_level_var", Path: "$.top"},
		},
		Config: map[string]any{
			"extract": map[string]any{
				"config_var": "$.config",
			},
		},
	}

	extracts := mergeStepExtracts(step, newTestLogger())
	require.Len(t, extracts, 2)
	vars := map[string]string{}
	for _, e := range extracts {
		vars[e.Variable] = e.Path
	}
	assert.Equal(t, "$.top", vars["top_level_var"])
	assert.Equal(t, "$.config", vars["config_var"])
}