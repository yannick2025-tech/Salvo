# html-report-export Specification

## Purpose
TBD - created by archiving change fix-exported-report-empty-charts. Update Purpose after archive.
## Requirements
### Requirement: HTML report exports with complete chart data

The exported HTML report SHALL embed all time-series chart data as a JavaScript object literal (not a string), so that ECharts can render charts correctly.

#### Scenario: Export report with chart data
- **WHEN** user clicks "导出 HTML" button on a report detail page
- **THEN** the downloaded HTML file contains a `<script>` block with `const reportData = {...}` where the value is a JavaScript object literal (not a quoted string)
- **AND** all ECharts charts (overview, error rate, latency bar, QPS trend, latency trend, node charts, error breakdown) render with data

#### Scenario: Chart data structure
- **WHEN** the HTML report is opened in a browser
- **THEN** `reportData.metrics` SHALL be a valid JavaScript object containing `timestamps`, `ts_qps`, `ts_p50`, `ts_p90`, `ts_p95`, `ts_p99`, `ts_total`, `ts_fail` arrays
- **AND** `reportData.node_metrics` SHALL be an array of node metric objects, each with `timestamps`, `ts_qps`, `ts_p50`, `ts_p90`, `ts_p95`, `ts_p99` arrays
- **AND** `reportData.error_breakdown` SHALL be an array of error objects with `error_type`, `message`, `count` fields

#### Scenario: Backward compatibility
- **WHEN** the report detail page is viewed online (not exported)
- **THEN** the online page SHALL continue to work unchanged
- **AND** existing API endpoints SHALL return the same response format

