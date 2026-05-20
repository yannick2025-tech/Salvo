# report-export Specification

## Purpose
TBD - created by archiving change export-html-report-enhancement. Update Purpose after archive.
## Requirements
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

