package runner

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
)

// TimeSeriesRecord represents a single time-series metric sample for database storage.
type TimeSeriesRecord struct {
	RunID          snowflake.ID
	NodeID         string
	SampleTime     time.Time
	WindowDuration int

	QPS           float64
	TotalRequests int64
	SuccessCount  int64
	FailCount     int64

	AvgLatencyMs float64
	P50LatencyMs float64
	P90LatencyMs float64
	P95LatencyMs float64
	P99LatencyMs float64
	MinLatencyMs float64
	MaxLatencyMs float64
}

// TimeSeriesStore defines the interface for time-series data persistence.
type TimeSeriesStore interface {
	BatchInsert(ctx context.Context, records []TimeSeriesRecord) error
	QueryByRunID(ctx context.Context, runID snowflake.ID) ([]TimeSeriesRecord, error)
	QueryByNodeID(ctx context.Context, runID snowflake.ID, nodeID string) ([]TimeSeriesRecord, error)
	DeleteByRunID(ctx context.Context, runID snowflake.ID) error
	Close() error
}

// SQLiteTimeSeriesStore implements TimeSeriesStore using SQLite.
type SQLiteTimeSeriesStore struct {
	db   *sql.DB
	mu   sync.RWMutex
}

// NewSQLiteTimeSeriesStore creates a new SQLite-backed time series store.
func NewSQLiteTimeSeriesStore(db *sql.DB) *SQLiteTimeSeriesStore {
	return &SQLiteTimeSeriesStore{db: db}
}

// BatchInsert inserts multiple time-series records in a single transaction.
func (s *SQLiteTimeSeriesStore) BatchInsert(ctx context.Context, records []TimeSeriesRecord) error {
	if len(records) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO time_series_samples
			(run_id, node_id, sample_time, window_duration,
			 qps, total_requests, success_count, fail_count,
			 avg_latency_ms, p50_latency_ms, p90_latency_ms, p95_latency_ms, p99_latency_ms,
			 min_latency_ms, max_latency_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, r := range records {
		_, err := stmt.ExecContext(ctx,
			r.RunID,
			r.NodeID,
			r.SampleTime.UTC(),
			r.WindowDuration,
			r.QPS,
			r.TotalRequests,
			r.SuccessCount,
			r.FailCount,
			r.AvgLatencyMs,
			r.P50LatencyMs,
			r.P90LatencyMs,
			r.P95LatencyMs,
			r.P99LatencyMs,
			r.MinLatencyMs,
			r.MaxLatencyMs,
		)
		if err != nil {
			return fmt.Errorf("insert record: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// QueryByRunID retrieves all time-series samples for a given run.
func (s *SQLiteTimeSeriesStore) QueryByRunID(ctx context.Context, runID snowflake.ID) ([]TimeSeriesRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, `
		SELECT run_id, node_id, sample_time, window_duration,
			   qps, total_requests, success_count, fail_count,
			   avg_latency_ms, p50_latency_ms, p90_latency_ms, p95_latency_ms, p99_latency_ms,
			   min_latency_ms, max_latency_ms
		FROM time_series_samples
		WHERE run_id = ?
		ORDER BY sample_time ASC
	`, runID)
	if err != nil {
		return nil, fmt.Errorf("query by run_id: %w", err)
	}
	defer rows.Close()

	return scanTimeSeriesRows(rows)
}

// QueryByNodeID retrieves all time-series samples for a specific node within a run.
func (s *SQLiteTimeSeriesStore) QueryByNodeID(ctx context.Context, runID snowflake.ID, nodeID string) ([]TimeSeriesRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, `
		SELECT run_id, node_id, sample_time, window_duration,
			   qps, total_requests, success_count, fail_count,
			   avg_latency_ms, p50_latency_ms, p90_latency_ms, p95_latency_ms, p99_latency_ms,
			   min_latency_ms, max_latency_ms
		FROM time_series_samples
		WHERE run_id = ? AND node_id = ?
		ORDER BY sample_time ASC
	`, runID, nodeID)
	if err != nil {
		return nil, fmt.Errorf("query by node_id: %w", err)
	}
	defer rows.Close()

	return scanTimeSeriesRows(rows)
}

// DeleteByRunID removes all time-series samples for a given run.
func (s *SQLiteTimeSeriesStore) DeleteByRunID(ctx context.Context, runID snowflake.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.ExecContext(ctx, `DELETE FROM time_series_samples WHERE run_id = ?`, runID)
	if err != nil {
		return fmt.Errorf("delete by run_id: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		fmt.Printf("deleted %d time-series samples for run %d\n", rowsAffected, runID)
	}

	return nil
}

// Close releases resources held by the store.
func (s *SQLiteTimeSeriesStore) Close() error {
	return nil
}

func scanTimeSeriesRows(rows *sql.Rows) ([]TimeSeriesRecord, error) {
	var records []TimeSeriesRecord

	for rows.Next() {
		var r TimeSeriesRecord
		err := rows.Scan(
			&r.RunID,
			&r.NodeID,
			&r.SampleTime,
			&r.WindowDuration,
			&r.QPS,
			&r.TotalRequests,
			&r.SuccessCount,
			&r.FailCount,
			&r.AvgLatencyMs,
			&r.P50LatencyMs,
			&r.P90LatencyMs,
			&r.P95LatencyMs,
			&r.P99LatencyMs,
			&r.MinLatencyMs,
			&r.MaxLatencyMs,
		)
		if err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		records = append(records, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return records, nil
}
