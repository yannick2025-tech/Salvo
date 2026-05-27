## ADDED Requirements

### Requirement: Runtime metrics collector initialization
The system SHALL provide a `RuntimeMetricsCollector` component that initializes when a test run starts and follows the test lifecycle.

#### Scenario: Collector creation on test start
- **WHEN** Runner.Start() is called with EnableSystemMetrics=true (default)
- **THEN** system creates a new RuntimeMetricsCollector instance with 2-second sample interval
- **AND** collector stores reference to Runner context for cancellation

#### Scenario: Collector disabled by configuration
- **WHEN** Runner config sets EnableSystemMetrics=false
- **THEN** no RuntimeMetricsCollector is created
- **AND** ReportDetail.SystemMetrics field remains nil/empty

### Requirement: Go runtime metrics sampling
The collector SHALL sample Go runtime statistics at each interval using standard library functions.

#### Scenario: Successful goroutine count collection
- **WHEN** collector calls runtime.NumGoroutine()
- **THEN** snapshot.GoroutineCount contains current goroutine count as int64

#### Scenario: Heap memory metrics collection
- **WHEN** collector calls runtime.ReadMemStats()
- **THEN** snapshot.HeapAllocMB equals memstats.Alloc / 1024 / 1024
- **AND** snapshot.HeapSysMB equals memstats.Sys / 1024 / 1024
- **AND** snapshot.HeapIdleMB equals memstats.Idle / 1024 / 1024
- **AND** snapshot.NextGC equals memstats.NextGC
- **AND** snapshot.GCCount equals memstats.NumGC

#### Scenario: GC pause duration tracking
- **WHEN** collector reads memstats.PauseTotalNs and PauseNs
- **THEN** snapshot.GCPauseTotalNs contains cumulative pause time
- **AND** snapshot.GCPauseLastNs contains most recent GC pause duration (last element of PauseNs[256] array)

### Requirement: System resource metrics sampling (platform-aware)
The collector SHALL gather OS-level metrics using platform-specific implementations.

#### Scenario: Linux system metrics collection
- **WHEN** running on Linux (runtime.GOOS == "linux")
- **THEN** collector reads /proc/self/stat for CPU usage calculation
- **AND** collector reads /proc/self/status for VmRSS in KB, converts to MB
- **AND** collector counts threads from /proc/self/stat's num_threads field

#### Scenario: macOS system metrics collection
- **WHEN** running on macOS (runtime.GOOS == "darwin")
- **THEN** collector uses sysctl or ps command to get RSS memory
- **AND** CPU usage calculated from process CPU time delta over sample interval
- **AND** thread count obtained from task_info Mach API or fallback to goroutine count

#### Scenario: Windows graceful degradation
- **WHEN** running on Windows (runtime.GOOS == "windows")
- **THEN** collector only collects Go runtime metrics (goroutine, heap, GC)
- **AND** system metrics (CPU, RSS, ThreadCount) set to 0 or -1 indicating unavailable

### Requirement: Runner internal state exposure
The Runner SHALL expose internal operational metrics through the StatsProvider interface.

#### Scenario: Active worker count reporting
- **WHEN** collector requests runtime metrics during active test execution
- **THEN** snapshot.ActiveWorkers equals number of currently executing worker goroutines
- **AND** value ranges from 0 to configured WorkerCount

#### Scenario: Pending queue length reporting
- **WHEN** collector samples metrics
- **THEN** snapshot.PendingQueueLen equals length of request pending channel/buffer
- **AND** value is 0 if no queue mechanism exists

### Requirement: Task Wait Time statistics collection (⭐ Core Feature)
The Pool component SHALL track task queue wait time and provide P50/P95/P99 statistics for scheduling bottleneck detection.

#### Scenario: Task wait time recording on submit
- **WHEN** Pool.Submit() is called with a Task function
- **THEN** system records current timestamp as submitTime before pushing to tasks channel
- **AND** wraps original task function with wait duration measurement logic
- **AND** wrapped task calculates `waitDuration = time.Now() - submitTime` upon execution start
- **AND** calls p.waitTracker.Record(waitDuration) before executing original task

