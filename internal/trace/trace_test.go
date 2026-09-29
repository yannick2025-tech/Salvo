package trace

import (
	"context"
	"errors"
	"fmt"
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

func TestTracerFinishIsIdempotent(t *testing.T) {
	tracer, err := NewTracer(Config{BufferSize: 100})
	require.NoError(t, err)

	tctx := tracer.Start(context.Background(), newID(t), newID(t))
	tctx.StartSpan("node-A").Finish(`{"status":200}`, nil)

	// Mirror dag.ExecuteWithTrace: the error branch finishes the trace,
	// then the deferred Finish runs a second time on the same context.
	tctx.FinishWithError("boom")
	tctx.Finish()

	traces := tracer.List(10, 0)
	require.Len(t, traces, 1, "double Finish must not record the trace twice")
	assert.Equal(t, SpanStatusError, traces[0].Status)
	assert.Equal(t, "boom", traces[0].Error)
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

	traces := tracer.List(3, 0)
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

	traces := tracer.ListByScene(sceneID, 10, 0)
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

func TestNodeStatsCumulativeAcrossIterations(t *testing.T) {
	tracer, err := NewTracer(Config{BufferSize: 100})
	require.NoError(t, err)

	runID := newID(t)
	tctx := tracer.Start(context.Background(), newID(t), runID)
	span := tctx.StartSpan("node-iter")
	span.SetChainID("chain-1")

	// Simulate 3 loop iterations: fail, fail, success. Cumulative counters
	// must keep both failures; running markers must be cleared per iteration.
	span.BroadcastRunning(0)
	span.BroadcastIterationResult(0, true)
	span.BroadcastRunning(1)
	span.BroadcastIterationResult(1, true)
	span.BroadcastRunning(2)
	span.BroadcastIterationResult(2, false)
	span.Finish(`{"status":200}`, nil)
	tctx.Finish()

	stats := tracer.SnapshotRunStats(runID)
	ns := stats["chain-1"]["node-iter"]
	require.NotNil(t, ns)
	assert.Equal(t, 2, ns.Fail, "both failed iterations must be counted")
	assert.Equal(t, 1, ns.Pass, "final iteration success must be counted once")
	assert.Equal(t, 0, ns.Skip)
	assert.Empty(t, ns.RunningIdx, "no in-flight iterations after finish")
	assert.Equal(t, 2, ns.LastIndex)
}

// TestRunStatsAggregateAcrossChains reproduces the page re-entry bug: the
// runner spawns each chain as its own trace under the same run ID, so the
// run-level snapshot must aggregate every chain — not just the latest one.
func TestRunStatsAggregateAcrossChains(t *testing.T) {
	tracer, err := NewTracer(Config{BufferSize: 100})
	require.NoError(t, err)

	runID := newID(t)

	// Simulate 45 finished chains, each executing one node iteration.
	for c := 0; c < 45; c++ {
		tctx := tracer.Start(context.Background(), newID(t), runID)
		span := tctx.StartSpan("node-group")
		span.SetChainID(fmt.Sprintf("chain-%d", c))
		span.BroadcastRunning(0)
		span.BroadcastIterationResult(0, c%7 == 0) // every 7th chain fails
		span.Finish("ok", nil)
		tctx.Finish()
	}

	stats := tracer.SnapshotRunStats(runID)
	require.Len(t, stats, 45)
	var pass, fail int
	for _, nodes := range stats {
		ns := nodes["node-group"]
		require.NotNil(t, ns)
		pass += ns.Pass
		fail += ns.Fail
	}
	assert.Equal(t, 38, pass, "snapshot must aggregate all chains, not just the latest")
	assert.Equal(t, 7, fail)
}

func TestNodeStatsFinishDoesNotDoubleCount(t *testing.T) {
	tracer, err := NewTracer(Config{BufferSize: 100})
	require.NoError(t, err)

	tctx := tracer.Start(context.Background(), newID(t), newID(t))
	span := tctx.StartSpan("node-dbl")
	span.SetChainID("chain-1")

	// The executor broadcasts the final iteration result, then Finish
	// repeats the same loop index: the duplicate must be ignored.
	span.BroadcastRunning(0)
	span.BroadcastIterationResult(0, false)
	span.Finish("ok", nil)
	tctx.Finish()

	ns := tracer.SnapshotRunStats(tctx2RunID(tctx))["chain-1"]["node-dbl"]
	require.NotNil(t, ns)
	assert.Equal(t, 1, ns.Pass, "Finish broadcast must not double-count the final iteration")
}

func TestNodeStatsCanceledClearsRunning(t *testing.T) {
	tracer, err := NewTracer(Config{BufferSize: 100})
	require.NoError(t, err)

	tctx := tracer.Start(context.Background(), newID(t), newID(t))
	span := tctx.StartSpan("node-cancel")
	span.SetChainID("chain-1")

	// Iteration 4 is in flight when the run is manually stopped.
	span.BroadcastRunning(4)
	span.FinishCanceled("", errors.New("context canceled"))
	tctx.Finish()

	ns := tracer.SnapshotRunStats(tctx2RunID(tctx))["chain-1"]["node-cancel"]
	require.NotNil(t, ns)
	assert.Empty(t, ns.RunningIdx, "canceled iteration must not stay running")
	assert.Equal(t, 1, ns.Skip, "canceled iteration counts as skip")
}

func TestFinishRunClearsAllRunning(t *testing.T) {
	tracer, err := NewTracer(Config{BufferSize: 100})
	require.NoError(t, err)

	runID := newID(t)
	tctx := tracer.Start(context.Background(), newID(t), runID)
	spanA := tctx.StartSpan("node-a")
	spanA.SetChainID("chain-1")
	spanB := tctx.StartSpan("node-b")
	spanB.SetChainID("chain-2")
	spanA.BroadcastRunning(0)
	spanB.BroadcastRunning(3)
	// Neither node finishes: the run-level finish must clear both markers
	// while keeping the chains visible for the snapshot.
	tctx.Finish()
	tracer.FinishRun(runID)

	stats := tracer.SnapshotRunStats(runID)
	assert.Empty(t, stats["chain-1"]["node-a"].RunningIdx)
	assert.Empty(t, stats["chain-2"]["node-b"].RunningIdx)
}

func TestSnapshotStatsConcurrentWithBroadcast(t *testing.T) {
	tracer, err := NewTracer(Config{BufferSize: 100})
	require.NoError(t, err)

	runID := newID(t)
	tctx := tracer.Start(context.Background(), newID(t), runID)

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			span := tctx.StartSpan("node-race")
			span.SetChainID("chain-race")
			for i := 0; i < 50; i++ {
				span.BroadcastRunning(i)
				span.BroadcastIterationResult(i, i%2 == 0)
			}
		}()
	}
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				_ = tracer.SnapshotRunStats(runID)
			}
		}()
	}
	wg.Wait()
	tctx.Finish()
}

