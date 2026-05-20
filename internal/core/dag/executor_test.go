package dag

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingNode is a test Node that records its execution order and supports
// configurable mode, timeout, and loop count.
type recordingNode struct {
	id        string
	mode      ExecMode
	timeout   time.Duration
	loopCount int
	executed  atomic.Int32
	mu        sync.Mutex
	order     []int
}

func (n *recordingNode) ID() string             { return n.id }
func (n *recordingNode) Timeout() time.Duration { return n.timeout }
func (n *recordingNode) LoopCount() int         { return n.loopCount }
func (n *recordingNode) Mode() ExecMode         { return n.mode }

func (n *recordingNode) Execute(_ context.Context, input *Input) (*Output, error) {
	n.executed.Add(1)
	n.mu.Lock()
	n.order = append(n.order, int(n.executed.Load()))
	n.mu.Unlock()
	return &Output{Response: n.id}, nil
}

func (n *recordingNode) ExecCount() int32 {
	return n.executed.Load()
}

func TestExecutorSyncChain(t *testing.T) {
	g := New()
	a := &recordingNode{id: "A", mode: ExecSync}
	b := &recordingNode{id: "B", mode: ExecSync}
	c := &recordingNode{id: "C", mode: ExecSync}

	require.NoError(t, g.AddNode(a))
	require.NoError(t, g.AddNode(b))
	require.NoError(t, g.AddNode(c))
	require.NoError(t, g.AddEdge("A", "B", EdgeNormal, ""))
	require.NoError(t, g.AddEdge("B", "C", EdgeNormal, ""))

	ex := NewExecutor(g)
	result, err := ex.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, int32(1), a.ExecCount())
	assert.Equal(t, int32(1), b.ExecCount())
	assert.Equal(t, int32(1), c.ExecCount())

	assert.Equal(t, "C", result.Response)
}

func TestExecutorFanOut(t *testing.T) {
	g := New()
	a := &recordingNode{id: "A", mode: ExecSync}
	b := &recordingNode{id: "B", mode: ExecSync}
	c := &recordingNode{id: "C", mode: ExecSync}

	require.NoError(t, g.AddNode(a))
	require.NoError(t, g.AddNode(b))
	require.NoError(t, g.AddNode(c))
	require.NoError(t, g.AddEdge("A", "B", EdgeNormal, ""))
	require.NoError(t, g.AddEdge("A", "C", EdgeNormal, ""))

	ex := NewExecutor(g)
	result, err := ex.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, int32(1), a.ExecCount())
	assert.Equal(t, int32(1), b.ExecCount())
	assert.Equal(t, int32(1), c.ExecCount())
}

func TestExecutorFanIn(t *testing.T) {
	g := New()
	a := &recordingNode{id: "A", mode: ExecSync}
	b := &recordingNode{id: "B", mode: ExecSync}
	c := &recordingNode{id: "C", mode: ExecSync}

	require.NoError(t, g.AddNode(a))
	require.NoError(t, g.AddNode(b))
	require.NoError(t, g.AddNode(c))
	require.NoError(t, g.AddEdge("A", "C", EdgeNormal, ""))
	require.NoError(t, g.AddEdge("B", "C", EdgeNormal, ""))

	ex := NewExecutor(g)
	result, err := ex.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, int32(1), a.ExecCount())
	assert.Equal(t, int32(1), b.ExecCount())
	assert.Equal(t, int32(1), c.ExecCount())
}

func TestExecutorDiamond(t *testing.T) {
	g := New()
	a := &recordingNode{id: "A", mode: ExecSync}
	b := &recordingNode{id: "B", mode: ExecSync}
	c := &recordingNode{id: "C", mode: ExecSync}
	d := &recordingNode{id: "D", mode: ExecSync}

	require.NoError(t, g.AddNode(a))
	require.NoError(t, g.AddNode(b))
	require.NoError(t, g.AddNode(c))
	require.NoError(t, g.AddNode(d))
	require.NoError(t, g.AddEdge("A", "B", EdgeNormal, ""))
	require.NoError(t, g.AddEdge("A", "C", EdgeNormal, ""))
	require.NoError(t, g.AddEdge("B", "D", EdgeNormal, ""))
	require.NoError(t, g.AddEdge("C", "D", EdgeNormal, ""))

	ex := NewExecutor(g)
	result, err := ex.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, int32(1), a.ExecCount())
	assert.Equal(t, int32(1), b.ExecCount())
	assert.Equal(t, int32(1), c.ExecCount())
	assert.Equal(t, int32(1), d.ExecCount())
}

func TestExecutorLoopCount(t *testing.T) {
	g := New()
	a := &recordingNode{id: "A", mode: ExecSync, loopCount: 5}

	require.NoError(t, g.AddNode(a))

	ex := NewExecutor(g)
	result, err := ex.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, int32(5), a.ExecCount())
}

