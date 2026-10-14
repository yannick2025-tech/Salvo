package runner

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yannick2025-tech/Salvo/internal/core/dag"
	"github.com/yannick2025-tech/Salvo/internal/logger"
)

// subFlowConfig holds the parsed configuration for a sub-flow node.
type subFlowConfig struct {
	SceneID string `json:"scene_id"`
	Async   bool   `json:"async"`
}

// context keys for sub-flow depth and cycle detection.
type subFlowCtxKey string

const (
	subFlowDepthKey   subFlowCtxKey = "sub_flow_depth"
	subFlowVisitedKey subFlowCtxKey = "sub_flow_visited"
)

func (n *sceneNode) executeSubFlow(ctx context.Context, input *dag.Input, nodeLog logger.Logger) (*dag.Output, error) {
	var cfg subFlowConfig
	if err := json.Unmarshal([]byte(n.config), &cfg); err != nil {
		nodeLog.Error("failed to parse sub-flow config", logger.F("error", err))
		return nil, fmt.Errorf("parse sub-flow config: %w", err)
	}

	if cfg.SceneID == "" {
		return nil, fmt.Errorf("sub-flow node %s: scene_id is required", n.id)
	}

	if n.subFlowRunner == nil {
		return nil, fmt.Errorf("sub-flow node %s: sub-flow runner not available (scene_id: %s)", n.id, cfg.SceneID)
	}

	// Track depth and cycle detection via context.
	depth := 0
	if d, ok := ctx.Value(subFlowDepthKey).(int); ok {
		depth = d
	}

	visited := []string{}
	if v, ok := ctx.Value(subFlowVisitedKey).([]string); ok {
		visited = v
	}

	// Depth limit check.
	if depth >= 5 {
		return nil, fmt.Errorf("sub-flow depth limit exceeded (max 5) for scene %s", cfg.SceneID)
	}

	// Cycle detection.
	for _, id := range visited {
		if id == cfg.SceneID {
			return nil, fmt.Errorf("circular sub-flow reference detected: scene %s appears twice in chain %v", cfg.SceneID, visited)
		}
	}

	// Build context for sub-flow with incremented depth and updated visited list.
	newVisited := append(append([]string{}, visited...), cfg.SceneID)
	subCtx := context.WithValue(ctx, subFlowDepthKey, depth+1)
	subCtx = context.WithValue(subCtx, subFlowVisitedKey, newVisited)

	// Collect input variables.
	variables := make(map[string]any)
	if input != nil && input.Variables != nil {
		for k, v := range input.Variables {
			variables[k] = v
		}
	}

	if cfg.Async {
		// Async mode: fire-and-forget, sub-scene runs in background.
		go func() {
			out, err := n.subFlowRunner(subCtx, cfg.SceneID, variables)
			if err != nil {
				nodeLog.Warn("async sub-flow finished with error",
					logger.F("scene_id", cfg.SceneID),
					logger.F("error", err))
				return
			}
			nodeLog.Info("async sub-flow completed",
				logger.F("scene_id", cfg.SceneID),
				logger.F("output", out))
		}()

		nodeLog.Info("sub-flow started async", logger.F("scene_id", cfg.SceneID))
		return &dag.Output{
			Response: map[string]any{
				"node_id":  n.id,
				"type":     "sub_flow",
				"scene_id": cfg.SceneID,
				"async":    true,
			},
		}, nil
	}

	// Sync mode: wait for sub-scene completion.
	nodeLog.Info("sub-flow executing sync",
		logger.F("scene_id", cfg.SceneID),
		logger.F("depth", depth+1))
	out, err := n.subFlowRunner(subCtx, cfg.SceneID, variables)
	if err != nil {
		nodeLog.Error("sub-flow execution failed",
			logger.F("scene_id", cfg.SceneID),
			logger.F("depth", depth+1),
			logger.F("error", err))
		return nil, fmt.Errorf("sub-flow %s: %w", cfg.SceneID, err)
	}

	nodeLog.Info("sub-flow completed",
		logger.F("scene_id", cfg.SceneID),
		logger.F("depth", depth+1))

	// Extract merged variables from sub-flow output.
	resp, ok := out.Response.(map[string]any)
	if !ok {
		resp = map[string]any{"node_id": n.id, "type": "sub_flow", "scene_id": cfg.SceneID}
	}

	return &dag.Output{
		Response: resp,
	}, nil
}