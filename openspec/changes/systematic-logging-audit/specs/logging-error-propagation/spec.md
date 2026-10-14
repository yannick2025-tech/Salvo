## ADDED Requirements

### Requirement: Runner status accessible after failure

When a runner fails after `Manager.Start` but before creating a run record, the Manager SHALL store the error and make it accessible through a method.

#### Scenario: Runner fails before run record creation
- **WHEN** `Runner.Run()` fails during buildDAG, buildScope, or lifecycle setup
- **THEN** the Manager SHALL record the error message with the Runner instance
- **AND** the error message SHALL be accessible via `runner.Error()` method

### Requirement: Run record created for early failures

The system SHALL create a run record with `Status=failed` and `ErrorMsg` when initialization steps (buildDAG, buildScope, lifecycle setup) fail.

#### Scenario: Early initialization failure creates run record
- **WHEN** buildDAG or buildScope fails
- **THEN** a run record is created (or updated) with `Status="failed"` and `ErrorMsg` contains the failure reason

#### Scenario: Frontend can display failed run reason
- **WHEN** a scene's latest run record has `Status="failed"` and non-empty `ErrorMsg`
- **THEN** the scene list page SHALL display the failure status with the error message
- **AND** the detailed run record API SHALL return the `error_msg` field

### Requirement: API returns runner error for active runs

The `RunScene` API handler SHALL return the runner error when a scene fails to start.

#### Scenario: Scene start fails with error
- **WHEN** `Manager.Start()` returns an error (e.g., scene not found, pool full)
- **THEN** the API SHALL return the error message in the response
- **AND** log at error level with scene_id and error details