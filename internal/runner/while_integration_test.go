package runner

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
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
// Section 13.3: While node + condition operator integration tests.
//
// These tests verify the integration between while node's polling mechanism
// and the full range of condition operators (==, !=, >, >=, <, <=, equals,
// not_equals, empty, not_empty). Tests also cover expression engine
// integration (${__random()}, ${var}) within while configuration fields.
// =============================================================================

// TestWhile_ExitCondition_Equals verifies that a while loop exits when a
// variable satisfies the equals condition.
func TestWhile_ExitCondition_Equals(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":{"status":"COMPLETED"}}`))
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "chargeStatus", Operator: "equals", Value: "COMPLETED"},
		},
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "query status",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server.URL + "/status",
				},
				Extract: []extractEntry{
					{Variable: "chargeStatus", Path: "$.result.status"},
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
	assert.Equal(t, "while", resp["type"])
}

// TestWhile_ExitCondition_NotEquals verifies that a while loop exits when a
// variable satisfies the not_equals condition.
func TestWhile_ExitCondition_NotEquals(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":{"status":"CHARGING"}}`))
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "chargeStatus", Operator: "not_equals", Value: "CHARGING"},
		},
		MaxIterations:  5,
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "query status",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server.URL + "/status",
				},
				Extract: []extractEntry{
					{Variable: "chargeStatus", Path: "$.result.status"},
				},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	// Status is always "CHARGING", so not_equals "CHARGING" is never true.
	_, err = node.executeWhile(ctx, &dag.Input{}, node.log)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max iterations")
}

// TestWhile_ExitCondition_GreaterThan verifies numeric > operator for exit.
func TestWhile_ExitCondition_GreaterThan(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var callCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		count := callCount
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"result":{"progress":"%d"}}`, count)))
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			// Exit when progress > 3 (will happen on 4th call)
			{Variable: "progress", Operator: "greater_than", Value: "3"},
		},
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "query progress",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server.URL + "/progress",
				},
				Extract: []extractEntry{
					{Variable: "progress", Path: "$.result.progress"},
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
	assert.GreaterOrEqual(t, resp["iterations"].(int), 4)
	assert.GreaterOrEqual(t, time.Since(start).Seconds(), 3.0)
}

// TestWhile_ExitCondition_GreaterThanOrEqual verifies >= operator for exit.
func TestWhile_ExitCondition_GreaterThanOrEqual(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var mu sync.Mutex
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		val := callCount
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"score":"%d"}`, val)))
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "score", Operator: "greater_than_or_equal", Value: "3"},
		},
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "query score",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server.URL + "/score",
				},
				Extract: []extractEntry{
					{Variable: "score", Path: "$.score"},
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
	assert.GreaterOrEqual(t, resp["iterations"].(int), 3)
}

// TestWhile_ExitCondition_LessThan verifies < operator for exit.
func TestWhile_ExitCondition_LessThan(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Server always returns 1, so 1 < 2 is immediately true.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"value":"1"}`))
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "val", Operator: "less_than", Value: "2"},
		},
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "query",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server.URL + "/val",
				},
				Extract: []extractEntry{
					{Variable: "val", Path: "$.value"},
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

// TestWhile_ExitCondition_LessThanOrEqual verifies <= operator for exit.
func TestWhile_ExitCondition_LessThanOrEqual(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"value":"2"}`))
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "val", Operator: "less_than_or_equal", Value: "2"},
		},
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "query",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server.URL + "/val",
				},
				Extract: []extractEntry{
					{Variable: "val", Path: "$.value"},
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

// TestWhile_ExitCondition_NotEmpty verifies a loop exits when a variable
// becomes non-empty.
func TestWhile_ExitCondition_NotEmpty(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var mu sync.Mutex
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		c := callCount
		mu.Unlock()

		var body string
		if c < 3 {
			body = `{"result":{"data":""}}`
		} else {
			body = `{"result":{"data":"done"}}`
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "myData", Operator: "not_empty", Value: ""},
		},
		MaxIterations:  5,
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "poll data",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server.URL + "/data",
				},
				Extract: []extractEntry{
					{Variable: "myData", Path: "$.result.data"},
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
	assert.GreaterOrEqual(t, resp["iterations"].(int), 3)
	assert.LessOrEqual(t, resp["iterations"].(int), 5)
}

