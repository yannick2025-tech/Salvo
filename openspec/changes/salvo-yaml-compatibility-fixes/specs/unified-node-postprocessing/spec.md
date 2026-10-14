## ADDED Requirements

### Requirement: Unified extract support for top-level nodes
The system SHALL support `extract` configuration on all top-level node types (HTTP, Generator, etc.) in the node config JSON. The extract configuration SHALL be a map of variable names to JSON path expressions. After node execution completes successfully, the system SHALL extract values from the node's response and write them to the shared variable scope using Executor.SetVariable.

#### Scenario: HTTP node with extract
- **WHEN** an HTTP node has config `{"extract": {"token": "$.data.token", "order_id": "$.data.orderId"}}` and the response body is `{"data": {"token": "abc123", "orderId": 456}}`
- **THEN** the variables `token` and `order_id` SHALL be set to `"abc123"` and `456` respectively in the shared scope

#### Scenario: Generator node with extract
- **WHEN** a Generator node has config `{"extract": {"api_sign": "$.sign", "api_ts": "$.ts"}}` and the expression result is a JSON string `{"sign": "xyz", "ts": "123456"}`
- **THEN** the variables `api_sign` and `api_ts` SHALL be set to `"xyz"` and `"123456"` respectively

#### Scenario: Extract with missing path
- **WHEN** an extract path does not exist in the response (e.g., `$.data.nonexistent`)
- **THEN** the corresponding variable SHALL NOT be set, and no error SHALL be raised

#### Scenario: Extract with invalid JSON response
- **WHEN** the node response is not valid JSON
- **THEN** extract SHALL be skipped, and a warning SHALL be logged

### Requirement: Unified retry support with exponential backoff
The system SHALL support `retry` configuration on all top-level node types. The retry configuration SHALL include: `max_attempts` (integer, default 1), `initial_backoff` (duration string, default "100ms"), `multiplier` (float, default 2.0), `max_backoff` (duration string, default "30s"), `jitter` (boolean, default true), and `on_status` (array of HTTP status codes, default [429, 503]). When a node fails with a matching status code or error, the system SHALL retry up to `max_attempts` times with exponential backoff.

#### Scenario: Retry on HTTP 429 with exponential backoff
- **WHEN** an HTTP node has config `{"retry": {"max_attempts": 3, "initial_backoff": "100ms", "multiplier": 2.0}}` and the first request returns HTTP 429
- **THEN** the system SHALL retry after ~100ms, then ~200ms, then ~400ms (with jitter if enabled)

#### Scenario: Retry respects max_backoff
- **WHEN** retry configuration has `initial_backoff: "1s"`, `multiplier: 10`, `max_backoff: "5s"`
- **THEN** the backoff sequence SHALL be capped at 5s: 1s → 5s → 5s (not 1s → 10s → 100s)

#### Scenario: Retry with jitter
- **WHEN** retry configuration has `jitter: true`
- **THEN** each backoff duration SHALL be randomly adjusted by ±50% to prevent thundering herd

#### Scenario: Retry on non-matching status code
- **WHEN** an HTTP node returns HTTP 500 and `on_status` is `[429, 503]`
- **THEN** the system SHALL NOT retry and SHALL return the error immediately

#### Scenario: Retry exhausted
- **WHEN** `max_attempts: 3` and all 3 attempts fail
- **THEN** the system SHALL return the error from the last attempt

### Requirement: Backward compatibility for while/parallel/loop step extract and retry
The existing extract and retry support in while/parallel/loop steps SHALL remain unchanged. The unified node-level extract and retry SHALL only apply to top-level nodes, not to steps within container nodes.

#### Scenario: While step with extract
- **WHEN** a while step has `extract: {"status": "$.data.status"}`
- **THEN** the extract SHALL be handled by the while node's internal logic, not the unified post-processing

#### Scenario: Top-level HTTP node after while node
- **WHEN** a top-level HTTP node has `extract: {"token": "$.data.token"}` and follows a while node
- **THEN** the extract SHALL be handled by the unified post-processing layer

### Requirement: Multipart format compatibility
The system SHALL support both flat format (`multipart: {field1: "value1", file: "path.jpg"}`) and nested format (`form: {fields: {...}, files: {...}}`) for multipart/form-data requests. When flat format is used, the system SHALL automatically infer whether each key-value pair is a field or file based on the value pattern.

#### Scenario: Flat format with file path
- **WHEN** config has `multipart: {"token": "abc123", "file": "asserts/image.jpg"}`
- **THEN** the system SHALL treat `token` as a form field and `file` as a file upload

#### Scenario: Nested format
- **WHEN** config has `form: {fields: {token: "abc123"}, files: {file: "asserts/image.jpg"}}`
- **THEN** the system SHALL use the explicit field/file separation

#### Scenario: Flat format inference logic
- **WHEN** a flat format value contains a file extension (e.g., `.jpg`, `.png`, `.pdf`) or path separator (`/`)
- **THEN** the system SHALL treat it as a file; otherwise, as a field
