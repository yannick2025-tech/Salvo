package dag

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testNode struct {
	id        string
	mode      ExecMode
	timeout   time.Duration
	loopCount int
}

func (n *testNode) ID() string                 { return n.id }
func (n *testNode) Execute(_ context.Context, _ *Input) (*Output, error) {
	return &Output{Response: n.id}, nil
}
func (n *testNode) Timeout() time.Duration { return n.timeout }
func (n *testNode) LoopCount() int         { return n.loopCount }
func (n *testNode) Mode() ExecMode         { return n.mode }
func (n *testNode) BlockOnError() bool      { return false }

func TestAddNode(t *testing.T) {
	d := New()
	err := d.AddNode(&testNode{id: "A"})
	assert.NoError(t, err)

	node, ok := d.Node("A")
	assert.True(t, ok)
	assert.Equal(t, "A", node.ID())
}

func TestAddDuplicateNode(t *testing.T) {
	d := New()
	require.NoError(t, d.AddNode(&testNode{id: "A"}))

	err := d.AddNode(&testNode{id: "A"})
	assert.Error(t, err)
	var dupErr *DuplicateNodeError
	assert.ErrorAs(t, err, &dupErr)
}

func TestAddNilNode(t *testing.T) {
	d := New()
	err := d.AddNode(nil)
	assert.ErrorIs(t, err, ErrNilNode)
}

func TestAddEdge(t *testing.T) {
	d := New()
	require.NoError(t, d.AddNode(&testNode{id: "A"}))
	require.NoError(t, d.AddNode(&testNode{id: "B"}))

	err := d.AddEdge("A", "B", EdgeNormal, "")
	assert.NoError(t, err)

	edges := d.OutEdges("A")
	assert.Len(t, edges, 1)
	assert.Equal(t, "B", edges[0].To)
}

func TestAddEdgeNodeNotFound(t *testing.T) {
	d := New()
	require.NoError(t, d.AddNode(&testNode{id: "A"}))

	err := d.AddEdge("A", "B", EdgeNormal, "")
	assert.Error(t, err)
	var nfErr *NodeNotFoundError
	assert.ErrorAs(t, err, &nfErr)

	err = d.AddEdge("B", "A", EdgeNormal, "")
	assert.ErrorAs(t, err, &nfErr)
}

func TestAddEdgeCreatesCycle(t *testing.T) {
	d := New()
	require.NoError(t, d.AddNode(&testNode{id: "A"}))
	require.NoError(t, d.AddNode(&testNode{id: "B"}))
	require.NoError(t, d.AddNode(&testNode{id: "C"}))

	require.NoError(t, d.AddEdge("A", "B", EdgeNormal, ""))
	require.NoError(t, d.AddEdge("B", "C", EdgeNormal, ""))

	err := d.AddEdge("C", "A", EdgeNormal, "")
	assert.Error(t, err)
	var cycErr *CycleError
	assert.ErrorAs(t, err, &cycErr)

	assert.Len(t, d.Edges(), 2)
}

func TestAddConditionalEdge(t *testing.T) {
	d := New()
	require.NoError(t, d.AddNode(&testNode{id: "A"}))
	require.NoError(t, d.AddNode(&testNode{id: "B"}))
	require.NoError(t, d.AddNode(&testNode{id: "C"}))

	require.NoError(t, d.AddEdge("A", "B", EdgeCondition, "status == 200"))
	require.NoError(t, d.AddEdge("A", "C", EdgeCondition, "status != 200"))

	edges := d.OutEdges("A")
	assert.Len(t, edges, 2)
	assert.Equal(t, EdgeCondition, edges[0].Type)
	assert.Equal(t, "status == 200", edges[0].Condition)
}

