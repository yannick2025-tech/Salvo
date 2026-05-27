// Package sqlite implements the Repository interfaces using SQLite
// as the backing store. All queries automatically exclude soft-deleted
// records (WHERE deleted_at IS NULL).
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
	"github.com/yannick2025-tech/Salvo/internal/store/model"
	"github.com/yannick2025-tech/Salvo/internal/store/repo"
)

// DB wraps a sql.DB with helper methods for the Salvo data layer.
type DB struct {
	*sql.DB
	node *snowflake.Node
}

// Open creates a new DB backed by a SQLite database at dsn.
func Open(dsn string, nodeID int64) (*DB, error) {
	n, err := snowflake.NewNode(nodeID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: create snowflake node: %w", err)
	}

	db, err := sql.Open("sqlite3", dsn+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("sqlite: open: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("sqlite: ping: %w", err)
	}

	return &DB{DB: db, node: n}, nil
}

// NextID generates a new Snowflake ID.
func (db *DB) NextID() snowflake.ID {
	return db.node.Generate()
}

// Close closes the underlying database connection.
func (db *DB) Close() error {
	return db.DB.Close()
}

// --- SceneRepo ---

type SceneRepo struct {
	db *DB
}

func NewSceneRepo(db *DB) *SceneRepo {
	return &SceneRepo{db: db}
}

func (r *SceneRepo) Create(ctx context.Context, scene *model.Scene) error {
	now := time.Now().UTC()
	scene.ID = r.db.NextID()
	scene.CreatedAt = now
	scene.UpdatedAt = now

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO scenes (id, name, description, dag_json, variables, plugins, status, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
		scene.ID, scene.Name, scene.Description, scene.DAGJSON,
		scene.Variables, scene.Plugins, scene.Status,
		scene.CreatedAt, scene.UpdatedAt)
	return err
}

func (r *SceneRepo) GetByID(ctx context.Context, id snowflake.ID) (*model.Scene, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, description, dag_json, variables, plugins, status, created_at, updated_at
		FROM scenes WHERE id = ? AND deleted_at IS NULL`, id)
	return scanScene(row)
}

func (r *SceneRepo) List(ctx context.Context, filter repo.Filter) ([]*model.Scene, error) {
	query := `SELECT id, name, description, dag_json, variables, plugins, status, created_at, updated_at
		FROM scenes WHERE deleted_at IS NULL`
	args := []any{}

	if filter.Status != "" {
		query += ` AND status = ?`
		args = append(args, filter.Status)
	}
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var scenes []*model.Scene
	for rows.Next() {
		s, err := scanSceneRow(rows)
		if err != nil {
			return nil, err
		}
		scenes = append(scenes, s)
	}
	return scenes, rows.Err()
}

func (r *SceneRepo) Update(ctx context.Context, scene *model.Scene) error {
	scene.UpdatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE scenes SET name=?, description=?, dag_json=?, variables=?, plugins=?, status=?, updated_at=?
		WHERE id=? AND deleted_at IS NULL`,
		scene.Name, scene.Description, scene.DAGJSON, scene.Variables,
		scene.Plugins, scene.Status, scene.UpdatedAt, scene.ID)
	return err
}

func (r *SceneRepo) UpdateStatus(ctx context.Context, id snowflake.ID, status string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE scenes SET status=?, updated_at=?
		WHERE id=? AND deleted_at IS NULL`,
		status, now, id)
	return err
}

func (r *SceneRepo) Delete(ctx context.Context, id snowflake.ID) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `UPDATE scenes SET deleted_at=? WHERE id=?`, now, id)
	return err
}

func scanScene(row interface {
	Scan(dest ...any) error
}) (*model.Scene, error) {
	s := &model.Scene{}
	err := row.Scan(&s.ID, &s.Name, &s.Description, &s.DAGJSON,
		&s.Variables, &s.Plugins, &s.Status, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func scanSceneRow(rows interface {
	Scan(dest ...any) error
}) (*model.Scene, error) {
	return scanScene(rows)
}

// --- NodeRepo ---

type NodeRepo struct {
	db *DB
}

func NewNodeRepo(db *DB) *NodeRepo {
	return &NodeRepo{db: db}
}

func (r *NodeRepo) Create(ctx context.Context, node *model.Node) error {
	now := time.Now().UTC()
	node.ID = r.db.NextID()
	node.CreatedAt = now
	node.UpdatedAt = now

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO nodes (id, scene_id, name, type, config, position, loop_count, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
		node.ID, node.SceneID, node.Name, node.Type, node.Config,
		node.Position, node.LoopCount, node.CreatedAt, node.UpdatedAt)
	return err
}

func (r *NodeRepo) GetByID(ctx context.Context, id snowflake.ID) (*model.Node, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, scene_id, name, type, config, position, loop_count, created_at, updated_at
		FROM nodes WHERE id=? AND deleted_at IS NULL`, id)
	n := &model.Node{}
	err := row.Scan(&n.ID, &n.SceneID, &n.Name, &n.Type, &n.Config,
		&n.Position, &n.LoopCount, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return n, nil
}

