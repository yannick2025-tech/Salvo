## ADDED Requirements

### Requirement: Aggregate execution status badges
In aggregate view, each DAG node SHALL display a set of status badges at the top-right corner showing the count of chains in each state: PASS (green ✓), FAILED (red ✗), SKIP (yellow ⊘), RUNNING (blue ⟳). Idle chains SHALL show a gray ◦ badge.

#### Scenario: Multiple chains with mixed status on one node
- **WHEN** 5 chains have executed node-A with results: 3 pass, 1 fail, 1 skip
- **THEN** node-A SHALL display badges: ✓3 ✗1 ⊘1

#### Scenario: Node with running chains
- **WHEN** 2 chains are currently executing node-B while 3 have passed
- **THEN** node-B SHALL display badges: ⟳2 ✓3

### Requirement: Single chain execution status overlay
In single chain view, each DAG node SHALL display an execution state via border color and background: PASS (green border + light green bg), FAILED (red border + light red bg), SKIP (yellow border + semi-transparent), RUNNING (blue pulsing border + glow), IDLE (semi-transparent, dimmed).

#### Scenario: Chain where node passed
- **WHEN** viewing chain-2 and node-A has status "ok"
- **THEN** node-A SHALL have a green left border and light green background tint

#### Scenario: Chain where node is running
- **WHEN** viewing chain-1 and node-B has status "running"
- **THEN** node-B SHALL have a blue pulsing border animation

#### Scenario: Chain where node was skipped
- **WHEN** viewing chain-3 and node-C has status "skip"
- **THEN** node-C SHALL have a yellow left border and reduced opacity (0.7)

### Requirement: Chain selector component
The system SHALL provide a chain selector UI with two tabs: "聚合视图" (aggregate) and "单链路视图" (single chain). In single chain mode, a dropdown SHALL list all chain_ids for the current run. Selecting a chain SHALL update the DAG to show that chain's execution state.

#### Scenario: Switch to single chain view
- **WHEN** user clicks "单链路视图" tab
- **THEN** a chain dropdown SHALL appear listing all chain_ids, and the DAG SHALL switch to single-chain styling

#### Scenario: Select a specific chain
- **WHEN** user selects "chain-3" from the dropdown
- **THEN** the DAG SHALL highlight the execution path of chain-3, with active nodes showing status and inactive nodes dimmed

### Requirement: Edge path highlighting in single chain view
In single chain view, edges SHALL be styled based on whether they are on the active execution path. Active path edges SHALL be solid with full opacity. Inactive path edges SHALL be semi-transparent (opacity 0.15). Running path edges SHALL have a blue dashed animation.

#### Scenario: Active vs inactive edges
- **WHEN** viewing chain-1 which took the TRUE branch of an IF-ELSE
- **THEN** the TRUE edge SHALL be solid green and the FALSE edge SHALL be semi-transparent

### Requirement: Loop progress indicator
For nodes with loop_count > 1, the system SHALL display a loop progress badge at the bottom-right of the node showing current iteration / total iterations (e.g., "L2/5"). In aggregate view, the badge SHALL also show the number of chains that have loop progress.

#### Scenario: Single chain loop progress
- **WHEN** viewing chain-1 and node-C (loop_count=3) is on iteration 2
- **THEN** node-C SHALL display "L2/3" badge at bottom-right

#### Scenario: Aggregate loop progress
- **WHEN** in aggregate view and node-C (loop_count=3) has 2 chains at iteration 3 and 1 chain at iteration 1
- **THEN** node-C SHALL display "L3/3×2" badge showing max iteration / total × chain count

### Requirement: DAG execution mode toggle
The DAG component SHALL detect the scene's execution state. When the scene is running or recently completed, the DAG SHALL enter "execution mode" (read-only, status overlay visible). When the scene is idle (not running), the DAG SHALL be in "edit mode" (current behavior).

#### Scenario: Scene starts running
- **WHEN** the scene status changes to "running"
- **THEN** the DAG SHALL switch to execution mode: nodes become non-draggable, status badges appear, and the chain selector becomes visible

#### Scenario: Scene execution completes
- **WHEN** the scene status changes to "done" or "failed"
- **THEN** the DAG SHALL remain in execution mode showing the final status of all nodes, and the status badge in the top bar SHALL change to "已完成"
