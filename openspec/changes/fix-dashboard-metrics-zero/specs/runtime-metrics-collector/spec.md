## MODIFIED Requirements

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

#### Scenario: loadSystemMetricsFromDB returns correct P50 and P95 values
- **WHEN** DashboardOverview loads system metrics from database for a completed run
- **THEN** the returned RuntimeMetricsDTO SHALL have TaskWaitP50Ms populated from the stored summary's P50 value (not hardcoded 0)
- **AND** TaskWaitP95Ms SHALL be populated from the stored summary's P95 value (not hardcoded 0)
- **AND** TaskWaitP99Ms SHALL be populated from the stored summary's P99 max value
- **AND** if the summary does not contain P50/P95 fields, the SystemMetricsSummary SHALL be extended to compute and store these values during ComputeSummary()
