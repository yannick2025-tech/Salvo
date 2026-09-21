package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/core/dag"
)

// TestExecuteWhile_SoftFailedStepCarriesFirstError verifies that a step
// assertion failure swallowed by the while loop (block_on_error=false)
// is carried on the final Output.Error, keeping the FIRST failure.
func TestExecuteWhile_SoftFailedStepCarriesFirstError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Body always fails the errorCode assertion but satisfies the exit
	// condition, so the while loop finishes normally after iteration 1.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"errorCode": 50028, "result":{"status":"4"}}`))
	}))
	defer server.Close()

	cfg := whileConfig{
		// Exit via max_iterations (extraction cannot run when the step
		// assertion fails before extract), treating it as success.
		MaxIterations:     2,
		IntervalSeconds:   0,
		FailOnMaxIterations: boolPtr(false),
		Steps: []stepConfig{
			{
				Name: "first step",
				Request: &stepRequestConfig{
					Method:     "GET",
					URL:        server.URL + "/status",
					ExpectBody: map[string]any{"errorCode": 0},
				},
			},
			{
				Name: "second step",
				Request: &stepRequestConfig{
					Method:     "GET",
					URL:        server.URL + "/status",
					ExpectBody: map[string]any{"errorCode": 0},
				},
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	node := newTestSceneNode()
	node.config = string(cfgBytes)

	output, err := node.executeWhile(ctx, &dag.Input{}, node.log)
	require.NoError(t, err, "swallowed step failures must not fail the while node")
	require.NotNil(t, output)

	require.Error(t, output.Error, "Output.Error must carry the swallowed step failure")
	assert.Contains(t, output.Error.Error(), `step "first step"`, "must keep the FIRST failed step")
	assert.Contains(t, output.Error.Error(), "errorCode")
}

func boolPtr(b bool) *bool { return &b }

// TestExecuteWhile_AllStepsPassKeepsNoError guards the happy path.
func TestExecuteWhile_AllStepsPassKeepsNoError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"errorCode": 0, "result":{"status":"4"}}`))
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
					Method:     "GET",
					URL:        server.URL + "/status",
					ExpectBody: map[string]any{"errorCode": 0},
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
	assert.NoError(t, output.Error, "all-pass while must not carry Output.Error")
}
