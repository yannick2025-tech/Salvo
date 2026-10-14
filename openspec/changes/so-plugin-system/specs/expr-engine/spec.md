## ADDED Requirements

### Requirement: Expression engine with function call support
The system SHALL provide an expression engine at `internal/core/expr/` that resolves `${...}` expressions in strings. The engine SHALL support three expression types:
1. **Variable reference**: `${var}` — replaced by the variable's value from the current scope
2. **Function call**: `${__funcName(arg1, arg2, ...)}` — calls a registered system function and replaces with the return value
3. **Math expression**: `${var} * 100 / 50` — evaluates arithmetic with +, -, *, / and parentheses

The engine SHALL resolve expressions recursively (nested `${...}` inside function arguments), up to 10 levels deep.

#### Scenario: Variable reference
- **WHEN** input string is `"Hello ${name}"` and variable `name` = `"World"`
- **THEN** output is `"Hello World"`

#### Scenario: Function call
- **WHEN** input string is `"${__random(60, 600)}"` and `__random` is registered
- **THEN** output is a number string between 60 and 600

#### Scenario: Math expression
- **WHEN** input string is `"${chargeTime} * ${ranking} / 100"` and chargeTime=60, ranking=50
- **THEN** output is `"30"`

#### Scenario: Nested function call
- **WHEN** input string is `"${__random(${min}, ${max})}"` and min=1, max=100
- **THEN** the inner `${min}` and `${max}` are resolved first, then `__random(1, 100)` is called

### Requirement: Unified `__` prefix naming convention
All system functions SHALL use the `__` prefix (e.g., `__random`, `__oneOf`, `__so`). Existing generator functions (e.g., `generator.email`) SHALL be migrated to the `__` prefix format (e.g., `__email()`). The `__` prefix + `()` parentheses visually distinguishes function calls from variable references `${var}`.

#### Scenario: Migrated generator function
- **WHEN** user writes `${__email()}` in a URL or body field
- **THEN** the expression engine resolves it to a generated email address

### Requirement: Function registry
The system SHALL maintain a function registry where system functions are registered by name. The registry SHALL support:
- `Register(name, handler)` — register a function
- `Get(name)` — retrieve a function handler
- `List()` — list all registered function names

Functions are registered at startup in `internal/generator/builtin/registry.go`.

#### Scenario: Register and call a function
- **WHEN** `__random` is registered with a handler that accepts string args and returns a string
- **THEN** `${__random(1, 100)}` resolves by calling the handler with args `["1", "100"]`

### Requirement: Math expression evaluator
The system SHALL provide a safe math expression evaluator that supports `+`, `-`, `*`, `/`, parentheses, and numeric literals. The evaluator SHALL NOT support variable assignment, function calls, or any non-arithmetic operations. Division by zero SHALL return error.

#### Scenario: Simple arithmetic
- **WHEN** input is `"60 * 50 / 100"`
- **THEN** output is `"30"`

#### Scenario: Parentheses
- **WHEN** input is `"(10 + 20) * 3"`
- **THEN** output is `"90"`
