# Tasks: Independent Chart Smooth/Step Toggle

## 1. Dashboard Page Refactoring

### 1.1 State Management Migration
**File:** `web/app/src/views/dashboard/DashboardPage.vue`

- [x] 1.1.1 Replace single `errorChartType` ref with `chartTypes` reactive Record: `const chartTypes = ref<Record<string, 'smooth' | 'step'>>({ errorRate: 'smooth', qpsTrend: 'smooth', latTrend: 'smooth' })`
- [x] 1.1.2 Replace single `nodeChartType` ref with dynamic node entries in `chartTypes`: initialize each node as `chartTypes.value[\`node-${nodeId}\`] = 'smooth'`
- [x] 1.1.3 Create unified `switchChartType(chartId: string, type: 'smooth' | 'step')` function that updates only the specified chart's state
- [x] 1.1.4 Update all chart rendering functions to read from `chartTypes.value[chartId]` instead of individual refs

### 1.2 UI Enhancements
**File:** `web/app/src/views/dashboard/DashboardPage.vue`

- [x] 1.2.1 Add smooth/step toggle buttons to QPS trend chart section (currently missing)
- [x] 1.2.2 Add smooth/step toggle buttons to latency trend chart section (currently missing)
- [x] 1.2.3 Update error rate chart toggle button to use new `switchChartType('errorRate', type)` signature
- [x] 1.2.4 Update node chart toggle buttons to use `switchChartType(\`node-${nodeId}\`, type)` with node-specific ID
- [x] 1.2.5 Ensure all toggle buttons use consistent template structure matching existing `.chart-type-toggle` pattern

### 1.3 Chart Rendering Updates
**File:** `web/app/src/views/dashboard/DashboardPage.vue`

- [x] 1.3.1 Update `renderErrorChart()` to use `const isSmooth = chartTypes.value.errorRate === 'smooth'`
- [x] 1.3.2 Create or update QPS trend rendering to use `const isSmooth = chartTypes.value.qpsTrend === 'smooth'`
- [x] 1.3.3 Create or update latency trend rendering to use `const isSmooth = chartTypes.value.latTrend === 'smooth'`
- [x] 1.3.4 Update `renderNodeDetailChart(nodeId)` to use `const isSmooth = chartTypes.value[\`node-${nodeId}\`] === 'smooth'`
- [x] 1.3.5 Verify all charts follow ECharts conventions: pre-computed `isSmooth` boolean, `smooth: isSmooth`, `step: isSmooth ? false : 'middle'`, `setOption(option, true)`

---

## 2. Report Detail Page Refactoring

### 2.1 State Management Migration
**File:** `web/app/src/views/reports/ReportDetailPage.vue`

- [x] 2.1.1 Replace four separate refs (`errorRateChartType`, `qpsChartType`, `latTrendChartType`, `nodeChartType`) with single `chartTypes` reactive Record
- [x] 2.1.2 Initialize chartTypes with default values: `{ errorRate: 'smooth', qpsTrend: 'smooth', latTrend: 'smooth' }`
- [x] 2.1.3 Initialize node chart types dynamically when `nodeTimeSeries` data loads
- [x] 2.1.4 Create unified `switchChartType(chartId: string, type: 'smooth' | 'step')` function

### 2.2 Toggle Button Updates
**File:** `web/app/src/views/reports/ReportDetailPage.vue`

- [x] 2.2.1 Update error rate toggle button binding: `@click="switchChartType('errorRate', 'smooth')"` and active state check
- [x] 2.2.2 Update QPS trend toggle button binding: `@click="switchChartType('qpsTrend', type)"`
- [x] 2.2.3 Update latency trend toggle button binding: `@click="switchChartType('latTrend', type)"`
- [x] 2.2.4 Update node chart toggle buttons to use dynamic chartId: `@click.stop="switchChartType(\`node-${idx}\`, type)"`
- [x] 2.2.5 Verify all button active states reference correct chartId in `chartTypes`

### 2.3 Chart Rendering Updates
**File:** `web/app/src/views/reports/ReportDetailPage.vue`

- [x] 2.3.1 Update error rate rendering to use `chartTypes.value.errorRate`
- [x] 2.3.2 Update QPS trend rendering (`renderQPSTrend`) to use `chartTypes.value.qpsTrend`
- [x] 2.3.3 Update latency trend rendering (`renderLatencyTrend`) to use `chartTypes.value.latTrend`
- [x] 2.3.4 Update node charts rendering (`renderNodeCharts`) to use per-node `chartTypes.value[\`node-${idx}\`]`
- [x] 2.3.5 Ensure all render functions re-render only the target chart when its type changes (not full page re-render)

---

## 3. Exported HTML Report Template Update

