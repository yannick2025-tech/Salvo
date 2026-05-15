# Tasks: Export HTML Report Enhancement

## Backend Tasks

### B-1: Enhance HTML Template
**File:** `internal/api/report_generator.go`

**Subtasks:**
- [x] Create new template constant with ECharts integration
- [x] Add CSS variables for theming (light/dark)
- [x] Implement responsive layout grid system
- [x] Add executive summary section with metric cards
- [x] Add QPS trend chart section
- [x] Add latency distribution charts (P50/P90/P95/P99)
- [x] Add error breakdown chart
- [x] Add node performance table
- [x] Add test configuration details section
- [x] Add theme toggle JavaScript function
- [x] Add print-friendly media queries

**Acceptance Criteria:**
- Template renders valid HTML5
- All sections render correctly with sample data
- Light/dark themes apply correctly
- Layout adapts to mobile/tablet/desktop

---

### B-2: Implement Export API Endpoint
**File:** `internal/api/handler.go`

**Subtasks:**
- [x] Add `GET /api/v1/reports/{id}/export` route
- [x] Implement `ExportReportHTML` handler
  - Parse query params: `format`, `theme`
  - Validate report exists and is completed
  - Call `GenerateEnhancedHTML()`
  - Set `Content-Disposition` header
  - Return HTML as attachment
- [x] Add input validation and error handling
- [x] Write unit tests for handler

**API Contract:**
```go
func (h *Handler) ExportReportHTML(w http.ResponseWriter, r *http.Request) {
    // 1. Extract report ID from URL params
    // 2. Parse query parameters (format=html, theme=light|dark)
    // 3. Fetch report from database
    // 4. Generate enhanced HTML using new template
    // 5. Set headers:
    //    Content-Type: text/html; charset=utf-8
    //    Content-Disposition: attachment; filename="report-{id}.html"
    // 6. Write response
}
```

**Acceptance Criteria:**
- Returns valid HTML file
- Filename format: `report-{timestamp}.html`
- Supports light/dark theme parameter
- Returns 404 for non-existent reports
- Returns 400 for invalid format parameter

---

### B-3: Implement Batch Export Endpoint
**File:** `internal/api/handler.go`

**Subtasks:**
- [x] Add `POST /api/v1/reports/batch-export` route
- [x] Implement `BatchExportReports` handler
  - Parse request body: `{ ids: [], format, theme }`
  - Validate all report IDs exist
  - Generate individual HTML files
  - Create ZIP archive with index.html
  - Return ZIP as download
- [x] Add batch size limit validation (max 20 reports)
- [x] Implement ZIP generation utility function
- [x] Write unit tests

**Request/Response:**
```go
type BatchExportRequest struct {
    IDs          []string `json:"ids"`
    Format       string   `json:"format"`       // "html"
    Theme        string   `json:"theme"`        // "light" | "dark"
    IncludeIndex bool     `json:"include_index"` // default true
}

// Response: application/zip
// Filename: reports-batch-{timestamp}.zip
```

**ZIP Contents:**
```
reports-batch-2026-05-13.zip
├── index.html          # Links to all reports
├── report-id1.html
├── report-id2.html
└── report-id3.html
```

**Acceptance Criteria:**
- Generates valid ZIP file
- Contains all requested reports
- Includes navigation index page
- Returns 400 if > 20 reports requested
- Returns 404 if any ID doesn't exist

---

## Frontend Tasks

### F-1: Add Export Button to Report Detail Page
**File:** `web/app/src/views/reports/ReportDetailPage.vue`

**Subtasks:**
- [x] Add "导出报告" button in action bar
- [x] Show dropdown menu with options:
  - 导出 HTML（浅色主题）
  - 导出 HTML（深色主题）
- [x] Implement export function:
  ```typescript
  async function exportReport(theme: 'light' | 'dark') {
    const token = localStorage.getItem('salvo_token')
    const response = await fetch(`/api/v1/reports/${reportId.value}/export?format=html&theme=${theme}`, {
      headers: { Authorization: `Bearer ${token}` }
    })
    
    const blob = await response.blob()
    const url = window.URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `report-${reportId.value}.html`
    a.click()
    window.URL.revokeObjectURL(url)
  }
  ```
- [x] Add loading state during export
- [x] Add success/error toast notifications
- [x] Disable button while exporting

**UI Design:**
```
┌─────────────────────────────────────┐
│  报告详情                    [返回] │
├─────────────────────────────────────┤
│                                     │
│  [查看] [编辑] [📥 导出 ▼]         │
│                     ├ 浅色主题      │
│                     └ 深色主题      │
│                                     │
│  ... report content ...             │
└─────────────────────────────────────┘
```

