# Design: Export HTML Report Enhancement

## Core Requirement (CRITICAL)

**MUST** produce an HTML report that is **pixel-perfect identical** to the online report detail page ([ReportDetailPage.vue](../../web/app/src/views/reports/ReportDetailPage.vue)).

### What "Completely Identical" Means:
- ✅ Same data items (all 8 metric cards, not just 6)
- ✅ Same visual styling (colors, fonts, spacing, borders, shadows)
- ✅ Same layout structure (grid system, responsive breakpoints)
- ✅ Same ECharts configurations (QPS trend, latency distribution, error breakdown)
- ✅ Same Chinese labels and tooltips
- ✅ Same chart type toggle buttons (平滑/阶梯)
- ✅ Same node ranking table with all columns
- ✅ Same node details section with individual charts
- ✅ Same run configuration table
- ✅ Same performance overview list

### Reference Implementation:
All visual elements **MUST** be copied from [ReportDetailPage.vue](../../web/app/src/views/reports/ReportDetailPage.vue):
- Template structure: Lines 1-1152
- CSS styles: Lines 1153-1452
- ECharts configs: Functions `renderOverviewChart`, `renderErrorRateChart`, `renderLatencyChart`, `renderQPSTrend`, `renderLatencyTrend`, `renderNodeCharts`, `renderErrorBreakdownChart`

## Context

### Current Architecture
- **Report Generation**: [`report_generator.go`](../../internal/api/report_generator.go) uses Go `html/template`
- **Data Source**: Report data stored in SQLite as JSON (`Detail` field)
- **Frontend**: Vue.js with ECharts for online viewing
- **API**: Existing `/api/v1/reports/{id}` endpoint returns JSON

### Constraints
- Must use Go standard library + existing dependencies (no new build tools)
- HTML must work offline (CDN acceptable)
- Maintain backward compatibility with existing reports

## Goals / Non-Goals

**Goals:**
1. Produce production-quality HTML reports with interactive charts
2. Support responsive layouts for all device sizes
3. Enable batch export workflow
4. Provide RESTful API for export operations

**Non-Goals:**
- PDF generation (use browser print-to-PDF)
- Real-time report updates
- Custom branding/logo upload (future)

## Decisions

### Decision 1: Template Engine - Go html/template vs. Alternative

**Chosen: Go html/template (existing)**

| Option | Pros | Cons |
|--------|------|------|
| **Go html/template** ✅ | Already in use, type-safe, no new dependency | Verbose for complex templates |
| **quicktemplate** | Faster, cleaner syntax | New dependency, learning curve |
| **Gomponents** | Component-based, modern | Very different paradigm |

**Rationale:** Keep it simple, leverage existing codebase, avoid introducing new build tools.

### Decision 2: Chart Library - ECharts via CDN

**Chosen: ECharts 5.x (CDN)**

```html
<script src="https://cdn.jsdelivr.net/npm/echarts@5/dist/echarts.min.js"></script>
```

**Why ECharts:**
- Already used in frontend (consistent visual style)
- Rich chart types (line, bar, pie, heatmap)
- Excellent performance with large datasets
- Active community and documentation
- Works offline after CDN load (cached)

**Alternative Considered:** Chart.js
- Lighter but less feature-rich
- Different visual style from frontend

### Decision 3: Theme Implementation - CSS Variables + JS Toggle

**Chosen: CSS Custom Properties with JavaScript toggle**

```css
:root {
    --primary-color: #6366f1;
    --bg-primary: #ffffff;
}

[data-theme="dark"] {
    --primary-color: #818cf8;
    --bg-primary: #1f2937;
}
```

```javascript
function toggleTheme() {
    const current = document.documentElement.getAttribute('data-theme');
    document.documentElement.setAttribute('data-theme', current === 'dark' ? 'light' : 'dark');
    localStorage.setItem('theme', document.documentElement.getAttribute('data-theme'));
}
```

**Why CSS Variables:**
- No page reload needed
- Easy to extend with more themes
- Browser support > 97%
- Performant (GPU-accelerated)

### Decision 4: Batch Export - ZIP Archive on Server

**Chosen: Server-side ZIP generation using `archive/zip`**

```go
import "archive/zip"

func GenerateBatchZip(reports []*model.Report) ([]byte, error) {
    var buf bytes.Buffer
    zipWriter := zip.NewWriter(&buf)
    
    // Add each report as individual file
    // Add index.html with links
    
    return buf.Bytes(), zipWriter.Close()
}
```

**Why Server-Side:**
- Avoid large payloads in browser memory
- Consistent implementation across clients
- Can add server-side processing (watermarks, etc.)

**Alternative:** Client-side ZIP with JSZip
- Pro: Reduces server load
- Con: Memory issues with large batches, inconsistent behavior

## Technical Architecture

### API Endpoints Design

#### 1. Single Report Export
```
GET /api/v1/reports/{id}/export?format=html&theme=light

Response:
  Content-Type: text/html; charset=utf-8
  Content-Disposition: attachment; filename="report-{id}-{timestamp}.html"
  Body: Complete HTML document
```

