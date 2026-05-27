## 
D Requirements

### Requirement: Variable editing GUI
The system SHALL provide a variable editing panel on the Scene Detail page that allows users to add, edit, and delete scene-level variables. Each variable entry SHALL have a key and value field. Changes SHALL be persisted to the scene's `variables` JSON field.

#### Scenario: Add a new variable
- **WHEN** user clicks the "+ Add Variable" button in the variable panel
- **THEN** a new empty key-value row appears, and the user can type the key and value

#### Scenario: Edit an existing variable
- **WHEN** user modifies the value of an existing variable in the panel
- **THEN** the change is saved to the scene's variables on blur or explicit save

#### Scenario: Delete a variable
- **WHEN** user clicks the delete icon next to a variable entry
- **THEN** the variable is removed from the list and the change is persisted

### Requirement: Nested variable reference resolution
The system SHALL support nested variable references where a variable's value contains `${other_var}` placeholders that reference other variables. Resolution SHALL be recursive up to a maximum depth of 10 levels. If a circular reference is detected (depth exceeded), the system SHALL return an error instead of infinite recursion.

#### Scenario: Variable A references variable B
- **WHEN** variable `api_path` is set to `/api/v1` and variable `full_url` is set to `${base_url}${api_path}`
- **THEN** resolving `full_url` produces `http://localhost:8080/api/v1` (assuming `base_url` = `http://localhost:8080`)

#### Scenario: Circular reference detection
- **WHEN** variable `a` = `${b}` and variable `b` = `${a}`
- **THEN** the system returns an error "circular variable reference detected" instead of infinite recursion

#### Scenario: Expression concatenation
- **WHEN** a node's URL config is `${base_url}/api/v1/${path}` and `base_url` = `http://host` and `path` = `orders`
- **THEN** the resolved URL is `http://host/api/v1/orders`

### Requirement: Variable display in YAML import
The YAML import dialog SHALL display the variables parsed from the YAML content in a preview section before confirming import.

#### Scenario: YAML with variables
- **WHEN** user imports a YAML containing `variables: [{key: base_url, value: http://localhost}]`
- **THEN** the import dialog shows a preview listing `base_url = http://localhost`
