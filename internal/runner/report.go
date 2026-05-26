package runner

import (
	"time"

	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
	"github.com/yannick2025-tech/Salvo/internal/store/model"
)

// ReportDetail represents a complete test report with all metrics and time-series data.
type ReportDetail struct {
	Metadata                 ReportMetadata     `json:"metadata"`
	GlobalSummary            GlobalSummary      `json:"global_summary"`
	GlobalTimeSeries         []Sample           `json:"global_time_series,omitempty"`
	HttpOnlyGlobalTimeSeries []Sample           `json:"http_only_global_time_series,omitempty"`
	NodeMetrics              []NodeMetricDetail `json:"node_metrics"`
	ErrorSummary             []ErrorItem        `json:"error_summary,omitempty"`
	SystemMetrics            *SystemMetricsData `json:"system_metrics,omitempty"`
}

// SystemMetricsData holds runtime and system performance metrics collected
// during a test run. It is nil for legacy reports that predate this feature.
type SystemMetricsData struct {
	TimeSeries []RuntimeMetricsSnapshot `json:"time_series,omitempty"`
	Summary    SystemMetricsSummary     `json:"summary"`
}

// ReportMetadata contains metadata about the test run.
type ReportMetadata struct {
	RunID           snowflake.ID `json:"run_id"`
	SceneID         snowflake.ID `json:"scene_id"`
	SceneName       string       `json:"scene_name,omitempty"`
	Status          string       `json:"status"`
	StartedAt       time.Time    `json:"started_at"`
	FinishedAt      time.Time    `json:"finished_at"`
	DurationSec     float64      `json:"duration_sec"`
	WorkerCount     int          `json:"worker_count"`
	RunMode         string       `json:"run_mode"`
	Count           int64        `json:"count"`
	PlannedDuration float64      `json:"planned_duration,omitempty"`
	PlannedCount    int64        `json:"planned_count,omitempty"`
	GeneratedAt     time.Time    `json:"generated_at"`
	Version         string       `json:"version"`
}

// GlobalSummary contains aggregated global statistics.
type GlobalSummary struct {
	TotalRequests int64   `json:"total_requests"`
	SuccessCount  int64   `json:"success_count"`
	FailCount     int64   `json:"fail_count"`
	SuccessRate   float64 `json:"success_rate"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	P50LatencyMs  float64 `json:"p50_latency_ms"`
	P90LatencyMs  float64 `json:"p90_latency_ms"`
	P95LatencyMs  float64 `json:"p95_latency_ms"`
	P99LatencyMs  float64 `json:"p99_latency_ms"`
	MinLatencyMs  float64 `json:"min_latency_ms"`
	TTFB          float64 `json:"ttfb_ms"`
	Throughput    float64 `json:"throughput"`
	PeakQPS       float64 `json:"peak_qps"`
}

// NodeMetricDetail contains detailed metrics for a single node.
type NodeMetricDetail struct {
	NodeID     string           `json:"node_id"`
	NodeName   string           `json:"node_name"`
	NodeType   string           `json:"node_type,omitempty"`
	Summary    NodeSummaryStats `json:"summary"`
	TimeSeries []Sample         `json:"time_series,omitempty"`
}

// NodeSummaryStats contains summary statistics for a node.
type NodeSummaryStats struct {
	TotalRequests int64   `json:"total_requests"`
	SuccessCount  int64   `json:"success_count"`
	FailCount     int64   `json:"fail_count"`
	SuccessRate   float64 `json:"success_rate"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	P50LatencyMs  float64 `json:"p50_latency_ms"`
	P90LatencyMs  float64 `json:"p90_latency_ms"`
	P95LatencyMs  float64 `json:"p95_latency_ms"`
	P99LatencyMs  float64 `json:"p99_latency_ms"`
	MinLatencyMs  float64 `json:"min_latency_ms"`
	TTFB          float64 `json:"ttfb_ms"`
	AvgQPS        float64 `json:"avg_qps"`
	PeakQPS       float64 `json:"peak_qps"`
}

func calculateThroughput(runRecord *model.RunRecord) float64 {
	if runRecord.Duration <= 0 {
		return 0
	}
	return float64(runRecord.TotalReqs) / runRecord.Duration
}

func calculateNodeAvgQPS(snapshot *NodeSnapshot, durationSec float64) float64 {
	if durationSec <= 0 || snapshot.TotalReqs == 0 {
		return 0
	}
	return float64(snapshot.TotalReqs) / durationSec
}

func getNodeName(nodeID string) string {
	return nodeID
}

func recordsToGlobalSamples(records []TimeSeriesRecord) []Sample {
	var result []Sample
	for _, r := range records {
		if r.NodeID != "" {
			continue
		}
		result = append(result, Sample{
			Timestamp:     r.SampleTime,
			WindowSeconds: r.WindowDuration,
			QPS:           r.QPS,
			TotalRequests: r.TotalRequests,
			SuccessCount:  r.SuccessCount,
			FailCount:     r.FailCount,
			AvgLatencyMs:  r.AvgLatencyMs,
			P50LatencyMs:  r.P50LatencyMs,
			P90LatencyMs:  r.P90LatencyMs,
			P95LatencyMs:  r.P95LatencyMs,
			P99LatencyMs:  r.P99LatencyMs,
			MinLatencyMs:  r.MinLatencyMs,
			MaxLatencyMs:  r.MaxLatencyMs,
		})
	}
	return result
}

func recordsToNodeSamples(records []TimeSeriesRecord) map[string][]Sample {
	result := make(map[string][]Sample)
	for _, r := range records {
		if r.NodeID == "" {
			continue
		}
		result[r.NodeID] = append(result[r.NodeID], Sample{
			Timestamp:     r.SampleTime,
			WindowSeconds: r.WindowDuration,
			QPS:           r.QPS,
			TotalRequests: r.TotalRequests,
			SuccessCount:  r.SuccessCount,
			FailCount:     r.FailCount,
			AvgLatencyMs:  r.AvgLatencyMs,
			P50LatencyMs:  r.P50LatencyMs,
			P90LatencyMs:  r.P90LatencyMs,
			P95LatencyMs:  r.P95LatencyMs,
			P99LatencyMs:  r.P99LatencyMs,
			MinLatencyMs:  r.MinLatencyMs,
			MaxLatencyMs:  r.MaxLatencyMs,
		})
	}
	return result
}
