package runner

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// testTimeSeriesStore wraps SQLiteTimeSeriesStore with cleanup logic for tests.
type testTimeSeriesStore struct {
	*SQLiteTimeSeriesStore
	db   *sql.DB
	path string
}

// Close removes the temporary database file.
func (s *testTimeSeriesStore) Close() error {
	if err := s.SQLiteTimeSeriesStore.Close(); err != nil {
		return err
	}
	if s.db != nil {
		s.db.Close()
	}
	if s.path != "" {
		os.Remove(s.path)
	}
	return nil
}

// newTestTimeSeriesStore creates a temporary SQLite database for testing.
func newTestTimeSeriesStore(t *testing.T) *testTimeSeriesStore {
	t.Helper()

	tmpfile, err := os.CreateTemp("", "timeseries-test-*.db")
	if err != nil {
		t.Fatalf("create temp db: %v", err)
	}
	tmpfile.Close()
	path := tmpfile.Name()

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		os.Remove(path)
		t.Fatalf("open sqlite: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS time_series_samples (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id          INTEGER    NOT NULL,
			node_id         TEXT       NOT NULL DEFAULT '',
			sample_time     DATETIME   NOT NULL,
			window_duration INTEGER    NOT NULL DEFAULT 1,
			qps             REAL       NOT NULL DEFAULT 0,
			total_requests  INTEGER    NOT NULL DEFAULT 0,
			success_count   INTEGER    NOT NULL DEFAULT 0,
			fail_count      INTEGER    NOT NULL DEFAULT 0,
			avg_latency_ms  REAL       NOT NULL DEFAULT 0,
			p50_latency_ms  REAL       NOT NULL DEFAULT 0,
			p95_latency_ms  REAL       NOT NULL DEFAULT 0,
			p99_latency_ms  REAL       NOT NULL DEFAULT 0,
			min_latency_ms  REAL       NOT NULL DEFAULT 0,
			max_latency_ms  REAL       NOT NULL DEFAULT 0,
			created_at      DATETIME   NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT uk_run_node_time UNIQUE (run_id, node_id, sample_time)
		);
	`)
	if err != nil {
		db.Close()
		os.Remove(path)
		t.Fatalf("create table: %v", err)
	}

	store := NewSQLiteTimeSeriesStore(db)

	return &testTimeSeriesStore{
		SQLiteTimeSeriesStore: store,
		db:                    db,
		path:                  path,
	}
}
