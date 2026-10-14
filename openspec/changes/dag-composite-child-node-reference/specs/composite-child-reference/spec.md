## ADDED Requirements

### Requirement: Composite node child reference model

The system SHALL support a unified child-reference model for composite nodes (`group`, `while`, `loop`): the node's config contains a `node_ids` field (ordered list referencing scene nodes by name or snowflake ID), children are resolved at DAG build time into `childNodes`, and referenced children SHALL be excluded from the main DAG topology (executed by their parent composite node instead).

#### Scenario: while node referencing two child nodes
- **WHEN** a scene contains a while node with `config.node_ids = ["child-a", "child-b"]` and two HTTP nodes named child-a/child-b
- **THEN** buildDAG mounts both children on the while node's childNodes and neither child appears as an independent DAG node

#### Scenario: loop node referencing a child node
- **WHEN** a loop node has `config.node_ids = ["child-c"]` and `loop_count = 2`
- **THEN** child-c is excluded from the DAG topology and executed by the loop node twice

#### Scenario: node_ids references support both name and ID format
- **WHEN** node_ids contains a mix of node names and snowflake IDs
- **THEN** both formats resolve to the same child nodes at build time

#### Scenario: unresolvable child reference fails the build
- **WHEN** a composite node references a child that does not exist in the scene
- **THEN** buildDAG returns an error naming the offending reference

### Requirement: While reference-mode execution

When a while node has children mounted (node_ids reference mode), the system SHALL execute the child chain sequentially each iteration while preserving the full while skeleton: exit conditions, max_iterations, max_duration, fail_on_max_iterations, fail_on_max_duration, and interval_seconds. Exit conditions SHALL be evaluated against variables refreshed after each child step.

#### Scenario: child extract drives exit condition
- **WHEN** a while node's child extracts `chargingStatus` from an HTTP response and the exit condition is `chargingStatus equals "4"`
- **THEN** the loop exits on the iteration where the child observes status "4", and total child executions equal the number of iterations performed

#### Scenario: reference mode takes precedence over embedded steps
- **WHEN** a while node has both node_ids children and embedded steps configured
- **THEN** the reference-mode children execute and the embedded steps do not

#### Scenario: max_iterations cap in reference mode
- **WHEN** a reference-mode while node has max_iterations = 3 and the exit condition never becomes true
- **THEN** the child chain executes exactly 3 iterations and the node fails per fail_on_max_iterations

### Requirement: Loop reference-mode execution

When a loop node has children mounted, the system SHALL execute the child chain sequentially `loop_count` times with group-style soft-failure semantics: a child reporting failure via Output.Error does not abort the chain, and the first child soft failure SHALL surface on the loop node's Output.Error.

