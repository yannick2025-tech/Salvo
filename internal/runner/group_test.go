package runner

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/core/dag"
	"github.com/yannick2025-tech/Salvo/internal/logger"
)

// mockNode is a simple dag.Node implementation for testing.
type mockNode struct {
	id        string
	loopCount int
	mode      dag.ExecMode
	executed  atomic.Int32
	result    string
}

func (m *mockNode) ID() string                 { return m.id }
func (m *mockNode) Timeout() time.Duration      { return 0 }
func (m *mockNode) LoopCount() int              { return m.loopCount }
func (m *mockNode) Mode() dag.ExecMode          { return m.mode }
func (m *mockNode) Execute(ctx context.Context, input *dag.Input) (*dag.Output, error) {
	m.executed.Add(1)
	return &dag.Output{Response: map[string]any{"id": m.id, "result": m.result}}, nil
}

func newTestLogger() logger.Logger {
	l, _ := logger.New(logger.Config{Level: "error"})
	return l
}

func TestExecuteGroupSync(t *testing.T) {
	child1 := &mockNode{id: "child-1", loopCount: 1, mode: dag.ExecSync, result: "hello"}
	child2 := &mockNode{id: "child-2", loopCount: 1, mode: dag.ExecSync, result: "world"}

	groupConfig, _ := json.Marshal(map[string]any{
		"node_ids":   []string{"child-1", "child-2"},
		"loop_count": 2,
		"async":      false,
	})

	group := &sceneNode{
		id:         "group-1",
		nodeType:   "group",
		config:     string(groupConfig),
		loopCount:  1,
		mode:       dag.ExecSync,
		childNodes: []dag.Node{child1, child2},
		log:        newTestLogger(),
	}

	output, err := group.Execute(context.Background(), &dag.Input{})
	require.NoError(t, err)
	require.NotNil(t, output)

	// Each child should be executed 2 times (loop_count=2)
	assert.Equal(t, int32(2), child1.executed.Load())
	assert.Equal(t, int32(2), child2.executed.Load())

	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "group", resp["type"])
	assert.Equal(t, 2, resp["iterations"])
}

func TestExecuteGroupSingleChild(t *testing.T) {
	child := &mockNode{id: "child-1", loopCount: 1, mode: dag.ExecSync, result: "only"}

	groupConfig, _ := json.Marshal(map[string]any{
		"node_ids":   []string{"child-1"},
		"loop_count": 1,
		"async":      false,
	})

	group := &sceneNode{
		id:         "group-1",
		nodeType:   "group",
		config:     string(groupConfig),
		loopCount:  1,
		mode:       dag.ExecSync,
		childNodes: []dag.Node{child},
		log:        newTestLogger(),
	}

	output, err := group.Execute(context.Background(), &dag.Input{})
	require.NoError(t, err)
	assert.Equal(t, int32(1), child.executed.Load())

	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 1, resp["iterations"])
}

func TestExecuteGroupNoChildren(t *testing.T) {
	groupConfig, _ := json.Marshal(map[string]any{
		"node_ids":   []string{},
		"loop_count": 1,
		"async":      false,
	})

	group := &sceneNode{
		id:         "group-1",
		nodeType:   "group",
		config:     string(groupConfig),
		loopCount:  1,
		mode:       dag.ExecSync,
		childNodes: nil,
		log:        newTestLogger(),
	}

	output, err := group.Execute(context.Background(), &dag.Input{})
	require.NoError(t, err)

	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 0, resp["iterations"])
}

func TestExecuteGroupLoopCountOne(t *testing.T) {
	child := &mockNode{id: "child-1", loopCount: 1, mode: dag.ExecSync}

	groupConfig, _ := json.Marshal(map[string]any{
		"node_ids":   []string{"child-1"},
		"loop_count": 1,
	})

	group := &sceneNode{
		id:         "group-1",
		nodeType:   "group",
		config:     string(groupConfig),
		loopCount:  1,
		mode:       dag.ExecSync,
		childNodes: []dag.Node{child},
		log:        newTestLogger(),
	}

	_, err := group.Execute(context.Background(), &dag.Input{})
	require.NoError(t, err)
	assert.Equal(t, int32(1), child.executed.Load())
}
