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
					nodeLog.Error("loop step request failed",
						logger.F("step", step.Name),
						logger.F("iteration", i+1),
						logger.F("error", err))
					return nil, fmt.Errorf("loop iteration %d step %s: %w", i+1, step.Name, err)
				}

				// Extract variables from response.
				if httpResp, ok := resp.(*httpprotocol.HTTPResponse); ok && len(step.Extract) > 0 {
					extractVarsFromResponse(httpResp.Body, step.Extract, mergedVars)
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