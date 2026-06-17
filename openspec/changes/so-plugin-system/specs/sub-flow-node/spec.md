## ADDED Requirements

### Requirement: Sub-flow node type
The system SHALL support a `sub_flow` node type that references another scene as a sub-flow. The sub_flow node's config SHALL contain:
- `scene_id`: the ID of the referenced scene (string)
- `async`: boolean, if true the sub-flow runs in background and does not block the main chain

#### Scenario: Synchronous sub-flow
- **WHEN** a sub_flow node has `scene_id: "123"` and `async: false`
- **THEN** the referenced scene's DAG is executed, and the node waits for completion before proceeding

#### Scenario: Asynchronous sub-flow
- **WHEN** a sub_flow node has `scene_id: "123"` and `async: true`
- **THEN** the referenced scene's DAG is launched in a background goroutine, and the node returns immediately without waiting

### Requirement: Sub-flow variable passing
When a sub_flow node executes synchronously, variables extracted by the sub-flow's nodes SHALL be merged into the parent scope. When async, no variable merging occurs (fire-and-forget).

#### Scenario: Sync sub-flow variable merge
- **WHEN** a sync sub_flow extracts `subToken` in its execution
- **THEN** `subToken` is available in the parent scope after the sub_flow node completes

### Requirement: Sub-flow depth limit
The system SHALL enforce a maximum sub-flow nesting depth of 5 to prevent infinite recursion. If the depth limit is exceeded, the node SHALL return an error.

#### Scenario: Exceed depth limit
- **WHEN** scene A references scene B which references scene C (depth 3) and scene C references scene A
- **THEN** the sub_flow node in scene C returns an error "sub-flow depth limit exceeded"
