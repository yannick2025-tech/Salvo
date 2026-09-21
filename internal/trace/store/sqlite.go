// Package tracestore provides SQLite persistence for trace and span data.
package tracestore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
	tracelib "github.com/yannick2025-tech/Salvo/internal/trace"
)

// Store persists trace data to SQLite.
type Store struct {
	db *sql.DB
}

// New creates a new trace Store.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// SaveTrace persists a completed trace and all its spans.
func (s *Store) SaveTrace(ctx context.Context, tr *tracelib.Trace) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("tracestore: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO traces (id, scene_id, run_id, status, error, started_at, finished_at, duration_ns)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		tr.ID, tr.SceneID, tr.RunID, string(tr.Status), tr.Error,
		tr.StartedAt, tr.FinishedAt, tr.Duration.Nanoseconds())
	if err != nil {
		return fmt.Errorf("tracestore: insert trace: %w", err)
	}

	for _, sp := range tr.Spans {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO spans (id, trace_id, node_id, status, error, input, output, started_at, finished_at, duration_ns)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			sp.ID, sp.TraceID, sp.NodeID, string(sp.Status), sp.Error,
			sp.Input, sp.Output, sp.StartedAt, sp.FinishedAt, sp.Duration.Nanoseconds())
		if err != nil {
			return fmt.Errorf("tracestore: insert span: %w", err)
		}
	}

	return tx.Commit()
}

// GetTrace retrieves a trace by ID including all its spans.
func (s *Store) GetTrace(ctx context.Context, id snowflake.ID) (*tracelib.Trace, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, scene_id, run_id, status, error, started_at, finished_at, duration_ns
		FROM traces WHERE id = ?`, id)

	tr := &tracelib.Trace{}
	var status, errMsg string
	var durationNs int64
	var finishedAt sql.NullTime

	if err := row.Scan(&tr.ID, &tr.SceneID, &tr.RunID, &status, &errMsg,
		&tr.StartedAt, &finishedAt, &durationNs); err != nil {
		return nil, fmt.Errorf("tracestore: get trace: %w", err)
	}

	tr.Status = tracelib.SpanStatus(status)
	tr.Error = errMsg
	tr.Duration = time.Duration(durationNs)
	if finishedAt.Valid {
		tr.FinishedAt = finishedAt.Time
	}

	spans, err := s.listSpans(ctx, id)
	if err != nil {
		return nil, err
	}
	tr.Spans = spans

	return tr, nil
}

// TraceFilter holds combined (AND) query conditions for listing traces.
// All fields are optional; zero values are ignored.
type TraceFilter struct {
	TraceID     string        // smart match: exact on traces.id OR traces.run_id
	SceneID     snowflake.ID  // exact match on traces.scene_id
	SceneName   string        // fuzzy (contains) match on scenes.name
	Status      string        // exact match on traces.status (ok|error|skip|canceled)
	MinDuration time.Duration // inclusive lower bound
	MaxDuration time.Duration // inclusive upper bound
}

// escapeLike escapes LIKE wildcards so user input is matched literally.
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// buildTraceWhere builds the shared WHERE clause (with LEFT JOIN on scenes)
// for both ListTraces and CountTraces. Conditions are combined with AND.
func buildTraceWhere(f TraceFilter) (string, []any) {
	conds := []string{}
	args := []any{}

	if f.TraceID != "" {
		// Smart match: the input may be either the trace DB primary key
		// (traces.id) or the business run ID (traces.run_id) used for log search.
		conds = append(conds, "(t.id = ? OR t.run_id = ?)")
		args = append(args, f.TraceID, f.TraceID)
	}
	if f.SceneID != 0 {
		conds = append(conds, "t.scene_id = ?")
		args = append(args, f.SceneID)
	}
	if f.SceneName != "" {
		conds = append(conds, `sc.name LIKE '%' || ? || '%' ESCAPE '\'`)
		args = append(args, escapeLike(f.SceneName))
	}
	if f.Status != "" {
		conds = append(conds, "t.status = ?")
		args = append(args, f.Status)
	}
	if f.MinDuration > 0 {
		conds = append(conds, "t.duration_ns >= ?")
		args = append(args, f.MinDuration.Nanoseconds())
	}
	if f.MaxDuration > 0 {
		conds = append(conds, "t.duration_ns <= ?")
		args = append(args, f.MaxDuration.Nanoseconds())
	}

	where := ` FROM traces t LEFT JOIN scenes sc ON sc.id = t.scene_id`
	if len(conds) > 0 {
		where += ` WHERE ` + strings.Join(conds, " AND ")
	}
	return where, args
}

