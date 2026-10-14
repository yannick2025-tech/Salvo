package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/core/dag"
	"github.com/yannick2025-tech/Salvo/internal/core/expr"
	"github.com/yannick2025-tech/Salvo/internal/logger"
	httpprotocol "github.com/yannick2025-tech/Salvo/internal/protocol/http"
)

// loopConfig holds the parsed configuration for a loop node.
type loopConfig struct {
	LoopCount int          `json:"loop_count"`
	Steps     []stepConfig `json:"steps"`
}

func (n *sceneNode) executeLoop(ctx context.Context, input *dag.Input, nodeLog logger.Logger) (*dag.Output, error) {
	var cfg loopConfig
	if err := json.Unmarshal([]byte(n.config), &cfg); err != nil {
		nodeLog.Error("failed to parse loop node config", logger.F("error", err))
		return nil, fmt.Errorf("parse loop config: %w", err)
	}

	// Reference mode: children resolved from config.node_ids (mounted by
	// buildDAG) take precedence over the embedded steps fallback.
	if len(n.childNodes) > 0 {
		return n.executeLoopRefMode(ctx, input, &cfg, nodeLog)
	}

	if cfg.LoopCount <= 0 || len(cfg.Steps) == 0 {
		nodeLog.Warn("loop node has no iterations (loop_count=0 or no steps), skipping")
		return &dag.Output{
			Response: map[string]any{
				"node_id":     n.id,
				"type":        "loop",
				"iterations":  0,
				"merged_vars": map[string]any{},
			},
		}, nil
	}

	// Collect initial variables from input.
	mergedVars := make(map[string]any)
	if input != nil && input.Variables != nil {
		for k, v := range input.Variables {
			mergedVars[k] = v
		}
	}

	for i := 0; i < cfg.LoopCount; i++ {
		nodeLog.Info("loop iteration", logger.F("iteration", i+1), logger.F("total", cfg.LoopCount))

		for _, step := range cfg.Steps {
			// Check context cancellation.
			select {
			case <-ctx.Done():
				nodeLog.Warn("loop iteration interrupted by context cancellation",
					logger.F("iteration", i+1),
					logger.F("error", ctx.Err()))
				return nil, ctx.Err()
			default:
			}

			// Check condition: if condition is set and not met, skip this step.
			if step.Condition != nil {
				condExpr := fmt.Sprintf("${%s} %s \"%s\"", step.Condition.Variable, step.Condition.Operator, step.Condition.Value)
				if !expr.EvaluateConditionExpr(condExpr, mergedVars) {
					nodeLog.Debug("skipping loop step due to condition not met",
						logger.F("step", step.Name),
						logger.F("iteration", i+1),
						logger.F("condition", condExpr))
					continue
				}
			}

			if step.Request != nil {
				// Apply think time between steps.
				if step.ThinkTime != nil {
					delay := rand.Intn(step.ThinkTime.Max-step.ThinkTime.Min+1) + step.ThinkTime.Min
					time.Sleep(time.Duration(delay) * time.Millisecond)
				}

				// Build and execute the HTTP request.
				req := buildStepHTTPRequest(step.Request, mergedVars, nodeLog)
				proto := httpprotocol.NewProtocol()
				resp, err := proto.Execute(ctx, req)
				if err != nil {
					// Connection/protocol error — record as failed request.
					if n.stats != nil {
						n.stats.RecordLatency(0, false)
					}
					if n.httpOnlyStats != nil {
						n.httpOnlyStats.RecordLatency(0, false)
					}
					if n.nodeStats != nil {
						n.nodeStats.RecordLatency(0, false)
					}
					nodeLog.Debug("loop step stats recorded",
						logger.F("step", step.Name),
						logger.F("iteration", i+1),
						logger.F("success", false),
						logger.F("latency_ms", 0),
						logger.F("reason", "connection_error"))
					nodeLog.Error("loop step request failed",
						logger.F("step", step.Name),
						logger.F("iteration", i+1),
						logger.F("error", err))
					return nil, fmt.Errorf("loop iteration %d step %s: %w", i+1, step.Name, err)
				}

				httpResp, ok := resp.(*httpprotocol.HTTPResponse)
				if ok {
					if httpResp.IsSuccess() {
						if n.stats != nil {
							n.stats.RecordLatency(httpResp.Latency, true)
						}
						if n.httpOnlyStats != nil {
							n.httpOnlyStats.RecordLatency(httpResp.Latency, true)
						}
						if n.nodeStats != nil {
							n.nodeStats.RecordLatency(httpResp.Latency, true)
						}
						nodeLog.Debug("loop step stats recorded",
							logger.F("step", step.Name),
							logger.F("iteration", i+1),
							logger.F("success", true),
							logger.F("latency_ms", httpResp.Latency.Milliseconds()),
							logger.F("status_code", httpResp.StatusCode))
					} else {
						if n.stats != nil {
							n.stats.RecordLatency(httpResp.Latency, false)
						}
						if n.httpOnlyStats != nil {
							n.httpOnlyStats.RecordLatency(httpResp.Latency, false)
						}
						if n.nodeStats != nil {
							n.nodeStats.RecordLatency(httpResp.Latency, false)
						}
						nodeLog.Debug("loop step stats recorded",
							logger.F("step", step.Name),
							logger.F("iteration", i+1),
							logger.F("success", false),
							logger.F("latency_ms", httpResp.Latency.Milliseconds()),
							logger.F("status_code", httpResp.StatusCode),
							logger.F("reason", "http_error"))
					}
					// Extract variables from response.
					if len(step.Extract) > 0 {
						extractVarsFromResponse(httpResp.Body, step.Extract, mergedVars, nodeLog)
					}
				}
			}
		}
	}

	return &dag.Output{
		Response: map[string]any{
			"node_id":     n.id,
			"type":        "loop",
			"iterations":  cfg.LoopCount,
			"merged_vars": mergedVars,
		},
	}, nil
}

