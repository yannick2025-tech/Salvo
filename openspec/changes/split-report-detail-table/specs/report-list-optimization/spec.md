# Report List Optimization

## ADDED Requirements

### Requirement: Lightweight list API without detail field

The system SHALL provide a lightweight list API that returns report metadata without the `detail` field to optimize query performance.

#### Scenario: List reports with performance target

- **WHEN** client calls `POST /api/v1/reports/list` with pagination parameters
- **THEN** system SHALL return a list of `ReportListItemDTO` objects within 1 second
- **AND** each item SHALL contain only `id`, `scene_id`, `run_id`, `status`, `summary`, `started_at`, `finished_at`, `created_at`, `updated_at`
- **AND** each item SHALL NOT contain the `detail` field

#### Scenario: List reports with filters

- **WHEN** client calls `POST /api/v1/reports/list` with `scene_id` filter
- **THEN** system SHALL return only reports matching the specified scene
- **AND** response time SHALL be less than 1 second

### Requirement: List API maintains backward-compatible field structure

The system SHALL maintain the same field structure for list items as the original `ReportDTO` (excluding `detail`), ensuring frontend compatibility.

#### Scenario: Frontend displays list without modification

- **WHEN** frontend receives the lightweight list response
- **THEN** system SHALL provide all fields required by the current list page (ID, Scene, Status, Total Requests, Success Rate, P50/P95/P99, Times)
- **AND** these fields SHALL be extracted from the `summary` field
- **AND** frontend SHALL NOT require code changes to display the list

### Requirement: Summary field contains essential metrics

The system SHALL ensure the `summary` field contains all metrics required by the list page, including `total_reqs`, `success_rate`, `p50`, `p95`, `p99`.

#### Scenario: Extract metrics from summary

- **WHEN** frontend parses the `summary` field
- **THEN** system SHALL provide the following metrics:
  - `total_reqs`: total number of requests
  - `success_rate`: success rate (percentage)
  - `p50`: 50th percentile latency in milliseconds
  - `p95`: 95th percentile latency in milliseconds
  - `p99`: 99th percentile latency in milliseconds
- **AND** all metrics SHALL be accurate and consistent with the detailed report data

### Requirement: Database query optimization

The system SHALL query only the `reports` table for list operations, avoiding joins with the `report_details` table.

#### Scenario: List query performance

- **WHEN** system executes a list query for 50 reports
- **THEN** system SHALL query only the `reports` table
- **AND** query execution time SHALL be less than 100 milliseconds
- **AND** response payload size SHALL be less than 50KB (compared to 50MB+ before optimization)

### Requirement: Pagination support

The system SHALL support pagination for the list API with configurable offset and limit parameters.

#### Scenario: Paginated list request

- **WHEN** client calls `POST /api/v1/reports/list` with `offset=10` and `limit=20`
- **THEN** system SHALL return 20 reports starting from the 11th report
- **AND** response SHALL include pagination metadata (total count, offset, limit)

#### Scenario: Default pagination

- **WHEN** client calls `POST /api/v1/reports/list` without pagination parameters
- **THEN** system SHALL return up to 50 reports by default
- **AND** system SHALL use `offset=0` as the default offset