func (r *NodeRepo) List(ctx context.Context, filter repo.Filter) ([]*model.Node, error) {
	query := `SELECT id, scene_id, name, type, config, position, loop_count, created_at, updated_at
		FROM nodes WHERE deleted_at IS NULL`
	args := []any{}

	if filter.SceneID != 0 {
		query += ` AND scene_id=?`
		args = append(args, filter.SceneID)
	}
	query += ` ORDER BY created_at ASC LIMIT ? OFFSET ?`
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var nodes []*model.Node
	for rows.Next() {
		n := &model.Node{}
		if err := rows.Scan(&n.ID, &n.SceneID, &n.Name, &n.Type, &n.Config,
			&n.Position, &n.LoopCount, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

func (r *NodeRepo) Update(ctx context.Context, node *model.Node) error {
	node.UpdatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE nodes SET name=?, type=?, config=?, position=?, loop_count=?, updated_at=?
		WHERE id=? AND deleted_at IS NULL`,
		node.Name, node.Type, node.Config, node.Position, node.LoopCount,
		node.UpdatedAt, node.ID)
	return err
}

func (r *NodeRepo) Delete(ctx context.Context, id snowflake.ID) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `UPDATE nodes SET deleted_at=? WHERE id=?`, now, id)
	return err
}

// --- EdgeRepo ---

type EdgeRepo struct {
	db *DB
}

func NewEdgeRepo(db *DB) *EdgeRepo {
	return &EdgeRepo{db: db}
}

func (r *EdgeRepo) Create(ctx context.Context, edge *model.Edge) error {
	now := time.Now().UTC()
	edge.ID = r.db.NextID()
	edge.CreatedAt = now
	edge.UpdatedAt = now

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO edges (id, scene_id, from_node, to_node, condition, priority, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
		edge.ID, edge.SceneID, edge.FromNode, edge.ToNode,
		edge.Condition, edge.Priority, edge.CreatedAt, edge.UpdatedAt)
	return err
}

func (r *EdgeRepo) GetByID(ctx context.Context, id snowflake.ID) (*model.Edge, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, scene_id, from_node, to_node, condition, priority, created_at, updated_at
		FROM edges WHERE id=? AND deleted_at IS NULL`, id)
	e := &model.Edge{}
	err := row.Scan(&e.ID, &e.SceneID, &e.FromNode, &e.ToNode,
		&e.Condition, &e.Priority, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (r *EdgeRepo) List(ctx context.Context, filter repo.Filter) ([]*model.Edge, error) {
	query := `SELECT id, scene_id, from_node, to_node, condition, priority, created_at, updated_at
		FROM edges WHERE deleted_at IS NULL`
	args := []any{}

	if filter.SceneID != 0 {
		query += ` AND scene_id=?`
		args = append(args, filter.SceneID)
	}
	query += ` ORDER BY priority ASC, created_at ASC LIMIT ? OFFSET ?`
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var edges []*model.Edge
	for rows.Next() {
		e := &model.Edge{}
		if err := rows.Scan(&e.ID, &e.SceneID, &e.FromNode, &e.ToNode,
			&e.Condition, &e.Priority, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		edges = append(edges, e)
	}
	return edges, rows.Err()
}

func (r *EdgeRepo) Update(ctx context.Context, edge *model.Edge) error {
	edge.UpdatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE edges SET from_node=?, to_node=?, condition=?, priority=?, updated_at=?
		WHERE id=? AND deleted_at IS NULL`,
		edge.FromNode, edge.ToNode, edge.Condition, edge.Priority,
		edge.UpdatedAt, edge.ID)
	return err
}

func (r *EdgeRepo) Delete(ctx context.Context, id snowflake.ID) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `UPDATE edges SET deleted_at=? WHERE id=?`, now, id)
	return err
}

// --- VariableRepo ---

type VariableRepo struct {
	db *DB
}

func NewVariableRepo(db *DB) *VariableRepo {
	return &VariableRepo{db: db}
}

func (r *VariableRepo) Create(ctx context.Context, v *model.Variable) error {
	now := time.Now().UTC()
	v.ID = r.db.NextID()
	v.CreatedAt = now
	v.UpdatedAt = now

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO variables (id, scene_id, scope, key, value, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, NULL)`,
		v.ID, v.SceneID, v.Scope, v.Key, v.Value, v.CreatedAt, v.UpdatedAt)
	return err
}

func (r *VariableRepo) GetByID(ctx context.Context, id snowflake.ID) (*model.Variable, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, scene_id, scope, key, value, created_at, updated_at
		FROM variables WHERE id=? AND deleted_at IS NULL`, id)
	v := &model.Variable{}
	err := row.Scan(&v.ID, &v.SceneID, &v.Scope, &v.Key, &v.Value, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (r *VariableRepo) List(ctx context.Context, filter repo.Filter) ([]*model.Variable, error) {
	query := `SELECT id, scene_id, scope, key, value, created_at, updated_at
		FROM variables WHERE deleted_at IS NULL`
	args := []any{}

	if filter.SceneID != 0 {
		query += ` AND scene_id=?`
		args = append(args, filter.SceneID)
	}
	query += ` ORDER BY scope, key LIMIT ? OFFSET ?`
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var vars []*model.Variable
	for rows.Next() {
		v := &model.Variable{}
		if err := rows.Scan(&v.ID, &v.SceneID, &v.Scope, &v.Key, &v.Value, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		vars = append(vars, v)
	}
	return vars, rows.Err()
}

func (r *VariableRepo) Update(ctx context.Context, v *model.Variable) error {
	v.UpdatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE variables SET scope=?, key=?, value=?, updated_at=?
		WHERE id=? AND deleted_at IS NULL`,
		v.Scope, v.Key, v.Value, v.UpdatedAt, v.ID)
	return err
}

func (r *VariableRepo) Delete(ctx context.Context, id snowflake.ID) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `UPDATE variables SET deleted_at=? WHERE id=?`, now, id)
	return err
}

// --- PluginConfigRepo ---

type PluginConfigRepo struct {
	db *DB
}

func NewPluginConfigRepo(db *DB) *PluginConfigRepo {
	return &PluginConfigRepo{db: db}
}

func (r *PluginConfigRepo) Create(ctx context.Context, pc *model.PluginConfig) error {
	now := time.Now().UTC()
	pc.ID = r.db.NextID()
	pc.CreatedAt = now
	pc.UpdatedAt = now

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO plugin_configs (id, scene_id, name, type, config, phase, priority, enabled, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
		pc.ID, pc.SceneID, pc.Name, pc.Type, pc.Config,
		pc.Phase, pc.Priority, pc.Enabled, pc.CreatedAt, pc.UpdatedAt)
	return err
}

func (r *PluginConfigRepo) GetByID(ctx context.Context, id snowflake.ID) (*model.PluginConfig, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, scene_id, name, type, config, phase, priority, enabled, created_at, updated_at
		FROM plugin_configs WHERE id=? AND deleted_at IS NULL`, id)
	pc := &model.PluginConfig{}
	err := row.Scan(&pc.ID, &pc.SceneID, &pc.Name, &pc.Type, &pc.Config,
		&pc.Phase, &pc.Priority, &pc.Enabled, &pc.CreatedAt, &pc.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return pc, nil
}

func (r *PluginConfigRepo) List(ctx context.Context, filter repo.Filter) ([]*model.PluginConfig, error) {
	query := `SELECT id, scene_id, name, type, config, phase, priority, enabled, created_at, updated_at
		FROM plugin_configs WHERE deleted_at IS NULL`
	args := []any{}

	if filter.SceneID != 0 {
		query += ` AND scene_id=?`
		args = append(args, filter.SceneID)
	}
	query += ` ORDER BY priority ASC, created_at ASC LIMIT ? OFFSET ?`
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var configs []*model.PluginConfig
	for rows.Next() {
		pc := &model.PluginConfig{}
		if err := rows.Scan(&pc.ID, &pc.SceneID, &pc.Name, &pc.Type, &pc.Config,
			&pc.Phase, &pc.Priority, &pc.Enabled, &pc.CreatedAt, &pc.UpdatedAt); err != nil {
			return nil, err
		}
		configs = append(configs, pc)
	}
	return configs, rows.Err()
}

func (r *PluginConfigRepo) Update(ctx context.Context, pc *model.PluginConfig) error {
	pc.UpdatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE plugin_configs SET name=?, type=?, config=?, phase=?, priority=?, enabled=?, updated_at=?
		WHERE id=? AND deleted_at IS NULL`,
		pc.Name, pc.Type, pc.Config, pc.Phase, pc.Priority, pc.Enabled,
		pc.UpdatedAt, pc.ID)
	return err
}

