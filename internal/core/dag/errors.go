package dag

import "fmt"

var (
	// ErrNilNode is returned when a nil Node is passed to AddNode.
	ErrNilNode = fmt.Errorf("node cannot be nil")
	// ErrCycleDetected is returned by TopologicalSort when the graph contains a cycle.
	ErrCycleDetected = fmt.Errorf("cycle detected in DAG")
)

// DuplicateNodeError indicates that a node with the same ID already exists.
type DuplicateNodeError struct {
	NodeID string
}

// Error implements the error interface.
func (e *DuplicateNodeError) Error() string {
	return fmt.Sprintf("duplicate node: %s", e.NodeID)
}

// NewDuplicateNodeError constructs a DuplicateNodeError for the given node ID.
func NewDuplicateNodeError(nodeID string) *DuplicateNodeError {
	return &DuplicateNodeError{NodeID: nodeID}
}

// NodeNotFoundError indicates that a referenced node does not exist in the DAG.
type NodeNotFoundError struct {
	NodeID string
}

// Error implements the error interface.
func (e *NodeNotFoundError) Error() string {
	return fmt.Sprintf("node not found: %s", e.NodeID)
}

// NewNodeNotFoundError constructs a NodeNotFoundError for the given node ID.
func NewNodeNotFoundError(nodeID string) *NodeNotFoundError {
	return &NodeNotFoundError{NodeID: nodeID}
}

// CycleError indicates that adding an edge would introduce a cycle.
type CycleError struct {
	From string
	To   string
}

// Error implements the error interface.
func (e *CycleError) Error() string {
	return fmt.Sprintf("adding edge %s -> %s would create a cycle", e.From, e.To)
}

// NewCycleError constructs a CycleError for the given edge.
func NewCycleError(from, to string) *CycleError {
	return &CycleError{From: from, To: to}
}
