package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/api/dto"
	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
	tracelib "github.com/yannick2025-tech/Salvo/internal/trace"
)

func tracesReq(t *testing.T, body any) *http.Request {
	t.Helper()
	data, err := json.Marshal(body)
	require.NoError(t, err)
	return httptest.NewRequest(http.MethodPost, "/traces/list", bytes.NewReader(data))
}

func seedTracesForFilter(t *testing.T, srv *Server) {
	t.Helper()
	// Two scenes so scene-name fuzzy filtering can be exercised.
	_, err := srv.db.DB.Exec(`INSERT INTO scenes (id, name, status, created_at, updated_at) VALUES (?,?,?,?,?)`,
		101, "电商下单场景", "draft", time.Now(), time.Now())
	require.NoError(t, err)
	_, err = srv.db.DB.Exec(`INSERT INTO scenes (id, name, status, created_at, updated_at) VALUES (?,?,?,?,?)`,
		102, "支付链路", "draft", time.Now(), time.Now())
	require.NoError(t, err)

	mk := func(id, sceneID int64, status string, dur time.Duration) *tracelib.Trace {
		return &tracelib.Trace{
			ID:        snowflake.ID(id),
			SceneID:   snowflake.ID(sceneID),
			RunID:     snowflake.ID(id),
			Status:    tracelib.SpanStatus(status),
			StartedAt: time.Now(),
			Duration:  dur,
		}
	}
	require.NoError(t, srv.handler.traceStore.SaveTrace(context.Background(), mk(1001, 101, "ok", 1200*time.Millisecond)))
	require.NoError(t, srv.handler.traceStore.SaveTrace(context.Background(), mk(1002, 101, "error", 5500*time.Millisecond)))
	require.NoError(t, srv.handler.traceStore.SaveTrace(context.Background(), mk(1003, 102, "ok", 300*time.Millisecond)))
}

func TestListTraces_FiltersCombined(t *testing.T) {
	srv := newTestServer(t)
	seedTracesForFilter(t, srv)

	// Scene fuzzy + status AND combination → only 1001.
	resp := srv.handler.ListTraces(tracesReq(t, map[string]any{
		"scene_name": "下单", "status": "ok",
	}))
	require.Equal(t, 0, resp.Code)
	data, ok := resp.Data.(dto.ListResponse[[]dto.TraceDTO])
	require.True(t, ok, "unexpected data type %T", resp.Data)
	require.Len(t, data.Items, 1)
	assert.EqualValues(t, 1001, data.Items[0].ID)
	assert.Equal(t, "电商下单场景", data.Items[0].SceneName)
}

func TestListTraces_TraceIDExact(t *testing.T) {
	srv := newTestServer(t)
	seedTracesForFilter(t, srv)

	resp := srv.handler.ListTraces(tracesReq(t, map[string]any{"trace_id": "1002"}))
	require.Equal(t, 0, resp.Code)
	data := resp.Data.(dto.ListResponse[[]dto.TraceDTO])
	require.Len(t, data.Items, 1)
	assert.EqualValues(t, 1002, data.Items[0].ID)
	assert.EqualValues(t, 1, data.Pagination.Total) // total reflects the filtered count
}

func TestListTraces_TraceIDMatchesRunID(t *testing.T) {
	srv := newTestServer(t)
	seedTracesForFilter(t, srv)

	// A trace whose run_id differs from its DB primary key.
	extra := &tracelib.Trace{
		ID:        snowflake.ID(1004),
		SceneID:   snowflake.ID(102),
		RunID:     snowflake.ID(9001),
		Status:    tracelib.SpanStatusOK,
		StartedAt: time.Now(),
		Duration:  800 * time.Millisecond,
	}
	require.NoError(t, srv.handler.traceStore.SaveTrace(context.Background(), extra))

	// The trace_id param smart-matches traces.id OR traces.run_id.
	resp := srv.handler.ListTraces(tracesReq(t, map[string]any{"trace_id": "9001"}))
	require.Equal(t, 0, resp.Code)
	data := resp.Data.(dto.ListResponse[[]dto.TraceDTO])
	require.Len(t, data.Items, 1)
	assert.EqualValues(t, 1004, data.Items[0].ID)
	assert.EqualValues(t, 9001, data.Items[0].RunID)
}

func TestListTraces_DurationRange(t *testing.T) {
	srv := newTestServer(t)
	seedTracesForFilter(t, srv)

	// 1s-5s with float ms values → 1001 only (1002 is 5.5s, 1003 is 0.3s).
	resp := srv.handler.ListTraces(tracesReq(t, map[string]any{
		"min_duration_ms": 1000.5, "max_duration_ms": 5000,
	}))
	require.Equal(t, 0, resp.Code)
	data := resp.Data.(dto.ListResponse[[]dto.TraceDTO])
	require.Len(t, data.Items, 1)
	assert.EqualValues(t, 1001, data.Items[0].ID)
}

func TestListTraces_InvalidStatus(t *testing.T) {
	srv := newTestServer(t)
	seedTracesForFilter(t, srv)

	resp := srv.handler.ListTraces(tracesReq(t, map[string]any{"status": "boom"}))
	assert.Equal(t, 400, resp.Code)
}

func TestListTraces_MinGreaterThanMax(t *testing.T) {
	srv := newTestServer(t)
	seedTracesForFilter(t, srv)

	resp := srv.handler.ListTraces(tracesReq(t, map[string]any{
		"min_duration_ms": 6000, "max_duration_ms": 4000,
	}))
	assert.Equal(t, 400, resp.Code)
	assert.Contains(t, resp.Message, "最小")
}

func TestListTraces_InvalidTraceID(t *testing.T) {
	srv := newTestServer(t)
	seedTracesForFilter(t, srv)

	resp := srv.handler.ListTraces(tracesReq(t, map[string]any{"trace_id": "not-a-number"}))
	assert.Equal(t, 400, resp.Code)
}