### 3.1 JavaScript State Management
**File:** `internal/api/report_generator_enhanced.go`

- [x] 3.1.1 Replace separate global variables (`errorRateType`, `qpsType`, `latTrendType`, `nodeType`) with single `chartTypes` object
- [x] 3.1.2 Initialize `chartTypes` with default smooth values for all charts
- [x] 3.1.3 Dynamically add node entries when rendering node sections
- [x] 3.1.4 Implement `switchChartType(chartId, type)` function that updates object and re-renders target chart

### 3.2 Toggle Button HTML Generation
**File:** `internal/api/report_generator_enhanced.go`

- [x] 3.2.1 Add toggle button HTML for error rate chart (if missing)
- [x] 3.2.2 Add toggle button HTML for QPS chart (if missing)
- [x] 3.2.3 Update latency trend toggle button to call `switchLatTrendType('smooth'/'step')` → `switchChartType('latTrend', 'smooth'/'step')`
- [x] 3.2.4 Update node chart toggle buttons to call `switchNodeType(type)` → `switchChartType('node' + idx, type)`
- [x] 3.2.5 Ensure button onclick handlers pass correct chartId parameter

### 3.3 Chart Rendering Functions Update
**File:** `internal/api/report_generator_enhanced.go`

- [x] 3.3.1 Update `renderErrorRateChart()` to read `chartTypes.errorRate`
- [x] 3.3.2 Update `renderQPSTrend()` to read `chartTypes.qps`
- [x] 3.3.3 Update `renderLatencyTrend()` to read `chartTypes.latTrend`
- [x] 3.3.4 Update `renderNodeCharts()` to read per-index `chartTypes['node' + idx]`
- [x] 3.3.5 Add helper function `updateToggleButtons(chartId)` to sync button active states after toggle

### 3.4 CSS Consistency Verification
**File:** `internal/api/report_generator_enhanced.go`

- [x] 3.4.1 Verify exported report CSS includes `.chart-type-toggle` and `.type-btn` styles matching online version
- [x] 3.4.2 Ensure hover effects and active states are identical to Dashboard/ReportDetailPage
- [x] 3.4.3 Test responsive layout of toggle buttons on mobile/tablet views

---

## 4. Testing & Verification

### 4.1 Manual Testing - Dashboard
- [x] 4.1.1 Verify all 4+ chart types have visible toggle buttons on Dashboard page
- [x] 4.1.2 Test toggling error rate chart does not affect QPS, latency, or node charts
- [x] 4.1.3 Test toggling node-1 does not affect node-2, node-3, etc.
- [x] 4.1.4 Verify chart re-rendering is smooth (< 100ms) without flicker
- [x] 4.1.5 Check browser console for errors during toggle operations

### 4.2 Manual Testing - Report Detail Page
- [x] 4.2.1 Verify all line charts (error rate, QPS, latency, nodes) have toggle buttons
- [x] 4.2.2 Test independent toggle behavior across all chart types
- [x] 4.2.3 Verify button active states update correctly on click
- [x] 4.2.4 Test with reports containing varying numbers of nodes (0, 1, 5, 10+)

### 4.3 Manual Testing - Exported HTML Report
- [x] 4.3.1 Export a report with multiple nodes and open in browser
- [x] 4.3.2 Verify all charts display toggle buttons
- [x] 4.3.3 Test independent toggle functionality works offline (no network)
- [x] 4.3.4 Compare visual appearance with online version (screenshots)
- [x] 4.3.5 Verify no JavaScript errors in console during interactions

### 4.4 Cross-Browser Compatibility
- [x] 4.4.1 Test toggle functionality in Chrome (latest)
- [x] 4.4.2 Test toggle functionality in Firefox (latest)
- [x] 4.4.3 Test toggle functionality in Safari (latest)
- [x] 4.4.4 Verify ECharts rendering consistency across browsers

---

## Implementation Order

**Recommended Sequence:**

```
Phase 1: Dashboard (Foundation)
  Task 1.1 → 1.2 → 1.3 → Test 4.1

Phase 2: Report Detail Page (Similar Pattern)
  Task 2.1 → 2.2 → 2.3 → Test 4.2

Phase 3: Exported Report (Template Update)
  Task 3.1 → 3.2 → 3.3 → 3.4 → Test 4.3

Phase 4: Final Validation
  Test 4.4 (Cross-browser)
```

**Dependencies:**
```
Dashboard (Phase 1) ──→ ReportDetailPage (Phase 2) ──→ Exported Report (Phase 3)
     ↓                        ↓                           ↓
   Test 4.1                 Test 4.2                    Test 4.3
                                                        ↓
                                                   Test 4.4 (Final)
```