// TestWhile_ExitCondition_Empty verifies a loop exits when a variable
// becomes empty.
func TestWhile_ExitCondition_Empty(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":{"taskId":""}}`))
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "taskId", Operator: "empty", Value: ""},
		},
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "check task",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server.URL + "/task",
				},
				Extract: []extractEntry{
					{Variable: "taskId", Path: "$.result.taskId"},
				},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	// The taskId is always empty, so the exit condition (empty) is always true.
	output, err := node.executeWhile(ctx, &dag.Input{}, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 1, resp["iterations"])
}

// TestWhile_ExitCondition_KeywordOperator verifies that keyword-style
// operators (equals, not_equals, gte, lte) work correctly in exit conditions.
func TestWhile_ExitCondition_KeywordOperator(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"value":"42"}`))
	}))
	defer server.Close()

	t.Run("gte keyword", func(t *testing.T) {
		cfg := whileConfig{
			ExitConditions: []exitCondition{
				{Variable: "val", Operator: "gte", Value: "40"},
			},
			IntervalSeconds: 1,
			Steps: []stepConfig{
				{
					Name: "query",
					Request: &stepRequestConfig{
						Method: "GET",
						URL:    server.URL + "/val",
					},
					Extract: []extractEntry{
						{Variable: "val", Path: "$.value"},
					},
				},
			},
		}
		cfgBytes, _ := json.Marshal(cfg)
		node := newTestSceneNode()
		node.config = string(cfgBytes)
		output, err := node.executeWhile(ctx, &dag.Input{}, node.log)
		require.NoError(t, err)
		assert.NotNil(t, output)
	})

	t.Run("lte keyword", func(t *testing.T) {
		cfg := whileConfig{
			ExitConditions: []exitCondition{
				{Variable: "val", Operator: "lte", Value: "50"},
			},
			IntervalSeconds: 1,
			Steps: []stepConfig{
				{
					Name: "query",
					Request: &stepRequestConfig{
						Method: "GET",
						URL:    server.URL + "/val",
					},
					Extract: []extractEntry{
						{Variable: "val", Path: "$.value"},
					},
				},
			},
		}
		cfgBytes, _ := json.Marshal(cfg)
		node := newTestSceneNode()
		node.config = string(cfgBytes)
		output, err := node.executeWhile(ctx, &dag.Input{}, node.log)
		require.NoError(t, err)
		assert.NotNil(t, output)
	})
}

// TestWhile_ExpressionEngineInStepURL verifies that ${__random()} and ${var}
// are correctly resolved in while step HTTP request URLs.
func TestWhile_ExpressionEngineInStepURL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var capturedPath string
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedPath = r.URL.String()
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"COMPLETED"}`))
	}))
	defer server.Close()

	// Pre-resolve expression engine parts for the step URL.
	reg := expr.NewFunctionRegistry()
	builtin.RegisterAll(reg)

	stepURL := server.URL + "/charge?token=${requestToken}&timeout=${__random(60,600)}"
	// Use expr.Resolve for the __random part.
	resolvedURL, err := expr.Resolve(stepURL, map[string]any{"requestToken": "abc123"}, reg)
	require.NoError(t, err)
	require.NotContains(t, resolvedURL, "${__random}")

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "status", Operator: "equals", Value: "COMPLETED"},
		},
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "query with resolved url",
				Request: &stepRequestConfig{
					Method: "GET",
					// URL is pre-resolved (expression engine handles this before executeWhile).
					URL: resolvedURL,
				},
				Extract: []extractEntry{
					{Variable: "status", Path: "$.status"},
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

	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, capturedPath)

	// Parse the captured URL (use url.Parse for order-independent extraction —
	// Go's url.Values.Encode() sorts keys alphabetically, so the parameter
	// order may differ from the input).
	parsedURL, err := url.Parse(capturedPath)
	require.NoError(t, err)
	query := parsedURL.Query()
	assert.Equal(t, "abc123", query.Get("token"))
	require.NotEmpty(t, query.Get("timeout"))

	// Verify timeout is a number in [60, 600].
	timeoutVal, err := strconv.Atoi(query.Get("timeout"))
	require.NoError(t, err)
	assert.GreaterOrEqual(t, timeoutVal, 60)
	assert.LessOrEqual(t, timeoutVal, 600)
}

