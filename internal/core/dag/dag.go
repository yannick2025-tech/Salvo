// Package dag implements a Directed Acyclic Graph data structure and executor
// for the Salvo test engine.
//
// A DAG models a test scenario as a set of Nodes connected by Edges. Nodes
// represent individual API calls (or protocol operations), while edges
// represent execution dependencies. The DAG supports:
//   - Synchronous and asynchronous execution modes per node
//   - Conditional edges for branching logic
//   - Loop counts for repeated invocations
//   - Fan-out (parallel children) and fan-in (wait all parents)
//   - Cycle detection at insertion time
package dag

import (
	"context"
	"time"
)

// ExecMode determines whether a node blocks its dependants until it completes.
type ExecMode string

const (
	// ExecSync means downstream nodes wait for this node to finish.
	ExecSync ExecMode = "sync"
	// ExecAsync means downstream nodes proceed without waiting.
	ExecAsync ExecMode = "async"
)

// EdgeType classifies the kind of dependency between two nodes.
type EdgeType string

const (
	// EdgeNormal is an unconditional dependency.
	EdgeNormal EdgeType = "normal"
	// EdgeCondition is a conditional dependency; the edge is traversed only
	// when its Condition expression evaluates to true.
	EdgeCondition EdgeType = "condition"
)

// Input carries the data available to a Node during execution.
type Input struct {
	// Variables holds the current variable scope (global + scene + API).
	Variables map[string]any
	// Response carries the output of the immediate predecessor node
	// (for parameter correlation).
	Response any
	// Executor provides access to the DAG executor for cross-node operations
	// (e.g. writing variables back to the shared scope).
	Executor *Executor
}

// Output is the result produced by a Node's Execute method.
type Output struct {
	// Response is the payload returned by the node (e.g. HTTP response body).
	Response any
	// Error is set when the node execution failed.
	Error error
	// Variables is a map of variable assignments to propagate to subsequent nodes.
	Variables map[string]any
}

// Node is the core abstraction for a single step in a test scenario.
// Implementations must be safe for concurrent use when the DAG executor
// runs multiple branches in parallel.
type Node interface {
	// ID returns the unique identifier of this node within the DAG.
	ID() string
	// Execute runs the node's logic. The context carries timeout and
	// cancellation signals; the input provides variables and upstream data.
	Execute(ctx context.Context, input *Input) (*Output, error)
	// Timeout returns the per-node execution timeout. Zero means no timeout.
	Timeout() time.Duration
	// LoopCount returns how many times this node should be invoked per
	// traversal. A value of 0 or 1 means execute once.
	LoopCount() int
	// Mode returns whether this node blocks its dependants (sync) or not
	// (async).
	Mode() ExecMode
}

// Edge represents a directed connection from one node to another.
type Edge struct {
	// From is the source node ID.
	From string
	// To is the destination node ID.
	To string
	// Type classifies the edge as normal or conditional.
	Type EdgeType
	// Condition is an expression string evaluated when Type is EdgeCondition.
	Condition string
}

// DAG is a mutable directed acyclic graph of test Nodes.
// It validates cycle-freedom at edge insertion time, so a successfully
// constructed DAG is guaranteed to be acyclic.
type DAG struct {
	nodes     map[string]Node
	edges     []Edge
	fromEdges map[string][]Edge
	toEdges   map[string][]Edge
}

// New creates an empty DAG.
func New() *DAG {
	return &DAG{
		nodes:     make(map[string]Node),
		edges:     make([]Edge, 0),
		fromEdges: make(map[string][]Edge),
		toEdges:   make(map[string][]Edge),
	}
}

// AddNode registers a node in the DAG.
// Returns ErrNilNode if node is nil, or DuplicateNodeError if a node with
// the same ID already exists.
func (d *DAG) AddNode(node Node) error {
	if node == nil {
		return ErrNilNode
	}
	id := node.ID()
	if _, exists := d.nodes[id]; exists {
		return NewDuplicateNodeError(id)
	}
	d.nodes[id] = node
	return nil
}

// AddEdge creates a directed edge from one node to another.
// If adding the edge would introduce a cycle, the edge is not added and
// a CycleError is returned. NodeNotFoundError is returned if either
// endpoint does not exist.
func (d *DAG) AddEdge(from, to string, edgeType EdgeType, condition string) error {
	if _, exists := d.nodes[from]; !exists {
		return NewNodeNotFoundError(from)
	}
	if _, exists := d.nodes[to]; !exists {
		return NewNodeNotFoundError(to)
	}

	edge := Edge{
		From:      from,
		To:        to,
		Type:      edgeType,
		Condition: condition,
	}
	d.edges = append(d.edges, edge)
	d.fromEdges[from] = append(d.fromEdges[from], edge)
	d.toEdges[to] = append(d.toEdges[to], edge)

	if d.hasCycle() {
		d.edges = d.edges[:len(d.edges)-1]
		fe := d.fromEdges[from]
		d.fromEdges[from] = fe[:len(fe)-1]
		te := d.toEdges[to]
		d.toEdges[to] = te[:len(te)-1]
		return NewCycleError(from, to)
	}

	return nil
}

// Node retrieves a node by ID. The second return value is false if not found.
func (d *DAG) Node(id string) (Node, bool) {
	n, ok := d.nodes[id]
	return n, ok
}

// Nodes returns all registered nodes keyed by ID.
func (d *DAG) Nodes() map[string]Node {
	return d.nodes
}

// Edges returns all edges in insertion order.
func (d *DAG) Edges() []Edge {
	return d.edges
}

// RootNodes returns all nodes with no incoming edges (entry points).
func (d *DAG) RootNodes() []Node {
	var roots []Node
	for id := range d.nodes {
		if len(d.toEdges[id]) == 0 {
			roots = append(roots, d.nodes[id])
		}
	}
	return roots
}

// OutEdges returns all edges originating from the given node.
func (d *DAG) OutEdges(nodeID string) []Edge {
	return d.fromEdges[nodeID]
}

// InEdges returns all edges pointing to the given node.
func (d *DAG) InEdges(nodeID string) []Edge {
	return d.toEdges[nodeID]
}

// TopologicalSort returns the node IDs in a valid execution order.
// Returns ErrCycleDetected if the graph contains a cycle (should not happen
// with the cycle-checking AddEdge, but is kept as a safety net).
func (d *DAG) TopologicalSort() ([]string, error) {
	inDegree := make(map[string]int)
	for id := range d.nodes {
		inDegree[id] = 0
	}
	for _, edge := range d.edges {
		inDegree[edge.To]++
	}

	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	var order []string
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		order = append(order, curr)

		for _, edge := range d.fromEdges[curr] {
			inDegree[edge.To]--
			if inDegree[edge.To] == 0 {
				queue = append(queue, edge.To)
			}
		}
	}

	if len(order) != len(d.nodes) {
		return nil, ErrCycleDetected
	}

	return order, nil
}

// hasCycle returns true if the current graph contains a cycle.
func (d *DAG) hasCycle() bool {
	_, err := d.TopologicalSort()
	return err != nil
}
