## ADDED Requirements

### Requirement: Parallel node type
The system SHALL support a `parallel` node type that executes multiple child steps concurrently. The parallel node's config SHALL contain:
- `steps`: array of child step configs (HTTP requests, etc.)

All child steps SHALL be launched as goroutines. The parallel node SHALL wait for ALL child steps to complete (using sync.WaitGroup) before returning. If any child step fails, the parallel node SHALL return an error, but SHALL NOT cancel other in-flight child steps.

#### Scenario: Parallel HTTP requests
- **WHEN** a parallel node has 3 child steps (GET config, POST ad space, GET user profile)
- **THEN** all 3 HTTP requests are sent concurrently, and the node waits for all 3 responses

#### Scenario: One step fails
- **WHEN** a parallel node has 3 child steps and step 2 fails
- **THEN** steps 1 and 3 continue to completion, but the node returns an error

### Requirement: Parallel node variable isolation
Each child step in a parallel node SHALL execute with a copy of the current variable scope. Variables extracted by one child step SHALL NOT be visible to other child steps running in parallel (isolation). After all steps complete, the parallel node SHALL merge extracted variables from all child steps into the parent scope (last-write-wins for conflicting keys).

#### Scenario: Concurrent variable extraction
- **WHEN** step A extracts `tokenA` and step B extracts `tokenB` concurrently
- **THEN** both `tokenA` and `tokenB` are available in the parent scope after the parallel node completes