func (r *PluginConfigRepo) Delete(ctx context.Context, id snowflake.ID) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `UPDATE plugin_configs SET deleted_at=? WHERE id=?`, now, id)
	return err
}

// --- ReportRepo ---

type ReportRepo struct {
	db *DB
}

func NewReportRepo(db *DB) *ReportRepo {
	return &ReportRepo{db: db}
}

func (r *ReportRepo) Create(ctx context.Context, report *model.Report) error {
	now := time.Now().UTC()
	report.ID = r.db.NextID()
	report.CreatedAt = now
	report.UpdatedAt = now

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO reports (id, scene_id, run_id, status, summary, detail, started_at, finished_at, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
		report.ID, report.SceneID, report.RunID, report.Status,
		report.Summary, report.Detail, report.StartedAt, report.FinishedAt,
		report.CreatedAt, report.UpdatedAt)
	return err
}

func (r *ReportRepo) GetByID(ctx context.Context, id snowflake.ID) (*model.Report, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, scene_id, run_id, status, summary, detail, started_at, finished_at, created_at, updated_at
		FROM reports WHERE id=? AND deleted_at IS NULL`, id)
	rp := &model.Report{}
	err := row.Scan(&rp.ID, &rp.SceneID, &rp.RunID, &rp.Status,
		&rp.Summary, &rp.Detail, &rp.StartedAt, &rp.FinishedAt,
		&rp.CreatedAt, &rp.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return rp, nil
}

func (r *ReportRepo) GetByRunID(ctx context.Context, runID snowflake.ID) (*model.Report, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, scene_id, run_id, status, summary, detail, started_at, finished_at, created_at, updated_at
		FROM reports WHERE run_id=? AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 1`, runID)
	rp := &model.Report{}
	err := row.Scan(&rp.ID, &rp.SceneID, &rp.RunID, &rp.Status,
		&rp.Summary, &rp.Detail, &rp.StartedAt, &rp.FinishedAt,
		&rp.CreatedAt, &rp.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return rp, nil
}

