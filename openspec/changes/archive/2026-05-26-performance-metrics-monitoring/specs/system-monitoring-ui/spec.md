## ADDED Requirements

### Requirement: Dashboard system monitoring section layout
The Dashboard page SHALL display a dedicated "System Monitoring" section below the existing charts row, containing gauge cards and trend charts.

#### Scenario: Section visibility during active test
- **WHEN** user views Dashboard with running test session
- **THEN** "System Monitoring" section visible below QPS/Latency/ErrorRate charts
- **AND** section contains 4 gauge cards in one row: Goroutines, Memory (Heap), CPU Usage, GC Pause
- **AND** below gauges, 3 line charts in responsive grid: Goroutine Trend, Heap Memory Trend, CPU Trend

#### Scenario: Section hidden when no test running or data unavailable
- **WHEN** no active test session exists OR system metrics data is empty
- **THEN** System Monitoring section not rendered OR shows placeholder message "No system metrics available"

#### Scenario: Responsive layout on different screen sizes
- **WHEN** viewport width >= 1200px
- **THEN** 4 gauge cards displayed in single row (25% width each)
- **AND** 3 charts displayed in single row (33% width each)
- **WHEN** viewport width between 768px and 1199px
- **THEN** gauge cards wrap to 2x2 grid
- **AND** charts stack vertically (full width)

### Requirement: Gauge card component specification
Each gauge card SHALL display current metric value with color-coded status indicator.

#### Scenario: Normal status display (green)
- **WHEN** current value within normal range (e.g., goroutine count < 10,000)
- **THEN** gauge card background has green accent/border
- **AND** value text displayed in large font (24-28px) with dark color
- **AND** label shown below value in smaller font (12px) with secondary text color

#### Scenario: Warning status display (orange)
- **WHEN** current value in warning range (e.g., heap alloc between 512MB - 1GB)
- **THEN** gauge card background has orange accent/border
- **AND** optional warning icon or badge shown near value

#### Scenario: Danger status display (red)
- **WHEN** current value exceeds danger threshold (e.g., CPU > 90%)
- **THEN** gauge card background has red accent/border
- **AND** value text displayed in red color for emphasis
- **AND** pulsing animation or blink effect to draw attention (CSS animation)

#### Scenario: Gauge card content structure
- **WHEN** rendering any gauge card
- **THEN** structure follows:
  ```
  ┌─────────────────────┐
  │ [Label]             │
  │     [Value]         │
  │   [Unit/Status]     │
  └─────────────────────┘
  ```
- **AND** Label uses Chinese (e.g., "协程数量", "堆内存", "CPU占用", "GC暂停")
- **AND** Value formatted per project rules:
  - Integers: comma-separated thousands (Number().toLocaleString())
  - Memory: fixed 1 decimal + "MB"
  - Percentages: toFixed(2) + "%"
  - Time (GC Pause): auto-scale ms/μs/ns

### Requirement: System metrics trend charts
The Dashboard SHALL render ECharts line charts showing historical trends for key system metrics.

#### Scenario: Goroutine count trend chart
- **WHEN** system metrics time-series data available with ≥ 2 samples
- **THEN** chart displays goroutine_count over time on Y-axis (integers, no decimals)
- **AND** X-axis shows relative time from test start (format: "MM:SS")
- **AND** line color uses primary blue theme color
- **AND** supports smooth/step toggle button below chart (centered)
- **AND** includes horizontal markLine at warning (10k) and danger (50k) thresholds

#### Scenario: Heap memory trend chart
- **WHEN** heap_alloc_mb time-series data available
- **THEN** chart displays dual lines: HeapAlloc (solid) and HeapSys (dashed)
- **AND** Y-axis unit: MB, formatted with 1 decimal
- **AND** area fill under HeapAlloc line with gradient (12% to 1% opacity)
- **AND** tooltip shows both values simultaneously on hover