// ListTraces returns traces ordered by creation time descending.
func (s *Store) ListTraces(ctx context.Context, filter TraceFilter, limit, offset int) ([]*tracelib.Trace, error) {
	if limit <= 0 {
		limit = 50
	}

	where, whereArgs := buildTraceWhere(filter)
	query := `SELECT t.id, t.scene_id, t.run_id, t.status, t.error, t.started_at, t.finished_at, t.duration_ns` +
		where + ` ORDER BY t.started_at DESC LIMIT ? OFFSET ?`
	args := append([]any{}, whereArgs...)
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("tracestore: list traces: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var traces []*tracelib.Trace
	for rows.Next() {
		tr := &tracelib.Trace{}
		var status, errMsg string
		var durationNs int64
		var finishedAt sql.NullTime

		if err := rows.Scan(&tr.ID, &tr.SceneID, &tr.RunID, &status, &errMsg,
			&tr.StartedAt, &finishedAt, &durationNs); err != nil {
			return nil, err
		}

		tr.Status = tracelib.SpanStatus(status)
		tr.Error = errMsg
		tr.Duration = time.Duration(durationNs)
		if finishedAt.Valid {
			tr.FinishedAt = finishedAt.Time
		}
		traces = append(traces, tr)
	}

	return traces, rows.Err()
}

// GetTraceByRunID retrieves a trace by its run ID.
func (s *Store) GetTraceByRunID(ctx context.Context, runID snowflake.ID) (*tracelib.Trace, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, scene_id, run_id, status, error, started_at, finished_at, duration_ns
		FROM traces WHERE run_id = ?`, runID)

	tr := &tracelib.Trace{}
	var status, errMsg string
	var durationNs int64
	var finishedAt sql.NullTime

	if err := row.Scan(&tr.ID, &tr.SceneID, &tr.RunID, &status, &errMsg,
		&tr.StartedAt, &finishedAt, &durationNs); err != nil {
		return nil, fmt.Errorf("tracestore: get trace by run_id: %w", err)
	}

	tr.Status = tracelib.SpanStatus(status)
	tr.Error = errMsg
	tr.Duration = time.Duration(durationNs)
	if finishedAt.Valid {
		tr.FinishedAt = finishedAt.Time
	}

	spans, err := s.listSpans(ctx, tr.ID)
	if err != nil {
		return nil, err
	}
	tr.Spans = spans

	return tr, nil
}

func (s *Store) listSpans(ctx context.Context, traceID snowflake.ID) ([]*tracelib.Span, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, trace_id, node_id, status, error, input, output, started_at, finished_at, duration_ns
		FROM spans WHERE trace_id = ?
		ORDER BY started_at ASC`, traceID)
	if err != nil {
		return nil, fmt.Errorf("tracestore: list spans: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var spans []*tracelib.Span
	for rows.Next() {
		sp := &tracelib.Span{}
		var status, errMsg, input, output string
		var durationNs int64
		var finishedAt sql.NullTime

		if err := rows.Scan(&sp.ID, &sp.TraceID, &sp.NodeID, &status, &errMsg,
			&input, &output, &sp.StartedAt, &finishedAt, &durationNs); err != nil {
			return nil, err
		}

		sp.Status = tracelib.SpanStatus(status)
		sp.Error = errMsg
		sp.Input = input
		sp.Output = output
		sp.Duration = time.Duration(durationNs)
		if finishedAt.Valid {
			sp.FinishedAt = finishedAt.Time
		}
		spans = append(spans, sp)
	}

	return spans, rows.Err()
}

// CountTraces returns the total number of traces matching the filter.
func (s *Store) CountTraces(ctx context.Context, filter TraceFilter) (int, error) {
	where, args := buildTraceWhere(filter)
	query := `SELECT COUNT(*)` + where

	var count int
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("tracestore: count traces: %w", err)
	}
	return count, nil
}

// ListAllTraces returns all traces with their spans, ordered by started_at DESC.
// Used for loading historical data into memory on startup.
func (s *Store) ListAllTraces(ctx context.Context, limit int) ([]*tracelib.Trace, error) {
	if limit <= 0 {
		limit = 1000
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, scene_id, run_id, status, error, started_at, finished_at, duration_ns
		FROM traces ORDER BY started_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("tracestore: list all traces: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var traces []*tracelib.Trace
	for rows.Next() {
		tr := &tracelib.Trace{}
		var status, errMsg string
		var durationNs int64
		var finishedAt sql.NullTime

		if err := rows.Scan(&tr.ID, &tr.SceneID, &tr.RunID, &status, &errMsg,
			&tr.StartedAt, &finishedAt, &durationNs); err != nil {
			return nil, err
		}

		tr.Status = tracelib.SpanStatus(status)
		tr.Error = errMsg
		tr.Duration = time.Duration(durationNs)
		if finishedAt.Valid {
			tr.FinishedAt = finishedAt.Time
		}

		spans, err := s.listSpans(ctx, tr.ID)
		if err != nil {
			return nil, err
		}
		tr.Spans = spans

		traces = append(traces, tr)
	}

	return traces, rows.Err()
}