func (r *ReportRepo) List(ctx context.Context, filter repo.Filter) ([]*model.Report, error) {
	query := `SELECT id, scene_id, run_id, status, summary, detail, started_at, finished_at, created_at, updated_at
		FROM reports WHERE deleted_at IS NULL`
	args := []any{}

	if filter.SceneID != 0 {
		query += ` AND scene_id=?`
		args = append(args, filter.SceneID)
	}
	if filter.Status != "" {
		query += ` AND status=?`
		args = append(args, filter.Status)
	}
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var reports []*model.Report
	for rows.Next() {
		rp := &model.Report{}
		if err := rows.Scan(&rp.ID, &rp.SceneID, &rp.RunID, &rp.Status,
			&rp.Summary, &rp.Detail, &rp.StartedAt, &rp.FinishedAt,
			&rp.CreatedAt, &rp.UpdatedAt); err != nil {
			return nil, err
		}
		reports = append(reports, rp)
	}
	return reports, rows.Err()
}

func (r *ReportRepo) Update(ctx context.Context, report *model.Report) error {
	report.UpdatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE reports SET status=?, summary=?, detail=?, started_at=?, finished_at=?, updated_at=?
		WHERE id=? AND deleted_at IS NULL`,
		report.Status, report.Summary, report.Detail,
		report.StartedAt, report.FinishedAt, report.UpdatedAt, report.ID)
	return err
}

func (r *ReportRepo) Delete(ctx context.Context, id snowflake.ID) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `UPDATE reports SET deleted_at=? WHERE id=?`, now, id)
	return err
}

// --- RunRecordRepo ---

type RunRecordRepo struct {
	db *DB
}

func NewRunRecordRepo(db *DB) *RunRecordRepo {
	return &RunRecordRepo{db: db}
}

func (r *RunRecordRepo) Create(ctx context.Context, rec *model.RunRecord) error {
	now := time.Now().UTC()
	rec.ID = r.db.NextID()
	rec.CreatedAt = now
	rec.UpdatedAt = now

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO run_records (id, scene_id, status, worker_count, run_mode, duration, count,
			total_reqs, success_reqs, failed_reqs, avg_latency, p50_latency, p90_latency, p95_latency, p99_latency,
			error_msg, started_at, finished_at, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
		rec.ID, rec.SceneID, rec.Status, rec.WorkerCount, rec.RunMode, rec.Duration, rec.Count,
		rec.TotalReqs, rec.SuccessReqs, rec.FailedReqs,
		rec.AvgLatency, rec.P50Latency, rec.P90Latency, rec.P95Latency, rec.P99Latency,
		rec.ErrorMsg, rec.StartedAt, rec.FinishedAt,
		rec.CreatedAt, rec.UpdatedAt)
	return err
}

func (r *RunRecordRepo) GetByID(ctx context.Context, id snowflake.ID) (*model.RunRecord, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, scene_id, status, worker_count, run_mode, duration, count,
			total_reqs, success_reqs, failed_reqs, avg_latency, p50_latency, p90_latency, p95_latency, p99_latency,
			error_msg, started_at, finished_at, created_at, updated_at
		FROM run_records WHERE id=? AND deleted_at IS NULL`, id)
	return scanRunRecord(row)
}

