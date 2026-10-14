## ADDED Requirements

### Requirement: While loop node type
The system SHALL support a `while` node type that repeatedly executes a list of child steps until exit conditions are met. The while node's config SHALL contain:
- `exit_conditions`: array of `{variable, operator, value}` conditions; loop exits when ANY condition is met
- `interval_seconds`: wait time between iterations (default: 0, no wait)
- `max_iterations`: maximum number of iterations (default: 0 = unlimited)
- `max_duration_minutes`: maximum total duration in minutes (default: 0 = unlimited)
- `steps`: array of child step configs (HTTP requests, conditions, etc.)
- `fail_after_consecutive`: if set, fail the node after N consecutive failures of any child step (default: 0 = disabled)
- `fail_message`: error message when fail_after_consecutive triggers

#### Scenario: Polling with exit condition
- **WHEN** a while node has `exit_conditions: [{variable: "status", operator: "equals", value: "4"}]` and `interval_seconds: 30`
- **THEN** the node executes steps, checks if `status == "4"`, if not waits 30s and repeats

#### Scenario: Max iterations limit
- **WHEN** a while node has `max_iterations: 6` and exit conditions are never met
- **THEN** the node stops after 6 iterations and returns an error

#### Scenario: Consecutive failure detection
- **WHEN** a while node has `fail_after_consecutive: 10` and a child step fails 10 times in a row
- **THEN** the node fails immediately with `fail_message`

### Requirement: While node child step execution
Each child step in a while node SHALL be executed sequentially within each iteration. Steps SHALL support:
- HTTP requests (method, url, headers, body, extract)
- Condition checks (variable, operator, value)
- Think time (min/max random delay in ms)
- Timed triggers (after_seconds, once)

Variables extracted by child steps SHALL be available to subsequent steps within the same iteration and to the exit condition check.

#### Scenario: Extract variable and check exit condition
- **WHEN** step 1 extracts `chargingStatus` from response and exit_condition checks `chargingStatus equals "4"`
- **THEN** the extracted value is used in the exit condition evaluation

### Requirement: While node timed trigger
Child steps in a while node SHALL support `timed_trigger` config with `after_seconds` (delay from loop start) and `once: true` (execute only in the first matching iteration). The timed trigger fires asynchronously — it does not block the main loop.

#### Scenario: Timed trigger fires once
- **WHEN** a child step has `timed_trigger: {after_seconds: "${chargePostOfflineTime}", once: true}` and the condition is met
- **THEN** a goroutine waits for the specified duration, then executes the step's HTTP request once
