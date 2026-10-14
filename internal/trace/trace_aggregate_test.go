package trace

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFinishAggregatesErrorSpan verifies that a trace finished without an
// explicit error is marked error when any span errored (soft failure).
func TestFinishAggregatesErrorSpan(t *testing.T) {
	tracer, err := NewTracer(Config{BufferSize: 10})
	require.NoError(t, err)

	tctx := tracer.Start(context.Background(), newID(t), newID(t))

	ok := tctx.StartSpan("node-A")
	ok.Finish(`{"status":200}`, nil)

	bad := tctx.StartSpan("node-B")
	bad.Finish(`{"errorCode":50028}`, errors.New(`expect_body field "errorCode": expected 0, got 50028`))

	tctx.Finish()

	tr, found := tracer.Get(tctx.TraceID())
	require.True(t, found)
	assert.Equal(t, SpanStatusError, tr.Status, "trace must aggregate error spans")
	assert.Contains(t, tr.Error, "node-B")
	assert.Contains(t, tr.Error, "errorCode")
}

// TestFinishCanceledKeepsPriorityOverErrorSpan verifies an explicit canceled
// status is not overwritten by span aggregation.
func TestFinishCanceledKeepsPriorityOverErrorSpan(t *testing.T) {
	tracer, err := NewTracer(Config{BufferSize: 10})
	require.NoError(t, err)

	tctx := tracer.Start(context.Background(), newID(t), newID(t))

	bad := tctx.StartSpan("node-A")
	bad.Finish("", errors.New("boom"))

	tctx.FinishWithCanceled("manual stop")

	tr, found := tracer.Get(tctx.TraceID())
	require.True(t, found)
	assert.Equal(t, SpanStatusCanceled, tr.Status, "explicit canceled status must win")
	assert.Equal(t, "manual stop", tr.Error)
}

// TestFinishAllOkStaysOk verifies no aggregation kicks in when all spans are ok.
func TestFinishAllOkStaysOk(t *testing.T) {
	tracer, err := NewTracer(Config{BufferSize: 10})
	require.NoError(t, err)

	tctx := tracer.Start(context.Background(), newID(t), newID(t))

	a := tctx.StartSpan("node-A")
	a.Finish("", nil)
	b := tctx.StartSpan("node-B")
	b.Skip("no active parent path")

	tctx.Finish()

	tr, found := tracer.Get(tctx.TraceID())
	require.True(t, found)
	assert.Equal(t, SpanStatusOK, tr.Status)
	assert.Empty(t, tr.Error)
}