func (r *RunRecordRepo) List(ctx context.Context, filter repo.Filter) ([]*model.RunRecord, error) {
	query := `SELECT id, scene_id, status, worker_count, run_mode, duration, count,
		total_reqs, success_reqs, failed_reqs, avg_latency, p50_latency, p90_latency, p95_latency, p99_latency,
		error_msg, started_at, finished_at, created_at, updated_at
		FROM run_records WHERE deleted_at IS NULL`
	args := []any{}

	if filter.SceneID != 0 {
		query += ` AND scene_id=?`
		args = append(args, filter.SceneID)
	}
	if filter.Status != "" {
		query += ` AND status=?`
		args = append(args, filter.Status)
	}
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var records []*model.RunRecord
	for rows.Next() {
		rec, err := scanRunRecordRow(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

func (r *RunRecordRepo) Update(ctx context.Context, rec *model.RunRecord) error {
	rec.UpdatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE run_records SET status=?, worker_count=?, run_mode=?, duration=?,
			total_reqs=?, success_reqs=?, failed_reqs=?,
			avg_latency=?, p50_latency=?, p95_latency=?, p99_latency=?,
			error_msg=?, started_at=?, finished_at=?, updated_at=?
		WHERE id=? AND deleted_at IS NULL`,
		rec.Status, rec.WorkerCount, rec.RunMode, rec.Duration,
		rec.TotalReqs, rec.SuccessReqs, rec.FailedReqs,
		rec.AvgLatency, rec.P50Latency, rec.P95Latency, rec.P99Latency,
		rec.ErrorMsg, rec.StartedAt, rec.FinishedAt, rec.UpdatedAt, rec.ID)
	return err
}

func (r *RunRecordRepo) Delete(ctx context.Context, id snowflake.ID) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `UPDATE run_records SET deleted_at=? WHERE id=?`, now, id)
	return err
}

func scanRunRecord(row interface {
	Scan(dest ...any) error
}) (*model.RunRecord, error) {
	rec := &model.RunRecord{}
	err := row.Scan(&rec.ID, &rec.SceneID, &rec.Status, &rec.WorkerCount,
		&rec.RunMode, &rec.Duration, &rec.Count, &rec.TotalReqs, &rec.SuccessReqs,
		&rec.FailedReqs, &rec.AvgLatency, &rec.P50Latency, &rec.P90Latency, &rec.P95Latency,
		&rec.P99Latency, &rec.ErrorMsg, &rec.StartedAt, &rec.FinishedAt,
		&rec.CreatedAt, &rec.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return rec, nil
}

func scanRunRecordRow(rows interface {
	Scan(dest ...any) error
}) (*model.RunRecord, error) {
	return scanRunRecord(rows)
}

// --- UserRepo ---

type UserRepo struct {
	db *DB
}

func NewUserRepo(db *DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(ctx context.Context, user *model.User) error {
	now := time.Now().UTC()
	user.ID = r.db.NextID()
	user.CreatedAt = now
	user.UpdatedAt = now

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, nickname, role_id, status, last_login_at, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
		user.ID, user.Email, user.PasswordHash, user.Nickname,
		user.RoleID, user.Status, user.LastLoginAt,
		user.CreatedAt, user.UpdatedAt)
	return err
}

func (r *UserRepo) GetByID(ctx context.Context, id snowflake.ID) (*model.User, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, email, password_hash, nickname, role_id, status, last_login_at, created_at, updated_at
		FROM users WHERE id=? AND deleted_at IS NULL`, id)
	return scanUser(row)
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, email, password_hash, nickname, role_id, status, last_login_at, created_at, updated_at
		FROM users WHERE email=? AND deleted_at IS NULL`, email)
	return scanUser(row)
}

func (r *UserRepo) List(ctx context.Context, filter repo.Filter) ([]*model.User, error) {
	query := `SELECT id, email, password_hash, nickname, role_id, status, last_login_at, created_at, updated_at
		FROM users WHERE deleted_at IS NULL`
	args := []any{}

	if filter.Status != "" {
		query += ` AND status=?`
		args = append(args, filter.Status)
	}
	query += ` ORDER BY created_at ASC LIMIT ? OFFSET ?`
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var users []*model.User
	for rows.Next() {
		u, err := scanUserRow(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *UserRepo) Update(ctx context.Context, user *model.User) error {
	user.UpdatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE users SET email=?, nickname=?, role_id=?, status=?, last_login_at=?, updated_at=?
		WHERE id=? AND deleted_at IS NULL`,
		user.Email, user.Nickname, user.RoleID, user.Status,
		user.LastLoginAt, user.UpdatedAt, user.ID)
	return err
}

func (r *UserRepo) Delete(ctx context.Context, id snowflake.ID) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `UPDATE users SET deleted_at=? WHERE id=?`, now, id)
	return err
}

