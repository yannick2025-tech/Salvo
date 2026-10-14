package dag

import (
	"context"
	"fmt"
<<<<<<< HEAD
=======
	"runtime/debug"
>>>>>>> dev
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

<<<<<<< HEAD
=======
// WithConditionWarnLogger sets a warn-level logger for condition evaluation
// events (e.g. condition not met, node skipped).
func WithConditionWarnLogger(fn func(msg string, keysAndValues ...any)) ExecutorOption {
	return func(e *Executor) {
		e.logWarn = fn
	}
}

// WithConditionErrorLogger sets an error-level logger for condition evaluation
// failures (e.g. panic in condition evaluator).
func WithConditionErrorLogger(fn func(msg string, keysAndValues ...any)) ExecutorOption {
	return func(e *Executor) {
		e.logError = fn
	}
}

>>>>>>> dev
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
<<<<<<< HEAD
=======
	skipped       map[string]bool // nodes skipped due to conditional edge (not failed)
>>>>>>> dev
	mu            sync.RWMutex
	traceHook     TraceHook
	traceSceneID  snowflake.ID
	traceRunID    snowflake.ID
	initialVars   map[string]any
<<<<<<< HEAD
=======
	varsMu        sync.Mutex // protects initialVars for cross-node variable writes
	logWarn       func(msg string, keysAndValues ...any)
	logError      func(msg string, keysAndValues ...any)
	cancel        context.CancelFunc // cancel function for chain cancellation on block_on_error
}

// WithInitialVars sets the initial variables on the executor.
func WithInitialVars(vars map[string]any) ExecutorOption {
	return func(e *Executor) {
		e.initialVars = vars
	}
>>>>>>> dev
}

