## 1. Backend: Runtime Metrics Collector

- [x] 1.1 Create `internal/runner/runtime_metrics.go` with RuntimeMetricsSnapshot struct definition
- [x] 1.2 Implement `RuntimeMetricsCollector` struct with Config (sample interval, enabled flag)
- [x] 1.3 Implement Go runtime metrics sampling: goroutine count, heap alloc/sys/idle, GC pause, nextGC
- [x] 1.4 Implement platform-specific system metrics: Linux (/proc/self/stat, /proc/self/status) and macOS (sysctl/ps)
- [x] 1.5 Implement Windows fallback (runtime metrics only, system metrics return 0/-1)
- [x] 1.6 Add Runner internal state exposure: ActiveWorkers, PendingQueueLen via StatsProvider interface extension
- [x] 1.7 Implement time-series storage: in-memory slice append on each sample cycle
- [x] 1.8 Implement aggregated statistics computation: Min/Max/Avg for all numeric fields, peak timestamps
- [x] 1.9 Implement lifecycle management: Start() launches goroutine, Stop() final sample + cleanup
- [x] 1.10 Integrate collector into Runner.Start() and Runner.Stop() lifecycle hooks

## 2. Backend: Task Wait Time Tracking (⭐ Core Feature)

- [x] 2.1 Create `internal/core/pool/wait_time_tracker.go` with WaitTimeTracker struct definition
  - Fields: mu sync.Mutex, samples []time.Duration, pos int, count int64, totalNs int64
  - Methods: Record(d time.Duration), Stats() WaitTimeStats, Reset()
- [x] 2.2 Implement WaitTimeStats return struct: Avg, P50, P95, P99, Max (time.Duration), SampleCount int64
- [x] 2.3 Implement helper function percentile(sorted []time.Duration, p float64) time.Duration
  - Use linear interpolation for non-integer index positions
  - Handle empty array edge case (return 0)
- [x] 2.4 Modify Pool struct in `pool.go`: add waitTracker *WaitTimeTracker field
- [x] 2.5 Initialize waitTracker in Pool.New() constructor (if not nil)
- [x] 2.6 Refactor Pool.Submit() method to wrap task function:
  ```go
  func (p *Pool) Submit(task Task) error {
      submitTime := time.Now()
      wrappedTask := func(ctx context.Context) error {
          if p.waitTracker != nil {
              p.waitTracker.Record(time.Since(submitTime))
          }
          return task(ctx)
      }
      // ... existing channel push logic using wrappedTask
  }
  ```
- [x] 2.7 Add conditional check: only track when pool context is active (not cancelled)
- [x] 2.8 Add public method Pool.TaskWaitStats() WaitTimeStats for external access by collector
- [x] 2.9 Write unit tests for WaitTimeTracker:
  - Test Record() with single sample returns correct Stats()
  - Test circular buffer overflow (1001+ records, verify oldest overwritten)
  - Test concurrent Record() calls from multiple goroutines (race detector)
  - Test Stats() on empty tracker returns zero values
  - Test percentile calculation accuracy with known datasets
- [x] 2.10 Write integration test: create pool, submit tasks, verify wait times recorded correctly
- [x] 2.11 Add benchmark test: measure overhead of Submit() wrapping (< 100ns target)
- [x] 2.12 Update RuntimeMetricsSnapshot to include TaskWait* fields (Avg/P50/P95/P99/Max/SampleCount)
- [x] 2.13 Integrate into RuntimeMetricsCollector.Sample(): call pool.TaskWaitStats() and populate snapshot fields

## 3. Backend: Data Model Extension

- [x] 3.1 Add SystemMetrics field to ReportDetail struct in `internal/runner/report.go`
- [x] 3.2 Create SystemMetricsSummary struct with aggregated stats fields
- [x] 3.3 Modify report generation to populate SystemMetrics from collector data
- [x] 3.4 Ensure JSON serialization includes new fields without breaking existing API consumers
- [x] 3.5 Add unit tests for RuntimeMetricsCollector: sampling accuracy, lifecycle, edge cases
- [x] 3.6 Add integration test: run short test with collector enabled, verify ReportDetail contains system metrics

## 4. Frontend: Dashboard System Monitoring UI

