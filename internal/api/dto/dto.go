// Package dto defines the request and response data transfer objects
// for the Salvo REST API. All API endpoints use POST method; query
// parameters are passed via the JSON request body.
package dto

import (
	"time"

	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
)

// --- Common ---

// IDRequest is a generic request that identifies a single entity by ID.
type IDRequest struct {
	ID snowflake.ID `json:"id"`
}

// SceneIDRequest is a generic request that identifies entities by scene ID.
type SceneIDRequest struct {
	SceneID snowflake.ID `json:"scene_id"`
}

// Response is the standard envelope for all API responses.
type Response struct {
	// Code is the business status code (0 = success).
	Code int `json:"code"`
	// Message is a human-readable status description.
	Message string `json:"message"`
	// Data carries the response payload (can be any type or null).
	Data any `json:"data,omitempty"`
}

// OK returns a success response with the given data.
func OK(data any) Response {
	return Response{Code: 0, Message: "ok", Data: data}
}

// ErrorResp returns an error response with the given code and message.
func ErrorResp(code int, msg string) Response {
	return Response{Code: code, Message: msg}
}

// --- Pagination ---

// Pagination carries page metadata for list responses.
type Pagination struct {
	// Total is the total number of matching records.
	Total int `json:"total"`
	// Offset is the record offset used for this page.
	Offset int `json:"offset"`
	// Limit is the page size used for this query.
	Limit int `json:"limit"`
}

// ListResponse is a generic paginated list response.
type ListResponse[T any] struct {
	Items      T          `json:"items"`
	Pagination Pagination `json:"pagination"`
}

// --- Scene ---

// CreateSceneRequest is the request body for POST /api/v1/scenes/create.
type CreateSceneRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	DAGJSON     string `json:"dag_json,omitempty"`
	Variables   string `json:"variables,omitempty"`
	Plugins     string `json:"plugins,omitempty"`
	Status      string `json:"status,omitempty"`
}

// ImportYAMLRequest is the request body for POST /api/v1/scenes/import.
type ImportYAMLRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	YAML        string `json:"yaml"`
}

// UpdateSceneRequest is the request body for POST /api/v1/scenes/update.
type UpdateSceneRequest struct {
	ID          snowflake.ID `json:"id"`
	Name        string       `json:"name,omitempty"`
	Description string       `json:"description,omitempty"`
	DAGJSON     string       `json:"dag_json,omitempty"`
	Variables   string       `json:"variables,omitempty"`
	Plugins     string       `json:"plugins,omitempty"`
	Status      string       `json:"status,omitempty"`
}