func TestExecutorLoopCountWithChain(t *testing.T) {
	g := New()
	a := &recordingNode{id: "A", mode: ExecSync, loopCount: 3}
	b := &recordingNode{id: "B", mode: ExecSync}

	require.NoError(t, g.AddNode(a))
	require.NoError(t, g.AddNode(b))
	require.NoError(t, g.AddEdge("A", "B", EdgeNormal, ""))

	ex := NewExecutor(g)
	result, err := ex.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, int32(3), a.ExecCount())
	assert.Equal(t, int32(1), b.ExecCount())
}

func TestExecutorConditionalEdge(t *testing.T) {
	g := New()
	a := &recordingNode{id: "A", mode: ExecSync}
	b := &recordingNode{id: "B", mode: ExecSync}
	c := &recordingNode{id: "C", mode: ExecSync}

	require.NoError(t, g.AddNode(a))
	require.NoError(t, g.AddNode(b))
	require.NoError(t, g.AddNode(c))

	// Both edges from A are conditional; evaluator will decide which to take.
	require.NoError(t, g.AddEdge("A", "B", EdgeCondition, "status == 200"))
	require.NoError(t, g.AddEdge("A", "C", EdgeCondition, "status != 200"))

	// Evaluator that always picks the "B" branch.
	eval := func(_ context.Context, cond string, _ *Output) bool {
		return cond == "status == 200"
	}

	ex := NewExecutor(g, WithConditionEvaluator(eval))
	result, err := ex.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, int32(1), a.ExecCount())
	assert.Equal(t, int32(1), b.ExecCount())
	assert.Equal(t, int32(0), c.ExecCount())
}

func TestExecutorContextCancellation(t *testing.T) {
	g := New()
	a := &recordingNode{id: "A", mode: ExecSync}

	require.NoError(t, g.AddNode(a))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ex := NewExecutor(g)
	_, err := ex.Execute(ctx)
	assert.Error(t, err)
}

func TestExecutorSingleNode(t *testing.T) {
	g := New()
	a := &recordingNode{id: "A", mode: ExecSync}

	require.NoError(t, g.AddNode(a))

	ex := NewExecutor(g)
	result, err := ex.Execute(context.Background())
	require.NoError(t, err)

	assert.Equal(t, int32(1), a.ExecCount())
	assert.Equal(t, "A", result.Response)
}

func TestExecutorNodeResultMap(t *testing.T) {
	g := New()
	a := &recordingNode{id: "A", mode: ExecSync}
	b := &recordingNode{id: "B", mode: ExecSync}

	require.NoError(t, g.AddNode(a))
	require.NoError(t, g.AddNode(b))
	require.NoError(t, g.AddEdge("A", "B", EdgeNormal, ""))

	ex := NewExecutor(g)
	_, err := ex.Execute(context.Background())
	require.NoError(t, err)

	results := ex.Results()
	assert.Contains(t, results, "A")
	assert.Contains(t, results, "B")
	assert.Equal(t, "A", results["A"].Response)
	assert.Equal(t, "B", results["B"].Response)
}

func TestExecutorDefaultConditionEvaluator(t *testing.T) {
	// Without a custom evaluator, all conditional edges should be traversed
	// (default evaluator returns true).
	g := New()
	a := &recordingNode{id: "A", mode: ExecSync}
	b := &recordingNode{id: "B", mode: ExecSync}

	require.NoError(t, g.AddNode(a))
	require.NoError(t, g.AddNode(b))
	require.NoError(t, g.AddEdge("A", "B", EdgeCondition, "any condition"))

	ex := NewExecutor(g)
	result, err := ex.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, int32(1), a.ExecCount())
	assert.Equal(t, int32(1), b.ExecCount())
}

// slowNode simulates a node that takes time to execute.
type slowNode struct {
	id    string
	mode  ExecMode
	delay time.Duration
}

func (n *slowNode) ID() string             { return n.id }
func (n *slowNode) Timeout() time.Duration { return n.delay * 2 }
func (n *slowNode) LoopCount() int         { return 1 }
func (n *slowNode) Mode() ExecMode         { return n.mode }

func (n *slowNode) Execute(ctx context.Context, _ *Input) (*Output, error) {
	select {
	case <-time.After(n.delay):
		return &Output{Response: n.id}, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("node %s cancelled: %w", n.id, ctx.Err())
	}
}

func TestExecutorAsyncNodeDoesNotBlock(t *testing.T) {
	g := New()
	a := &slowNode{id: "A", mode: ExecSync, delay: 10 * time.Millisecond}
	b := &slowNode{id: "B", mode: ExecAsync, delay: 200 * time.Millisecond}
	c := &slowNode{id: "C", mode: ExecSync, delay: 10 * time.Millisecond}

	require.NoError(t, g.AddNode(a))
	require.NoError(t, g.AddNode(b))
	require.NoError(t, g.AddNode(c))
	require.NoError(t, g.AddEdge("A", "B", EdgeNormal, ""))
	require.NoError(t, g.AddEdge("A", "C", EdgeNormal, ""))

	ex := NewExecutor(g)

	start := time.Now()
	result, err := ex.Execute(context.Background())
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, result)

	// If B blocked, total time would be >= 200ms. With async B, C should
	// complete quickly and the overall time should be well under 200ms.
	assert.Less(t, elapsed, 150*time.Millisecond,
		"async node B should not block the overall execution")
}
