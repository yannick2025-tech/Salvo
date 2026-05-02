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
		INSERT INTO run_records (id, scene_id, status, worker_count, run_mode, duration,
			total_reqs, success_reqs, failed_reqs, avg_latency, p50_latency, p95_latency, p99_latency,
			error_msg, started_at, finished_at, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
		rec.ID, rec.SceneID, rec.Status, rec.WorkerCount, rec.RunMode, rec.Duration,
		rec.TotalReqs, rec.SuccessReqs, rec.FailedReqs,
		rec.AvgLatency, rec.P50Latency, rec.P95Latency, rec.P99Latency,
		rec.ErrorMsg, rec.StartedAt, rec.FinishedAt,
		rec.CreatedAt, rec.UpdatedAt)
	return err
}

func (r *RunRecordRepo) GetByID(ctx context.Context, id snowflake.ID) (*model.RunRecord, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, scene_id, status, worker_count, run_mode, duration,
			total_reqs, success_reqs, failed_reqs, avg_latency, p50_latency, p95_latency, p99_latency,
			error_msg, started_at, finished_at, created_at, updated_at
		FROM run_records WHERE id=? AND deleted_at IS NULL`, id)
	return scanRunRecord(row)
}

func (r *RunRecordRepo) List(ctx context.Context, filter repo.Filter) ([]*model.RunRecord, error) {
	query := `SELECT id, scene_id, status, worker_count, run_mode, duration,
		total_reqs, success_reqs, failed_reqs, avg_latency, p50_latency, p95_latency, p99_latency,
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
		&rec.RunMode, &rec.Duration, &rec.TotalReqs, &rec.SuccessReqs,
		&rec.FailedReqs, &rec.AvgLatency, &rec.P50Latency, &rec.P95Latency,
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