// NewExecutor creates a new Executor for the given DAG.
func NewExecutor(d *DAG, opts ...ExecutorOption) *Executor {
	e := &Executor{
		dag:     d,
		results: make(map[string]*Output),
<<<<<<< HEAD
=======
		skipped: make(map[string]bool),
>>>>>>> dev
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

<<<<<<< HEAD
=======
	// Create a cancellable context for chain-wide cancellation on block_on_error
	ctx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	defer cancel() // Ensure cleanup

>>>>>>> dev
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
<<<<<<< HEAD
=======
			// Always-run nodes (teardown) still execute when the chain is
			// cancelled or their parent failed — cleanup must always run.
			alwaysRun := false
			if ar, ok := n.(AlwaysRunner); ok {
				alwaysRun = ar.AlwaysRun()
			}
>>>>>>> dev
			if isSync {
				defer syncWg.Done()
			}

			// Wait for all parent nodes to signal readiness.
			inEdges := e.dag.InEdges(n.ID())
<<<<<<< HEAD
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
=======
			parentFailed := false
			hasActiveParent := false
			if e.logWarn != nil {
				e.logWarn("node waiting for parents",
					"node_id", n.ID(),
					"in_edge_count", len(inEdges),
				)
			}
			for _, edge := range inEdges {
				parentSig := signals[edge.From]
				if !alwaysRun {
					select {
					case <-parentSig:
					case <-ctx.Done():
						errCh <- fmt.Errorf("waiting for parent of %s: %w", n.ID(), ctx.Err())
						// Still close the signal so dependants don't hang.
						close(sig)
						return
					}
				} else {
					// Always-run nodes ignore chain cancellation while
					// waiting for parents: teardown must still execute
					// after the parent signals (success or failure).
					<-parentSig
				}
				e.mu.RLock()
				parentOutput, parentSucceeded := e.results[edge.From]
				e.mu.RUnlock()

				if edge.Type == EdgeCondition {
					// For conditional edges, a skipped parent is expected
					// when the condition is not met. This is OR-join
					// semantics: a skipped conditional parent does NOT
					// block the child. If the parent was skipped (no
					// result in map), this edge is inactive.
					if !parentSucceeded {
						if e.logWarn != nil {
							e.logWarn("conditional parent skipped (expected)",
								"node_id", n.ID(),
								"parent_id", edge.From,
								"condition", edge.Condition,
							)
						}
						continue
					}
					// Parent executed — evaluate the condition.
					if e.logWarn != nil {
						outputDebug := "<nil>"
						if parentOutput != nil {
							outputDebug = fmt.Sprintf("%v", parentOutput.Response)
						}
						e.logWarn("evaluating conditional edge",
							"condition", edge.Condition,
							"edge_from", edge.From,
							"edge_to", edge.To,
							"current_node", n.ID(),
							"parent_output_nil", parentOutput == nil,
							"parent_output_response", outputDebug,
						)
					}

					conditionMet := true
					func() {
						defer func() {
							if r := recover(); r != nil {
								if e.logError != nil {
									e.logError("condition evaluation panicked",
										"condition", edge.Condition,
										"edge_from", edge.From,
										"edge_to", edge.To,
										"panic", fmt.Sprintf("%v", r),
										"stacktrace", string(debug.Stack()),
									)
								}
								conditionMet = false
							}
						}()
						conditionMet = e.evalCondition(ctx, edge.Condition, parentOutput)
					}()

					if conditionMet {
						hasActiveParent = true
					}
					// Condition not met → this edge is inactive, but
					// don't skip the node yet; other edges may provide
					// an active path (OR-join).
				} else {
					// Normal edge: parent must succeed.
					// If the parent was skipped (due to conditional edge
					// upstream), treat this edge as inactive rather than
					// failed — OR-join semantics.
					if !parentSucceeded {
						e.mu.RLock()
						isSkipped := e.skipped[edge.From]
						e.mu.RUnlock()
						if isSkipped {
							// Parent was conditionally skipped, not failed.
							// This edge is inactive, don't set parentFailed.
						} else {
							parentFailed = true
						}
					} else {
						hasActiveParent = true
					}
					if e.logWarn != nil {
						outputDebug := "<no result>"
						if parentSucceeded && parentOutput != nil {
							outputDebug = fmt.Sprintf("%v", parentOutput.Response)
						}
						e.logWarn("parent signal received",
							"node_id", n.ID(),
							"parent_id", edge.From,
							"edge_type", edge.Type,
							"condition", edge.Condition,
							"parent_succeeded", parentSucceeded,
							"parent_failed_so_far", parentFailed,
							"parent_output", outputDebug,
						)
					}
				}
			}

			// Skip this node if any normal-edge parent failed.
			// If the node has parents but none are active (all conditional
			// edges were not met or parents skipped), also skip — no path
			// leads here. Nodes with zero parents (entry nodes) always execute.
			// Always-run nodes are exempt: teardown must execute even when
			// the parent failed or the chain was cancelled.
			if !alwaysRun && len(inEdges) > 0 && (parentFailed || !hasActiveParent) {
				if e.logWarn != nil {
					reason := "parent failed"
					if !hasActiveParent && !parentFailed {
						reason = "no active parent path"
					}
					e.logWarn("skipping node: "+reason,
						"node_id", n.ID(),
						"parent_failed", parentFailed,
						"has_active_parent", hasActiveParent,
					)
				}
				// Record this node as skipped so downstream merge nodes
				// can distinguish conditional skips from actual failures.
				e.mu.Lock()
				e.skipped[n.ID()] = true
				e.mu.Unlock()
				close(sig)
				return
>>>>>>> dev
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
<<<<<<< HEAD
				select {
				case <-ctx.Done():
					errCh <- fmt.Errorf("node %s cancelled: %w", n.ID(), ctx.Err())
					if isSync {
						close(sig)
					}
					return
				default:
=======
				if !alwaysRun {
					select {
					case <-ctx.Done():
						errCh <- fmt.Errorf("node %s cancelled: %w", n.ID(), ctx.Err())
						if isSync {
							close(sig)
						}
						return
					default:
					}
>>>>>>> dev
				}

				input := e.buildInput(n.ID())

<<<<<<< HEAD
				output, err := n.Execute(ctx, input)
				if err != nil {
=======
				// Create a per-node context: if the node has a Timeout > 0,
				// wrap the parent context with context.WithTimeout so the
				// node fails independently without killing the whole scene.
				// Always-run nodes detach from the chain context first so a
				// chain cancellation does not kill the cleanup request; the
				// node's own timeout still applies.
				nodeCtx := ctx
				if alwaysRun {
					nodeCtx = context.WithoutCancel(ctx)
				}
				nodeTimeout := n.Timeout()
				if nodeTimeout > 0 {
					var nodeCancel context.CancelFunc
					nodeCtx, nodeCancel = context.WithTimeout(nodeCtx, nodeTimeout)
					defer nodeCancel()
				}

				output, err := n.Execute(nodeCtx, input)
				if err != nil {
					// Check if this node should block the entire chain on error
					if n.BlockOnError() {
						if e.logError != nil {
							e.logError("chain cancelled due to block_on_error",
								"node_id", n.ID(),
								"error", err.Error(),
							)
						}
						// Cancel the entire chain
						cancel()
					}
>>>>>>> dev
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

<<<<<<< HEAD
	select {
	case <-syncDone:
	case err := <-errCh:
		return nil, err
	case <-ctx.Done():
		return nil, fmt.Errorf("executor cancelled: %w", ctx.Err())
=======
	var execErr error
	select {
	case <-syncDone:
	case err := <-errCh:
		execErr = err
	case <-ctx.Done():
		execErr = fmt.Errorf("executor cancelled: %w", ctx.Err())
	}
	if execErr != nil {
		// Always-run nodes (teardown) are still executing on a cancelled
		// chain — wait for them so cleanup results are recorded and no
		// goroutine outlives the chain before returning the error.
		<-syncDone
		return nil, execErr
>>>>>>> dev
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
<<<<<<< HEAD
	
	// Start with initial variables
	variables := make(map[string]any)
=======

	// Start with initial variables (use lock to avoid race condition with SetVariable)
	variables := make(map[string]any)
	e.varsMu.Lock()
>>>>>>> dev
	if e.initialVars != nil {
		for k, v := range e.initialVars {
			variables[k] = v
		}
	}
<<<<<<< HEAD
	
	if len(inEdges) == 0 {
		return &Input{Variables: variables}
=======
	e.varsMu.Unlock()

	if len(inEdges) == 0 {
		return &Input{Variables: variables, Executor: e}
>>>>>>> dev
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	var lastParentOutput *Output
	for _, edge := range inEdges {
		if out, ok := e.results[edge.From]; ok {
			lastParentOutput = out
<<<<<<< HEAD
=======
			// Merge parent output variables into input variables.
			// This ensures variables set by generator nodes (e.g. jwt_token)
			// are available to subsequent nodes.
			if out.Variables != nil {
				for k, v := range out.Variables {
					variables[k] = v
				}
			}
>>>>>>> dev
		}
	}

	input := &Input{
		Variables: variables,
<<<<<<< HEAD
=======
		Executor:  e,
>>>>>>> dev
	}
	if lastParentOutput != nil {
		input.Response = lastParentOutput.Response
	}
	return input
}
<<<<<<< HEAD
=======

// SetVariable writes a value back to the shared initialVars map,
// making it available to subsequent nodes in the DAG.
func (e *Executor) SetVariable(key string, value any) {
	e.varsMu.Lock()
	if e.initialVars == nil {
		e.initialVars = make(map[string]any)
	}
	e.initialVars[key] = value
	e.varsMu.Unlock()
}

// SnapshotVariables returns a copy of the current shared variables.
// Composite nodes (group/while/loop) refresh the input variable map from
// this snapshot after each child step so that variables extracted by one
// child (via SetVariable) are immediately visible to the next child and
// to loop exit conditions.
func (e *Executor) SnapshotVariables() map[string]any {
	e.varsMu.Lock()
	defer e.varsMu.Unlock()
	vars := make(map[string]any, len(e.initialVars))
	for k, v := range e.initialVars {
		vars[k] = v
	}
	return vars
}
>>>>>>> dev
