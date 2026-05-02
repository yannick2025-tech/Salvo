package trace

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
)

var (
	testNodeOnce sync.Once
	testNode     *snowflake.Node
)

func getTestNode(t *testing.T) *snowflake.Node {
	t.Helper()
	testNodeOnce.Do(func() {
		var err error
		testNode, err = snowflake.NewNode(1)
		require.NoError(t, err)
	})
	return testNode
}

func newID(t *testing.T) snowflake.ID {
	t.Helper()
	return getTestNode(t).Generate()
}

func TestTracerStartAndFinish(t *testing.T) {
	tracer, err := NewTracer(Config{BufferSize: 100})
	require.NoError(t, err)

	sceneID := newID(t)
	runID := newID(t)

	tctx := tracer.Start(context.Background(), sceneID, runID)
	assert.NotZero(t, tctx.TraceID())

	span := tctx.StartSpan("node-A")
	span.Finish(`{"status":200}`, nil)

	tctx.Finish()

	tr, ok := tracer.Get(tctx.TraceID())
	assert.True(t, ok)
	assert.Equal(t, sceneID, tr.SceneID)
	assert.Equal(t, runID, tr.RunID)
	assert.Equal(t, SpanStatusOK, tr.Status)
	assert.Len(t, tr.Spans, 1)
	assert.Equal(t, "node-A", tr.Spans[0].NodeID)
	assert.Equal(t, SpanStatusOK, tr.Spans[0].Status)
	assert.True(t, tr.Duration > 0)
}

func TestTracerSpanWithError(t *testing.T) {
	tracer, err := NewTracer(Config{BufferSize: 100})
	require.NoError(t, err)

	tctx := tracer.Start(context.Background(), newID(t), newID(t))
	span := tctx.StartSpan("node-B")
	span.Finish("", errors.New("connection refused"))

	tctx.Finish()

	tr, _ := tracer.Get(tctx.TraceID())
	assert.Len(t, tr.Spans, 1)
	assert.Equal(t, SpanStatusError, tr.Spans[0].Status)
	assert.Equal(t, "connection refused", tr.Spans[0].Error)
}

func TestTracerSpanSkip(t *testing.T) {
	tracer, err := NewTracer(Config{BufferSize: 100})
	require.NoError(t, err)

	tctx := tracer.Start(context.Background(), newID(t), newID(t))
	span := tctx.StartSpan("node-C")
	span.Skip("condition not met: status != 200")

	tctx.Finish()

	tr, _ := tracer.Get(tctx.TraceID())
	assert.Len(t, tr.Spans, 1)
	assert.Equal(t, SpanStatusSkip, tr.Spans[0].Status)
	assert.Contains(t, tr.Spans[0].Error, "condition not met")
}

func TestTracerFinishWithError(t *testing.T) {
	tracer, err := NewTracer(Config{BufferSize: 100})
	require.NoError(t, err)

	tctx := tracer.Start(context.Background(), newID(t), newID(t))
	tctx.FinishWithError("timeout exceeded")

	tr, _ := tracer.Get(tctx.TraceID())
	assert.Equal(t, SpanStatusError, tr.Status)
	assert.Equal(t, "timeout exceeded", tr.Error)
}

func TestTracerSpanInput(t *testing.T) {
	tracer, err := NewTracer(Config{BufferSize: 100})
	require.NoError(t, err)

	tctx := tracer.Start(context.Background(), newID(t), newID(t))
	span := tctx.StartSpan("node-D")
	span.SetInput(`{"url":"/api/login"}`)
	span.Finish(`{"token":"abc"}`, nil)

	tctx.Finish()

	tr, _ := tracer.Get(tctx.TraceID())
	assert.Equal(t, `{"url":"/api/login"}`, tr.Spans[0].Input)
	assert.Equal(t, `{"token":"abc"}`, tr.Spans[0].Output)
}

func TestTracerList(t *testing.T) {
	tracer, err := NewTracer(Config{BufferSize: 100})
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		tctx := tracer.Start(context.Background(), newID(t), newID(t))
		tctx.Finish()
	}

	traces := tracer.List(3)
	assert.Len(t, traces, 3)
}

