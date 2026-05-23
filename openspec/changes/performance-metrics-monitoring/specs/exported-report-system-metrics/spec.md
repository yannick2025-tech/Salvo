## ADDED Requirements

### Requirement: Exported HTML report system metrics section structure
The exported HTML report SHALL include a "System Performance" section within the report body, positioned after the main metrics overview and before detailed trend analysis.

#### Scenario: Section placement and visibility
- **WHEN** user opens exported HTML report that contains system metrics data
- **THEN** "System Performance" (系统性能监控) section visible in document flow
- **AND** located between "Overview Metrics" card grid and "Trend Analysis" charts
- **AND** section has distinct background styling (subtle gray or themed color) to separate from adjacent sections

#### Scenario: Report without system metrics data
- **WHEN** exported report from older version or disabled monitoring
- **THEN** System Performance section not rendered OR shows minimal placeholder:
  - Single info banner: "System metrics not available for this test run"
  - No broken layout or empty containers

### Requirement: Static gauge visualization in exported report
The exported report SHALL render gauge-style indicators using pure CSS/SVG (no JavaScript runtime required for initial display).

#### Scenario: Gauge card rendering (static snapshot)
- **WHEN** report contains system metrics summary data
- **THEN** 4 gauge cards displayed in responsive flex row:
  1. **Goroutines** (协程数): Shows peak value, with min/max range below
  2. **Heap Memory** (堆内存): Shows peak value in MB, with avg below
  3. **CPU Usage** (CPU占用): Shows average % value, with peak below
  4. **GC Pause** (GC暂停): Shows total pause time in ms, with count below

#### Scenario: Gauge visual design specifications
- **WHEN** any gauge card renders
- **THEN** follows static HTML/CSS specification:
  ```html
  <div class="sys-gauge-card {status-class}">
    <div class="gauge-label">[Chinese Label]</div>
    <div class="gauge-value">[Formatted Value][Unit]</div>
    <div class="gauge-sub">Peak: [value] | Avg: [value]</div>
    <div class="gauge-indicator"></div> <!-- Color bar indicating status -->
  </div>
  ```
  - Status classes: `status-normal` (green border-left), `status-warning` (orange), `status-danger` (red)
  - Value font size: 28px, bold, monospace-numeric font family
  - Responsive: stack vertically on mobile (< 768px)

#### Scenario: Threshold-based status coloring
- **WHEN** gauge value falls into specific range
- **THEN** status class applied per rules:
  - Goroutines: normal <10k, warning 10k-50k, danger >50k
  - Heap MB: normal <512, warning 512-1024, danger >1024
  - CPU %: normal <70, warning 70-90, danger >90
  - GC ms: normal <5ms total, warning 5-20ms, danger >20ms (per-pause)

### Requirement: Interactive ECharts charts in exported report
The exported report SHALL include fully functional ECharts line charts for system metric trends, matching Dashboard/ReportDetailPage behavior.

#### Scenario: Chart initialization on page load
- **WHEN** user opens exported HTML file in browser
- **THEN** JavaScript initializes 3 ECharts instances after DOM ready:
  1. `sys-goroutine-chart`: Goroutine count over time
  2. `sys-heap-chart`: Heap Alloc + Heap Sys dual-line chart
  3. `sys-cpu-chart`: CPU usage percentage over time
- **AND** each chart container has fixed height: 280px
- **AND** charts use same theme colors as other report charts (from CSS variables or inline config)

#### Scenario: Chart data source from embedded JSON
- **WHEN** ECharts initialization executes
- **THEN** data loaded from embedded `<script type="application/json">` block containing SystemMetrics.TimeSeries array
- **AND** timestamps converted to relative time labels (seconds from start)
- **AND** no external API calls required (fully offline capable)

#### Scenario: Smooth/step toggle functionality
- **WHEN** user interacts with toggle buttons below each chart
- **THEN** buttons present with Chinese labels: "平滑" / "阶梯"
- **AND** clicking toggles series.smooth property and re-renders single chart
- **AND** state maintained per-chart using module-level variables (not reactive framework)
- **AND** default mode: 'smooth' for all three charts

