## ADDED Requirements

### Requirement: Log level definitions

The system SHALL define and document explicit log level usage rules:

| Level | Usage |
|-------|-------|
| `fatal` | Process-terminating conditions (current behavior: logger initialization failure, DB open failure) |
| `error` | Definite system failures requiring human intervention |
| `warn` | Non-fatal anomalies that may affect partial results |
| `info` | Key lifecycle events and traceable business flow steps |
| `debug` | Diagnostic details for development and troubleshooting |

#### Scenario: Error-level logging for definite failures
- **WHEN** a system operation fails definitively (e.g., buildScope parse failure, HTTP request timeout, panic recovery)
- **THEN** the log level SHALL be `error`
- **AND** the log message SHALL include the failing operation name, error detail, and relevant identifiers (trace_id, scene_id, node_id)

#### Scenario: Warn-level logging for non-fatal anomalies
- **WHEN** a non-critical operation fails or degrades (e.g., data source parse failure, fallback path activated, unknown node type)
- **THEN** the log level SHALL be `warn`
- **AND** the log message SHALL explain the impact (e.g., "data source rows will be empty")

### Requirement: Logger.WithContext injects trace context

The `logger.WithContext(ctx)` method SHALL automatically extract `trace_id`, `chain_id`, `node_id`, and `scene_id` from the context and inject them into every subsequent log entry.

#### Scenario: WithContext creates contextualized logger
- **WHEN** `log.WithContext(ctx)` is called and the context contains `trace_id`, `chain_id`, `node_id`
- **THEN** all subsequent log entries from the returned logger SHALL include these fields

#### Scenario: WithContext with partial context
- **WHEN** `log.WithContext(ctx)` is called and the context only contains `trace_id` (no chain_id or node_id)
- **THEN** the returned logger SHALL only include `trace_id`
- **AND** SHALL NOT produce errors for missing fields

### Requirement: Error messages are actionable

Every `error` level log message SHALL include sufficient context for an operator to identify the problem without reading code.

#### Scenario: Error message format
- **WHEN** an error is logged
- **THEN** the log message SHALL include: operation name, specific error, affected identifiers (scene_id, run_id, node_id), and a human-readable description
- **AND** SHALL NOT just log "error" or "failed" without explaining what failed and why

### Requirement: Debug panic stack trace

When a panic is recovered, the log output SHALL include the full stack trace in addition to the panic value.

#### Scenario: Panic recovery log includes stack trace
- **WHEN** a goroutine panic is recovered by `safeGo` or the HTTP recovery middleware
- **THEN** the log entry SHALL include both `"panic": <value>` and `"stacktrace": "<stack>"` fields