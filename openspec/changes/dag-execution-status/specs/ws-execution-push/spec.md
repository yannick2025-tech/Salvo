## ADDED Requirements

### Requirement: WebSocket endpoint for execution status
The system SHALL provide a WebSocket endpoint at `/ws` that accepts upgrade requests. After connection, the client SHALL send a subscribe message to receive Span status updates for a specific run.

#### Scenario: Client connects and subscribes
- **WHEN** a client connects to `/ws` and sends `{ "type": "subscribe", "run_id": "123" }`
- **THEN** the server SHALL register the client for updates related to run_id "123"

#### Scenario: Client subscribes to invalid run_id
- **WHEN** a client subscribes with a run_id that does not exist
- **THEN** the server SHALL accept the subscription silently (updates will arrive when spans are created)

### Requirement: Span status broadcast
The system SHALL broadcast a `span_update` event via WebSocket whenever a Span is completed (ok, error, skip, or canceled). The event SHALL contain: run_id, chain_id, node_id, status, duration_ns, error (if any), and loop_index.

#### Scenario: Span finishes with ok status
- **WHEN** a node execution completes successfully
- **THEN** the system SHALL broadcast `{ "type": "span_update", "run_id": "...", "chain_id": "...", "node_id": "...", "status": "ok", "duration_ns": 12345, "error": "", "loop_index": 0 }` to all subscribed clients

#### Scenario: Span is skipped
- **WHEN** a node is skipped due to conditional edge or parent failure
- **THEN** the system SHALL broadcast a span_update with `"status": "skip"` and `"error"` containing the skip reason

#### Scenario: Span fails
- **WHEN** a node execution fails
- **THEN** the system SHALL broadcast a span_update with `"status": "error"` and `"error"` containing the error message

### Requirement: Loop iteration intermediate events
For nodes with loop_count > 1, the system SHALL broadcast a `span_update` event with `"status": "running"` and `"loop_index": i` at the start of each loop iteration (i = 0 to loop_count-1), in addition to the final completion event.

#### Scenario: Loop node with 3 iterations
- **WHEN** a node with loop_count=3 executes
- **THEN** the system SHALL broadcast 3 intermediate running events (loop_index 0, 1, 2) and 1 final completion event

### Requirement: WebSocket Hub connection management
The system SHALL maintain a Hub that manages all active WebSocket connections. The Hub SHALL limit concurrent connections to 100. When a client disconnects, the Hub SHALL remove the client from all subscriptions.

#### Scenario: Connection limit
- **WHEN** 101 clients attempt to connect simultaneously
- **THEN** the 101st client SHALL receive a close frame with code 1013 (Try Again Later)

#### Scenario: Client disconnect cleanup
- **WHEN** a client disconnects
- **THEN** the Hub SHALL remove the client from its subscriber map within 1 second

### Requirement: Frontend WebSocket client
The frontend SHALL establish a WebSocket connection when a scene enters running state. The client SHALL auto-reconnect with exponential backoff (1s, 2s, 4s, max 30s) on disconnection. The client SHALL re-subscribe to the current run_id after reconnection.

#### Scenario: Connection drops during execution
- **WHEN** the WebSocket connection drops while a scene is running
- **THEN** the frontend SHALL attempt reconnection after 1 second, doubling the delay on each failure up to 30 seconds, and re-subscribe to the run_id

#### Scenario: Scene execution ends
- **WHEN** the scene execution completes and the final span_update is received
- **THEN** the frontend SHALL keep the WebSocket connection open for 5 seconds to catch late events, then close it
