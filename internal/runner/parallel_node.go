package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/core/dag"
	"github.com/yannick2025-tech/Salvo/internal/core/expr"
	"github.com/yannick2025-tech/Salvo/internal/logger"
	httpprotocol "github.com/yannick2025-tech/Salvo/internal/protocol/http"
)

// parallelConfig holds the parsed configuration for a parallel node.
type parallelConfig struct {
	Steps []stepConfig `json:"steps"`
}

// stepResult captures the result of a single parallel step execution.
type stepResult struct {
	index     int
	name      string
	err       error
	variables map[string]any // extracted variables from this step
}

func (n *sceneNode) executeParallel(ctx context.Context, input *dag.Input, nodeLog logger.Logger) (*dag.Output, error) {
	var cfg parallelConfig
	if err := json.Unmarshal([]byte(n.config), &cfg); err != nil {
		nodeLog.Error("failed to parse parallel node config", logger.F("error", err))
		return nil, fmt.Errorf("parse parallel config: %w", err)
	}

	if len(cfg.Steps) == 0 {
		nodeLog.Warn("parallel node has no steps, skipping")
		return &dag.Output{Response: map[string]any{"node_id": n.id, "type": "parallel", "completed": 0}}, nil
	}

	// Collect initial variables from input.
	initVars := make(map[string]any)
	if input != nil && input.Variables != nil {
		for k, v := range input.Variables {
			initVars[k] = v
		}
	}

	// Check if context is already cancelled before starting goroutines.
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("parallel node: context cancelled before execution: %w", ctx.Err())
	default:
	}

	var wg sync.WaitGroup
	results := make(chan stepResult, len(cfg.Steps))

	for i, step := range cfg.Steps {
		// Check condition: if condition is set and not met, skip the step.
		if step.Condition != nil {
			condExpr := fmt.Sprintf("${%s} %s \"%s\"", step.Condition.Variable, step.Condition.Operator, step.Condition.Value)
			if !expr.EvaluateConditionExpr(condExpr, initVars) {
				nodeLog.Debug("skipping parallel step due to condition not met",
					logger.F("step", step.Name),
					logger.F("condition", condExpr))
				continue
			}
		}

		// Skip steps with no request (no-op steps).
		if step.Request == nil {
			nodeLog.Debug("skipping parallel step with no request",
				logger.F("step", step.Name))
			continue
		}

		wg.Add(1)
		go func(idx int, s stepConfig) {
			defer wg.Done()
			result := stepResult{
				index:     idx,
				name:      s.Name,
				variables: make(map[string]any),
			}

			// Copy initial variables for this step's isolation.
			localVars := make(map[string]any, len(initVars))
			for k, v := range initVars {
				localVars[k] = v
			}

			// Execute the HTTP request.
			result.err = n.executeParallelStepHTTP(ctx, &s, localVars, nodeLog)

			// Collect extracted variables from localVars (keys that are not in initVars).
			for k, v := range localVars {
				if _, wasInitial := initVars[k]; !wasInitial {
					result.variables[k] = v
				}
			}

			results <- result
		}(i, step)
	}

	// Wait for all goroutines to complete.
	wg.Wait()
	close(results)

	// Merge results.
	mergedVars := make(map[string]any)
	for k, v := range initVars {
		mergedVars[k] = v
	}

	var firstErr error
	completedCount := 0

	for res := range results {
		if res.err != nil {
			nodeLog.Error("parallel step failed",
				logger.F("step", res.name),
				logger.F("error", res.err))
			if firstErr == nil {
				firstErr = fmt.Errorf("parallel node: %s failed: %w", res.name, res.err)
			}
		} else {
			completedCount++
			// Merge extracted variables (last-write-wins).
			for k, v := range res.variables {
				mergedVars[k] = v
			}
			nodeLog.Debug("parallel step completed",
				logger.F("step", res.name),
				logger.F("extracted_vars", len(res.variables)))
		}
	}

	// Record stats for successful steps.
	if n.stats != nil {
		n.stats.RecordLatency(0, firstErr == nil)
	}
	if n.httpOnlyStats != nil {
		n.httpOnlyStats.RecordLatency(0, firstErr == nil)
	}
	if n.nodeStats != nil {
		n.nodeStats.RecordLatency(0, firstErr == nil)
	}

	nodeLog.Info("parallel node completed",
		logger.F("completed", completedCount),
		logger.F("total", len(cfg.Steps)),
		logger.F("has_error", firstErr != nil))

	if firstErr != nil {
		return &dag.Output{
			Response: map[string]any{
				"node_id":     n.id,
				"type":        "parallel",
				"completed":   completedCount,
				"total":       len(cfg.Steps),
				"merged_vars": mergedVars,
			},
			Error: firstErr,
		}, firstErr
	}

	return &dag.Output{
		Response: map[string]any{
			"node_id":     n.id,
			"type":        "parallel",
			"completed":   completedCount,
			"total":       len(cfg.Steps),
			"merged_vars": mergedVars,
		},
	}, nil
}

// executeParallelStepHTTP executes a single HTTP request step for the parallel node.
func (n *sceneNode) executeParallelStepHTTP(ctx context.Context, step *stepConfig, localVars map[string]any, nodeLog logger.Logger) error {
	if step.Request == nil {
		return nil
	}

	req := buildStepHTTPRequest(step.Request, localVars, nodeLog)
	proto := httpprotocol.NewProtocol()
	resp, err := proto.Execute(ctx, req)
	if err != nil {
		return fmt.Errorf("step %q HTTP request: %w", step.Name, err)
	}

	httpResp, ok := resp.(*httpprotocol.HTTPResponse)
	if !ok {
		return fmt.Errorf("step %q: unexpected response type %T", step.Name, resp)
	}

	if !httpResp.IsSuccess() {
		return fmt.Errorf("step %q HTTP %d", step.Name, httpResp.StatusCode)
	}

	// Extract variables from response into localVars.
	if len(step.Extract) > 0 {
		extractVarsFromResponse(httpResp.Body, step.Extract, localVars)
	}

	// Apply think_time if configured (random delay).
	if step.ThinkTime != nil && step.ThinkTime.Min > 0 && step.ThinkTime.Max > 0 {
		delay := randomInt(step.ThinkTime.Min, step.ThinkTime.Max)
		select {
		case <-ctx.Done():
			return fmt.Errorf("step %q cancelled during think time: %w", step.Name, ctx.Err())
		case <-time.After(time.Duration(delay) * time.Millisecond):
		}
	}

	return nil
}