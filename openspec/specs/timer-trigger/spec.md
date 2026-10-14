# timer-trigger Specification

## Purpose
TBD - created by archiving change scene-orchestration-upgrade. Update Purpose after archive.
## Requirements
### Requirement: Timer node type
The system SHALL support a `timer` node type that triggers downstream execution based on time, independent of the DAG's main chain. The timer node's config SHALL contain:
- `mode`: either `"delay"` (one-shot after X seconds) or `"interval"` (repeating every X seconds)
- `seconds`: positive number indicating the delay or interval duration

Timer nodes SHALL always execute in async mode — they never block the DAG main chain.

#### Scenario: Create a delay timer node
- **WHEN** user creates a timer node with `mode: "delay"` and `seconds: 30`
- **THEN** a Node record is created with type="timer", config `{"mode":"delay","seconds":30}`

#### Scenario: Create an interval timer node
- **WHEN** user creates a timer node with `mode: "interval"` and `seconds: 10`
- **THEN** a Node record is created with type="timer", config `{"mode":"interval","seconds":10}`

### Requirement: Delay trigger execution
When the DAG executor encounters a delay timer node, it SHALL start a goroutine that waits for the specified duration, then triggers the node's downstream chain. The delay is measured from the start of the test run (not from when the executor reaches the node). The timer SHALL respect context cancellation.

#### Scenario: Delay trigger fires after specified time
- **WHEN** a delay timer with seconds=30 is in the DAG, and the test starts at t=0
- **THEN** at t=30s, the timer's downstream nodes begin execution

#### Scenario: Delay trigger cancelled by test stop
- **WHEN** a delay timer with seconds=60 is scheduled, but the test stops at t=30s
- **THEN** the timer goroutine exits without triggering downstream nodes

### Requirement: Interval trigger execution
When the DAG executor encounters an interval timer node, it SHALL start a goroutine that triggers the downstream chain every `seconds` interval, starting from the first interval after test start. The timer SHALL continue until the test context is cancelled.

#### Scenario: Interval trigger fires repeatedly
- **WHEN** an interval timer with seconds=10 is in the DAG, and the test runs for 35 seconds
- **THEN** downstream nodes are triggered at t=10s, t=20s, and t=30s

#### Scenario: Interval trigger stops on test end
- **WHEN** an interval timer is running and the test stops
- **THEN** no further triggers occur after the test stop time

### Requirement: Timer node in DAG visualization
In the DAG flow canvas, a timer node SHALL render with a clock icon and display its mode and duration (e.g., "⏱ 30s delay" or "⏱ 10s interval"). Timer nodes SHALL have output ports only (no input dependency from the main chain).

#### Scenario: Timer node visual
- **WHEN** a timer node with mode="interval" and seconds=10 is rendered
- **THEN** it displays with a clock icon and label "⏱ 10s interval"

### Requirement: Timer node in YAML
The YAML format SHALL support timer nodes with `type: timer` and config containing `mode` and `seconds`.

#### Scenario: YAML timer definition
- **WHEN** YAML contains a node with `type: timer` and `config: {mode: "interval", seconds: 10}`
- **THEN** the import creates a timer node with the specified configuration