**Acceptance Criteria:**
- Button visible on completed reports only
- Dropdown shows theme options
- Download starts automatically
- Loading spinner shown during request
- Success toast: "报告已开始下载"

---

### F-2: Add Batch Export to Reports List Page
**File:** `web/app/src/views/reports/ReportsPage.vue`

**Subtasks:**
- [x] Add checkbox column to report table
- [x] Add "全选" checkbox in table header
- [x] Show floating action bar when items selected:
  ```
  已选择 3 项  [批量导出 ▼]  [取消选择]
  ```
- [x] Implement batch export function:
  ```typescript
  async function batchExportReports(theme: 'light' | 'dark') {
    const selectedIds = Array.from(selectedReportIds.value)
    
    const response = await fetch('/api/v1/reports/batch-export', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`
      },
      body: JSON.stringify({
        ids: selectedIds,
        format: 'html',
        theme,
        include_index: true
      })
    })
    
    const blob = await response.blob()
    const url = window.URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `reports-batch-${Date.now()}.zip`
    a.click()
    window.URL.revokeObjectURL(url)
  }
  ```
- [x] Add selection count indicator
- [x] Disable export when 0 items selected
- [x] Clear selection after successful export

**UI Design:**
```
┌─────────────────────────────────────────────┐
│  测试报告                      [+ 新建测试] │
├──────┬──────────┬────────┬────────┬────────┤
│ ☑️   │ 场景名称  │ 状态   │ 成功率  │ 时间   │
│ ☐   │ Scene A  │ ✅ 完成│ 98.5%  │ 10:30  │
│ ☑️   │ Scene B  │ ✅ 完成│ 95.2%  │ 11:15  │
│ ☐   │ Scene C  │ 🔄 运行中│ --    │ --    │
├──────┴──────────┴────────┴────────┴────────┤
│ ✅ 已选择 2 项   [📦 批量导出 ▼]  [✕ 取消] │
│                   ├ 浅色主题               │
│                   └ 深色主题               │
└─────────────────────────────────────────────┘
```

**Acceptance Criteria:**
- Checkboxes appear on each row
- Selection persists across pagination
- Action bar appears when ≥1 item selected
- Batch export downloads ZIP file
- Progress indicator for large batches (>5 reports)

---

## Testing Tasks

### T-1: Unit Tests for Report Generator
**File:** `internal/api/report_generator_test.go`

**Test Cases:**
- [x] Test template renders with valid data
- [x] Test empty data handling (no time series)
- [x] Test theme switching in generated HTML
- [x] Test responsive meta tags present
- [x] Test ECharts script tags included
- [x] Test special characters escaped properly

---

### T-2: Integration Tests for API Endpoints
**File:** `internal/api/server_test.go`

**Test Cases:**
- [x] Test GET /export returns HTML for existing report
- [x] Test GET /export returns 404 for missing report
- [x] Test GET /export respects theme parameter
- [x] Test POST /batch-export creates valid ZIP
- [x] Test POST /batch-export rejects > 20 IDs
- [x] Test POST /batch-export validates all IDs exist

---

### T-3: E2E Tests for Frontend
**File:** `web/e2e/export.spec.ts`

**Test Scenarios:**
- [x] User can export single report from detail page
- [x] User can choose theme before export
- [x] User sees success notification after export
- [x] User can select multiple reports on list page
- [x] User can batch export selected reports
- [x] Downloaded HTML opens correctly in browser
- [x] Downloaded ZIP contains correct files

---

## Documentation Tasks

### D-1: Update API Documentation
- [x] Add `/export` endpoint to API docs
- [x] Add `/batch-export` endpoint to API docs
- [x] Include request/response examples
- [x] Document query parameters and error codes

### D-2: Update User Guide
- [x] Add "Exporting Reports" section
- [x] Include screenshots of UI
- [x] Explain batch export workflow
- [x] Document offline viewing capabilities

---

## Implementation Order

**Recommended Sequence:**

```
Week 1:
  Day 1-2: B-1 (Template Enhancement) ← Foundation
  Day 3:   B-2 (Single Export API)    ← Quick win

Week 2:
  Day 4:   F-1 (Detail Page Export)   ← User-visible feature
  Day 5:   B-3 + F-2 (Batch Export)   ← Advanced feature
  Day 6-7: T-1, T-2, D-1 (Tests & Docs) ← Polish
```

**Dependencies:**
```
B-1 ──→ B-2 ──→ F-1
              └──→ B-3 ──→ F-2
                        └──→ T-2, T-3, D-1, D-2
B-1 ──→ T-1 (can be parallel)
```