#### Scenario: WaitTimeTracker sliding window statistics
- **WHEN** collector calls p.waitTracker.Stats() during metrics sampling
- **THEN** returned WaitTimeStats contains:
  - Avg: mean of all recorded wait durations in window
  - P50: 50th percentile of sorted sample array
  - P95: 95th percentile (tail latency indicator)
  - P99: 99th percentile (extreme tail indicator)
  - Max: maximum observed wait duration
  - SampleCount: total number of recorded samples

#### Scenario: Fixed-size circular buffer implementation
- **WHEN** WaitTimeTracker initialized with default configuration
- **THEN** internal samples slice has fixed capacity of 1000 entries
- **AND** new samples overwrite oldest entries when buffer full (circular behavior)
- **AND** memory allocation occurs only once during initialization
- **AND** no dynamic memory growth during test execution

#### Scenario: Thread-safe concurrent access
- **WHEN** multiple worker goroutines call Record() simultaneously
- **THEN** mutex ensures data consistency without race conditions
- **AND** Stats() call blocks until all pending Record() calls complete
- **AND** no deadlock possible (single mutex, non-reentrant)

#### Scenario: Integration into RuntimeMetricsSnapshot
- **WHEN** RuntimeMetricsCollector takes a sample at 2-second interval
- **THEN** snapshot.TaskWaitAvgMs equals Stats().Avg converted to milliseconds
- **AND** snapshot.TaskWaitP50Ms equals Stats().P50 in milliseconds
- **AND** snapshot.TaskWaitP95Ms equals Stats().P95 in milliseconds
- **AND** snapshot.TaskWaitP99Ms equals Stats().P99 in milliseconds
- **AND** snapshot.TaskWaitMaxMs equals Stats().Max in milliseconds
- **AND** snapshot.TaskWaitSampleCount equals Stats().SampleCount

#### Scenario: Zero overhead when pool not yet started or already stopped
- **WHEN** Submit() called but pool context cancelled or not running
- **THEN** wait time tracking skipped (no Record() call)
- **AND** original task executed normally without wrapping overhead

### Requirement: Time-series data storage and aggregation
The collector SHALL maintain in-memory time-series data and compute aggregated statistics upon test completion.

#### Scenario: Sample storage during test run
- **WHEN** each 2-second interval elapses
- **THEN** collector appends RuntimeMetricsSnapshot to internal slice
- **AND** slice grows unbounded until test stops

#### Scenario: Aggregated statistics computation
- **WHEN** test completes and Stop() is called
- **THEN** collector computes SystemMetricsSummary containing:
  - Min/Max/Avg for each numeric metric (GoroutineCount, HeapAllocMB, etc.)
  - Peak values with timestamps
  - Total GC pause time percentage of test duration
  - Sample count and time range coverage

#### Scenario: Data integration into ReportDetail
- **WHEN** report generation occurs after test end
- **THEN** ReportDetail.SystemMetrics field populated with:
  - TimeSeries []RuntimeMetricsSnapshot (all raw samples)
  - Summary SystemMetricsSummary (aggregated stats)
- **AND** JSON serialization includes both arrays for frontend consumption

### Requirement: Resource cleanup and lifecycle management
The collector SHALL properly release resources when test ends or is cancelled.

#### Scenario: Graceful stop on test completion
- **WHEN** Runner.Stop() called after normal test finish
- **THEN** collector takes one final sample
- **AND** computes aggregated summary
- **AND** releases all goroutines and channels
- **AND** sets internal state to stopped

#### Scenario: Immediate stop on context cancellation
- **WHEN** parent context cancelled (user aborts test)
- **THEN** collector stops sampling within 1 second
- **AND** partial data saved up to last successful sample
- **AND** resources freed without blocking Runner shutdown

### Requirement: Performance overhead limits
The collector SHALL impose minimal performance impact on the test execution.

#### Scenario: Sampling overhead measurement
- **WHEN** collector executes one full sample cycle (runtime + system metrics)
- **THEN** total execution time SHALL be less than 1 millisecond
- **AND** memory allocation per sample SHALL be less than 2 KB

#### Scenario: GC pressure assessment
- **WHEN** test runs for 1 hour with 2-second sampling interval
- **THEN** additional memory consumed by collector SHALL be less than 500 KB
- **AND** no additional GC cycles triggered beyond normal Go runtime behavior