#### 2. Batch Export
```
POST /api/v1/reports/batch-export
Content-Type: application/json

Request Body:
{
  "ids": ["id1", "id2", "id3"],
  "format": "html",
  "theme": "light",
  "include_index": true
}

Response:
  Content-Type: application/zip
  Content-Disposition: attachment; filename="reports-batch-{timestamp}.zip"
  Body: ZIP archive containing:
    ├── report-id1.html
    ├── report-id2.html
    ├── report-id3.html
    └── index.html (optional, if include_index=true)
```

### File Structure After Implementation

```
internal/
├── api/
│   ├── handler.go              # Add export handlers
│   └── report_generator.go     # Enhanced template (MAJOR CHANGES)
│       ├── template_enhanced.go  # New enhanced template (~500 lines)
│       └── theme_styles.go       # CSS variables and themes
└── store/
    └── repo/
        └── report_repo.go      # Add batch query method

web/app/src/views/
└── reports/
    ├── ReportsPage.vue          # Add batch export UI
    └── ReportDetailPage.vue     # Add export button
```

### Data Flow Diagram

```
┌─────────────┐    GET /export    ┌──────────────────┐
│   Frontend  │ ──────────────→  │   API Handler     │
│  (Vue.js)   │                  │                  │
└─────────────┘                  │  1. Validate ID   │
                                 │  2. Fetch Report  │
                                 │  3. Parse Detail   │
         Response (HTML)         │  4. Render Template│
         ←─────────────────────  │  5. Return HTML    │
                                 └──────────────────┘
                                          │
                                          ↓
                                 ┌──────────────────┐
                                 │ Report Generator  │
                                 │                  │
                                 │ • Build Context   │
                                 │ • Execute Template│
                                 │ • Inject ECharts  │
                                 │ • Apply Theme     │
                                 └──────────────────┘
```

## Risks / Trade-offs

### Risk 1: Large HTML File Size
**Concern:** Reports with extensive time series data could exceed 2MB

**Mitigation:**
- Limit time series points to last N samples (configurable)
- Use data sampling for charts (ECharts built-in)
- Compress whitespace in production builds
- Monitor average file size in metrics

**Trade-off:** Less historical detail vs. manageable file sizes

### Risk 2: CDN Availability Offline
**Concern:** ECharts CDN might not be accessible in air-gapped environments

**Mitigation:**
- Option to embed ECharts inline (increases size by ~800KB)
- Document CDN requirement clearly
- Fallback message if CDN fails to load

**Trade-off:** Smaller files (CDN) vs. true offline capability (inline)

### Risk 3: Template Complexity
**Concern:** Complex Go templates are hard to maintain

**Mitigation:**
- Split template into logical sections (header, charts, tables)
- Add comprehensive comments
- Create helper functions for common patterns
- Write unit tests for template rendering

**Trade-off:** Simplicity vs. maintainability

### Risk 4: Batch Export Performance
**Concern:** Generating 10+ reports could timeout

**Mitigation:**
- Implement async job queue for large batches
- Return job ID, poll for completion
- Set reasonable limits (max 20 reports per batch)
- Show progress indicator in frontend

**Trade-off:** Synchronous simplicity vs. async scalability

## Migration Plan

### Phase 1: Backend Template Enhancement (Days 1-3)
1. ✅ Create new enhanced template (`report_template_enhanced.go`)
2. ✅ Implement ECharts integration
3. ✅ Add theme support (CSS variables)
4. ✅ Make responsive layout
5. ✅ Unit test template rendering
6. ✅ Switch `GenerateHTML()` to use new template

**Rollback:** Revert to old template, feature flag to switch back

### Phase 2: API Endpoints (Day 4)
1. ✅ Add `GET /reports/{id}/export` handler
2. ✅ Add `POST /reports/batch-export` handler
3. ✅ Implement ZIP generation utility
4. ✅ Add input validation
5. ✅ Integration tests

**Rollback:** Remove routes, keep old endpoints working

### Phase 3: Frontend Integration (Days 5-6)
1. ✅ Add export button to `ReportDetailPage.vue`
2. ✅ Add batch export UI to `ReportsPage.vue`
3. ✅ Implement download logic
4. ✅ Add progress indicators
5. ✅ Error handling & notifications

**Rollback:** Remove buttons, hide behind feature flag

### Phase 4: Testing & Polish (Day 7)
1. ✅ Cross-browser testing (Chrome, Firefox, Safari, Edge)
2. ✅ Mobile responsiveness verification
3. ✅ Performance benchmarking
4. ✅ Documentation update
5. ✅ User acceptance testing

**Rollback:** Revert UI changes

## Open Questions

1. **Should we support custom branding?** (Logo upload, company name)
   - *Decision deferred to v2*

2. **Should we add password protection to exported HTML?**
   - *Security concern, need user feedback*

3. **Should we support exporting to other formats?** (PDF, Markdown)
   - *PDF can be done via browser print, others need investigation*

4. **What's the retention policy for batch export ZIPs?**
   - *Current: generate on-demand, don't store*
