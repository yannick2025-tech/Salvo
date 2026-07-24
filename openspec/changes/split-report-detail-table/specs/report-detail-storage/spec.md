# Report Detail Storage

## ADDED Requirements

### Requirement: Separate table for report details

The system SHALL store report details in a separate `report_details` table with a one-to-one relationship to the `reports` table.

#### Scenario: Report detail table schema

- **WHEN** system initializes the database schema
- **THEN** system SHALL create a `report_details` table with the following structure:
  - `report_id`: INTEGER PRIMARY KEY (foreign key to `reports.id`)
  - `detail`: TEXT (contains the complete ReportDetail JSON)
- **AND** system SHALL create a foreign key constraint with `ON DELETE CASCADE`
- **AND** system SHALL ensure one-to-one relationship between `reports` and `report_details`

#### Scenario: Insert report with detail

- **WHEN** system creates a new report with detail data
- **THEN** system SHALL insert a row into both `reports` and `report_details` tables
- **AND** system SHALL use the same ID for both tables
- **AND** system SHALL ensure atomicity (both inserts succeed or both fail)

#### Scenario: Delete report with detail

- **WHEN** system deletes a report
- **THEN** system SHALL automatically delete the corresponding row in `report_details` table
- **AND** deletion SHALL be atomic (both deletions succeed or both fail)
- **AND** deletion SHALL cascade via foreign key constraint

### Requirement: Remove detail field from reports table

The system SHALL remove the `detail` field from the `reports` table after migration.

#### Scenario: Reports table schema after migration

- **WHEN** migration completes successfully
- **THEN** `reports` table SHALL NOT have a `detail` column
- **AND** `reports` table SHALL retain all other fields: `id`, `scene_id`, `run_id`, `status`, `summary`, `started_at`, `finished_at`, `created_at`, `updated_at`, `deleted_at`

#### Scenario: List reports without detail field

- **WHEN** system queries the `reports` table
- **THEN** query SHALL NOT include the `detail` field (since it's been removed)
- **AND** query SHALL only return fields from the `reports` table
- **AND** query performance SHALL be optimized due to smaller row size

### Requirement: JOIN query for report detail retrieval

The system SHALL use JOIN queries to retrieve complete report data when detail is needed.

#### Scenario: Get single report with detail

- **WHEN** system retrieves a report by ID with detail
- **THEN** system SHALL JOIN `reports` and `report_details` tables on `id = report_id`
- **AND** system SHALL return the complete `ReportDTO` including the `detail` field
- **AND** query execution time SHALL be less than 100 milliseconds

#### Scenario: Get report without detail (for list operations)

- **WHEN** system retrieves reports for list display
- **THEN** system SHALL query only the `reports` table
- **AND** system SHALL NOT JOIN with `report_details` table
- **AND** query SHALL be optimized for performance

### Requirement: Data migration strategy

The system SHALL provide a migration script to split existing data into two tables.

#### Scenario: Migration script execution

- **WHEN** migration script is executed
- **THEN** system SHALL:
  1. Create `report_details` table
  2. Copy `id` and `detail` from `reports` to `report_details`
  3. Drop `detail` column from `reports` table
  4. Create necessary indexes
- **AND** migration SHALL be atomic (all steps succeed or all fail)
- **AND** migration SHALL preserve all existing data

#### Scenario: Migration rollback

- **WHEN** migration fails or rollback is requested
- **THEN** system SHALL restore the original schema:
  1. Add `detail` column back to `reports` table
  2. Copy `detail` from `report_details` to `reports`
  3. Drop `report_details` table
- **AND** rollback SHALL restore all data to original state
- **AND** rollback SHALL not lose any data

### Requirement: Index optimization for queries

The system SHALL create indexes to optimize common query patterns.

#### Scenario: Index on reports table

- **WHEN** system initializes the database schema
- **THEN** system SHALL create the following indexes on the `reports` table:
  - `idx_reports_scene_id` on `scene_id`
  - `idx_reports_run_id` on `run_id`
  - `idx_reports_status` on `status`
  - `idx_reports_started_at` on `started_at`
- **AND** indexes SHALL optimize filtering and sorting operations

#### Scenario: Primary key on report_details

- **WHEN** system creates the `report_details` table
- **THEN** system SHALL use `report_id` as PRIMARY KEY
- **AND** primary key SHALL automatically create an index
- **AND** index SHALL optimize JOIN operations

### Requirement: Referential integrity

The system SHALL maintain referential integrity between `reports` and `report_details` tables.

#### Scenario: Foreign key constraint

- **WHEN** system creates the `report_details` table
- **THEN** system SHALL add a foreign key constraint:
  - `FOREIGN KEY (report_id) REFERENCES reports(id) ON DELETE CASCADE`
- **AND** constraint SHALL ensure that every `report_id` in `report_details` exists in `reports`
- **AND** constraint SHALL prevent orphaned detail rows

#### Scenario: Prevent invalid detail insertion

- **WHEN** system attempts to insert a row into `report_details` with a non-existent `report_id`
- **THEN** database SHALL reject the insertion
- **AND** database SHALL raise a foreign key constraint violation error

### Requirement: Storage efficiency

The system SHALL optimize storage for large detail fields.

#### Scenario: Detail field size

- **WHEN** system stores a report detail
- **THEN** system SHALL store the complete `ReportDetail` JSON in the `detail` TEXT field
- **AND** field size SHALL be sufficient to hold large reports (up to 5MB)
- **AND** system SHALL not apply size limits that truncate data

#### Scenario: Database storage optimization

- **WHEN** database engine stores TEXT fields
- **THEN** database engine SHALL use efficient storage mechanisms (e.g., SQLite's BLOB storage for large TEXT)
- **AND** system SHALL consider future compression optimizations (not implemented in this phase)

### Requirement: Query performance validation

The system SHALL validate that the new schema meets performance targets.

#### Scenario: List query performance benchmark

- **WHEN** system executes a list query for 50 reports
- **THEN** query execution time SHALL be less than 100 milliseconds
- **AND** response payload size SHALL be less than 50KB
- **AND** performance SHALL be at least 10x better than before optimization

#### Scenario: Detail query performance benchmark

- **WHEN** system executes a JOIN query to retrieve a single report with detail
- **THEN** query execution time SHALL be less than 100 milliseconds
- **AND** performance SHALL be comparable to the single-table query before optimization