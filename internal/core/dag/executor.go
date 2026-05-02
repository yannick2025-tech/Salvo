package dag

import (
	"context"
	"fmt"
	"sync"

	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
)

// ConditionEvaluator decides whether a conditional edge should be traversed.
// It receives the condition expression string and the output of the source node.
type ConditionEvaluator func(ctx context.Context, condition string, output *Output) bool

// ExecutorOption configures an Executor during construction.
type ExecutorOption func(*Executor)

// WithConditionEvaluator sets a custom condition evaluator.
// By default all conditional edges are traversed (evaluator returns true).
func WithConditionEvaluator(fn ConditionEvaluator) ExecutorOption {
	return func(e *Executor) {
		e.evalCondition = fn
	}
}

// Executor traverses a DAG and executes each node respecting dependencies,
// execution modes (sync/async), loop counts, and conditional edges.
//
// Sync nodes block their dependants until they complete. Async nodes signal
// their dependants immediately (without waiting for execution to finish) so
// that downstream nodes are not blocked by slow async operations.
type Executor struct {
	dag           *DAG
	evalCondition ConditionEvaluator
	results       map[string]*Output
	mu            sync.RWMutex
	traceHook     TraceHook
	traceSceneID  snowflake.ID
	traceRunID    snowflake.ID
}

// NewExecutor creates a new Executor for the given DAG.
func NewExecutor(d *DAG, opts ...ExecutorOption) *Executor {
	e := &Executor{
		dag:     d,
		results: make(map[string]*Output),
		evalCondition: func(_ context.Context, _ string, _ *Output) bool {
			return true
		},
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Execute runs all nodes in the DAG respecting the dependency order.
// It returns the output of the last sync leaf node or an error if the
// context is cancelled. Async nodes are fire-and-forget; their results
// are stored but do not block the return.
func (e *Executor) Execute(ctx context.Context) (*Output, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("executor context already cancelled: %w", err)
	}

	order, err := e.dag.TopologicalSort()
	if err != nil {
		return nil, fmt.Errorf("topological sort failed: %w", err)
	}

	errCh := make(chan error, len(order))

	// Signal channels: each node gets a channel that is closed when the
	// node is "ready" — for sync nodes this means after execution; for
	// async nodes this means immediately after starting (fire-and-forget).
	signals := make(map[string]chan struct{})
	for _, id := range order {
		signals[id] = make(chan struct{})
	}

	// syncWg tracks only sync nodes; Execute returns when all sync nodes
	// are done. Async nodes run in the background.
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

			// Wait for all parent nodes to signal readiness.
			inEdges := e.dag.InEdges(n.ID())
			for _, edge := range inEdges {
				parentSig := signals[edge.From]
				select {
				case <-parentSig:
				case <-ctx.Done():
					errCh <- fmt.Errorf("waiting for parent of %s: %w", n.ID(), ctx.Err())
					// Still close the signal so dependants don't hang.
					close(sig)
					return
				}

				// For conditional edges, evaluate the condition.
				if edge.Type == EdgeCondition {
					e.mu.RLock()
					parentOutput := e.results[edge.From]
					e.mu.RUnlock()

					if !e.evalCondition(ctx, edge.Condition, parentOutput) {
						// Condition not met; skip this node but signal
						// dependants so they don't hang.
						close(sig)
						return
					}
				}
			}

			// For async nodes, signal dependants immediately (fire-and-forget).
			// The node still executes, but downstream nodes are unblocked.
			if !isSync {
				close(sig)
			}

			// Execute the node (with loop count).
			loopCount := n.LoopCount()
			if loopCount <= 0 {
				loopCount = 1
			}

			var lastOutput *Output
			for i := 0; i < loopCount; i++ {
				select {
				case <-ctx.Done():
					errCh <- fmt.Errorf("node %s cancelled: %w", n.ID(), ctx.Err())
					if isSync {
						close(sig)
					}
					return
				default:
				}

				input := e.buildInput(n.ID())

				output, err := n.Execute(ctx, input)
				if err != nil {
					errCh <- fmt.Errorf("node %s execute: %w", n.ID(), err)
					if isSync {
						close(sig)
					}
					return
				}
				lastOutput = output
			}

			e.mu.Lock()
			e.results[n.ID()] = lastOutput
			e.mu.Unlock()

			// For sync nodes, signal dependants after execution completes.
			if isSync {
				close(sig)
			}
		}(node, signals[nodeID])
	}

	// Wait for all sync nodes to complete.
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

	// Return the output of the last sync leaf node.
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

// Results returns a copy of all node outputs keyed by node ID.
func (e *Executor) Results() map[string]*Output {
	e.mu.RLock()
	defer e.mu.RUnlock()

	cp := make(map[string]*Output, len(e.results))
	for k, v := range e.results {
		cp[k] = v
	}
	return cp
}

// buildInput constructs the Input for a node by collecting parent outputs
// for parameter correlation.
func (e *Executor) buildInput(nodeID string) *Input {
	inEdges := e.dag.InEdges(nodeID)
	if len(inEdges) == 0 {
		return &Input{Variables: make(map[string]any)}
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	var lastParentOutput *Output
	for _, edge := range inEdges {
		if out, ok := e.results[edge.From]; ok {
			lastParentOutput = out
		}
	}

	input := &Input{
		Variables: make(map[string]any),
	}
	if lastParentOutput != nil {
		input.Response = lastParentOutput.Response
	}
	return input
}