func scanUser(row interface {
	Scan(dest ...any) error
}) (*model.User, error) {
	u := &model.User{}
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Nickname,
		&u.RoleID, &u.Status, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func scanUserRow(rows interface {
	Scan(dest ...any) error
}) (*model.User, error) {
	return scanUser(rows)
}

// --- RoleRepo ---

type RoleRepo struct {
	db *DB
}

func NewRoleRepo(db *DB) *RoleRepo {
	return &RoleRepo{db: db}
}

func (r *RoleRepo) Create(ctx context.Context, role *model.Role) error {
	now := time.Now().UTC()
	role.ID = r.db.NextID()
	role.CreatedAt = now
	role.UpdatedAt = now

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO roles (id, name, description, is_builtin, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, NULL)`,
		role.ID, role.Name, role.Description, role.IsBuiltin,
		role.CreatedAt, role.UpdatedAt)
	return err
}

func (r *RoleRepo) GetByID(ctx context.Context, id snowflake.ID) (*model.Role, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, description, is_builtin, created_at, updated_at
		FROM roles WHERE id=? AND deleted_at IS NULL`, id)
	return scanRole(row)
}

func (r *RoleRepo) GetByName(ctx context.Context, name string) (*model.Role, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, description, is_builtin, created_at, updated_at
		FROM roles WHERE name=? AND deleted_at IS NULL`, name)
	return scanRole(row)
}

func (r *RoleRepo) List(ctx context.Context, filter repo.Filter) ([]*model.Role, error) {
	query := `SELECT id, name, description, is_builtin, created_at, updated_at
		FROM roles WHERE deleted_at IS NULL`
	args := []any{}

	query += ` ORDER BY created_at ASC LIMIT ? OFFSET ?`
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var roles []*model.Role
	for rows.Next() {
		rl, err := scanRoleRow(rows)
		if err != nil {
			return nil, err
		}
		roles = append(roles, rl)
	}
	return roles, rows.Err()
}

func (r *RoleRepo) Update(ctx context.Context, role *model.Role) error {
	role.UpdatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE roles SET name=?, description=?, is_builtin=?, updated_at=?
		WHERE id=? AND deleted_at IS NULL`,
		role.Name, role.Description, role.IsBuiltin, role.UpdatedAt, role.ID)
	return err
}

