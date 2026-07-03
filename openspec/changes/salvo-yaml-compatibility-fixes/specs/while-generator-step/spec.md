## ADDED Requirements

### Requirement: While step supports generator node type
The system SHALL support `type: generator` in while step configurations. When a step has `type: generator`, the system SHALL execute the generator expression and store the result in the specified variable. The step config SHALL contain `expression` and `variable` fields, consistent with top-level generator nodes.

#### Scenario: While step with generator type
- **WHEN** a while step has `type: generator` and config `{"expression": "${__random(1, 100)}", "variable": "random_value"}`
- **THEN** the system SHALL execute the expression, store the result in `random_value`, and make it available to subsequent steps

#### Scenario: Generator step with extract
- **WHEN** a while step has `type: generator` and config `{"expression": "${__so(\"sign\",\"now_ms\")}", "variable": "timestamp", "extract": {"ts": "$.ts"}}`
- **THEN** the system SHALL execute the expression, store the full result in `timestamp`, and extract the `ts` field into the variable scope

#### Scenario: Default step type
- **WHEN** a while step does not specify `type`
- **THEN** the system SHALL default to HTTP request execution (backward compatible)

### Requirement: While step config structure extension
The stepConfig struct SHALL be extended to include `Type` (string) and `Config` (map[string]any) fields. The `Config` field SHALL contain the generator-specific configuration (expression, variable, extract). The existing `Request` field SHALL remain for HTTP steps.

#### Scenario: Generator step config parsing
- **WHEN** a while step has `type: generator` and `config: {"expression": "...", "variable": "..."}`
- **THEN** the system SHALL parse the Config field and execute as a generator node

#### Scenario: HTTP step config parsing
- **WHEN** a while step has no `type` or `type: http` and `request: {"method": "POST", ...}`
- **THEN** the system SHALL parse the Request field and execute as an HTTP request (backward compatible)

### Requirement: While step generator variable scope
Variables set by generator steps SHALL be written to the while loop's local variable scope (loopVars), making them available to subsequent steps in the same iteration and to exit condition checks.

#### Scenario: Variable propagation within while loop
- **WHEN** step 1 is a generator that sets `status = "ready"` and step 2 is an HTTP request using `${status}`
- **THEN** step 2 SHALL resolve `${status}` to `"ready"`

#### Scenario: Generator variable available in exit condition
- **WHEN** a generator step sets `iteration_count` and the while exit_condition checks `${iteration_count} >= 10`
- **THEN** the exit condition SHALL evaluate using the value set by the generator step