#### Scenario: Chart configuration standards compliance
- **WHEN** any system metric chart renders in exported report
- **THEN** MUST match project ECharts conventions exactly:
  - Pre-computed boolean: `const isSmooth = sysChartTypes.goroutine === 'smooth'`
  - Series config: `smooth: isSmooth, step: isSmooth ? false : 'middle'`
  - Complete replacement: `chart.setOption(option, true)`
  - Tooltip: backgroundColor with opacity, borderRadius 12, backdrop-filter blur(8px)
  - DataZoom: slider type, height 18px, bottom 4px, brushSelect enabled
  - Axis label formatters: integers no decimal, floats toFixed(1) or toFixed(2)
  - Grid spacing consistent with node detail charts

#### Scenario: Threshold lines on charts
- **WHEN** goroutine chart renders
- **THEN** markLine added at y=10000 (warning, dashed orange) and y=50000 (danger, dashed red)
- **AND** markLine.silent = true (not interactive)
- **AND** label text: "Warning: 10k" / "Danger: 50k", fontSize 9

### Requirement: System metrics summary table in exported report
The exported report SHALL include a tabular view of sampled system metrics data.

#### Scenario: Table structure and columns
- **WHEN** system metrics time-series has ≥ 1 sample
- **THEN** table rendered with columns:
  | Timestamp | Goroutines | Heap(MB) | CPU(%) | GC Pause(ms) | Workers | Queue |
  |-----------|------------|----------|--------|--------------|---------|--------|
  | 00:00:02  | 1,234      | 256.8    | 45.23  | 0.12         | 8       | 0      |
  | 00:00:04  | 1,245      | 257.1    | 47.12  | 0.08         | 8       | 2      |

#### Scenario: Table formatting rules
- **WHEN** table cells render values
- **THEN** formatting follows project number rules:
  - Goroutines: integer with thousands separator (toLocaleString())
  - Heap: float with 1 decimal + "MB"
  - CPU: float with toFixed(2) + "%"
  - GC Pause: float with toFixed(2) + "ms"
  - Workers/Queue: plain integers
  - Timestamp: format "HH:MM:SS" from relative seconds

#### Scenario: Table interactivity (client-side only)
- **WHEN** table has > 20 rows
- **THEN** pagination controls shown at bottom: "Showing 1-20 of X | < Previous | Next >"
- **AND** clicking Next/Prev switches displayed rows without page reload
- **AND** sorting by column header click supported (ascending/descending toggle)
- **AND** sort indicated by arrow icon (▲/▼) next to column name

#### Scenario: Table row highlighting for anomalies
- **WHEN** row contains value exceeding danger threshold
- **THEN** entire row has subtle red background tint (rgba(248,81,73,0.06))
- **AND** specific cell with dangerous value has bold text and red color
- **AND** tooltip on hover shows threshold comparison: "Value X exceeds danger limit Y"

### Requirement: Print and PDF export compatibility
The system metrics section SHALL maintain readability when printed or exported to PDF.

#### Scenario: Print stylesheet application
- **WHEN** user triggers browser print (Ctrl+P / Cmd+P)
- **THEN** @media print rules applied:
  - Background colors converted to white/grayscale
  - Charts rendered as static images (ECharts getDataURL() before print)
  - Gauge cards use borders instead of background fills
  - Table rows alternate light-gray background for readability
  - Page breaks avoided within chart containers (page-break-inside: avoid)

#### Scenario: Chart image fallback for print/PDF
- **WHEN** print dialog opened or PDF export initiated
- **THEN** JavaScript hook (window.onbeforeprint) converts each ECharts canvas to PNG data URL
- **AND** replaces chart container content with `<img>` tag
- **AND** original chart restored on window.onafterprint event

### Requirement: Dark mode support in exported report
The system metrics section SHALL respect the report's dark/light theme setting.

#### Scenario: Dark theme application
- **WHEN** report generated with isDark=true OR user clicks dark mode toggle
- **THEN** all system metric components update:
  - Gauge cards: dark background (#1e293b), light text (#e2e8f0)
  - Chart backgrounds: transparent or dark (#0f172a)
  - Axis labels: light color (#94a3b8)
  - Table: dark striping (#1e293b alternate rows)
  - Threshold lines adjusted for contrast (lighter dashed lines)

#### Scenario: Theme toggle persistence
- **WHEN** user toggles theme in exported report header
- **THEN** preference saved to localStorage key 'salvo-report-theme'
- **AND** system metrics section re-renders with new theme immediately
- **AND** ECharts themes updated via setOption with new color palette