- [x] 4.1 Create SysGaugeCard.vue component with props: value, label, unit, status ('normal'|'warning'|'danger')
- [x] 4.2 Implement gauge card CSS: color-coded borders, value formatting, responsive layout
- [x] 4.3 Add "System Monitoring" section to DashboardPage.vue below charts-row
- [x] 4.4 Layout 5 gauge cards (Goroutine, Heap, GC, Worker, CPU) in flex row with responsive breakpoints
- [x] 4.5 Add sysGoroutine, sysHeap, sysCpu, sysTaskWait, sysPendingQueue to chartTypes reactive record (default 'smooth')
- [x] 4.6 Implement renderSysGoroutineChart(): line chart with markLine thresholds at 10k/50k
- [x] 4.7 Implement renderSysHeapChart(): dual-line chart (HeapAlloc solid, HeapSys dashed)
- [x] 4.8 Implement renderSysCpuChart(): area chart with shaded threshold regions (0-70, 70-90, 90-100)
- [x] 4.9 **Implement renderSysTaskWaitChart()**: multi-series line chart (P50/P95/P99) with threshold markLines at 10ms/100ms
- [x] 4.10 Implement renderSysPendingQueueChart(): bar chart showing real-time queue length
- [x] 4.11 Add smooth/step toggle buttons below each system chart (centered, independent state)
- [x] 4.12 Update switchChartType() to handle all system chart IDs independently
- [x] 4.13 Connect real-time data polling: fetch system metrics with same interval as business metrics
- [x] 4.14 Implement gauge value update animation (CSS transition 0.3s ease on significant changes)
- [x] 4.15 Handle empty/no-data state: hide section or show placeholder message

## 5. Frontend: Report Detail Page System Performance Section

- [x] 5.1 Add "System Performance Analysis" section to ReportDetailPage.vue after Error Breakdown block
- [x] 5.2 Create 6 summary metric cards: Peak Goroutines, Peak Memory, Avg CPU, Total GC Time, Avg Task Wait Time, QPS Achievement %
- [x] 5.3 Reuse or adapt dashboard chart rendering functions for report page (full history, static data)
- [x] 5.4 Render 5 trend charts (Goroutine, Heap, CPU, Task Wait Time, Pending Queue) with complete time-series data
- [ ] 5.5 Implement data table component: columns (Timestamp, Goroutines, Heap, CPU, GC Pause, Workers, Queue, Wait P99)
- [ ] 5.6 Add table sorting functionality (column header click, asc/desc toggle)
- [ ] 5.7 Add table pagination (20 rows/page) with controls
- [ ] 5.8 Highlight rows/cells exceeding danger thresholds (red tint background, bold text)
- [ ] 5.9 Handle legacy reports without system data: show info banner explaining unavailability
- [x] 5.10 Ensure chart type independence: toggling system charts doesn't affect QPS/latency/error charts

## 6. Exported HTML Report: System Metrics Integration

- [x] 6.1 Update `report_generator_enhanced.go` template to include System Performance section
- [x] 6.2 Generate static HTML/CSS gauge cards with status classes (status-normal/warning/danger)
- [x] 6.3 Embed system metrics time-series JSON in `<script type="application/json">` block
- [x] 6.4 Initialize ECharts instances on DOM ready with embedded data:
  - goroutine chart (single line + markLines)
  - heap chart (dual line: Alloc/Sys)
  - cpu chart (area fill + threshold regions)
  - task wait time chart (multi-series: P50/P95/P99)
- [x] 6.5 Implement smooth/step toggle for exported report charts (module-level state variables)
- [x] 6.6 Add threshold markLines to goroutine chart (warning/danger levels)
- [x] 6.7 Add task wait time threshold markLines at 10ms (warning) and 100ms (danger)
- [ ] 6.8 Generate formatted data table with proper number formatting per project rules
- [ ] 6.9 Implement client-side table pagination and sorting (pure JavaScript, no framework)
- [ ] 6.10 Add @media print styles: convert charts to images, grayscale backgrounds, page-break control
- [ ] 6.11 Implement window.onbeforeprint hook to convert ECharts canvas to PNG for print/PDF
- [ ] 6.12 Add dark mode support for system metrics section (CSS variables / theme toggle handler)

## 7. Configuration & Testing

- [x] 7.1 Add EnableSystemMetrics flag to Runner.Config with default=true
- [x] 7.2 Document configuration option in code comments or README
- [x] 7.3 Write backend benchmark test: measure single sample cycle execution time (< 1ms target)
- [x] 7.4 Write backend memory test: verify collector memory footprint after 1-hour simulation (< 500KB)
- [ ] 7.5 Manual testing - Dashboard: verify gauge colors change correctly at threshold boundaries
- [ ] 7.6 Manual testing - Dashboard: toggle smooth/step on one system chart, verify others unaffected
- [ ] 7.7 Manual testing - Dashboard: verify Task Wait Time chart shows P50/P95/P99 lines correctly
- [ ] 7.8 Manual testing - Report Detail: open report with system data, verify all sections render correctly
- [ ] 7.9 Manual testing - Exported HTML: open file in browser, verify charts interactive and theme toggle works
- [ ] 7.10 Manual testing - Print/PDF: trigger browser print, verify system metrics section readable
- [ ] 7.11 Manual testing - Cross-platform: verify Linux and macOS system metrics collection (if environments available)
- [ ] 7.12 **Performance validation: run high-concurrency test, verify Task Wait Time overhead < 0.1% of total test duration**