func (r *RoleRepo) Delete(ctx context.Context, id snowflake.ID) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `UPDATE roles SET deleted_at=? WHERE id=?`, now, id)
	return err
}

func scanRole(row interface {
	Scan(dest ...any) error
}) (*model.Role, error) {
	rl := &model.Role{}
	err := row.Scan(&rl.ID, &rl.Name, &rl.Description, &rl.IsBuiltin,
		&rl.CreatedAt, &rl.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return rl, nil
}

func scanRoleRow(rows interface {
	Scan(dest ...any) error
}) (*model.Role, error) {
	return scanRole(rows)
}

// --- PermissionRepo ---

type PermissionRepo struct {
	db *DB
}

func NewPermissionRepo(db *DB) *PermissionRepo {
	return &PermissionRepo{db: db}
}

func (r *PermissionRepo) Create(ctx context.Context, perm *model.Permission) error {
	now := time.Now().UTC()
	perm.ID = r.db.NextID()
	perm.CreatedAt = now

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO permissions (id, resource, action, description, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		perm.ID, perm.Resource, perm.Action, perm.Description, perm.CreatedAt)
	return err
}

func (r *PermissionRepo) GetByID(ctx context.Context, id snowflake.ID) (*model.Permission, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, resource, action, description, created_at
		FROM permissions WHERE id=?`, id)
	return scanPermission(row)
}

func (r *PermissionRepo) GetByResourceAction(ctx context.Context, resource, action string) (*model.Permission, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, resource, action, description, created_at
		FROM permissions WHERE resource=? AND action=?`, resource, action)
	return scanPermission(row)
}

