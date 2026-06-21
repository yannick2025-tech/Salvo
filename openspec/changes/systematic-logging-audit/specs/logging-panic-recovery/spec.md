## ADDED Requirements

### Requirement: safeGo utility function

The system SHALL provide a `safeGo` utility function that wraps goroutine creation with panic recovery. The utility SHALL accept a context, a logger, a goroutine name string, and the function to execute.

#### Scenario: safeGo creates a goroutine with panic recovery
- **WHEN** `safeGo(ctx, log, "my-goroutine", fn)` is called
- **THEN** a goroutine is spawned that executes `fn()`
- **AND** if `fn()` panics, the panic is recovered and logged at error level with the goroutine name, panic value, and stack trace

#### Scenario: safeGo goroutine completes normally
- **WHEN** the function passed to safeGo completes without panic
- **THEN** no extra logging or recovery action is taken

### Requirement: Manager.Start goroutine panic recovery

The `Manager.Start` method SHALL use `safeGo` for the background goroutine that calls `r.Run()`.

#### Scenario: Runner.Run panics in Manager.Start goroutine
- **WHEN** the background goroutine created by `Manager.Start` panics during `r.Run()`
- **THEN** the panic is recovered and logged with: `{"msg":"runner goroutine panicked","scene_id":"<scene_id>","run_id":"<run_id>"}`

### Requirement: Worker pool task panic recovery

The worker pool SHALL recover from panics in individual task executions. A panicked task SHALL be counted as a failed execution.

#### Scenario: Pool task panics during execution
- **WHEN** a worker pool task function panics
- **THEN** the panic is recovered, logged at error level with the trace_id and chain_id, and the task is recorded as a failure

### Requirement: RuntimeMetricsCollector goroutine panic recovery

The `RuntimeMetricsCollector` sampling goroutine SHALL use panic recovery.

#### Scenario: Metrics collector sampling panics
- **WHEN** the `RuntimeMetricsCollector` background sampling goroutine panics
- **THEN** the panic is recovered, logged at error level, and the sampling loop continues (or terminates gracefully)