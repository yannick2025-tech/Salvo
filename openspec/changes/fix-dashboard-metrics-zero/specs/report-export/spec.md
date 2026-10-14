## ADDED Requirements

### Requirement: TimeSeriesCollector runID consistency with run_record

The system SHALL ensure that the TimeSeriesCollector's runID is identical to the run_record's ID, so that time series data stored by the collector can be queried using the run_record's ID.

#### Scenario: Single ID generation in Runner constructor
- **WHEN** Runner.New() creates a new Runner instance
- **THEN** the snowflake ID generator's Generate() SHALL be called exactly once to produce the runID
- **AND** the same runID SHALL be used for both the Runner.runID field and the TimeSeriesCollector's runID parameter
- **AND** the run_record created in Run() SHALL use this same runID as its primary key

#### Scenario: Time series data query by run_record ID succeeds
- **WHEN** Dashboard's buildTimeSeriesWithDB queries tsStore.QueryByRunID(ctx, run.ID)
- **THEN** the query SHALL return matching time series records
- **AND** QPS values in the returned data SHALL be non-zero when requests were made during the run
- **AND** P50/P95/P99 latency values SHALL reflect actual measured latencies

#### Scenario: Report time series data populated correctly
- **WHEN** createReport() calls r.tsStore.QueryByRunID(reportCtx, r.dbRecordID)
- **THEN** the query SHALL return matching time series records
- **AND** the report's GlobalTimeSeries field SHALL be populated with actual sampled data
- **AND** the report's NodeMetrics[].TimeSeries SHALL be populated with per-node sampled data

## MODIFIED Requirements

### Requirement: Frontend Report Detail Page - Export Button

**SHALL** add an export button to the report detail page allowing users to download the report as an HTML file with theme selection.

As a test engineer viewing a report
I want to see an export button on the report detail page
So that I can quickly download the report without navigating away

#### Scenario: View export button on completed report detail page

Given I am viewing the detail page of a completed report
When the page renders
Then I **SHALL** see an "导出报告" (Export Report) button in the action bar area
And button **SHALL HAVE** dropdown menu with options "导出 HTML（浅色主题）" and "导出 HTML（深色主题）"
And button **SHALL BE** enabled and clickable

---

#### Scenario: Click export button triggers download

Given I am on the report detail page
And I click "导出 HTML（浅色主题）"
Then the browser **SHALL** show loading spinner on button during request
And **SHALL** initiate file download automatically
And **SHALL** display success toast notification: "报告已开始下载"
And file name **SHALL MATCH** pattern: report-{id}-{timestamp}.html

---

#### Scenario: Export button hidden for running/incomplete reports

Given I am viewing a report that is still running or failed
When the page renders
Then the export button **SHALL NOT** be visible or **SHALL BE** disabled with tooltip explaining why

---

#### Scenario: Exported HTML report contains time series charts with data

Given I am viewing a completed report with time series data
When I click "导出 HTML（浅色主题）"
Then the downloaded HTML file **SHALL** contain QPS time series chart with non-zero values
And **SHALL** contain P50/P95/P99 latency time series charts with actual measured values
And **SHALL** contain error rate time series chart reflecting actual error rates
And all chart data **SHALL** come from the TimeSeriesCollector records matching the run_record ID

## ADDED Requirements

### Requirement: Failed request details in reports and HTML exports

The system SHALL include failed request details (FailedNodes) in both the database-stored report detail and the exported HTML report, ensuring users can inspect individual request failures.

#### Scenario: Failed nodes stored in report_details table
- **WHEN** a test run completes with failed HTTP requests
- **THEN** the report detail JSON SHALL include a `failed_nodes` array
- **AND** each entry SHALL contain: node_id, node_name, node_type, error_message, timestamp, request_url, request_method, request_headers, request_body, response_status, response_headers, response_body
- **AND** the `failed_nodes` array SHALL NOT be omitted by `omitempty` when entries exist

#### Scenario: HTML report renders failed request details section
- **WHEN** GenerateEnhancedHTML processes a report detail JSON with non-empty `failed_nodes`
- **THEN** the HTML output SHALL include a "失败节点详情" section
- **AND** each failed node SHALL be rendered as a card with error message, request details (method, URL, headers, body), and response details (status, headers, body)
- **AND** the section SHALL be visible without requiring user interaction (not collapsed by default for the error message)

#### Scenario: Failed nodes recorded for all failure types
- **WHEN** an HTTP request fails due to connection error (DNS, timeout, refused)
- **THEN** recordFailedNode SHALL be called with response_status=0 and empty response fields
- **AND** the error_message SHALL contain the connection error description
- **WHEN** an HTTP request returns a non-2xx status code
- **THEN** recordFailedNode SHALL be called regardless of the blockOnError setting
- **AND** the response_status, response_headers, and response_body SHALL be populated from the actual HTTP response
