## ADDED Requirements

### Requirement: __weightedChoice function
The system SHALL implement `__weightedChoice(keyValuePairs)` that performs weighted random selection. The argument is a string in format `"key1=weight1,key2=weight2,..."`. Weights are normalized if their sum is not 100. Returns the selected key.

#### Scenario: Binary weighted choice
- **WHEN** `${__weightedChoice("1=50,0=50")}` is called
- **THEN** returns "1" with ~50% probability, "0" with ~50% probability

#### Scenario: Multi-way weighted choice with normalization
- **WHEN** `${__weightedChoice("A=40,B=30,C=20")}` is called (sum=90)
- **THEN** weights are normalized: A=44.4%, B=33.3%, C=22.2%

#### Scenario: Single option
- **WHEN** `${__weightedChoice("A=100")}` is called
- **THEN** always returns "A"

### Requirement: __oneOf function
The system SHALL implement `__oneOf(item1, item2, ...)` that returns one of the arguments with equal probability.

#### Scenario: Equal probability selection
- **WHEN** `${__oneOf("A", "B", "C")}` is called
- **THEN** returns "A", "B", or "C" each with ~33.3% probability

### Requirement: __manOf function
The system SHALL implement `__manOf(item1, item2, ...)` that returns a comma-separated random subset (1 to N items) of the input arguments.

#### Scenario: Random subset
- **WHEN** `${__manOf(1, 2, 3, 4, 5, 6, 7)}` is called
- **THEN** returns a comma-separated subset like "2,4,7" (random size 1-7)

### Requirement: __Random function
The system SHALL implement `__Random(min, max)` for integers and `__Random(min, max, scale)` for floats. Range is [min, max] inclusive. For floats, `scale` specifies decimal places.

#### Scenario: Integer random
- **WHEN** `${__random(60, 600)}` is called
- **THEN** returns an integer between 60 and 600 inclusive

#### Scenario: Float random
- **WHEN** `${__random(1.5, 9.5, 2)}` is called
- **THEN** returns a float like "3.47" with 2 decimal places

### Requirement: __snowflakeId function
The system SHALL implement `__snowflakeId()` that returns a 19-digit snowflake ID as a string.

#### Scenario: Generate snowflake ID
- **WHEN** `${__snowflakeId()}` is called
- **THEN** returns a unique 19-digit string like "1234567890123456789"

### Requirement: __so function
The system SHALL implement `__so(pluginRef, op, args...)` that calls a loaded SO plugin. `pluginRef` can be `"name"` (latest version) or `"name@version"` (specific version). The function looks up the plugin, calls `Plugin.Call(op, args...)`, and returns the result string.

#### Scenario: Call SO plugin latest version
- **WHEN** `${__so("shell-aes", "encrypt", "data")}` is called and shell-aes is loaded
- **THEN** calls the latest version's `Call("encrypt", "data")` and returns the result

#### Scenario: Call SO plugin specific version
- **WHEN** `${__so("shell-aes@1.0.0", "encrypt", "data")}` is called
- **THEN** calls version 1.0.0's `Call("encrypt", "data")` and returns the result

#### Scenario: Plugin not found
- **WHEN** `${__so("unknown-plugin", "op", "arg")}` is called and plugin is not loaded
- **THEN** the expression is left unresolved (original text preserved), and an error is logged
