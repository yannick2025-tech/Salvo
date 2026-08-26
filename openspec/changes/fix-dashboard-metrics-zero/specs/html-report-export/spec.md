## ADDED Requirements

### Requirement: Time series data in exported HTML reports

The exported HTML report SHALL include time series charts (QPS, P50/P95/P99 latency, error rate) populated with actual data from the TimeSeriesCollector, not empty or zero-filled charts.

#### Scenario: HTML report QPS chart has data
- **WHEN** a report is exported as HTML after a test run with requests
- **THEN** the QPS time series chart SHALL display non-zero values at time points where requests were active
- **AND** the chart SHALL reflect the actual QPS measurements sampled during the run
- **AND** the data source SHALL be the TimeSeriesCollector records matching the run_record ID

#### Scenario: HTML report latency charts have data
- **WHEN** a report is exported as HTML after a test run with successful HTTP requests
- **THEN** the P50/P95/P99 latency time series charts SHALL display actual measured latency values
- **AND** latency values SHALL be in milliseconds with 3 decimal places precision
- **AND** the charts SHALL not show flat zero lines when actual latencies were measured

#### Scenario: HTML report error rate chart reflects actual error rate
- **WHEN** a report is exported as HTML after a test run
- **THEN** the error rate chart SHALL display the actual percentage of failed requests over time
- **AND** the error rate SHALL be calculated as (fail_count / total_requests * 100) per time bucket
- **AND** when all requests succeed, the error rate SHALL be 0 (not undefined or empty)
