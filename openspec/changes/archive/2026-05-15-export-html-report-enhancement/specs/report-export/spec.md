# Report Export Enhancement

## ADDED Requirements

### Requirement: Single HTML Export Endpoint

**MUST** provide a RESTful API endpoint to export a single test report as a rich, interactive HTML file that can be viewed offline and shared with stakeholders.

As a test engineer
I want to export a single test report as a rich HTML file
So that I can share it with stakeholders or view it offline

#### Scenario: Export completed report with default light theme

Given a completed test run with report ID "report-123"
And the report contains QPS data, latency metrics, and error breakdowns
When I call GET /api/v1/reports/report-123/export?format=html&theme=light
Then the response **MUST** have status code 200
And Content-Type header **MUST BE** text/html; charset=utf-8
And Content-Disposition header **MUST BE** attachment; filename="report-report-123-20260513.html"
And Body **MUST** contain valid HTML5 document with executive summary section showing key metrics (total requests, success rate, P99 latency)
And **MUST** include interactive ECharts QPS trend chart using time series data from report detail
And **MUST** include latency distribution charts (P50, P90, P95, P99)
And **MUST** include error rate breakdown by error code (pie chart)
And **MUST** include node performance summary table
And **MUST** include test configuration details (workers, duration/count, run mode)
And **MUST** apply responsive CSS layout adapting to viewport width
And **MUST** include theme toggle button in header (currently set to light theme)
And **MUST** include print-friendly styles (@media print)

---

#### Scenario: Export report with dark theme

Given a completed test run with report ID "report-456"
When I call GET /api/v1/reports/report-456/export?format=html&theme=dark
Then the generated HTML **MUST** apply dark theme CSS variables (--bg-primary: #1f2937, --text-primary: #f9fafb)
And all charts **MUST** use dark color scheme suitable for dark backgrounds
And text **MUST** remain readable (contrast ratio >= 4.5:1)
And theme toggle button **MUST** show current state as "dark"

---

#### Scenario: Attempt to export non-existent report

Given no report exists with ID "nonexistent-id"
When I call GET /api/v1/reports/nonexistent-id/export?format=html
Then the API **MUST** return status code 404
And JSON body **MUST BE** { "code": 404, "message": "report not found" }

---

#### Scenario: Attempt to export with invalid format parameter

Given a completed test run exists
When I call GET /api/v1/reports/report-123/export?format=pdf
Then the API **MUST** return status code 400
And JSON body **MUST BE** { "code": 400, "message": "invalid format. supported: html" }

---

### Requirement: Batch Export Endpoint

**MUST** provide a batch export endpoint that accepts multiple report IDs and generates a ZIP archive containing individual HTML reports and an index page.

As a QA manager
I want to export multiple reports as a ZIP archive
So that I can distribute them efficiently to my team

#### Scenario: Batch export 3 completed reports

Given 3 completed test reports exist with IDs ["r1", "r2", "r3"]
And each report has full metric and time series data
When I call POST /api/v1/reports/batch-export with body containing ids ["r1", "r2", "r3"], format "html", theme "light", include_index true
Then the response **MUST** have status code 200
And Content-Type **MUST BE** application/zip
And Content-Disposition **MUST BE** attachment; filename="reports-batch-20260513.zip"
And ZIP archive **MUST** contain index.html navigation page with links to all reports
And ZIP archive **MUST** contain r1.html full enhanced HTML report for r1
And ZIP archive **MUST** contain r2.html full enhanced HTML report for r2
And ZIP archive **MUST** contain r3.html full enhanced HTML report for r3
And each HTML file **MUST** be self-contained and work offline
And generation **MUST** complete within 5 seconds for <=10 reports

---

#### Scenario: Batch export exceeds maximum limit

Given 25 completed test reports exist
When I call POST /api/v1/reports/batch-export with body containing all 25 IDs
Then the API **MUST** return status code 400
And JSON body **MUST BE** { "code": 400, "message": "batch size exceeds maximum of 20 reports" }

---

#### Scenario: Batch export includes non-existent report ID

Given reports exist for IDs ["valid-id"]
When I call batch export with IDs ["valid-id", "invalid-id"]
Then the API **MUST** return status code 404
And JSON body **MUST** indicate which ID(s) were not found

---

## ADDED Requirements

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

### Requirement: Frontend Reports List Page - Batch Selection and Export

**SHALL** enhance the reports list page with checkboxes for multi-select and a batch export action to generate ZIP archives of selected reports.

As a QA manager reviewing multiple test results
I want to select multiple reports and batch export them
So that I can efficiently prepare a package for stakeholders

#### Scenario: See checkboxes on report list table

Given I am on the reports list page
And there are completed reports in the list
When the page renders
Then each row **SHALL** have a checkbox column on the left side
And a "全选" (Select All) checkbox **SHALL APPEAR** in the table header
And checkboxes **SHALL BE** only interactive for completed reports (disabled for running/failed)

---

#### Scenario: Select reports and show batch action bar

Given I am on the reports list page
And I check 2 completed reports
Then a floating action bar **SHALL APPEAR** at bottom showing selection count "✅ 已选择 2 项"
And "批量导出 ▼" (Batch Export) button **SHALL APPEAR** with dropdown for themes
 And "取消选择" (Clear Selection) button **SHALL APPEAR**

---

#### Scenario: Perform batch export from list page

Given I have selected 3 completed reports
And I click "批量导出 → 浅色主题"
Then the system **SHALL** show progress indicator if > 5 reports selected
And **SHALL CALL** POST /api/v1/reports/batch-export with selected IDs
And **SHALL** download ZIP file when complete
And **SHALL SHOW** success notification: "已成功导出 3 个报告"
And **SHALL CLEAR** selection after successful download
