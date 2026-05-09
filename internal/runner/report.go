package runner

import (
	"time"

	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
	"github.com/yannick2025-tech/Salvo/internal/store/model"
)

// ReportDetail represents a complete test report with all metrics and time-series data.
type ReportDetail struct {
	Metadata       ReportMetadata     `json:"metadata"`
	GlobalSummary  GlobalSummary      `json:"global_summary"`
	GlobalTimeSeries []Sample         `json:"global_time_series,omitempty"`
	NodeMetrics    []NodeMetricDetail `json:"node_metrics"`
	ErrorSummary   []ErrorItem        `json:"error_summary,omitempty"`
}

// ReportMetadata contains metadata about the test run.
type ReportMetadata struct {
	RunID       snowflake.ID `json:"run_id,string"`
	SceneID     snowflake.ID `json:"scene_id,string"`
	SceneName   string       `json:"scene_name,omitempty"`
	Status      string       `json:"status"`
	StartedAt   time.Time    `json:"started_at"`
	FinishedAt  time.Time    `json:"finished_at"`
	DurationSec float64      `json:"duration_sec"`
	WorkerCount int          `json:"worker_count"`
	RunMode     string       `json:"run_mode"`
	Count       int64        `json:"count"`
	GeneratedAt time.Time    `json:"generated_at"`
	Version     string       `json:"version"`
}

// GlobalSummary contains aggregated global statistics.
type GlobalSummary struct {
	TotalRequests int64   `json:"total_requests"`
	SuccessCount  int64   `json:"success_count"`
	FailCount     int64   `json:"fail_count"`
	SuccessRate   float64 `json:"success_rate"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	P50LatencyMs  float64 `json:"p50_latency_ms"`
	P95LatencyMs  float64 `json:"p95_latency_ms"`
	P99LatencyMs  float64 `json:"p99_latency_ms"`
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
	P95LatencyMs  float64 `json:"p95_latency_ms"`
	P99LatencyMs  float64 `json:"p99_latency_ms"`
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