func TestRootNodes(t *testing.T) {
	d := New()
	require.NoError(t, d.AddNode(&testNode{id: "A"}))
	require.NoError(t, d.AddNode(&testNode{id: "B"}))
	require.NoError(t, d.AddNode(&testNode{id: "C"}))
	require.NoError(t, d.AddEdge("A", "B", EdgeNormal, ""))
	require.NoError(t, d.AddEdge("A", "C", EdgeNormal, ""))

	roots := d.RootNodes()
	assert.Len(t, roots, 1)
	assert.Equal(t, "A", roots[0].ID())
}

func TestRootNodesMultiple(t *testing.T) {
	d := New()
	require.NoError(t, d.AddNode(&testNode{id: "A"}))
	require.NoError(t, d.AddNode(&testNode{id: "B"}))
	require.NoError(t, d.AddNode(&testNode{id: "C"}))
	require.NoError(t, d.AddEdge("A", "C", EdgeNormal, ""))
	require.NoError(t, d.AddEdge("B", "C", EdgeNormal, ""))

	roots := d.RootNodes()
	rootIDs := make(map[string]bool)
	for _, r := range roots {
		rootIDs[r.ID()] = true
	}
	assert.True(t, rootIDs["A"])
	assert.True(t, rootIDs["B"])
}

func TestTopologicalSort(t *testing.T) {
	d := New()
	require.NoError(t, d.AddNode(&testNode{id: "A"}))
	require.NoError(t, d.AddNode(&testNode{id: "B"}))
	require.NoError(t, d.AddNode(&testNode{id: "C"}))
	require.NoError(t, d.AddEdge("A", "B", EdgeNormal, ""))
	require.NoError(t, d.AddEdge("B", "C", EdgeNormal, ""))

	order, err := d.TopologicalSort()
	require.NoError(t, err)

	pos := make(map[string]int)
	for i, id := range order {
		pos[id] = i
	}
	assert.Less(t, pos["A"], pos["B"])
	assert.Less(t, pos["B"], pos["C"])
}

func TestTopologicalSortDiamond(t *testing.T) {
	d := New()
	require.NoError(t, d.AddNode(&testNode{id: "A"}))
	require.NoError(t, d.AddNode(&testNode{id: "B"}))
	require.NoError(t, d.AddNode(&testNode{id: "C"}))
	require.NoError(t, d.AddNode(&testNode{id: "D"}))
	require.NoError(t, d.AddEdge("A", "B", EdgeNormal, ""))
	require.NoError(t, d.AddEdge("A", "C", EdgeNormal, ""))
	require.NoError(t, d.AddEdge("B", "D", EdgeNormal, ""))
	require.NoError(t, d.AddEdge("C", "D", EdgeNormal, ""))

	order, err := d.TopologicalSort()
	require.NoError(t, err)

	pos := make(map[string]int)
	for i, id := range order {
		pos[id] = i
	}
	assert.Less(t, pos["A"], pos["B"])
	assert.Less(t, pos["A"], pos["C"])
	assert.Less(t, pos["B"], pos["D"])
	assert.Less(t, pos["C"], pos["D"])
}

func TestInEdges(t *testing.T) {
	d := New()
	require.NoError(t, d.AddNode(&testNode{id: "A"}))
	require.NoError(t, d.AddNode(&testNode{id: "B"}))
	require.NoError(t, d.AddEdge("A", "B", EdgeNormal, ""))

	inEdges := d.InEdges("B")
	assert.Len(t, inEdges, 1)
	assert.Equal(t, "A", inEdges[0].From)

	inEdges = d.InEdges("A")
	assert.Len(t, inEdges, 0)
}

func TestNodesMap(t *testing.T) {
	d := New()
	require.NoError(t, d.AddNode(&testNode{id: "A"}))
	require.NoError(t, d.AddNode(&testNode{id: "B"}))

	nodes := d.Nodes()
	assert.Len(t, nodes, 2)
}

func TestEmptyDAG(t *testing.T) {
	d := New()
	roots := d.RootNodes()
	assert.Len(t, roots, 0)

	order, err := d.TopologicalSort()
	assert.NoError(t, err)
	assert.Len(t, order, 0)
}
