## ADDED Requirements

### Requirement: Node-level error blocking configuration

The system SHALL support a `block_on_error` configuration field on all node types (http, generator, while, if-else, delay, etc.). This field controls whether a node failure should abort the entire chain execution or allow subsequent nodes to continue.

#### Scenario: Node with block_on_error=true fails
- **WHEN** a node has `block_on_error: true` configured and the node execution fails (HTTP non-2xx, error returned, or business logic error)
- **THEN** the system SHALL immediately cancel the entire chain execution via context cancellation
- **THEN** the system SHALL mark the chain status as "failed"
- **THEN** the system SHALL log the failure with `block_on_error: true` field for traceability

#### Scenario: Node with block_on_error=false fails (default)
- **WHEN** a node has `block_on_error: false` (or not configured, using default) and the node execution fails
- **THEN** the system SHALL log the error and continue executing subsequent nodes in the chain
- **THEN** the chain status SHALL remain "running" until all nodes complete

#### Scenario: Backward compatibility for existing scenes
- **WHEN** an existing scene configuration does not include `block_on_error` field
- **THEN** the system SHALL default to `block_on_error: false`
- **THEN** the behavior SHALL be identical to the current implementation (errors do not block chain)

### Requirement: Database schema support for block_on_error

The system SHALL persist the `block_on_error` configuration in the database.

#### Scenario: Node creation with block_on_error
- **WHEN** a node is created via API with `block_on_error: true`
- **THEN** the system SHALL store this value in the `nodes` table `block_on_error` column
- **THEN** the value SHALL be retrievable via node query APIs

#### Scenario: Node update with block_on_error
- **WHEN** a node is updated via API with a new `block_on_error` value
- **THEN** the system SHALL update the database record accordingly

#### Scenario: Database migration for existing nodes
- **WHEN** the migration is executed on an existing database
- **THEN** the system SHALL add a `block_on_error BOOLEAN DEFAULT FALSE` column to the `nodes` table
- **THEN** all existing nodes SHALL have `block_on_error = false` by default

### Requirement: HTTP node error detection with block_on_error

The system SHALL treat HTTP non-2xx responses as errors when `block_on_error: true` is configured.

#### Scenario: HTTP 404 with block_on_error=true
- **WHEN** an HTTP node with `block_on_error: true` receives a 404 response
- **THEN** the system SHALL treat this as a node execution error
- **THEN** the system SHALL cancel the chain execution

#### Scenario: HTTP 500 with block_on_error=false
- **WHEN** an HTTP node with `block_on_error: false` receives a 500 response
- **THEN** the system SHALL log the error but continue chain execution
- **THEN** the node SHALL be marked as "failed" in logs but the chain continues

#### Scenario: HTTP 200 with business error and expect_body assertion
- **WHEN** an HTTP node receives a 200 response but the response body contains a business error (e.g., `{"errorCode": 100000400, "errorMsg": "JSON parse error"}`)
- **AND** the node has `expect_body: {"errorCode": 0}` configured
- **THEN** the system SHALL treat the `expect_body` assertion failure as a node execution error
- **THEN** if `block_on_error: true` is configured, the system SHALL cancel the chain execution
- **THEN** if `block_on_error: false` (default), the system SHALL log the error and continue chain execution

### Requirement: While loop step-level error blocking

The system SHALL support `block_on_error` configuration on individual steps within a while loop node.

#### Scenario: While step with block_on_error=true fails
- **WHEN** a step within a while loop has `block_on_error: true` and the step execution fails
- **THEN** the system SHALL immediately abort the while loop
- **THEN** the system SHALL return an error from the while node
- **THEN** if the while node itself has `block_on_error: true`, the entire chain SHALL be cancelled

#### Scenario: While step with block_on_error=false fails
- **WHEN** a step within a while loop has `block_on_error: false` (default) and the step execution fails
- **THEN** the system SHALL log the error and continue executing subsequent steps in the same iteration
- **THEN** the while loop SHALL continue to the next iteration (subject to exit conditions and max_iterations)

#### Scenario: While step block_on_error interaction with fail_after_consecutive
- **WHEN** a step has both `block_on_error: false` and `fail_after_consecutive: 5` configured
- **THEN** the system SHALL continue executing after individual step failures
- **THEN** after 5 consecutive failures, the system SHALL abort the while loop (existing behavior preserved)

### Requirement: YAML configuration support for block_on_error

The system SHALL parse `block_on_error` from scene YAML configurations.

#### Scenario: YAML node with block_on_error
- **WHEN** a scene YAML file contains a node with `block_on_error: true`
- **THEN** the system SHALL parse this field and apply it to the node during DAG construction
- **THEN** the node SHALL block chain execution on failure

#### Scenario: YAML while step with block_on_error
- **WHEN** a scene YAML file contains a while node with a step configured as `block_on_error: true`
- **THEN** the system SHALL parse this field and apply it to the step during while loop execution
- **THEN** the step SHALL abort the while loop on failure

### Requirement: Logging and traceability for block_on_error

The system SHALL log `block_on_error` status for all node executions to aid debugging.

#### Scenario: Node execution log includes block_on_error
- **WHEN** a node executes (success or failure)
- **THEN** the system SHALL include `block_on_error: true/false` in the log fields
- **THEN** the log SHALL clearly indicate whether the node was configured to block on error

#### Scenario: Chain cancellation due to block_on_error
- **WHEN** a chain is cancelled because a node with `block_on_error: true` failed
- **THEN** the system SHALL log a specific message: "chain cancelled due to block_on_error"
- **THEN** the log SHALL include the node_id, node_name, and the original error