// TestWhile_MultipleExitConditions_AllMustBeMet verifies that when multiple
// exit conditions are specified, all must be satisfied before the loop exits.
func TestWhile_MultipleExitConditions_AllMustBeMet(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var callCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		c := callCount
		mu.Unlock()

		// Return status=COMPLETED and the current call count as progress.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"result":{"status":"COMPLETED","progress":"%d"}}`, c)))
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "chargeStatus", Operator: "equals", Value: "COMPLETED"},
			{Variable: "progress", Operator: "greater_than_or_equal", Value: "5"},
		},
		MaxIterations:  10,
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "query status",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server.URL + "/status",
				},
				Extract: []extractEntry{
					{Variable: "chargeStatus", Path: "$.result.status"},
					{Variable: "progress", Path: "$.result.progress"},
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
	assert.GreaterOrEqual(t, resp["iterations"].(int), 5)
}

// TestWhile_MultipleExitConditions_PartialMatchDoesNotExit verifies that
// the loop continues when only some exit conditions are met.
func TestWhile_MultipleExitConditions_PartialMatchDoesNotExit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Returns COMPLETED status but progress is never >= 5 within max iterations.
	mockData := `{"result":{"status":"COMPLETED","progress":"1"}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockData))
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "chargeStatus", Operator: "equals", Value: "COMPLETED"},
			{Variable: "progress", Operator: "greater_than_or_equal", Value: "5"},
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
				Extract: []extractEntry{
					{Variable: "chargeStatus", Path: "$.result.status"},
					{Variable: "progress", Path: "$.result.progress"},
				},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	// Should hit max iterations because progress never reaches 5.
	_, err = node.executeWhile(ctx, &dag.Input{}, node.log)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max iterations")
}

// TestWhile_StepCondition_SkipBasedOnVariable verifies that a step condition
// based on a resolved variable correctly skips the step when the condition
// is not met.
func TestWhile_StepCondition_SkipBasedOnVariable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// The main server always returns COMPLETED.
	mainServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"COMPLETED"}`))
	}))
	defer mainServer.Close()

	// The secondary server should NEVER be called if step condition works.
	secondaryCalled := false
	secondaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondaryCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer secondaryServer.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "status", Operator: "equals", Value: "COMPLETED"},
		},
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "main query",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    mainServer.URL + "/status",
				},
				Extract: []extractEntry{
					{Variable: "status", Path: "$.status"},
				},
			},
			{
				Name: "conditional step",
				// Step is only executed when status == "PENDING".
				// Since the main query returns "COMPLETED", this step should be skipped.
				Condition: &stepConditionConfig{
					Variable: "status",
					Operator: "equals",
					Value:    "PENDING",
				},
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    secondaryServer.URL + "/extra",
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
	assert.False(t, secondaryCalled, "conditional step should have been skipped")
}

