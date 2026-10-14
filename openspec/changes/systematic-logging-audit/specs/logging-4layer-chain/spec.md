## ADDED Requirements

### Requirement: Scene-level trace logging

The system SHALL emit an info-level log message when a scene run starts and when it ends. Both messages SHALL carry `trace_id` (= `run_id`) and `scene_id` fields.

#### Scenario: Scene run starts
- **WHEN** `Runner.Run()` is called and passes all initialization checks (buildDAG, buildScope, lifecycle setup)
- **THEN** system logs: `{"level":"info","msg":"scene run started","trace_id":"<run_id>","scene_id":"<scene_id>","workers":N,"run_mode":"count|duration","count|duration":V}`

#### Scenario: Scene run fails at initialization
- **WHEN** `Runner.Run()` fails during buildDAG, buildScope, or lifecycle setup
- **THEN** system logs at error level with the exact failure reason, trace_id, scene_id, and the failed step name (build_dag / build_scope / scene_setup)

#### Scenario: Scene run completes
- **WHEN** scene execution finishes (success, failure, or cancellation)
- **THEN** system logs at info level: `{"level":"info","msg":"run completed","trace_id":"<run_id>","status":"completed|failed|cancelled","total_reqs":N,"success_reqs":N,"failed_reqs":N}`

### Requirement: Chain-level trace logging

The system SHALL generate a unique `chain_id` for each DAG execution in a scene run. Each chain execution SHALL be logged with `trace_id`, `chain_id`, and resolved variable count.

#### Scenario: Chain execution starts
- **WHEN** a task is submitted to the worker pool and begins DAG execution
- **THEN** system logs at info level: `{"msg":"chain execution started","trace_id":"<run_id>","chain_id":"<chain_id>","variable_count":N}`

#### Scenario: Chain execution fails
- **WHEN** DAG execution returns an error for a chain
- **THEN** system logs at error level: `{"msg":"DAG execution failed","trace_id":"<run_id>","chain_id":"<chain_id>","error":"<error>"}`

### Requirement: API/Node-level trace logging

The system SHALL log the start and end of each node execution. Each node log SHALL carry `trace_id`, `chain_id`, `node_id`, and `node_type`.

#### Scenario: Node execution starts
- **WHEN** a DAG executor calls `sceneNode.Execute()`
- **THEN** system logs at info level: `{"msg":"node execution started","trace_id":"<run_id>","chain_id":"<chain_id>","node_id":"<node_id>","node_type":"<type>"}`

#### Scenario: Node execution succeeds
- **WHEN** a node executes without error
- **THEN** system logs at debug level with execution result summary

#### Scenario: Node execution fails
- **WHEN** a node returns an error
- **THEN** system logs at error level with the error message, node_id, and chain_id

### Requirement: Function-level trace logging

The system SHALL record generator function calls as Function-level spans. These spans SHALL be stored in-memory (not persisted to database) and associated with the parent node via `parent_node_id`.

#### Scenario: Generator function is called
- **WHEN** a generator function (`__uuid()`, `__randomInt()`, etc.) is evaluated during variable resolution
- **THEN** a Function-level span is recorded with `function_name`, input parameters, and output value

#### Scenario: Generator function fails
- **WHEN** a generator function returns an error
- **THEN** the Function-level span SHALL have `status: "error"` and include the error message