package dag

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
	tracelib "github.com/yannick2025-tech/Salvo/internal/trace"
)

// TraceHook is called by the executor when a node starts and finishes.
// It decouples the executor from the concrete tracing implementation.
type TraceHook interface {
	// StartTrace creates a new trace context for a DAG run.
	StartTrace(ctx context.Context, sceneID, runID snowflake.ID) TraceContext
}

// TraceContext provides span recording within a single trace.
type TraceContext interface {
	// StartSpan begins a span for the given node ID.
	StartSpan(nodeID string) SpanContext
	// FinishTrace marks the trace as completed.
	FinishTrace()
	// FinishTraceWithError marks the trace as failed.
	FinishTraceWithError(err string)
}

// SpanContext records the outcome of a single node execution.
type SpanContext interface {
	// SetInput records the span input summary.
	SetInput(s string)
	// Finish completes the span with output and error.
	Finish(output string, err error)
	// Skip marks the span as skipped.
	Skip(reason string)
}

// --- Adapters for the trace package ---

// HookAdapter adapts a tracelib.Tracer to the TraceHook interface.
type HookAdapter struct {
	Tracer *tracelib.Tracer
}

// StartTrace creates a new trace context.
func (h *HookAdapter) StartTrace(ctx context.Context, sceneID, runID snowflake.ID) TraceContext {
	return &contextAdapter{
		tctx: h.Tracer.Start(ctx, sceneID, runID),
	}
}

// contextAdapter adapts tracelib.Context to TraceContext.
type contextAdapter struct {
	tctx *tracelib.Context
}

// StartSpan begins a new span.
func (c *contextAdapter) StartSpan(nodeID string) SpanContext {
	return &spanAdapter{
		builder: c.tctx.StartSpan(nodeID),
	}
}

// FinishTrace marks the trace as completed.
func (c *contextAdapter) FinishTrace() {
	c.tctx.Finish()
}

// FinishTraceWithError marks the trace as failed.
func (c *contextAdapter) FinishTraceWithError(err string) {
	c.tctx.FinishWithError(err)
}

// spanAdapter adapts tracelib.SpanBuilder to SpanContext.
type spanAdapter struct {
	builder *tracelib.SpanBuilder
}

// SetInput records the span input summary.
func (s *spanAdapter) SetInput(v string) {
	s.builder.SetInput(v)
}

// Finish completes the span.
func (s *spanAdapter) Finish(output string, err error) {
	s.builder.Finish(output, err)
}

// Skip marks the span as skipped.
func (s *spanAdapter) Skip(reason string) {
	s.builder.Skip(reason)
}

// WithTraceHook adds a trace hook to the executor. When set, the executor
// records a span for every node execution and a trace for the full DAG run.
func WithTraceHook(hook TraceHook, sceneID, runID snowflake.ID) ExecutorOption {
	return func(e *Executor) {
		e.traceHook = hook
		e.traceSceneID = sceneID
		e.traceRunID = runID
	}
}

// ExecuteWithTrace runs the DAG with trace instrumentation. It is a
// wrapper around Execute that starts a trace, records spans for each
// node, and finishes the trace when done.
func (e *Executor) ExecuteWithTrace(ctx context.Context) (*Output, error) {
	if e.traceHook == nil {
		return e.Execute(ctx)
	}

	tctx := e.traceHook.StartTrace(ctx, e.traceSceneID, e.traceRunID)
	defer func() {
		tctx.FinishTrace()
	}()

	out, err := e.executeTraced(ctx, tctx)
	if err != nil {
		tctx.FinishTraceWithError(err.Error())
	}
	return out, err
}

// executeTraced runs the DAG execution with per-node span recording.
func (e *Executor) executeTraced(ctx context.Context, tctx TraceContext) (*Output, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("executor context already cancelled: %w", err)
	}

	order, err := e.dag.TopologicalSort()
	if err != nil {
		return nil, fmt.Errorf("topological sort failed: %w", err)
	}

	errCh := make(chan error, len(order))
	signals := make(map[string]chan struct{})
	for _, id := range order {
		signals[id] = make(chan struct{})
	}

	var syncWg sync.WaitGroup

	for _, nodeID := range order {
		node, ok := e.dag.Node(nodeID)
		if !ok {
			return nil, fmt.Errorf("node %s not found during execution", nodeID)
		}

		if node.Mode() == ExecSync {
			syncWg.Add(1)
		}

		go func(n Node, sig chan struct{}) {
			isSync := n.Mode() == ExecSync
			if isSync {
				defer syncWg.Done()
			}

			inEdges := e.dag.InEdges(n.ID())
			for _, edge := range inEdges {
				parentSig := signals[edge.From]
				select {
				case <-parentSig:
				case <-ctx.Done():
					span := tctx.StartSpan(n.ID())
					span.Skip("parent cancelled")
					errCh <- fmt.Errorf("waiting for parent of %s: %w", n.ID(), ctx.Err())
					close(sig)
					return
				}

				if edge.Type == EdgeCondition {
					e.mu.RLock()
					parentOutput := e.results[edge.From]
					e.mu.RUnlock()

					if !e.evalCondition(ctx, edge.Condition, parentOutput) {
						span := tctx.StartSpan(n.ID())
						span.Skip(fmt.Sprintf("condition not met: %s", edge.Condition))
						close(sig)
						return
					}
				}
			}

			if !isSync {
				close(sig)
			}

			loopCount := n.LoopCount()
			if loopCount <= 0 {
				loopCount = 1
			}

			span := tctx.StartSpan(n.ID())

			var lastOutput *Output
			for i := 0; i < loopCount; i++ {
				select {
				case <-ctx.Done():
					span.Finish("", fmt.Errorf("node %s cancelled: %w", n.ID(), ctx.Err()))
					errCh <- fmt.Errorf("node %s cancelled: %w", n.ID(), ctx.Err())
					if isSync {
						close(sig)
					}
					return
				default:
				}

				input := e.buildInput(n.ID())
				inputJSON, _ := json.Marshal(input.Variables)
				span.SetInput(string(inputJSON))

				output, err := n.Execute(ctx, input)
				if err != nil {
					span.Finish("", err)
					errCh <- fmt.Errorf("node %s execute: %w", n.ID(), err)
					if isSync {
						close(sig)
					}
					return
				}
				lastOutput = output
			}

			outputJSON, _ := json.Marshal(lastOutput.Response)
			span.Finish(string(outputJSON), nil)

			e.mu.Lock()
			e.results[n.ID()] = lastOutput
			e.mu.Unlock()

			if isSync {
				close(sig)
			}
		}(node, signals[nodeID])
	}

	syncDone := make(chan struct{})
	go func() {
		syncWg.Wait()
		close(syncDone)
	}()

	select {
	case <-syncDone:
	case err := <-errCh:
		return nil, err
	case <-ctx.Done():
		return nil, fmt.Errorf("executor cancelled: %w", ctx.Err())
	}

	for i := len(order) - 1; i >= 0; i-- {
		node, _ := e.dag.Node(order[i])
		if node != nil && node.Mode() == ExecSync {
			e.mu.RLock()
			out, exists := e.results[order[i]]
			e.mu.RUnlock()
			if exists {
				return out, nil
			}
		}
	}

	return nil, nil
}