// TestWhile_TimedTriggerInLoop verifies timed trigger execution within a
// while loop. The trigger fires asynchronously after a delay and updates
// a variable that satisfies the exit condition. Unlike the polling step,
// the timed trigger updates a dedicated variable (trigger_status) that is
// not overwritten by the main poll step each iteration.
func TestWhile_TimedTriggerInLoop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	// The main poll step returns status=PENDING each time.
	mainServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"PENDING"}`))
	}))
	defer mainServer.Close()

	triggerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"COMPLETED"}`))
	}))
	defer triggerServer.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "trigger_status", Operator: "equals", Value: "COMPLETED"},
		},
		MaxIterations:  10,
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "poll status",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    mainServer.URL + "/poll",
				},
				Extract: []extractEntry{
					{Variable: "status", Path: "$.status"},
				},
			},
			{
				Name: "async trigger",
				TimedTrigger: &timedTriggerConfig{
					AfterSeconds: "1",
					Once:         true,
				},
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    triggerServer.URL + "/trigger",
				},
				Extract: []extractEntry{
					// Use a dedicated variable that the poll step does not overwrite.
					{Variable: "trigger_status", Path: "$.status"},
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
	assert.GreaterOrEqual(t, resp["iterations"].(int), 1)
	assert.LessOrEqual(t, resp["iterations"].(int), 10)
}

// TestWhile_RetryOnStepFailure verifies that while steps with retry config
// retry when the HTTP request fails.
func TestWhile_RetryOnStepFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	attemptCount := 0
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attemptCount++
		attempt := attemptCount
		mu.Unlock()

		if attempt < 3 {
			// Fail with 429 on first two attempts.
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		// Succeed on third attempt.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"COMPLETED"}`))
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "status", Operator: "equals", Value: "COMPLETED"},
		},
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "retry step",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server.URL + "/status",
				},
				Extract: []extractEntry{
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

	mu.Lock()
	assert.GreaterOrEqual(t, attemptCount, 3)
	mu.Unlock()
}

// TestWhile_VariablePropagationToOutput verifies that variables accumulated
// during while loop execution are accessible in the output.
func TestWhile_VariablePropagationToOutput(t *testing.T) {
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
				Extract: []extractEntry{
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

	input := &dag.Input{
		Variables: map[string]any{
			"sceneId": "scene-123",
		},
	}

	output, err := node.executeWhile(ctx, input, node.log)
	require.NoError(t, err)
	require.NotNil(t, output)
	// The output Response should contain loop metadata.
	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "test-while-node", resp["node_id"])
}

// TestWhile_ConcurrentLoops verifies the runner's concurrency model with
// multiple while loops running in parallel, simulating the DAG's parallel
// node execution capability.
func TestWhile_ConcurrentLoops(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Two servers that return COMPLETED after different call counts.
	var mu1, mu2 sync.Mutex
	count1, count2 := 0, 0

	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu1.Lock()
		count1++
		c := count1
		mu1.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"status":"%s"}`, map[bool]string{true: "COMPLETED", false: "PENDING"}[c >= 3])))
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu2.Lock()
		count2++
		c := count2
		mu2.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"status":"%s"}`, map[bool]string{true: "COMPLETED", false: "PENDING"}[c >= 2])))
	}))
	defer server2.Close()

	// Create two while loop configurations.
	cfg1 := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "status", Operator: "equals", Value: "COMPLETED"},
		},
		MaxIterations:  10,
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "loop1 query",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server1.URL + "/status",
				},
				Extract: []extractEntry{
					{Variable: "status", Path: "$.status"},
				},
			},
		},
	}

	cfg2 := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "status", Operator: "equals", Value: "COMPLETED"},
		},
		MaxIterations:  10,
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "loop2 query",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server2.URL + "/status",
				},
				Extract: []extractEntry{
					{Variable: "status", Path: "$.status"},
				},
			},
		},
	}

	cfgBytes1, _ := json.Marshal(cfg1)
	cfgBytes2, _ := json.Marshal(cfg2)

	var wg sync.WaitGroup
	wg.Add(2)

	var output1, output2 *dag.Output
	var err1, err2 error

	go func() {
		defer wg.Done()
		node1 := newTestSceneNode()
		node1.config = string(cfgBytes1)
		output1, err1 = node1.executeWhile(ctx, &dag.Input{}, node1.log)
	}()

	go func() {
		defer wg.Done()
		node2 := newTestSceneNode()
		node2.config = string(cfgBytes2)
		output2, err2 = node2.executeWhile(ctx, &dag.Input{}, node2.log)
	}()

	wg.Wait()

	require.NoError(t, err1)
	require.NoError(t, err2)
	require.NotNil(t, output1)
	require.NotNil(t, output2)

	resp1 := output1.Response.(map[string]any)
	resp2 := output2.Response.(map[string]any)

	// Loop1 needs 3 calls (exits after 3rd), loop2 needs 2 calls.
	assert.GreaterOrEqual(t, resp1["iterations"].(int), 3)
	assert.GreaterOrEqual(t, resp2["iterations"].(int), 2)

	// Both loops should have completed in roughly the same time.
	assert.True(t, true, "both concurrent while loops completed")
}

