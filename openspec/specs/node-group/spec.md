# node-group Specification

## Purpose
TBD - created by archiving change scene-orchestration-upgrade. Update Purpose after archive.
## Requirements
### Requirement: Group node type
The system SHALL support a `group` node type that acts as a logical container for a sequence of child nodes. The Group node's config SHALL contain a `node_ids` field (ordered list of child node IDs) and a `loop_count` field (number of times to repeat the child node sequence). The Group node SHALL be stored as a regular Node in the database with type `group`.

#### Scenario: Create a group node
- **WHEN** user creates a group node named "Login Flow" with child nodes [D1, D2, D3] and loop_count=3
- **THEN** a Node record is created with type="group", config containing `{"node_ids":["id1","id2","id3"],"loop_count":3}`

#### Scenario: Group with default loop count
- **WHEN** user creates a group node without specifying loop_count
- **THEN** loop_count defaults to 1 (execute the child sequence once)

### Requirement: Group execution semantics
When the DAG executor encounters a Group node, it SHALL execute the child nodes in the order specified by `node_ids`, repeating the sequence `loop_count` times. The Group's sync/async mode determines whether downstream nodes wait for all loops to complete:
- **Sync Group**: downstream nodes wait until all loop iterations finish
- **Async Group**: downstream nodes proceed immediately after the Group starts its first iteration

#### Scenario: Sync group with loop_count=2
- **WHEN** a sync Group [D1→D2→D3] has loop_count=2
- **THEN** the execution order is D1→D2→D3→D1→D2→D3, and downstream nodes wait until the second D3 completes

#### Scenario: Async group does not block downstream
- **WHEN** an async Group [D1→D2] has loop_count=3
- **THEN** downstream nodes start immediately after the Group begins, while D1→D2 repeats 3 times in the background

#### Scenario: Group with single child node
- **WHEN** a Group contains only one child node [D1] with loop_count=5
- **THEN** D1 is executed 5 times sequentially (equivalent to setting LoopCount=5 on D1 directly)

### Requirement: Group DAG visualization
In the DAG flow canvas, a Group node SHALL render as a collapsible node. By default it displays in collapsed mode showing the group name and loop count (e.g., "Login Flow x3"). Double-clicking the node SHALL expand it to show child nodes inside a bordered region. The Group node SHALL have input and output ports for connecting external edges.

#### Scenario: Collapsed group display
- **WHEN** a Group node "Login Flow" with loop_count=3 is rendered in the DAG canvas
- **THEN** it displays as a single node with label "Login Flow x3" and a group icon

#### Scenario: Expand group to see children
- **WHEN** user double-clicks a collapsed Group node
- **THEN** the node expands to show child nodes D1, D2, D3 inside a bordered region with the group label

#### Scenario: Collapse expanded group
- **WHEN** user double-clicks an expanded Group node
- **THEN** it collapses back to the single-node view

### Requirement: Group node in YAML
The YAML format SHALL support group nodes with `type: group` and config containing `node_ids` (list of node names) and `loop_count`.

#### Scenario: YAML group definition
- **WHEN** YAML contains a node with `type: group` and `config: {node_ids: ["Login","Submit"], loop_count: 3}`
- **THEN** the import creates a Group node referencing the named child nodes

### Requirement: LoopCount UI for all node types
The Scene Detail page SHALL expose a "Loop Count" input field in the node editor for all node types (not just Group). The field SHALL default to 1 (execute once) and accept positive integers.

#### Scenario: Set loop count on HTTP node
- **WHEN** user sets Loop Count=5 on an HTTP node
- **THEN** the node's `loop_count` field is saved as 5, and the DAG executor invokes it 5 times per traversal

