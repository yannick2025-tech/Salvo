package dag

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tracelib "github.com/yannick2025-tech/Salvo/internal/trace"
)

// softFailNode reports failure via Output.Error without returning an
// error from Execute (soft-failure semantics, e.g. swallowed assertion).
type softFailNode struct {
	id       string
	executed bool
}

func (n *softFailNode) ID() string                 { return n.id }
func (n *softFailNode) Timeout() time.Duration      { return 0 }
func (n *softFailNode) LoopCount() int             { return 1 }
func (n *softFailNode) Mode() ExecMode              { return ExecSync }
func (n *softFailNode) BlockOnError() bool         { return false }
func (n *softFailNode) Execute(_ context.Context, _ *Input) (*Output, error) {
	n.executed = true
	return &Output{
		Response: map[string]any{"node_id": n.id},
		Error:    errors.New(`expect_body field "errorCode": expected 0, got 50028`),
	}, nil
}

// countingNode records whether it executed (downstream must not skip).
type countingNode struct {
	id       string
	executed bool
}

func (n *countingNode) ID() string                 { return n.id }
func (n *countingNode) Timeout() time.Duration     { return 0 }
func (n *countingNode) LoopCount() int            { return 1 }
func (n *countingNode) Mode() ExecMode             { return ExecSync }
func (n *countingNode) BlockOnError() bool        { return false }
func (n *countingNode) Execute(_ context.Context, _ *Input) (*Output, error) {
	n.executed = true
	return &Output{Response: map[string]any{"node_id": n.id}}, nil
}

// TestExecuteTraced_SpanReflectsOutputError verifies the span status is
// error when a node soft-fails via Output.Error, and downstream nodes
// still execute.
func TestExecuteTraced_SpanReflectsOutputError(t *testing.T) {
	tracer, err := tracelib.NewTracer(tracelib.Config{BufferSize: 10})
	require.NoError(t, err)

	d := New()
	a := &softFailNode{id: "A"}
	b := &countingNode{id: "B"}
	require.NoError(t, d.AddNode(a))
	require.NoError(t, d.AddNode(b))
	require.NoError(t, d.AddEdge("A", "B", EdgeNormal, ""))

	exec := NewExecutor(d, WithTraceHook(&HookAdapter{Tracer: tracer}, 1, 100))
	_, err = exec.ExecuteWithTrace(context.Background(), nil)
	require.NoError(t, err, "soft failure must not fail the chain")

	traces := tracer.List(1, 0)
	require.Len(t, traces, 1)
	spans := traces[0].Spans
	require.Len(t, spans, 2)

	spanByNode := map[string]tracelib.SpanStatus{}
	for _, s := range spans {
		spanByNode[s.NodeID] = s.Status
	}
	assert.Equal(t, tracelib.SpanStatusError, spanByNode["A"], "soft-failed node span must be error")
	assert.Equal(t, tracelib.SpanStatusOK, spanByNode["B"], "downstream node span must be ok")
	assert.True(t, b.executed, "downstream node must execute (not skip)")

	// Span error detail must carry the node's failure message.
	for _, s := range spans {
		if s.NodeID == "A" {
			assert.Contains(t, s.Error, "errorCode")
			assert.Contains(t, s.Error, "50028")
		}
	}
}

// TestExecuteTraced_HappyPathSpansOK guards that normal nodes keep ok spans.
func TestExecuteTraced_HappyPathSpansOK(t *testing.T) {
	tracer, err := tracelib.NewTracer(tracelib.Config{BufferSize: 10})
	require.NoError(t, err)

	d := New()
	a := &countingNode{id: "A"}
	b := &countingNode{id: "B"}
	require.NoError(t, d.AddNode(a))
	require.NoError(t, d.AddNode(b))
	require.NoError(t, d.AddEdge("A", "B", EdgeNormal, ""))

	exec := NewExecutor(d, WithTraceHook(&HookAdapter{Tracer: tracer}, 1, 101))
	_, err = exec.ExecuteWithTrace(context.Background(), nil)
	require.NoError(t, err)

	traces := tracer.List(1, 0)
	require.Len(t, traces, 1)
	for _, s := range traces[0].Spans {
		assert.Equal(t, tracelib.SpanStatusOK, s.Status, "node %s span should be ok", s.NodeID)
	}
}

// Compile-time guard: unused import avoidance.
var _ = time.Second
