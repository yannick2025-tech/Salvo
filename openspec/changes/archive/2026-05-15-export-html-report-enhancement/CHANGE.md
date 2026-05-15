# Export HTML Report Enhancement

## 📋 Overview

Enhance the HTML report export functionality to provide rich, interactive, and visually appealing test reports that can be easily shared and viewed offline.

## 🎯 Goals

1. **Rich Visualizations**: Include ECharts charts (QPS, Latency, Error Rate) in exported HTML
2. **Responsive Design**: Reports should look great on desktop, tablet, and mobile devices
3. **Customizable Styling**: Support light/dark themes and customizable branding
4. **Batch Export**: Allow exporting multiple reports at once
5. **Metadata Enrichment**: Include test environment info, test configuration, and executive summary

## 📊 Current State

### Existing Implementation

- **Backend**: [`report_generator.go`](../../internal/api/report_generator.go)
  - Basic HTML template with inline CSS
  - Simple metrics display (total requests, success rate, latency percentiles)
  - No chart visualizations
  - Limited styling options

- **Frontend**: [`ReportsPage.vue`](../../web/app/src/views/reports/ReportsPage.vue)
  - Report list view
  - Online report detail view with ECharts
  - No export functionality exposed to users

- **Data Model**: [`model.go`](../../internal/store/model/model.go)
  - `Report` struct: SceneID, RunID, Status, Summary, Detail
  - `Detail` JSON contains: metrics, node details, time series data

## 🔧 Requirements

### Functional Requirements

#### FR-1: Enhanced HTML Template
- [ ] Include ECharts library via CDN for offline rendering
- [ ] Render QPS trend chart (time series data from `Detail.TimeSeries`)
- [ ] Render P50/P90/P95/P99 latency distribution charts
- [ ] Render error rate breakdown by error code
- [ ] Display node-level performance summary table
- [ ] Show test configuration (workers, duration/count, mode)

#### FR-2: Responsive Layout
- [ ] Desktop: Full-width layout with sidebar navigation
- [ ] Tablet: Stacked layout with collapsible sections
- [ ] Mobile: Single-column optimized view
- [ ] Print-friendly CSS (@media print)

#### FR-3: Theme Support
- [ ] Light theme (default): White background, blue accents
- [ ] Dark theme option: Dark background, purple accents
- [ ] Auto-detect based on system preference
- [ ] Theme toggle button in header

#### FR-4: Export API Endpoint
- [ ] `GET /api/v1/reports/{id}/export?format=html&theme=light`
- [ ] Response: HTML file with Content-Disposition attachment header
- [ ] Support batch export: `POST /api/v1/reports/batch-export`
- [ ] Request body: `{ "ids": ["id1", "id2"], "format": "html", "theme": "light" }`
- [ ] Response: ZIP file containing multiple HTML reports

#### FR-5: Frontend Integration
- [ ] Add "Export" button on report detail page
- [ ] Add "Batch Export" button on report list page (with checkboxes)
- [ ] Export progress indicator for batch operations
- [ ] Success/error notifications

### Non-Functional Requirements

#### NFR-1: Performance
- Single report generation < 500ms
- Batch export (10 reports) < 5s
- HTML file size < 2MB per report

#### NFR-2: Compatibility
- Chrome 90+, Firefox 88+, Safari 14+, Edge 90+
- No JavaScript framework dependency (vanilla JS only)
- All assets embedded or CDN-linked

#### NFR-3: Accessibility
- Semantic HTML5 structure
- ARIA labels for charts
- Keyboard navigable
- Color contrast ratio ≥ 4.5:1

## 🎨 Design Specifications

### Page Structure

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <!-- Meta, Title, Styles -->
</head>
<body>
    <header class="report-header">
        <!-- Logo, Title, Theme Toggle -->
    </header>

    <nav class="report-nav">
        <!-- Navigation Links -->
    </nav>

    <main class="report-content">
        <section id="executive-summary">
            <!-- Key Metrics Cards -->
        </section>

        <section id="performance-charts">
            <!-- QPS, Latency, Error Charts -->
        </section>

        <section id="node-details">
            <!-- Node Performance Table -->
        </section>

        <section id="error-analysis">
            <!-- Error Breakdown -->
        </section>

        <section id="test-configuration">
            <!-- Test Settings -->
        </section>
    </main>

    <footer class="report-footer">
        <!-- Generated timestamp, Version -->
    </footer>
</body>
</html>
```

### Color Palette

```css
/* Light Theme */
--primary-color: #6366f1;      /* Indigo */
--success-color: #10b981;      /* Emerald */
--warning-color: #f59e0b;      /* Amber */
--danger-color: #ef4444;       /* Red */
--bg-primary: #ffffff;
--bg-secondary: #f9fafb;
--text-primary: #111827;
--text-secondary: #6b7280;

/* Dark Theme */
--primary-color: #818cf8;
--success-color: #34d399;
--warning-color: #fbbf24;
--danger-color: #f87171;
--bg-primary: #1f2937;
--bg-secondary: #374151;
--text-primary: #f9fafb;
--text-secondary: #9ca3af;
```

## 🧪 Acceptance Criteria

### AC-1: Single Report Export
**Given** a completed test run with report data
**When** user clicks "Export HTML" button
**Then** browser downloads an HTML file containing:
- Executive summary with key metrics
- Interactive ECharts (QPS, latency, error rate)
- Node performance breakdown
- Test configuration details
- Responsive layout working on all screen sizes

### AC-2: Batch Export
**Given** multiple completed test runs selected
**When** user clicks "Batch Export" button
**Then** system generates ZIP file containing:
- Individual HTML files for each report
- Index.html with links to all reports
- Total generation time < 5s for 10 reports

### AC-3: Theme Switching
**Given** an exported HTML report is open
**When** user clicks theme toggle button
**Then** report immediately switches between light/dark themes without reload

## 📎 References

- Current template: [`report_generator.go`](../../internal/api/report_generator.go)
- Report model: [`model.go - Report struct`](../../internal/store/model/model.go)
- Frontend detail view: [`ReportDetailPage.vue`](../../web/app/src/views/reports/ReportDetailPage.vue)
- Dashboard charts: [`DashboardPage.vue`](../../web/app/src/views/dashboard/DashboardPage.vue) (for ECharts config reference)

## ⚠️ Constraints

- Must maintain backward compatibility with existing report data format
- Cannot introduce external build tools (keep it simple Go templates)
- Must work offline once downloaded (CDN links acceptable)
