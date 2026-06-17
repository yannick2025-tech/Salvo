## ADDED Requirements

### Requirement: Condition operator evaluator
The system SHALL provide a condition evaluator at `internal/core/expr/evaluator.go` that evaluates `{variable, operator, value}` conditions against the current variable scope. The evaluator SHALL support 12 operators:

| Operator | Description | Example |
|----------|-------------|---------|
| `equals` | String/number equality | `{variable: "status", operator: "equals", value: "4"}` |
| `not_equals` | String/number inequality | `{variable: "status", operator: "not_equals", value: "COMPLETED"}` |
| `greater_than` | Numeric greater than | `{variable: "count", operator: "greater_than", value: "0"}` |
| `greater_than_or_equal` | Numeric greater than or equal | `{variable: "count", operator: "greater_than_or_equal", value: "1"}` |
| `less_than` | Numeric less than | `{variable: "count", operator: "less_than", value: "10"}` |
| `less_than_or_equal` | Numeric less than or equal | `{variable: "count", operator: "less_than_or_equal", value: "5"}` |
| `not_empty` | Variable is not empty/null | `{variable: "orderId", operator: "not_empty"}` |
| `empty` | Variable is empty/null | `{variable: "orderId", operator: "empty"}` |
| `size_equals` | Collection size equals | `{variable: "orders", operator: "size_equals", value: "1"}` |
| `size_greater_than` | Collection size greater than | `{variable: "orders", operator: "size_greater_than", value: "1"}` |
| `size_greater_than_or_equal` | Collection size >= | `{variable: "orders", operator: "size_greater_than_or_equal", value: "1"}` |
| `size_less_than` | Collection size less than | `{variable: "orders", operator: "size_less_than", value: "5"}` |

#### Scenario: Equals operator
- **WHEN** variable `status` = `"4"` and condition is `{variable: "status", operator: "equals", value: "4"}`
- **THEN** the condition evaluates to true

#### Scenario: Greater than operator
- **WHEN** variable `count` = `5` and condition is `{variable: "count", operator: "greater_than", value: "0"}`
- **THEN** the condition evaluates to true

#### Scenario: Not empty operator
- **WHEN** variable `orderId` = `"ORD123"` and condition is `{variable: "orderId", operator: "not_empty"}`
- **THEN** the condition evaluates to true

#### Scenario: Size equals operator
- **WHEN** variable `orders` is a JSON array of length 1 and condition is `{variable: "orders", operator: "size_equals", value: "1"}`
- **THEN** the condition evaluates to true

### Requirement: Condition evaluator reuse
The condition evaluator SHALL be reusable across multiple contexts:
- While node exit conditions
- If-else node branch conditions
- Step-level execution conditions (`condition` field on steps)
- DAG conditional edges

All contexts SHALL call the same `EvaluateCondition(variable, operator, value, variables)` function.

#### Scenario: While exit condition uses evaluator
- **WHEN** a while node has `exit_conditions: [{variable: "status", operator: "equals", value: "4"}]`
- **THEN** after each iteration, `EvaluateCondition("status", "equals", "4", variables)` is called to check if the loop should exit