// executeLoopRefMode runs the loop node with children mounted from
// config.node_ids (the unified composite child-reference model): the child
// chain executes loop_count times via runChildChain with group-style soft
// failure semantics — a child reporting failure via Output.Error does not
// abort the chain, and the first child soft failure surfaces on the loop
// node's Output.Error. Variables are refreshed after each child step so
// extracts performed by one child are visible to the next.
func (n *sceneNode) executeLoopRefMode(ctx context.Context, input *dag.Input, cfg *loopConfig, nodeLog logger.Logger) (*dag.Output, error) {
	if input == nil {
		return nil, fmt.Errorf("loop node %s: nil input in reference mode", n.id)
	}
	if input.Variables == nil {
		input.Variables = make(map[string]any)
	}

	loopCount := cfg.LoopCount
	if loopCount <= 0 {
		loopCount = 1
	}

	// firstChildErr keeps the FIRST child soft failure so the trace span
	// reflects the real node outcome while the loop continues.
	var firstChildErr error
	var lastOutput *dag.Output

	for i := 0; i < loopCount; i++ {
		nodeLog.Info("loop iteration", logger.F("iteration", i+1), logger.F("total", loopCount), logger.F("mode", "ref"))

		res, err := n.runChildChain(ctx, input, nodeLog)
		if err != nil {
			return nil, fmt.Errorf("loop ref-mode iteration %d: %w", i+1, err)
		}
		if res.FirstSoftErr != nil && firstChildErr == nil {
			firstChildErr = res.FirstSoftErr
		}
		lastOutput = res.LastOutput
	}

	resp := map[string]any{
		"node_id":    n.id,
		"type":       "loop",
		"iterations": loopCount,
	}
	if lastOutput != nil && lastOutput.Response != nil {
		if m, ok := lastOutput.Response.(map[string]any); ok {
			resp["last_output"] = m
		}
	}
	return &dag.Output{
		Response: resp,
		Error:    firstChildErr,
	}, nil
}