func (r *PermissionRepo) List(ctx context.Context) ([]*model.Permission, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, resource, action, description, created_at
		FROM permissions ORDER BY resource, action`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var perms []*model.Permission
	for rows.Next() {
		p, err := scanPermissionRow(rows)
		if err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

func (r *PermissionRepo) ListByRoleID(ctx context.Context, roleID snowflake.ID) ([]*model.Permission, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id, p.resource, p.action, p.description, p.created_at
		FROM permissions p
		INNER JOIN role_permissions rp ON rp.permission_id = p.id
		WHERE rp.role_id = ?
		ORDER BY p.resource, p.action`, roleID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var perms []*model.Permission
	for rows.Next() {
		p, err := scanPermissionRow(rows)
		if err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

func scanPermission(row interface {
	Scan(dest ...any) error
}) (*model.Permission, error) {
	p := &model.Permission{}
	err := row.Scan(&p.ID, &p.Resource, &p.Action, &p.Description, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func scanPermissionRow(rows interface {
	Scan(dest ...any) error
}) (*model.Permission, error) {
	return scanPermission(rows)
}

// --- RolePermissionRepo ---

type RolePermissionRepo struct {
	db *DB
}

func NewRolePermissionRepo(db *DB) *RolePermissionRepo {
	return &RolePermissionRepo{db: db}
}

func (r *RolePermissionRepo) Assign(ctx context.Context, roleID, permissionID snowflake.ID) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO role_permissions (role_id, permission_id) VALUES (?, ?)`,
		roleID, permissionID)
	return err
}

func (r *RolePermissionRepo) Revoke(ctx context.Context, roleID, permissionID snowflake.ID) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM role_permissions WHERE role_id=? AND permission_id=?`,
		roleID, permissionID)
	return err
}

func (r *RolePermissionRepo) RevokeAll(ctx context.Context, roleID snowflake.ID) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM role_permissions WHERE role_id=?`, roleID)
	return err
}

func (r *RolePermissionRepo) ListPermissions(ctx context.Context, roleID snowflake.ID) ([]*model.Permission, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id, p.resource, p.action, p.description, p.created_at
		FROM permissions p
		INNER JOIN role_permissions rp ON rp.permission_id = p.id
		WHERE rp.role_id = ?
		ORDER BY p.resource, p.action`, roleID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var perms []*model.Permission
	for rows.Next() {
		p, err := scanPermissionRow(rows)
		if err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

// --- DataSourceRepo ---

type DataSourceRepo struct {
	db *DB
}

func NewDataSourceRepo(db *DB) *DataSourceRepo {
	return &DataSourceRepo{db: db}
}

func (r *DataSourceRepo) Create(ctx context.Context, ds *model.DataSource) error {
	now := time.Now().UTC()
	ds.ID = r.db.NextID()
	ds.CreatedAt = now
	ds.UpdatedAt = now

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO data_sources (id, scene_id, name, file_name, columns, rows, row_count, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
		ds.ID, ds.SceneID, ds.Name, ds.FileName, ds.Columns, ds.Rows,
		ds.RowCount, ds.CreatedAt, ds.UpdatedAt)
	return err
}

func (r *DataSourceRepo) GetByID(ctx context.Context, id snowflake.ID) (*model.DataSource, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, scene_id, name, file_name, columns, rows, row_count, created_at, updated_at
		FROM data_sources WHERE id=? AND deleted_at IS NULL`, id)
	ds := &model.DataSource{}
	err := row.Scan(&ds.ID, &ds.SceneID, &ds.Name, &ds.FileName,
		&ds.Columns, &ds.Rows, &ds.RowCount, &ds.CreatedAt, &ds.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return ds, nil
}

func (r *DataSourceRepo) GetBySceneIDAndName(ctx context.Context, sceneID snowflake.ID, name string) (*model.DataSource, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, scene_id, name, file_name, columns, rows, row_count, created_at, updated_at
		FROM data_sources WHERE scene_id=? AND name=? AND deleted_at IS NULL`, sceneID, name)
	ds := &model.DataSource{}
	err := row.Scan(&ds.ID, &ds.SceneID, &ds.Name, &ds.FileName,
		&ds.Columns, &ds.Rows, &ds.RowCount, &ds.CreatedAt, &ds.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return ds, nil
}

func (r *DataSourceRepo) ListBySceneID(ctx context.Context, sceneID snowflake.ID) ([]*model.DataSource, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, scene_id, name, file_name, columns, rows, row_count, created_at, updated_at
		FROM data_sources WHERE scene_id=? AND deleted_at IS NULL
		ORDER BY created_at ASC`, sceneID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var sources []*model.DataSource
	for rows.Next() {
		ds := &model.DataSource{}
		if err := rows.Scan(&ds.ID, &ds.SceneID, &ds.Name, &ds.FileName,
			&ds.Columns, &ds.Rows, &ds.RowCount, &ds.CreatedAt, &ds.UpdatedAt); err != nil {
			return nil, err
		}
		sources = append(sources, ds)
	}
	return sources, rows.Err()
}

func (r *DataSourceRepo) Delete(ctx context.Context, id snowflake.ID) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `UPDATE data_sources SET deleted_at=? WHERE id=?`, now, id)
	return err
}