#### Scenario: loop with soft-failing child
- **WHEN** a reference-mode loop with loop_count = 3 has children A, D (soft failure), B
- **THEN** all three children execute 3 times each (B runs after D's soft failure in every iteration) and the loop output carries D's error

### Requirement: Child-chain variable freshness

After each child step execution inside a composite node's child chain, the system SHALL refresh `input.Variables` from the executor's shared variable scope (SnapshotVariables), so variables extracted by one child are immediately visible to subsequent children and to while/loop exit conditions.

#### Scenario: downstream child reads earlier child's extract
- **WHEN** child A extracts variable `orderId` and child B's request references `${orderId}` inside a composite child chain
- **THEN** child B's request uses the value extracted by child A in the same chain pass

### Requirement: Composite nesting rules

The system SHALL enforce composite nesting rules at DAG build time: a group node MUST NOT contain another group node; a while/loop node MUST NOT contain any composite node (group/while/loop) as a child. A group containing while/loop children is permitted and the nested composite's own node_ids SHALL be resolved recursively.

#### Scenario: while referencing a group child fails the build
- **WHEN** a while node's node_ids references a group node
- **THEN** buildDAG returns an error stating the while node cannot contain a composite child

#### Scenario: group containing a while child is resolved recursively
- **WHEN** a group node's node_ids references a while node which itself references two HTTP children
- **THEN** buildDAG resolves the while node's children recursively and mounts them on the nested while node

### Requirement: YAML import validation for composite node_ids

YAML scene import SHALL validate node_ids references for all three composite types (group/while/loop): every referenced child name must exist in the scene (400 on mismatch), names SHALL be resolved to node IDs on import, and while/loop node_ids referencing composite nodes SHALL be rejected with 400.

#### Scenario: importing while node_ids by name
- **WHEN** a YAML scene defines a while node with node_ids ["查询状态", "解析结果"] and both named nodes exist
- **THEN** import succeeds and the stored while config contains the resolved node IDs

#### Scenario: import rejects composite child in while
- **WHEN** a YAML scene defines a while node whose node_ids references a group node
- **THEN** import returns 400 with an error identifying the nested composite reference

### Requirement: YAML export converts node_ids to names

YAML scene export SHALL convert `node_ids` values from snowflake IDs to node names for all composite types (group/while/loop), so exported scenes can be re-imported. Values that cannot be resolved to a name SHALL be exported as-is.

#### Scenario: export-then-import round trip
- **WHEN** a scene with a group node (node_ids as IDs) and a while node (node_ids as IDs) is exported to YAML and re-imported
- **THEN** import succeeds with all child references intact

### Requirement: Frontend child-node management for while/loop panels

The scene editor's while and loop node configuration panels SHALL provide a child-node management section with the same interaction as group: a checkbox list of selectable scene nodes (excluding the node itself and other composite nodes), and an ordered selected list with move-up/move-down/remove controls bound to the node's node_ids.

#### Scenario: adding children to a while node
- **WHEN** the user checks two HTTP nodes in the while panel's child selection list
- **THEN** both nodes appear in the ordered selected list and are persisted into the while config's node_ids on save

#### Scenario: composite nodes are not selectable
- **WHEN** the child selection list is rendered for a while node
- **THEN** group/while/loop nodes are not offered as selectable children

#### Scenario: embedded steps coexistence hint
- **WHEN** a while node's config contains both node_ids and embedded steps
- **THEN** the panel displays a hint that N embedded YAML steps exist and reference-mode children take precedence

### Requirement: Frontend child editing from composite panel

The group/while/loop panels' selected-child lists SHALL provide an edit action per child that opens the child node's configuration panel, with a back action in the panel header returning to the parent composite node's panel.

#### Scenario: editing a child from the group panel
- **WHEN** the user clicks the edit action on a selected child in the group panel
- **THEN** the child node's configuration panel opens, and a back control returns to the group panel with the child order unchanged

### Requirement: Frontend while/loop child rendering on canvas

The DAG canvas SHALL render while/loop nodes' node_ids children the same way as group children: children are resolved against scene nodes, displayed when the node is expanded (double-click), and clickable for editing. Loop nodes SHALL gain the expand interaction. While nodes with embedded steps (no node_ids) SHALL keep the existing read-only step display.

#### Scenario: expanding a while node with node_ids children
- **WHEN** a while node with two node_ids children is double-clicked on the canvas
- **THEN** the children render inside the expanded node region and are clickable to open their config panels

#### Scenario: while node with only embedded steps
- **WHEN** a while node has no node_ids but embedded steps
- **THEN** the canvas keeps the existing read-only steps display

### Requirement: Frontend config save preserves unknown fields

Saving a while/loop node configuration from the editor SHALL shallow-merge form fields onto the node's original parsed config (unknown keys preserved), and SHALL NOT drop fields not loaded into the form (steps, fail_on_max_iterations, fail_on_max_duration, and any future keys).

#### Scenario: saving while panel preserves embedded steps
- **WHEN** a while node with embedded steps and fail_on_max_iterations=false has its poll interval changed and saved from the panel
- **THEN** the saved config retains the steps array and fail_on_max_iterations value, with only the interval updated

#### Scenario: stale panel reference refresh after save
- **WHEN** a node config is saved successfully
- **THEN** the node list is re-fetched and the selected panel state is re-synced to the fresh node record
