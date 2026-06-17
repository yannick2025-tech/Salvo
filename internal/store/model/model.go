// Package model defines the data models for the Salvo persistence layer.
//
// All models embed the Model base struct which provides a Snowflake ID
// (JSON-serialised as string), timestamps, and soft-delete support.
// Repository implementations must filter out soft-deleted records
// (where deleted_at IS NOT NULL) in every query.
package model

import (
	"time"

	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
)

// Model is the base struct embedded by all data models.
type Model struct {
	ID        snowflake.ID `json:"id,string"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
	DeletedAt *time.Time   `json:"deleted_at,omitempty"`
}

// IsDeleted returns true if the record has been soft-deleted.
func (m *Model) IsDeleted() bool {
	return m.DeletedAt != nil
}

// Scene represents a test scenario configuration.
type Scene struct {
	Model
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	DAGJSON     string `json:"dag_json"`
	Variables   string `json:"variables,omitempty"`
	Plugins     string `json:"plugins,omitempty"`
	Status      string `json:"status"`
}

const (
	SceneStatusDraft     = "draft"
	SceneStatusReady     = "ready"
	SceneStatusRunning   = "running"
	SceneStatusCompleted = "completed"
	SceneStatusFailed    = "failed"
)

// Node represents a DAG node within a scene.
type Node struct {
	Model
	SceneID   snowflake.ID `json:"scene_id,string"`
	Name      string       `json:"name"`
	Type      string       `json:"type"`
	Config    string       `json:"config,omitempty"`
	Position  string       `json:"position,omitempty"`
	LoopCount int          `json:"loop_count,omitempty"`
}

const (
	NodeTypeHTTP      = "http"
	NodeTypeDelay     = "delay"
	NodeTypeCondition = "condition"
	NodeTypeIfElse    = "if-else"
	NodeTypeLoop      = "loop"
	NodeTypeGroup     = "group"
	NodeTypeWhile     = "while"
	NodeTypeSetup     = "setup"
	NodeTypeTeardown  = "teardown"
	NodeTypeTimer     = "timer"
)

// DataSource represents a CSV data source attached to a scene.
type DataSource struct {
	Model
	SceneID   snowflake.ID `json:"scene_id,string"`
	Name      string       `json:"name"`
	FileName  string       `json:"file_name"`
	Columns   string       `json:"columns"`   // JSON array of column names
	Rows      string       `json:"rows"`      // JSON array of row objects
	RowCount  int          `json:"row_count"`
}

// Edge represents a directed edge between two DAG nodes.
type Edge struct {
	Model
	SceneID   snowflake.ID `json:"scene_id,string"`
	FromNode  snowflake.ID `json:"from_node,string"`
	ToNode    snowflake.ID `json:"to_node,string"`
	Condition string       `json:"condition,omitempty"`
	Priority  int          `json:"priority,omitempty"`
}

// Variable represents a scoped variable definition.
type Variable struct {
	Model
	SceneID snowflake.ID `json:"scene_id,string"`
	Scope   string       `json:"scope"`
	Key     string       `json:"key"`
	Value   string       `json:"value"`
}

const (
	VariableScopeGlobal = "global"
	VariableScopeScene  = "scene"
	VariableScopeAPI    = "api"
)

// PluginConfig represents a plugin configuration for a scene.
type PluginConfig struct {
	Model
	SceneID  snowflake.ID `json:"scene_id,string"`
	Name     string       `json:"name"`
	Type     string       `json:"type"`
	Config   string       `json:"config"`
	Phase    string       `json:"phase"`
	Priority int          `json:"priority"`
	Enabled  bool         `json:"enabled"`
}

const (
	PluginPhaseBefore = "before"
	PluginPhaseAfter  = "after"
)

// Report represents a test execution report.
type Report struct {
	Model
	SceneID    snowflake.ID `json:"scene_id,string"`
	RunID      snowflake.ID `json:"run_id,string"`
	Status     string       `json:"status"`
	Summary    string       `json:"summary,omitempty"`
	Detail     string       `json:"detail,omitempty"`
	StartedAt  *time.Time   `json:"started_at,omitempty"`
	FinishedAt *time.Time   `json:"finished_at,omitempty"`
}

const (
	ReportStatusSuccess = "success"
	ReportStatusFailed  = "failed"
	ReportStatusPartial = "partial"
)

// RunRecord represents a single test execution record.
type RunRecord struct {
	Model
	RunID       snowflake.ID `json:"run_id,string"`
	SceneID     snowflake.ID `json:"scene_id,string"`
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
	ErrorMsg    string       `json:"error_msg,omitempty"`
	StartedAt   *time.Time   `json:"started_at,omitempty"`
	FinishedAt  *time.Time   `json:"finished_at,omitempty"`
}

const (
	RunStatusRunning   = "running"
	RunStatusCompleted = "completed"
	RunStatusFailed    = "failed"
	RunStatusCancelled = "cancelled"
)

type User struct {
	Model
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	Nickname     string     `json:"nickname"`
	RoleID       snowflake.ID `json:"role_id,string"`
	Status       string     `json:"status"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
}

const (
	UserStatusActive   = "active"
	UserStatusDisabled = "disabled"
)

type Role struct {
	Model
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IsBuiltin   bool   `json:"is_builtin"`
}

type Permission struct {
	Model
	Resource    string `json:"resource"`
	Action      string `json:"action"`
	Description string `json:"description,omitempty"`
}

type RolePermission struct {
	RoleID       snowflake.ID `json:"role_id,string"`
	PermissionID snowflake.ID `json:"permission_id,string"`
}

// TimeSeriesSample represents a time-series metric sample for a run or node.
type TimeSeriesSample struct {
	ID            int64        `json:"id"`
	RunID         snowflake.ID  `json:"run_id,string"`
	NodeID        string       `json:"node_id"`
	SampleTime    time.Time     `json:"sample_time"`
	WindowDuration int          `json:"window_duration"`
	QPS           float64      `json:"qps"`
	TotalRequests int64         `json:"total_requests"`
	SuccessCount  int64         `json:"success_count"`
	FailCount     int64         `json:"fail_count"`
	AvgLatencyMs  float64      `json:"avg_latency_ms"`
	P50LatencyMs  float64      `json:"p50_latency_ms"`
	P95LatencyMs  float64      `json:"p95_latency_ms"`
	P99LatencyMs  float64      `json:"p99_latency_ms"`
	MinLatencyMs  float64      `json:"min_latency_ms"`
	MaxLatencyMs  float64      `json:"max_latency_ms"`
}