// ListScenesRequest is the request body for POST /api/v1/scenes/list.
type ListScenesRequest struct {
	Status string `json:"status,omitempty"`
	Offset int    `json:"offset,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

// SceneDTO is the scene detail returned in API responses.
type SceneDTO struct {
	ID          snowflake.ID `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	DAGJSON     string       `json:"dag_json"`
	Variables   string       `json:"variables"`
	Plugins     string       `json:"plugins"`
	Status      string       `json:"status"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// --- Node ---

// AddNodeRequest is the request body for POST /api/v1/scenes/nodes/add.
type AddNodeRequest struct {
	SceneID   snowflake.ID `json:"scene_id"`
	Name      string       `json:"name"`
	Type      string       `json:"type"`
	Config    string       `json:"config,omitempty"`
	Position  string       `json:"position,omitempty"`
	LoopCount int          `json:"loop_count,omitempty"`
}

// UpdateNodeRequest is the request body for POST /api/v1/scenes/nodes/update.
type UpdateNodeRequest struct {
	ID        snowflake.ID `json:"id"`
	Name      string       `json:"name,omitempty"`
	Type      string       `json:"type,omitempty"`
	Config    string       `json:"config,omitempty"`
	Position  string       `json:"position,omitempty"`
	LoopCount int          `json:"loop_count,omitempty"`
}

// DeleteNodeRequest is the request body for POST /api/v1/scenes/nodes/delete.
type DeleteNodeRequest struct {
	ID snowflake.ID `json:"id"`
}

// ListNodesRequest is the request body for POST /api/v1/scenes/nodes/list.
type ListNodesRequest struct {
	SceneID snowflake.ID `json:"scene_id"`
	Offset  int          `json:"offset,omitempty"`
	Limit   int          `json:"limit,omitempty"`
}

// NodeDTO is the node detail returned in API responses.
type NodeDTO struct {
	ID        snowflake.ID `json:"id"`
	SceneID   snowflake.ID `json:"scene_id"`
	Name      string       `json:"name"`
	Type      string       `json:"type"`
	Config    string       `json:"config"`
	Position  string       `json:"position"`
	LoopCount int          `json:"loop_count"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// --- Edge ---

// AddEdgeRequest is the request body for POST /api/v1/scenes/edges/add.
type AddEdgeRequest struct {
	SceneID   snowflake.ID `json:"scene_id"`
	FromNode  snowflake.ID `json:"from_node"`
	ToNode    snowflake.ID `json:"to_node"`
	Condition string       `json:"condition,omitempty"`
	Priority  int          `json:"priority,omitempty"`
}

// DeleteEdgeRequest is the request body for POST /api/v1/scenes/edges/delete.
type DeleteEdgeRequest struct {
	ID snowflake.ID `json:"id"`
}

// ListEdgesRequest is the request body for listing edges by scene.
type ListEdgesRequest struct {
	SceneID snowflake.ID `json:"scene_id"`
	Offset  int          `json:"offset,omitempty"`
	Limit   int          `json:"limit,omitempty"`
}

// EdgeDTO is the edge detail returned in API responses.
type EdgeDTO struct {
	ID        snowflake.ID `json:"id"`
	SceneID   snowflake.ID `json:"scene_id"`
	FromNode  snowflake.ID `json:"from_node"`
	ToNode    snowflake.ID `json:"to_node"`
	Condition string       `json:"condition"`
	Priority  int          `json:"priority"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// --- Variable ---

// ListVariablesRequest is the request body for POST /api/v1/scenes/variables/list.
type ListVariablesRequest struct {
	SceneID snowflake.ID `json:"scene_id"`
	Offset  int          `json:"offset,omitempty"`
	Limit   int          `json:"limit,omitempty"`
}

// SetVariableRequest is the request body for POST /api/v1/scenes/variables/set.
type SetVariableRequest struct {
	SceneID snowflake.ID `json:"scene_id"`
	Scope   string       `json:"scope"`
	Key     string       `json:"key"`
	Value   string       `json:"value"`
}

// BatchSetVariablesRequest is the request body for POST /api/v1/scenes/variables/batch-set.
// It replaces all scene-level variables with the provided key-value map.
type BatchSetVariablesRequest struct {
	SceneID   snowflake.ID        `json:"scene_id"`
	Variables map[string]string   `json:"variables"`
}

// BatchSetVariablesResponse is the response for the batch-set endpoint.
type BatchSetVariablesResponse struct {
	Variables map[string]string `json:"variables"`
}

// --- DataSource ---

// UploadDataSourceRequest is the request body for POST /api/v1/scenes/datasources/upload.
type UploadDataSourceRequest struct {
	SceneID  snowflake.ID `json:"scene_id"`
	FileName string       `json:"file_name"`
	Content  string       `json:"content"` // raw CSV content
}

// DataSourceDTO is the data source detail returned in API responses.
type DataSourceDTO struct {
	ID        snowflake.ID `json:"id"`
	SceneID   snowflake.ID `json:"scene_id"`
	Name      string       `json:"name"`
	FileName  string       `json:"file_name"`
	Columns   []string     `json:"columns"`
	RowCount  int          `json:"row_count"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// DataSourcePreviewDTO is the data source preview with sample rows.
type DataSourcePreviewDTO struct {
	ID        snowflake.ID       `json:"id"`
	SceneID   snowflake.ID       `json:"scene_id"`
	Name      string             `json:"name"`
	FileName  string             `json:"file_name"`
	Columns   []string           `json:"columns"`
	RowCount  int                `json:"row_count"`
	Rows      []map[string]string `json:"rows"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
}

// VariableDTO is the variable detail returned in API responses.
type VariableDTO struct {
	ID        snowflake.ID `json:"id"`
	SceneID   snowflake.ID `json:"scene_id"`
	Scope     string       `json:"scope"`
	Key       string       `json:"key"`
	Value     string       `json:"value"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// --- PluginConfig ---

// UpdatePluginConfigRequest is the request body for POST /api/v1/plugins/config.
type UpdatePluginConfigRequest struct {
	ID       snowflake.ID `json:"id"`
	Name     string       `json:"name,omitempty"`
	Type     string       `json:"type,omitempty"`
	Config   string       `json:"config,omitempty"`
	Phase    string       `json:"phase,omitempty"`
	Priority int          `json:"priority,omitempty"`
	Enabled  *bool        `json:"enabled,omitempty"`
}

// PluginConfigDTO is the plugin config detail returned in API responses.
type PluginConfigDTO struct {
	ID        snowflake.ID `json:"id"`
	SceneID   snowflake.ID `json:"scene_id"`
	Name      string       `json:"name"`
	Type      string       `json:"type"`
	Config    string       `json:"config"`
	Phase     string       `json:"phase"`
	Priority  int          `json:"priority"`
	Enabled   bool         `json:"enabled"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// --- Report ---

// ListReportsRequest is the request body for POST /api/v1/reports/list.
type ListReportsRequest struct {
	SceneID snowflake.ID `json:"scene_id,omitempty"`
	Status  string       `json:"status,omitempty"`
	Offset  int          `json:"offset,omitempty"`
	Limit   int          `json:"limit,omitempty"`
}

// GetReportRequest is the request body for POST /api/v1/reports/get.
type GetReportRequest struct {
	ID snowflake.ID `json:"id"`
}

// ReportDTO is the report detail returned in API responses.
type ReportDTO struct {
	ID         snowflake.ID `json:"id,string"`
	SceneID    snowflake.ID `json:"scene_id,string"`
	RunID      snowflake.ID `json:"run_id,string"`
	Status     string       `json:"status"`
	Summary    string       `json:"summary"`
	Detail     string       `json:"detail"`
	StartedAt  *time.Time   `json:"started_at,omitempty"`
	FinishedAt *time.Time   `json:"finished_at,omitempty"`
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
}

// --- RunRecord ---

// ListRunRecordsRequest is the request body for listing run records.
type ListRunRecordsRequest struct {
	SceneID snowflake.ID `json:"scene_id,omitempty"`
	Status  string       `json:"status,omitempty"`
	Offset  int          `json:"offset,omitempty"`
	Limit   int          `json:"limit,omitempty"`
}

// RunRecordDTO is the run record detail returned in API responses.
type RunRecordDTO struct {
	ID          snowflake.ID `json:"id"`
	RunID       snowflake.ID `json:"run_id"`
	SceneID     snowflake.ID `json:"scene_id"`
	Status      string       `json:"status"`
	WorkerCount int          `json:"worker_count"`
	RunMode     string       `json:"run_mode"`
	Duration    float64      `json:"duration"`
	Count       int64        `json:"count"`
	TotalReqs   int64        `json:"total_reqs"`
	SuccessReqs int64        `json:"success_reqs"`
	FailedReqs  int64        `json:"failed_reqs"`
	AvgLatency  float64      `json:"avg_latency"`
	P50Latency  float64      `json:"p50_latency"`
	P90Latency  float64      `json:"p90_latency"`
	P95Latency  float64      `json:"p95_latency"`
	P99Latency  float64      `json:"p99_latency"`
	ErrorMsg    string       `json:"error_msg"`
	StartedAt   *time.Time   `json:"started_at,omitempty"`
	FinishedAt  *time.Time   `json:"finished_at,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// --- Trace ---

// ListTracesRequest is the request for listing traces.
type ListTracesRequest struct {
	SceneID snowflake.ID `json:"scene_id,omitempty"`
	Limit   int          `json:"limit,omitempty"`
	Offset  int          `json:"offset,omitempty"`
}

// GetTraceRequest is the request for getting a single trace.
type GetTraceRequest struct {
	ID snowflake.ID `json:"id"`
}

// GetTraceByRunRequest is the request for getting a trace by run ID.
type GetTraceByRunRequest struct {
	RunID snowflake.ID `json:"run_id"`
}

// SpanDTO is the span detail returned in API responses.
type SpanDTO struct {
	ID           snowflake.ID `json:"id"`
	TraceID      snowflake.ID `json:"trace_id"`
	ChainID      string       `json:"chain_id"`
	NodeID       string       `json:"node_id"`
	NodeName     string       `json:"node_name,omitempty"`
	ParentNodeID string       `json:"parent_node_id,omitempty"`
	Status       string       `json:"status"`
	Error        string       `json:"error,omitempty"`
	Input        string       `json:"input,omitempty"`
	Output       string       `json:"output,omitempty"`
	StartedAt    time.Time    `json:"started_at"`
	FinishedAt   time.Time    `json:"finished_at"`
	Duration     int64        `json:"duration_ns"`
}

// TraceDTO is the trace detail returned in API responses.
type TraceDTO struct {
	ID         snowflake.ID `json:"id"`
	SceneID    snowflake.ID `json:"scene_id"`
	SceneName  string       `json:"scene_name,omitempty"`
	RunID      snowflake.ID `json:"run_id"`
	Status     string       `json:"status"`
	Error      string       `json:"error,omitempty"`
	Spans      []SpanDTO    `json:"spans"`
	StartedAt  time.Time    `json:"started_at"`
	FinishedAt time.Time    `json:"finished_at"`
	Duration   int64        `json:"duration_ns"`
}

// --- Runner ---

// StartSceneRequest is the request body for POST /api/v1/scenes/start.
type StartSceneRequest struct {
	SceneID   snowflake.ID      `json:"scene_id"`
	Workers   int               `json:"workers,omitempty"`
	RunMode   string            `json:"run_mode,omitempty"`
	Count     int64             `json:"count,omitempty"`
	Duration  float64           `json:"duration,omitempty"`
	Timeout   float64           `json:"timeout,omitempty"`
	Variables map[string]string `json:"variables,omitempty"`
}

// StopSceneRequest is the request body for POST /api/v1/scenes/stop.
type StopSceneRequest struct {
	SceneID snowflake.ID `json:"scene_id"`
}

// SceneStatusRequest is the request body for POST /api/v1/scenes/status.
type SceneStatusRequest struct {
	SceneID snowflake.ID `json:"scene_id"`
}

// SceneStatusDTO is the scene run status returned in API responses.
type SceneStatusDTO struct {
	SceneID     snowflake.ID `json:"scene_id"`
	RunID       snowflake.ID `json:"run_id"`
	Status      string       `json:"status"`
	Workers     int          `json:"workers"`
	TotalReqs   int64        `json:"total_reqs"`
	SuccessReqs int64        `json:"success_reqs"`
	FailedReqs  int64        `json:"failed_reqs"`
	Duration    float64      `json:"duration_seconds"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string  `json:"token"`
	User  UserDTO `json:"user"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type ResetPasswordRequest struct {
	UserID      snowflake.ID `json:"user_id"`
	NewPassword string       `json:"new_password"`
}

type UserDTO struct {
	ID          snowflake.ID `json:"id"`
	Email       string       `json:"email"`
	Nickname    string       `json:"nickname"`
	RoleID      snowflake.ID `json:"role_id"`
	RoleName    string       `json:"role_name"`
	Status      string       `json:"status"`
	LastLoginAt *time.Time   `json:"last_login_at,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

type CreateUserRequest struct {
	Email    string       `json:"email"`
	Password string       `json:"password"`
	Nickname string       `json:"nickname"`
	RoleID   snowflake.ID `json:"role_id"`
}

type UpdateUserRequest struct {
	ID       snowflake.ID `json:"id"`
	Nickname string       `json:"nickname,omitempty"`
	RoleID   snowflake.ID `json:"role_id,omitempty"`
	Status   string       `json:"status,omitempty"`
}

type ListUsersRequest struct {
	Status string `json:"status,omitempty"`
	Offset int    `json:"offset,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type RoleDTO struct {
	ID          snowflake.ID    `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	IsBuiltin   bool            `json:"is_builtin"`
	Permissions []PermissionDTO `json:"permissions"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type PermissionDTO struct {
	ID          snowflake.ID `json:"id"`
	Resource    string       `json:"resource"`
	Action      string       `json:"action"`
	Description string       `json:"description"`
}

type CreateRoleRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type UpdateRoleRequest struct {
	ID          snowflake.ID `json:"id"`
	Name        string       `json:"name,omitempty"`
	Description string       `json:"description,omitempty"`
	Permissions []string     `json:"permissions,omitempty"`
}

type ListRolesRequest struct {
	Offset int `json:"offset,omitempty"`
	Limit  int `json:"limit,omitempty"`
}

// --- Dashboard ---

type DashboardOverviewRequest struct {
	RangeSeconds int    `json:"range_seconds,omitempty"`
	SceneID      string `json:"scene_id,omitempty"`
}

type DashboardOverviewDTO struct {
	TotalReqs      int64                `json:"total_reqs"`
	SuccessReqs    int64                `json:"success_reqs"`
	FailedReqs     int64                `json:"failed_reqs"`
	P50Latency     float64              `json:"p50_latency"`
	P95Latency     float64              `json:"p95_latency"`
	P99Latency     float64              `json:"p99_latency"`
	AvgLatency     float64              `json:"avg_latency"`
	Running        int                  `json:"running"`
	SceneID        int64                `json:"scene_id,omitempty"`
	RecentRuns              []RunRecordDTO       `json:"recent_runs"`
	NodeMetrics             []NodeMetricDTO      `json:"node_metrics"`
	TimeSeries              *TimeSeriesDTO       `json:"time_series,omitempty"`
	HttpOnlyTimeSeries       *TimeSeriesDTO       `json:"http_only_time_series,omitempty"`
	SystemMetrics           *RuntimeMetricsDTO   `json:"system_metrics,omitempty"`
	SystemMetricsTimeSeries []RuntimeMetricsDTO  `json:"system_metrics_time_series,omitempty"`
}

// RuntimeMetricsDTO holds the latest runtime/system metrics snapshot
// for the Dashboard real-time monitoring section.
type RuntimeMetricsDTO struct {
	Timestamp       string  `json:"timestamp,omitempty"`
	GoroutineCount  int64   `json:"goroutine_count"`
	HeapAllocMB     float64 `json:"heap_alloc_mb"`
	HeapSysMB       float64 `json:"heap_sys_mb"`
	CPUUsagePercent float64 `json:"cpu_percent"`
	RSSMemoryMB     float64 `json:"rss_mb"`
	ActiveWorkers   int     `json:"active_workers"`
	PendingQueueLen int     `json:"pending_queue_len"`
	TaskWaitP50Ms   float64 `json:"task_wait_p50_ms"`
	TaskWaitP95Ms   float64 `json:"task_wait_p95_ms"`
	TaskWaitP99Ms   float64 `json:"task_wait_p99_ms"`
	GCPauseLastMs   float64 `json:"gc_pause_last_ms"`
}

type NodeMetricDTO struct {
	NodeID      string    `json:"node_id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	SortOrder   int       `json:"sort_order"`
	TotalReqs   int64     `json:"total_reqs"`
	SuccessReqs int64     `json:"success_reqs"`
	P50Latency  float64   `json:"p50_latency"`
	P95Latency  float64   `json:"p95_latency"`
	P99Latency  float64   `json:"p99_latency"`
	AvgLatency  float64   `json:"avg_latency"`
	Timestamps  []string  `json:"timestamps"`
	TSP50       []float64 `json:"ts_p50"`
	TSP95       []float64 `json:"ts_p95"`
	TSP99       []float64 `json:"ts_p99"`
	TSAvg       []float64 `json:"ts_avg"`
	TSQPS       []float64 `json:"ts_qps"`
}

type TimeSeriesDTO struct {
	Timestamps  []string  `json:"timestamps"`
	QPS         []float64 `json:"qps"`
	P50         []float64 `json:"p50"`
	P95         []float64 `json:"p95"`
	P99         []float64 `json:"p99"`
	ErrorRate   []float64 `json:"error_rate"`
	WindowStart string    `json:"window_start,omitempty"`
	WindowEnd   string    `json:"window_end,omitempty"`
	HasRunning  bool      `json:"has_running"`
}

// --- Dashboard History DTOs ---

type DashboardHistoryRequest struct {
	SceneID int64 `json:"scene_id,omitempty"`
	Limit   int   `json:"limit,omitempty"`
}

type DashboardHistoryDTO struct {
	History []RunHistoryDTO `json:"history"`
	Total   int             `json:"total"`
}

type RunHistoryDTO struct {
	RunID         snowflake.ID                     `json:"run_id,string"`
	SceneID       snowflake.ID                     `json:"scene_id,string"`
	Status        string                           `json:"status"`
	StartedAt     *time.Time                       `json:"started_at,omitempty"`
	FinishedAt    *time.Time                       `json:"finished_at,omitempty"`
	TotalReqs     int64                            `json:"total_reqs"`
	SuccessReqs   int64                            `json:"success_reqs"`
	FailedReqs    int64                            `json:"failed_reqs"`
	AvgLatency    float64                          `json:"avg_latency"`
	P50Latency    float64                          `json:"p50_latency"`
	P95Latency    float64                          `json:"p95_latency"`
	P99Latency    float64                          `json:"p99_latency"`
	GlobalSamples []TimeSeriesSampleDTO            `json:"global_samples,omitempty"`
	NodeSamples   map[string][]TimeSeriesSampleDTO `json:"node_samples,omitempty"`
}

type TimeSeriesSampleDTO struct {
	Timestamp     int64   `json:"timestamp"`
	QPS           float64 `json:"qps"`
	TotalRequests int64   `json:"total_requests"`
	SuccessCount  int64   `json:"success_count"`
	FailCount     int64   `json:"fail_count"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	P50LatencyMs  float64 `json:"p50_latency_ms"`
	P95LatencyMs  float64 `json:"p95_latency_ms"`
	P99LatencyMs  float64 `json:"p99_latency_ms"`
}