// tctx2RunID extracts the run ID from a trace context for assertions.
func tctx2RunID(c *Context) snowflake.ID {
	return c.trace.RunID
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

	traces := tracer.List(10, 0)
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

type broadcastCall struct {
	runID      string
	chainID    string
	nodeID     string
	status     string
	durationNs int64
	errMsg     string
	loopIndex  int
}

func TestTracer_BroadcastOnFinish(t *testing.T) {
	var calls []broadcastCall
	tracer, err := NewTracer(Config{
		BufferSize: 100,
		Broadcast: func(runID string, chainID string, nodeID string, status string, durationNs int64, errMsg string, loopIndex int) {
			calls = append(calls, broadcastCall{
				runID:      runID,
				chainID:    chainID,
				nodeID:     nodeID,
				status:     status,
				durationNs: durationNs,
				errMsg:     errMsg,
				loopIndex:  loopIndex,
			})
		},
	})
	require.NoError(t, err)

	runID := newID(t)
	tctx := tracer.Start(context.Background(), newID(t), runID)
	span := tctx.StartSpan("node-X")
	span.SetChainID("chain-1")
	span.Finish(`{"status":200}`, nil)
	tctx.Finish()

	require.Len(t, calls, 1)
	assert.Equal(t, runID.String(), calls[0].runID)
	assert.Equal(t, "chain-1", calls[0].chainID)
	assert.Equal(t, "node-X", calls[0].nodeID)
	assert.Equal(t, "ok", calls[0].status)
	assert.Equal(t, "", calls[0].errMsg)
	assert.Equal(t, 0, calls[0].loopIndex)
}

func TestTracer_BroadcastOnFinishWithError(t *testing.T) {
	var calls []broadcastCall
	tracer, err := NewTracer(Config{
		BufferSize: 100,
		Broadcast: func(runID string, chainID string, nodeID string, status string, durationNs int64, errMsg string, loopIndex int) {
			calls = append(calls, broadcastCall{
				runID:      runID,
				chainID:    chainID,
				nodeID:     nodeID,
				status:     status,
				durationNs: durationNs,
				errMsg:     errMsg,
				loopIndex:  loopIndex,
			})
		},
	})
	require.NoError(t, err)

	runID := newID(t)
	tctx := tracer.Start(context.Background(), newID(t), runID)
	span := tctx.StartSpan("node-err")
	span.SetChainID("chain-2")
	span.Finish("", errors.New("timeout"))
	tctx.Finish()

	require.Len(t, calls, 1)
	assert.Equal(t, runID.String(), calls[0].runID)
	assert.Equal(t, "chain-2", calls[0].chainID)
	assert.Equal(t, "node-err", calls[0].nodeID)
	assert.Equal(t, "error", calls[0].status)
	assert.Equal(t, "timeout", calls[0].errMsg)
	assert.Equal(t, 0, calls[0].loopIndex)
}

func TestTracer_BroadcastOnSkip(t *testing.T) {
	var calls []broadcastCall
	tracer, err := NewTracer(Config{
		BufferSize: 100,
		Broadcast: func(runID string, chainID string, nodeID string, status string, durationNs int64, errMsg string, loopIndex int) {
			calls = append(calls, broadcastCall{
				runID:      runID,
				chainID:    chainID,
				nodeID:     nodeID,
				status:     status,
				durationNs: durationNs,
				errMsg:     errMsg,
				loopIndex:  loopIndex,
			})
		},
	})
	require.NoError(t, err)

	runID := newID(t)
	tctx := tracer.Start(context.Background(), newID(t), runID)
	span := tctx.StartSpan("node-skip")
	span.SetChainID("chain-3")
	span.Skip("condition not met")
	tctx.Finish()

	require.Len(t, calls, 1)
	assert.Equal(t, runID.String(), calls[0].runID)
	assert.Equal(t, "chain-3", calls[0].chainID)
	assert.Equal(t, "node-skip", calls[0].nodeID)
	assert.Equal(t, "skip", calls[0].status)
	assert.Equal(t, "condition not met", calls[0].errMsg)
	assert.Equal(t, 0, calls[0].loopIndex)
}

func TestTracer_BroadcastOnCanceled(t *testing.T) {
	var calls []broadcastCall
	tracer, err := NewTracer(Config{
		BufferSize: 100,
		Broadcast: func(runID string, chainID string, nodeID string, status string, durationNs int64, errMsg string, loopIndex int) {
			calls = append(calls, broadcastCall{
				runID:      runID,
				chainID:    chainID,
				nodeID:     nodeID,
				status:     status,
				durationNs: durationNs,
				errMsg:     errMsg,
				loopIndex:  loopIndex,
			})
		},
	})
	require.NoError(t, err)

	runID := newID(t)
	tctx := tracer.Start(context.Background(), newID(t), runID)
	span := tctx.StartSpan("node-cancel")
	span.SetChainID("chain-4")
	span.FinishCanceled("", errors.New("manual stop"))
	tctx.Finish()

	require.Len(t, calls, 1)
	assert.Equal(t, runID.String(), calls[0].runID)
	assert.Equal(t, "chain-4", calls[0].chainID)
	assert.Equal(t, "node-cancel", calls[0].nodeID)
	assert.Equal(t, "canceled", calls[0].status)
	assert.Equal(t, "manual stop", calls[0].errMsg)
	assert.Equal(t, 0, calls[0].loopIndex)
}

func TestByRunIDFindsInFlightTrace(t *testing.T) {
	tracer, err := NewTracer(Config{BufferSize: 100})
	require.NoError(t, err)

	sceneID := newID(t)
	runID := newID(t)

	tctx := tracer.Start(context.Background(), sceneID, runID)

	// The trace must be discoverable BEFORE the run finishes — this is
	// what the WebSocket subscribe snapshot and the live trace endpoint
	// rely on to show a running scene's history.
	tr, ok := tracer.ByRunID(runID)
	require.True(t, ok, "ByRunID must find the in-flight trace")
	assert.Equal(t, runID, tr.RunID)

	// Finished spans recorded on the live trace are visible via the
	// locked snapshot, while concurrent AddSpan calls are safe.
	tctx.StartSpan("node-A").Finish("ok", nil)
	spans := tr.SnapshotSpans()
	assert.Len(t, spans, 1)
	assert.Equal(t, "node-A", spans[0].NodeID)

	// After Finish the trace moves from active to the completed buffer
	// and remains discoverable.
	tctx.Finish()
	tr, ok = tracer.ByRunID(runID)
	require.True(t, ok, "ByRunID must still find the completed trace")
	assert.Equal(t, SpanStatusOK, tr.Status)

	// A different run ID must not resolve.
	_, ok = tracer.ByRunID(newID(t))
	assert.False(t, ok)
}

func TestSnapshotSpansConcurrentWithAddSpan(t *testing.T) {
	tracer, err := NewTracer(Config{BufferSize: 100})
	require.NoError(t, err)

	runID := newID(t)
	tctx := tracer.Start(context.Background(), newID(t), runID)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				tctx.StartSpan("node").Finish("ok", nil)
			}
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 100; j++ {
			_ = tctx.trace.SnapshotSpans()
		}
	}()
	wg.Wait()

	assert.Len(t, tctx.trace.SnapshotSpans(), 400)
	tctx.Finish()
}
