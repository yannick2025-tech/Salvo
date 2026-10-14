package runner

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/core/dag"
)

func TestExecuteTimerDelay(t *testing.T) {
	cfg, _ := json.Marshal(map[string]any{
		"mode":    "delay",
		"seconds": 0.1,
	})

	node := &sceneNode{
		id:       "timer-1",
		nodeType: "timer",
		config:   string(cfg),
		loopCount: 1,
		mode:     dag.ExecAsync,
		log:      newTestLogger(),
	}

	start := time.Now()
	output, err := node.Execute(context.Background(), &dag.Input{})
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, output)
	assert.True(t, elapsed >= 90*time.Millisecond, "delay should wait at least ~100ms, got %v", elapsed)

	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "timer", resp["type"])
	assert.Equal(t, "delay", resp["mode"])
}

func TestExecuteTimerInterval(t *testing.T) {
	cfg, _ := json.Marshal(map[string]any{
		"mode":    "interval",
		"seconds": 0.05,
	})

	node := &sceneNode{
		id:       "timer-2",
		nodeType: "timer",
		config:   string(cfg),
		loopCount: 1,
		mode:     dag.ExecAsync,
		log:      newTestLogger(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	output, err := node.Execute(ctx, &dag.Input{})
	require.NoError(t, err)
	require.NotNil(t, output)

	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "timer", resp["type"])
	assert.Equal(t, "interval", resp["mode"])
	ticks, ok := resp["ticks"].(int)
	require.True(t, ok)
	assert.True(t, ticks >= 2, "interval timer should tick at least 2 times in 200ms with 50ms interval, got %d", ticks)
}

func TestExecuteTimerCancellation(t *testing.T) {
	cfg, _ := json.Marshal(map[string]any{
		"mode":    "delay",
		"seconds": 10,
	})

	node := &sceneNode{
		id:       "timer-3",
		nodeType: "timer",
		config:   string(cfg),
		loopCount: 1,
		mode:     dag.ExecAsync,
		log:      newTestLogger(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := node.Execute(ctx, &dag.Input{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

func TestExecuteTimerInvalidMode(t *testing.T) {
	cfg, _ := json.Marshal(map[string]any{
		"mode":    "invalid",
		"seconds": 1,
	})

	node := &sceneNode{
		id:       "timer-4",
		nodeType: "timer",
		config:   string(cfg),
		loopCount: 1,
		mode:     dag.ExecAsync,
		log:      newTestLogger(),
	}

	_, err := node.Execute(context.Background(), &dag.Input{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid timer mode")
}

func TestTimerNodeModeBasedExec(t *testing.T) {
	// Delay mode timer is sync: downstream nodes must wait for the
	// think-time to complete before executing.
	delayCfg, _ := json.Marshal(map[string]any{
		"mode":  "delay",
		"delay": 500,
	})
	delayNode := &sceneNode{
		id:        "timer-delay",
		nodeType:  "timer",
		config:    string(delayCfg),
		loopCount: 1,
		mode:      dag.ExecSync,
		log:       newTestLogger(),
	}
	assert.Equal(t, dag.ExecSync, delayNode.Mode())

	// Interval mode timer is async: it runs as a background heartbeat
	// while downstream nodes proceed immediately.
	intervalCfg, _ := json.Marshal(map[string]any{
		"mode":     "interval",
		"interval": 1000,
		"duration": 5000,
	})
	intervalNode := &sceneNode{
		id:        "timer-interval",
		nodeType:  "timer",
		config:    string(intervalCfg),
		loopCount: 1,
		mode:      dag.ExecAsync,
		log:       newTestLogger(),
	}
	assert.Equal(t, dag.ExecAsync, intervalNode.Mode())
}