func TestTracerListByScene(t *testing.T) {
	tracer, err := NewTracer(Config{BufferSize: 100})
	require.NoError(t, err)

	sceneID := newID(t)
	for i := 0; i < 3; i++ {
		tctx := tracer.Start(context.Background(), sceneID, newID(t))
		tctx.Finish()
	}
	tctx := tracer.Start(context.Background(), newID(t), newID(t))
	tctx.Finish()

	traces := tracer.ListByScene(sceneID, 10)
	assert.Len(t, traces, 3)
	for _, tr := range traces {
		assert.Equal(t, sceneID, tr.SceneID)
	}
}

func TestTracerByRunID(t *testing.T) {
	tracer, err := NewTracer(Config{BufferSize: 100})
	require.NoError(t, err)

	runID := newID(t)
	tctx := tracer.Start(context.Background(), newID(t), runID)
	tctx.Finish()

	tr, ok := tracer.ByRunID(runID)
	assert.True(t, ok)
	assert.Equal(t, runID, tr.RunID)

	_, ok = tracer.ByRunID(newID(t))
	assert.False(t, ok)
}

func TestTracerBufferEviction(t *testing.T) {
	tracer, err := NewTracer(Config{BufferSize: 3})
	require.NoError(t, err)

	var firstTraceID snowflake.ID
	for i := 0; i < 5; i++ {
		tctx := tracer.Start(context.Background(), newID(t), newID(t))
		tctx.Finish()
		if i == 0 {
			firstTraceID = tctx.TraceID()
		}
	}

	_, ok := tracer.Get(firstTraceID)
	assert.False(t, ok, "oldest trace should be evicted")

	traces := tracer.List(10)
	assert.Len(t, traces, 3)
}

func TestTracerGetNotFound(t *testing.T) {
	tracer, err := NewTracer(Config{BufferSize: 100})
	require.NoError(t, err)

	_, ok := tracer.Get(newID(t))
	assert.False(t, ok)
}

func TestTracerDefaultBufferSize(t *testing.T) {
	tracer, err := NewTracer(Config{})
	require.NoError(t, err)
	assert.Equal(t, 1000, tracer.cfg.BufferSize)
}

func TestSpanTiming(t *testing.T) {
	tracer, err := NewTracer(Config{BufferSize: 100})
	require.NoError(t, err)

	tctx := tracer.Start(context.Background(), newID(t), newID(t))
	span := tctx.StartSpan("timed-node")
	time.Sleep(10 * time.Millisecond)
	span.Finish("", nil)
	tctx.Finish()

	tr, _ := tracer.Get(tctx.TraceID())
	assert.True(t, tr.Spans[0].Duration >= 10*time.Millisecond)
	assert.False(t, tr.Spans[0].StartedAt.IsZero())
	assert.False(t, tr.Spans[0].FinishedAt.IsZero())
}

func TestMultipleSpansInTrace(t *testing.T) {
	tracer, err := NewTracer(Config{BufferSize: 100})
	require.NoError(t, err)

	tctx := tracer.Start(context.Background(), newID(t), newID(t))
	tctx.StartSpan("node-1").Finish("ok1", nil)
	tctx.StartSpan("node-2").Finish("ok2", nil)
	tctx.StartSpan("node-3").Skip("skipped")
	tctx.Finish()

	tr, _ := tracer.Get(tctx.TraceID())
	assert.Len(t, tr.Spans, 3)
	assert.Equal(t, SpanStatusOK, tr.Spans[0].Status)
	assert.Equal(t, SpanStatusOK, tr.Spans[1].Status)
	assert.Equal(t, SpanStatusSkip, tr.Spans[2].Status)
}

func TestTraceConcurrentSpans(t *testing.T) {
	tracer, err := NewTracer(Config{BufferSize: 100})
	require.NoError(t, err)

	tctx := tracer.Start(context.Background(), newID(t), newID(t))

	done := make(chan struct{}, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			span := tctx.StartSpan("concurrent-node")
			span.Finish("done", nil)
			done <- struct{}{}
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
	tctx.Finish()

	tr, _ := tracer.Get(tctx.TraceID())
	assert.Len(t, tr.Spans, 10)
}
