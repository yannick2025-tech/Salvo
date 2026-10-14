## ADDED Requirements

### Requirement: CSV file upload and parsing
The system SHALL allow users to upload CSV files (with header row) as data sources for a scene. The CSV file name (without extension) MUST consist of only ASCII letters, digits, and underscores (`[a-zA-Z0-9_]+`), matching the variable naming convention. This ensures clean dot-notation references like `${users.password}`. The CSV file SHALL be parsed on upload: the first row is treated as column headers, and subsequent rows are data rows. The system SHALL store the file name, column names, and all row data.

#### Scenario: Upload a valid CSV file
- **WHEN** user uploads a CSV file named `users.csv` with content `username,password\nuser1,pass1\nuser2,pass2`
- **THEN** the system creates a data source with name `users`, columns `["username","password"]`, and 2 data rows

#### Scenario: Upload exceeds size limit
- **WHEN** user uploads a CSV file larger than 10MB
- **THEN** the system rejects the upload with error "file size exceeds 10MB limit"

#### Scenario: Upload with invalid file name
- **WHEN** user uploads a CSV file named `user-list.csv` or `用户数据.csv`
- **THEN** the system rejects the upload with error "file name must contain only letters, digits, and underscores"

#### Scenario: Upload with duplicate column names
- **WHEN** user uploads a CSV with duplicate column headers `name,name,age`
- **THEN** the system rejects the upload with error "duplicate column names detected"

### Requirement: CSV data source row iteration
During test execution, the system SHALL provide a thread-safe row iterator for each data source. Each chain iteration SHALL advance to the next row. When all rows are consumed, the iterator SHALL wrap around to the first row. The current row's column values SHALL be injected into the variable scope as `${datasource_name.column_name}`.

#### Scenario: Sequential row access
- **WHEN** data source `users` has 3 rows and the test runs 5 chain iterations
- **THEN** iterations 1-3 use rows 1-3, iteration 4 uses row 1, iteration 5 uses row 2

#### Scenario: Reference current row in node config
- **WHEN** a node's URL is `${base_url}/login` and body is `{"user":"${users.username}","pass":"${users.password}"}`
- **THEN** on iteration 1 (assuming row 1 has username=user1, password=pass1), the body resolves to `{"user":"user1","pass":"pass1"}`

#### Scenario: Concurrent access from multiple workers
- **WHEN** 200 workers simultaneously request the next row from the same data source
- **THEN** each worker gets a distinct row (atomic increment), no data races occur

### Requirement: CSV data source management UI
The Scene Detail page SHALL provide a data source management section where users can upload, preview, and delete CSV data sources. The preview SHALL show column names and the first 5 rows.

#### Scenario: Preview uploaded CSV
- **WHEN** user clicks on a data source entry
- **THEN** a preview panel shows the column headers and first 5 rows in a table

#### Scenario: Delete a data source
- **WHEN** user clicks delete on a data source
- **THEN** the data source is removed and any node references to it become unresolved (variable lookup returns empty)

### Requirement: YAML data source definition
The YAML format SHALL support a `data_sources` section that references uploaded CSV files by name. During import, if the referenced file exists, it is linked; otherwise a warning is shown.

#### Scenario: YAML with data source reference
- **WHEN** YAML contains `data_sources: [{name: users, file: users.csv}]`
- **THEN** the import links the existing `users` data source to the scene