#### Scenario: CPU usage trend chart
- **WHEN** cpu_percent time-series data available
- **THEN** Y-axis range: 0-100%, formatted as percentage (toFixed(2))
- **AND** includes shaded regions: 0-70% (green), 70-90% (orange), 90-100% (red)
- **AND** data points above 90% highlighted with red circle symbol (symbolSize: 6)

#### Scenario: Chart configuration compliance
- **WHEN** any system monitoring chart renders
- **THEN** MUST follow project ECharts conventions:
  - `const isSmooth = chartTypes.value === 'xxx'` pre-computed variable
  - `smooth: isSmooth` and `step: isSmooth ? false : 'middle'`
  - `setOption(option, true)` for complete replacement
  - Grid: `{ top: 28, right: 48, bottom: 36, left: 48 }`
  - Tooltip: rounded corners (borderRadius: 12), backdrop-filter blur
  - DataZoom slider: height 18px, bottom 4px
  - Axis labels: fontSize 10, color from theme

### Requirement: Real-time data update mechanism
The Dashboard SHALL refresh system metrics display in sync with existing business metrics update cycle.

#### Scenario: Polling-based update during test execution
- **WHEN** test is running (status=running)
- **THEN** system metrics data refreshed via same polling interval as QPS/latency charts (configurable, default 2s)
- **AND** gauge cards show latest snapshot value (most recent sample)
- **AND** trend charts append new data points without full re-render

#### Scenario: Smooth transition on value change
- **WHEN** new sample arrives and value changes significantly (>5% delta)
- **THEN** gauge value updates with CSS transition (0.3s ease)
- **AND** color status recalculated immediately (no transition delay)

#### Scenario: Test completion final state
- **WHEN** test transitions to success/failed/partial status
- **THEN** last sample values remain displayed (not cleared)
- **AND** trend charts show complete time-range with DataZoom enabled
- **AND** summary statistics appear below charts (peak, avg, min)

### Requirement: Report detail page system performance analysis section
The ReportDetailPage SHALL include a comprehensive "System Performance Analysis" section after the Trends Analysis block.

#### Scenario: Section structure and ordering
- **WHEN** viewing completed test report with system metrics data
- **THEN** page sections ordered as:
  1. Overview Metrics (existing)
  2. Error Rate Trend (existing)
  3. Error Breakdown (existing)
  4. Latency Distribution (existing)
  5. **System Performance Analysis (NEW)** ← inserted here
  6. Trend Analysis (existing)
  7. Node Details (existing)

#### Scenario: System Performance Analysis content
- **WHEN** section renders with valid data
- **THEN** displays:
  - Row of 4 summary cards: Peak Goroutines, Peak Memory (MB), Avg CPU (%), Total GC Time (ms)
  - Each card shows: label, large value, comparison sub-text ("vs baseline" if applicable)
  - Below cards: 3 charts (Goroutine, Heap, CPU) with full history
  - Below charts: data table with columns: Timestamp, Goroutines, Heap(MB), CPU(%), GC Pause(ms), Workers, QueueLen
  - Table supports sorting by column click
  - Table paginated (20 rows/page) with page controls

#### Scenario: Empty state handling
- **WHEN** report has no system metrics data (older reports or disabled collection)
- **THEN** section displays message: "系统性能数据不可用（测试运行时未启用监控或报告版本过旧）"
- **AND** message styled as info notification (blue accent, subtle background)

### Requirement: Chart type independence for system metrics
Each system monitoring chart SHALL have independent smooth/step toggle state, isolated from other charts.

#### Scenario: Toggle isolation between system metric charts
- **WHEN** user clicks "平滑" on Goroutine trend chart
- **THEN** only Goroutine chart re-renders with smooth interpolation
- **AND** Heap Memory chart retains its previous mode (step or smooth)
- **AND** CPU chart retains its previous mode
- **AND** no other non-system charts (QPS, latency, error rate) affected

#### Scenario: State persistence across chart type toggles
- **WHEN** user switches between multiple system metric chart types
- **THEN** each chart's toggle state stored independently in chartTypes reactive record
- **AND** keys follow pattern: 'sysGoroutine', 'sysHeap', 'sysCpu'
- **AND** default value for all: 'smooth'
