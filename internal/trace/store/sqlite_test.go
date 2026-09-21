package tracestore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/store/migration"
	"github.com/yannick2025-tech/Salvo/internal/store/sqlite"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := sqlite.Open(t.TempDir()+"/test.db", 1)
	require.NoError(t, err)
	require.NoError(t, migration.Migrate(db.DB))
	return New(db.DB)
}

func insertScene(t *testing.T, s *Store, id int64, name string) {
	t.Helper()
	_, err := s.db.Exec(`INSERT INTO scenes (id, name, status, created_at, updated_at) VALUES (?,?,?,?,?)`,
		id, name, "draft", time.Now(), time.Now())
	require.NoError(t, err)
}

func insertTrace(t *testing.T, s *Store, id, sceneID int64, status string, durMs float64) {
	t.Helper()
	_, err := s.db.Exec(`INSERT INTO traces (id, scene_id, run_id, status, error, started_at, duration_ns) VALUES (?,?,?,?,?,?,?)`,
		id, sceneID, id, status, "", time.Now(), int64(durMs*1e6))
	require.NoError(t, err)
}

func seedTraceFilters(t *testing.T, s *Store) {
	insertScene(t, s, 101, "电商下单场景")
	insertScene(t, s, 102, "支付链路")
	insertTrace(t, s, 1001, 101, "ok", 1200)
	insertTrace(t, s, 1002, 101, "error", 5500)
	insertTrace(t, s, 1003, 102, "ok", 300)
	insertTrace(t, s, 1004, 102, "canceled", 4000)
}

func TestListTraces_NoFilter(t *testing.T) {
	s := newTestStore(t)
	seedTraceFilters(t, s)
	traces, err := s.ListTraces(context.Background(), TraceFilter{}, 50, 0)
	require.NoError(t, err)
	assert.Len(t, traces, 4)
}

func TestListTraces_FilterByTraceID(t *testing.T) {
	s := newTestStore(t)
	seedTraceFilters(t, s)
	traces, err := s.ListTraces(context.Background(), TraceFilter{TraceID: "1002"}, 50, 0)
	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.EqualValues(t, 1002, traces[0].ID)
}

func TestListTraces_FilterBySceneNameFuzzy(t *testing.T) {
	s := newTestStore(t)
	seedTraceFilters(t, s)
	traces, err := s.ListTraces(context.Background(), TraceFilter{SceneName: "下单"}, 50, 0)
	require.NoError(t, err)
	assert.Len(t, traces, 2) // 1001 + 1002

	// LIKE wildcards in user input must be escaped literally.
	traces, err = s.ListTraces(context.Background(), TraceFilter{SceneName: "%下单"}, 50, 0)
	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestListTraces_FilterByStatus(t *testing.T) {
	s := newTestStore(t)
	seedTraceFilters(t, s)
	traces, err := s.ListTraces(context.Background(), TraceFilter{Status: "error"}, 50, 0)
	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.EqualValues(t, 1002, traces[0].ID)
}

func TestListTraces_FilterByDurationRange(t *testing.T) {
	s := newTestStore(t)
	seedTraceFilters(t, s)

	// 1s-5s: 1001 (1.2s), 1004 (4.0s). 1002 (5.5s) excluded, 1003 (0.3s) excluded.
	traces, err := s.ListTraces(context.Background(), TraceFilter{
		MinDuration: 1000 * time.Millisecond,
		MaxDuration: 5000 * time.Millisecond,
	}, 50, 0)
	require.NoError(t, err)
	assert.Len(t, traces, 2)

	// Boundary: min/max are inclusive (1001 is exactly 1.2s).
	traces, err = s.ListTraces(context.Background(), TraceFilter{
		MinDuration: 1200 * time.Millisecond,
		MaxDuration: 4000 * time.Millisecond,
	}, 50, 0)
	require.NoError(t, err)
	assert.Len(t, traces, 2)

	// Open-ended: only min (>4s) → 1002, 1004.
	traces, err = s.ListTraces(context.Background(), TraceFilter{
		MinDuration: 4000 * time.Millisecond,
	}, 50, 0)
	require.NoError(t, err)
	assert.Len(t, traces, 2)

	// Open-ended: only max (<1s) → 1003.
	traces, err = s.ListTraces(context.Background(), TraceFilter{
		MaxDuration: 1000 * time.Millisecond,
	}, 50, 0)
	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.EqualValues(t, 1003, traces[0].ID)
}

func TestListTraces_CombinedFilter(t *testing.T) {
	s := newTestStore(t)
	seedTraceFilters(t, s)
	// Scene fuzzy + status AND combination → only 1003.
	traces, err := s.ListTraces(context.Background(), TraceFilter{
		SceneName: "链路",
		Status:    "ok",
	}, 50, 0)
	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.EqualValues(t, 1003, traces[0].ID)
}

func TestCountTraces_Filter(t *testing.T) {
	s := newTestStore(t)
	seedTraceFilters(t, s)
	total, err := s.CountTraces(context.Background(), TraceFilter{Status: "ok"})
	require.NoError(t, err)
	assert.Equal(t, 2, total)

	total, err = s.CountTraces(context.Background(), TraceFilter{})
	require.NoError(t, err)
	assert.Equal(t, 4, total)
}