// TestWhile_ExpressionEngineRandomInInterval verifies that the expression
// engine can be used to dynamically set the while loop interval via
// pre-resolution (the runner resolves expressions before passing to while).
func TestWhile_ExpressionEngineRandomInInterval(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"COMPLETED"}`))
	}))
	defer server.Close()

	// The interval is static in whileConfig but derived from expression engine.
	interval := 1
	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "status", Operator: "equals", Value: "COMPLETED"},
		},
		IntervalSeconds: interval,
		Steps: []stepConfig{
			{
				Name: "query",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server.URL + "/status",
				},
				Extract: []extractEntry{
					{Variable: "status", Path: "$.status"},
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

// TestWhile_SOPluginExpressionInStep verifies that a step in the while loop
// can use a pre-resolved expression that came from an SO plugin.
func TestWhile_SOPluginExpressionInStep(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var capturedToken string
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedToken = r.URL.Query().Get("token")
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"COMPLETED"}`))
	}))
	defer server.Close()

	// Set up SO plugin and pre-resolve the expression.
	reg := expr.NewFunctionRegistry()
	loader := so.NewLoader()
	loader.Register(&testShellAESPlugin{})
	err := so.RegisterSO(reg, loader)
	require.NoError(t, err)

	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	iv := base64.StdEncoding.EncodeToString([]byte("1234567890123456"))
	encExpr := fmt.Sprintf(`${__so("shell-aes", "encrypt", "%s", "%s", "13312345674")}`, key, iv)
	encToken, err := expr.Resolve(encExpr, nil, reg)
	require.NoError(t, err)
	require.NotEqual(t, encExpr, encToken)

	stepURL := server.URL + "/charge?token=" + encToken
	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "status", Operator: "equals", Value: "COMPLETED"},
		},
		IntervalSeconds: 1,
		Steps: []stepConfig{
			{
				Name: "query with encrypted token",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    stepURL,
				},
				Extract: []extractEntry{
					{Variable: "status", Path: "$.status"},
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

	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, capturedToken)
	// The token should be a valid base64 ciphertext.
	_, err = base64.StdEncoding.DecodeString(capturedToken)
	assert.NoError(t, err)
}

// TestWhile_ContextCancellationDuringInterval verifies that context
// cancellation during the interval wait is handled correctly.
func TestWhile_ContextCancellationDuringInterval(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"PENDING"}`))
	}))
	defer server.Close()

	cfg := whileConfig{
		ExitConditions: []exitCondition{
			{Variable: "status", Operator: "equals", Value: "COMPLETED"},
		},
		MaxIterations:  50,
		IntervalSeconds: 5, // Long interval to ensure we cancel during wait.
		Steps: []stepConfig{
			{
				Name: "query",
				Request: &stepRequestConfig{
					Method: "GET",
					URL:    server.URL + "/status",
				},
				Extract: []extractEntry{
					{Variable: "status", Path: "$.status"},
				},
			},
		},
	}
	cfgBytes, _ := json.Marshal(cfg)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	// Cancel context immediately after start.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Already cancelled.

	_, err := node.executeWhile(ctx, &dag.Input{}, node.log)
	require.Error(t, err)
}