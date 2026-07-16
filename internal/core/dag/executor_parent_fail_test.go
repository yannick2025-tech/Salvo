package dag

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingNode is a test Node that always returns an error.
type failingNode struct {
	id string
}

func (n *failingNode) ID() string             { return n.id }
func (n *failingNode) Timeout() time.Duration { return 0 }
func (n *failingNode) LoopCount() int         { return 1 }
func (n *failingNode) Mode() ExecMode         { return ExecSync }
func (n *failingNode) BlockOnError() bool     { return false }

func (n *failingNode) Execute(_ context.Context, _ *Input) (*Output, error) {
	return nil, fmt.Errorf("node %s failed", n.id)
}

// TestExecutorParentFailureSkipsDownstream verifies that when a parent node
// fails, downstream nodes are skipped and not executed.
func TestExecutorParentFailureSkipsDownstream(t *testing.T) {
	g := New()
	a := &recordingNode{id: "A", mode: ExecSync}
	b := &failingNode{id: "B"}
	c := &recordingNode{id: "C", mode: ExecSync}

	require.NoError(t, g.AddNode(a))
	require.NoError(t, g.AddNode(b))
	require.NoError(t, g.AddNode(c))
	require.NoError(t, g.AddEdge("A", "B", EdgeNormal, ""))
	require.NoError(t, g.AddEdge("B", "C", EdgeNormal, ""))

	ex := NewExecutor(g)
	_, err := ex.Execute(context.Background())
	require.Error(t, err)

	assert.Equal(t, int32(1), a.ExecCount(), "node A should execute")
	assert.Equal(t, int32(0), c.ExecCount(), "node C should NOT execute when parent B failed")
}

// TestExecutorParentFailureMultipleChildren verifies that when a parent fails,
// all downstream children are skipped.
func TestExecutorParentFailureMultipleChildren(t *testing.T) {
	g := New()
	a := &failingNode{id: "A"}
	b := &recordingNode{id: "B", mode: ExecSync}
	c := &recordingNode{id: "C", mode: ExecSync}

	require.NoError(t, g.AddNode(a))
	require.NoError(t, g.AddNode(b))
	require.NoError(t, g.AddNode(c))
	require.NoError(t, g.AddEdge("A", "B", EdgeNormal, ""))
	require.NoError(t, g.AddEdge("A", "C", EdgeNormal, ""))

	ex := NewExecutor(g)
	_, err := ex.Execute(context.Background())
	require.Error(t, err)

	assert.Equal(t, int32(0), b.ExecCount(), "node B should NOT execute when parent A failed")
	assert.Equal(t, int32(0), c.ExecCount(), "node C should NOT execute when parent A failed")
}
