package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
	"github.com/yannick2025-tech/Salvo/internal/store/migration"
	"github.com/yannick2025-tech/Salvo/internal/store/model"
	"github.com/yannick2025-tech/Salvo/internal/store/repo"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath, 1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	err = migration.Migrate(db.DB)
	require.NoError(t, err)

	return db
}

func TestOpenAndMigrate(t *testing.T) {
	db := openTestDB(t)
	assert.NotNil(t, db)

	var version int
	err := db.QueryRow(`SELECT version FROM schema_version ORDER BY version DESC LIMIT 1`).Scan(&version)
	require.NoError(t, err)
	assert.Equal(t, migration.CurrentVersion(), version)
}

func TestOpenInvalidPath(t *testing.T) {
	_, err := Open("/nonexistent/dir/test.db", 1)
	assert.Error(t, err)
}

func TestSceneRepoCRUD(t *testing.T) {
	db := openTestDB(t)
	r := NewSceneRepo(db)
	ctx := context.Background()

	scene := &model.Scene{
		Name:        "login-test",
		Description: "Login performance test",
		DAGJSON:     `{"nodes":[{"id":"A","type":"http"}]}`,
		Status:      "draft",
	}

	err := r.Create(ctx, scene)
	require.NoError(t, err)
	assert.NotZero(t, scene.ID)
	assert.False(t, scene.CreatedAt.IsZero())

	found, err := r.GetByID(ctx, scene.ID)
	require.NoError(t, err)
	assert.Equal(t, scene.Name, found.Name)
	assert.Equal(t, scene.Description, found.Description)
	assert.Equal(t, scene.Status, found.Status)
	assert.Equal(t, scene.DAGJSON, found.DAGJSON)

	found.Name = "login-test-v2"
	found.Status = "ready"
	err = r.Update(ctx, found)
	require.NoError(t, err)

	updated, err := r.GetByID(ctx, scene.ID)
	require.NoError(t, err)
	assert.Equal(t, "login-test-v2", updated.Name)
	assert.Equal(t, "ready", updated.Status)
}

func TestSceneRepoDelete(t *testing.T) {
	db := openTestDB(t)
	r := NewSceneRepo(db)
	ctx := context.Background()

	scene := &model.Scene{Name: "to-delete", Status: "draft"}
	err := r.Create(ctx, scene)
	require.NoError(t, err)

	err = r.Delete(ctx, scene.ID)
	require.NoError(t, err)

	_, err = r.GetByID(ctx, scene.ID)
	assert.Equal(t, sql.ErrNoRows, err)
}

func TestSceneRepoList(t *testing.T) {
	db := openTestDB(t)
	r := NewSceneRepo(db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		s := &model.Scene{Name: "scene-" + string(rune('A'+i)), Status: "draft"}
		err := r.Create(ctx, s)
		require.NoError(t, err)
	}

	scenes, err := r.List(ctx, repo.Filter{Offset: 0, Limit: 3})
	require.NoError(t, err)
	assert.Len(t, scenes, 3)

	all, err := r.List(ctx, repo.Filter{Offset: 0, Limit: 10})
	require.NoError(t, err)
	assert.Len(t, all, 5)
}

func TestSceneRepoListByStatus(t *testing.T) {
	db := openTestDB(t)
	r := NewSceneRepo(db)
	ctx := context.Background()

	require.NoError(t, r.Create(ctx, &model.Scene{Name: "s1", Status: "draft"}))
	require.NoError(t, r.Create(ctx, &model.Scene{Name: "s2", Status: "ready"}))
	require.NoError(t, r.Create(ctx, &model.Scene{Name: "s3", Status: "draft"}))

	drafts, err := r.List(ctx, repo.Filter{Status: "draft", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, drafts, 2)

	ready, err := r.List(ctx, repo.Filter{Status: "ready", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, ready, 1)
}

func TestSceneRepoSoftDeleteFilter(t *testing.T) {
	db := openTestDB(t)
	r := NewSceneRepo(db)
	ctx := context.Background()

	require.NoError(t, r.Create(ctx, &model.Scene{Name: "active", Status: "draft"}))
	deleted := &model.Scene{Name: "deleted", Status: "draft"}
	require.NoError(t, r.Create(ctx, deleted))
	require.NoError(t, r.Delete(ctx, deleted.ID))

	all, err := r.List(ctx, repo.Filter{Limit: 10})
	require.NoError(t, err)
	assert.Len(t, all, 1)
	assert.Equal(t, "active", all[0].Name)
}

func TestNodeRepoCRUD(t *testing.T) {
	db := openTestDB(t)
	sr := NewSceneRepo(db)
	nr := NewNodeRepo(db)
	ctx := context.Background()

	scene := &model.Scene{Name: "node-test", Status: "draft"}
	require.NoError(t, sr.Create(ctx, scene))

	node := &model.Node{
		SceneID:   scene.ID,
		Name:      "Login",
		Type:      "http",
		Config:    `{"url":"/api/login"}`,
		LoopCount: 1,
	}
	err := nr.Create(ctx, node)
	require.NoError(t, err)
	assert.NotZero(t, node.ID)

	found, err := nr.GetByID(ctx, node.ID)
	require.NoError(t, err)
	assert.Equal(t, "Login", found.Name)
	assert.Equal(t, "http", found.Type)

	found.Name = "LoginV2"
	err = nr.Update(ctx, found)
	require.NoError(t, err)

	updated, err := nr.GetByID(ctx, node.ID)
	require.NoError(t, err)
	assert.Equal(t, "LoginV2", updated.Name)
}

func TestNodeRepoListByScene(t *testing.T) {
	db := openTestDB(t)
	sr := NewSceneRepo(db)
	nr := NewNodeRepo(db)
	ctx := context.Background()

	s1 := &model.Scene{Name: "scene1", Status: "draft"}
	require.NoError(t, sr.Create(ctx, s1))
	s2 := &model.Scene{Name: "scene2", Status: "draft"}
	require.NoError(t, sr.Create(ctx, s2))

	require.NoError(t, nr.Create(ctx, &model.Node{SceneID: s1.ID, Name: "A", Type: "http"}))
	require.NoError(t, nr.Create(ctx, &model.Node{SceneID: s1.ID, Name: "B", Type: "delay"}))
	require.NoError(t, nr.Create(ctx, &model.Node{SceneID: s2.ID, Name: "C", Type: "http"}))

	nodes, err := nr.List(ctx, repo.Filter{SceneID: s1.ID, Limit: 10})
	require.NoError(t, err)
	assert.Len(t, nodes, 2)
}

func TestEdgeRepoCRUD(t *testing.T) {
	db := openTestDB(t)
	sr := NewSceneRepo(db)
	nr := NewNodeRepo(db)
	er := NewEdgeRepo(db)
	ctx := context.Background()

	scene := &model.Scene{Name: "edge-test", Status: "draft"}
	require.NoError(t, sr.Create(ctx, scene))

	n1 := &model.Node{SceneID: scene.ID, Name: "A", Type: "http"}
	require.NoError(t, nr.Create(ctx, n1))
	n2 := &model.Node{SceneID: scene.ID, Name: "B", Type: "delay"}
	require.NoError(t, nr.Create(ctx, n2))

	edge := &model.Edge{
		SceneID:  scene.ID,
		FromNode: n1.ID,
		ToNode:   n2.ID,
		Priority: 1,
	}
	err := er.Create(ctx, edge)
	require.NoError(t, err)

	found, err := er.GetByID(ctx, edge.ID)
	require.NoError(t, err)
	assert.Equal(t, n1.ID, found.FromNode)
	assert.Equal(t, n2.ID, found.ToNode)
}

func TestVariableRepoCRUD(t *testing.T) {
	db := openTestDB(t)
	sr := NewSceneRepo(db)
	vr := NewVariableRepo(db)
	ctx := context.Background()

	scene := &model.Scene{Name: "var-test", Status: "draft"}
	require.NoError(t, sr.Create(ctx, scene))

	v := &model.Variable{
		SceneID: scene.ID,
		Scope:   "global",
		Key:     "base_url",
		Value:   "http://localhost:8080",
	}
	err := vr.Create(ctx, v)
	require.NoError(t, err)

	found, err := vr.GetByID(ctx, v.ID)
	require.NoError(t, err)
	assert.Equal(t, "base_url", found.Key)
	assert.Equal(t, "http://localhost:8080", found.Value)

	found.Value = "http://api.example.com"
	err = vr.Update(ctx, found)
	require.NoError(t, err)

	updated, err := vr.GetByID(ctx, v.ID)
	require.NoError(t, err)
	assert.Equal(t, "http://api.example.com", updated.Value)
}

func TestPluginConfigRepoCRUD(t *testing.T) {
	db := openTestDB(t)
	sr := NewSceneRepo(db)
	pr := NewPluginConfigRepo(db)
	ctx := context.Background()

	scene := &model.Scene{Name: "plugin-test", Status: "draft"}
	require.NoError(t, sr.Create(ctx, scene))

	pc := &model.PluginConfig{
		SceneID:  scene.ID,
		Name:     "rate-limiter",
		Type:     "ratelimiter",
		Config:   `{"rps":100}`,
		Phase:    "before",
		Priority: 10,
		Enabled:  true,
	}
	err := pr.Create(ctx, pc)
	require.NoError(t, err)

	found, err := pr.GetByID(ctx, pc.ID)
	require.NoError(t, err)
	assert.Equal(t, "rate-limiter", found.Name)
	assert.True(t, found.Enabled)
}

func TestReportRepoCRUD(t *testing.T) {
	db := openTestDB(t)
	sr := NewSceneRepo(db)
	rr := NewReportRepo(db)
	ctx := context.Background()

	scene := &model.Scene{Name: "report-test", Status: "draft"}
	require.NoError(t, sr.Create(ctx, scene))

	now := time.Now().UTC()
	report := &model.Report{
		SceneID:    scene.ID,
		RunID:      db.NextID(),
		Status:     "success",
		Summary:    "All passed",
		StartedAt:  &now,
		FinishedAt: &now,
	}
	err := rr.Create(ctx, report)
	require.NoError(t, err)

	found, err := rr.GetByID(ctx, report.ID)
	require.NoError(t, err)
	assert.Equal(t, "success", found.Status)
	assert.Equal(t, "All passed", found.Summary)
}

func TestRunRecordRepoCRUD(t *testing.T) {
	db := openTestDB(t)
	sr := NewSceneRepo(db)
	rr := NewRunRecordRepo(db)
	ctx := context.Background()

	scene := &model.Scene{Name: "run-test", Status: "draft"}
	require.NoError(t, sr.Create(ctx, scene))

	now := time.Now().UTC()
	rec := &model.RunRecord{
		SceneID:     scene.ID,
		Status:      "completed",
		WorkerCount: 20,
		RunMode:     "count",
		Duration:    30.5,
		TotalReqs:   10000,
		SuccessReqs: 9950,
		FailedReqs:  50,
		AvgLatency:  45.5,
		P50Latency:  40.0,
		P95Latency:  80.0,
		P99Latency:  120.0,
		StartedAt:   &now,
		FinishedAt:  &now,
	}
	err := rr.Create(ctx, rec)
	require.NoError(t, err)

	found, err := rr.GetByID(ctx, rec.ID)
	require.NoError(t, err)
	assert.Equal(t, "completed", found.Status)
	assert.Equal(t, int64(10000), found.TotalReqs)
	assert.InDelta(t, 45.5, found.AvgLatency, 0.01)
	assert.InDelta(t, 120.0, found.P99Latency, 0.01)
}

func TestRunRecordRepoListByStatus(t *testing.T) {
	db := openTestDB(t)
	sr := NewSceneRepo(db)
	rr := NewRunRecordRepo(db)
	ctx := context.Background()

	scene := &model.Scene{Name: "run-list", Status: "draft"}
	require.NoError(t, sr.Create(ctx, scene))

	require.NoError(t, rr.Create(ctx, &model.RunRecord{SceneID: scene.ID, Status: "completed", WorkerCount: 10}))
	require.NoError(t, rr.Create(ctx, &model.RunRecord{SceneID: scene.ID, Status: "failed", WorkerCount: 10}))
	require.NoError(t, rr.Create(ctx, &model.RunRecord{SceneID: scene.ID, Status: "completed", WorkerCount: 10}))

	completed, err := rr.List(ctx, repo.Filter{Status: "completed", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, completed, 2)

	failed, err := rr.List(ctx, repo.Filter{Status: "failed", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, failed, 1)
}

func TestFullSceneWorkflow(t *testing.T) {
	db := openTestDB(t)
	sr := NewSceneRepo(db)
	nr := NewNodeRepo(db)
	er := NewEdgeRepo(db)
	vr := NewVariableRepo(db)
	pr := NewPluginConfigRepo(db)
	ctx := context.Background()

	scene := &model.Scene{Name: "full-workflow", Description: "E2E test", Status: "draft"}
	require.NoError(t, sr.Create(ctx, scene))

	n1 := &model.Node{SceneID: scene.ID, Name: "Login", Type: "http", Config: `{"url":"/api/login"}`}
	n2 := &model.Node{SceneID: scene.ID, Name: "Orders", Type: "http", Config: `{"url":"/api/orders"}`}
	require.NoError(t, nr.Create(ctx, n1))
	require.NoError(t, nr.Create(ctx, n2))

	edge := &model.Edge{SceneID: scene.ID, FromNode: n1.ID, ToNode: n2.ID, Priority: 1}
	require.NoError(t, er.Create(ctx, edge))

	v := &model.Variable{SceneID: scene.ID, Scope: "global", Key: "token", Value: "abc123"}
	require.NoError(t, vr.Create(ctx, v))

	pc := &model.PluginConfig{SceneID: scene.ID, Name: "rate-limiter", Type: "ratelimiter", Config: `{"rps":100}`, Phase: "before", Priority: 10, Enabled: true}
	require.NoError(t, pr.Create(ctx, pc))

	scene.Status = "ready"
	require.NoError(t, sr.Update(ctx, scene))

	nodes, err := nr.List(ctx, repo.Filter{SceneID: scene.ID, Limit: 10})
	require.NoError(t, err)
	assert.Len(t, nodes, 2)

	edges, err := er.List(ctx, repo.Filter{SceneID: scene.ID, Limit: 10})
	require.NoError(t, err)
	assert.Len(t, edges, 1)

	vars, err := vr.List(ctx, repo.Filter{SceneID: scene.ID, Limit: 10})
	require.NoError(t, err)
	assert.Len(t, vars, 1)

	plugins, err := pr.List(ctx, repo.Filter{SceneID: scene.ID, Limit: 10})
	require.NoError(t, err)
	assert.Len(t, plugins, 1)

	require.NoError(t, sr.Delete(ctx, scene.ID))
	_, err = sr.GetByID(ctx, scene.ID)
	assert.Equal(t, sql.ErrNoRows, err)
}

func TestNextID(t *testing.T) {
	db := openTestDB(t)
	ids := make(map[snowflake.ID]bool)
	for i := 0; i < 100; i++ {
		id := db.NextID()
		assert.False(t, ids[id], "duplicate ID: %d", id)
		ids[id] = true
	}
}

func TestOpenWALMode(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "wal.db")

	db, err := Open(dbPath, 1)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	err = migration.Migrate(db.DB)
	require.NoError(t, err)

	var journalMode string
	err = db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	require.NoError(t, err)
	assert.Equal(t, "wal", journalMode)
}

// --- ReportRepo tests with report_details ---

func TestReportRepoCreateWithDetail(t *testing.T) {
	db := openTestDB(t)
	sr := NewSceneRepo(db)
	rr := NewReportRepo(db)
	ctx := context.Background()

	scene := &model.Scene{Name: "report-detail-test", Status: "draft"}
	require.NoError(t, sr.Create(ctx, scene))

	now := time.Now().UTC()
	detailJSON := `{"metrics":{"total":1000,"success":990}}`

	report := &model.Report{
		SceneID:    scene.ID,
		RunID:      db.NextID(),
		Status:     "success",
		Summary:    "All passed",
		Detail:     detailJSON,
		StartedAt:  &now,
		FinishedAt: &now,
	}
	err := rr.Create(ctx, report)
	require.NoError(t, err)
	assert.NotZero(t, report.ID)

	// 验证 reports 表和 report_details 表都有数据
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM reports WHERE id = ?`, report.ID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	err = db.QueryRow(`SELECT COUNT(*) FROM report_details WHERE report_id = ?`, report.ID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// 验证可以通过 GetByID 获取 detail
	found, err := rr.GetByID(ctx, report.ID)
	require.NoError(t, err)
	assert.Equal(t, detailJSON, found.Detail)
}

func TestReportRepoCreateWithoutDetail(t *testing.T) {
	db := openTestDB(t)
	sr := NewSceneRepo(db)
	rr := NewReportRepo(db)
	ctx := context.Background()

	scene := &model.Scene{Name: "report-no-detail-test", Status: "draft"}
	require.NoError(t, sr.Create(ctx, scene))

	now := time.Now().UTC()
	report := &model.Report{
		SceneID:    scene.ID,
		RunID:      db.NextID(),
		Status:     "success",
		Summary:    "All passed",
		Detail:     "", // 空 detail
		StartedAt:  &now,
		FinishedAt: &now,
	}
	err := rr.Create(ctx, report)
	require.NoError(t, err)

	// 验证 report_details 表没有数据（因为 detail 为空）
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM report_details WHERE report_id = ?`, report.ID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// 验证 GetByID 返回空 detail
	found, err := rr.GetByID(ctx, report.ID)
	require.NoError(t, err)
	assert.Equal(t, "", found.Detail)
}

func TestReportRepoGetByIDWithDetail(t *testing.T) {
	db := openTestDB(t)
	sr := NewSceneRepo(db)
	rr := NewReportRepo(db)
	ctx := context.Background()

	scene := &model.Scene{Name: "get-report-test", Status: "draft"}
	require.NoError(t, sr.Create(ctx, scene))

	now := time.Now().UTC()
	detailJSON := `{"nodes":[{"id":"A","success":true}]}`

	report := &model.Report{
		SceneID:    scene.ID,
		RunID:      db.NextID(),
		Status:     "success",
		Summary:    "Test summary",
		Detail:     detailJSON,
		StartedAt:  &now,
		FinishedAt: &now,
	}
	require.NoError(t, rr.Create(ctx, report))

	// 测试 GetByID
	found, err := rr.GetByID(ctx, report.ID)
	require.NoError(t, err)
	assert.Equal(t, report.ID, found.ID)
	assert.Equal(t, report.SceneID, found.SceneID)
	assert.Equal(t, report.RunID, found.RunID)
	assert.Equal(t, "success", found.Status)
	assert.Equal(t, "Test summary", found.Summary)
	assert.Equal(t, detailJSON, found.Detail)
}

func TestReportRepoGetByRunIDWithDetail(t *testing.T) {
	db := openTestDB(t)
	sr := NewSceneRepo(db)
	rr := NewReportRepo(db)
	ctx := context.Background()

	scene := &model.Scene{Name: "get-by-run-test", Status: "draft"}
	require.NoError(t, sr.Create(ctx, scene))

	now := time.Now().UTC()
	runID := db.NextID()
	detailJSON := `{"metrics":{"latency":45.5}}`

	report := &model.Report{
		SceneID:    scene.ID,
		RunID:      runID,
		Status:     "success",
		Summary:    "Run summary",
		Detail:     detailJSON,
		StartedAt:  &now,
		FinishedAt: &now,
	}
	require.NoError(t, rr.Create(ctx, report))

	// 测试 GetByRunID
	found, err := rr.GetByRunID(ctx, runID)
	require.NoError(t, err)
	assert.Equal(t, report.ID, found.ID)
	assert.Equal(t, runID, found.RunID)
	assert.Equal(t, detailJSON, found.Detail)
}

func TestReportRepoListWithoutDetail(t *testing.T) {
	db := openTestDB(t)
	sr := NewSceneRepo(db)
	rr := NewReportRepo(db)
	ctx := context.Background()

	scene := &model.Scene{Name: "list-report-test", Status: "draft"}
	require.NoError(t, sr.Create(ctx, scene))

	now := time.Now().UTC()
	// 创建多个报告，有的有 detail，有的没有
	for i := 0; i < 5; i++ {
		report := &model.Report{
			SceneID:    scene.ID,
			RunID:      db.NextID(),
			Status:     "success",
			Summary:    "Summary " + string(rune('A'+i)),
			Detail:     `{"data":"test"}`,
			StartedAt:  &now,
			FinishedAt: &now,
		}
		require.NoError(t, rr.Create(ctx, report))
	}

	// 测试 List 方法不返回 detail
	reports, err := rr.List(ctx, repo.Filter{SceneID: scene.ID, Limit: 10})
	require.NoError(t, err)
	assert.Len(t, reports, 5)

	for _, r := range reports {
		assert.Equal(t, "", r.Detail, "List method should not return detail field")
		assert.NotEmpty(t, r.Summary)
		assert.NotEmpty(t, r.Status)
	}
}

func TestReportRepoUpdateDetail(t *testing.T) {
	db := openTestDB(t)
	sr := NewSceneRepo(db)
	rr := NewReportRepo(db)
	ctx := context.Background()

	scene := &model.Scene{Name: "update-report-test", Status: "draft"}
	require.NoError(t, sr.Create(ctx, scene))

	now := time.Now().UTC()
	report := &model.Report{
		SceneID:    scene.ID,
		RunID:      db.NextID(),
		Status:     "success",
		Summary:    "Original summary",
		Detail:     `{"old":"data"}`,
		StartedAt:  &now,
		FinishedAt: &now,
	}
	require.NoError(t, rr.Create(ctx, report))

	// 更新报告，包括 detail
	report.Status = "failed"
	report.Summary = "Updated summary"
	report.Detail = `{"new":"data"}`
	require.NoError(t, rr.Update(ctx, report))

	// 验证更新后的数据
	found, err := rr.GetByID(ctx, report.ID)
	require.NoError(t, err)
	assert.Equal(t, "failed", found.Status)
	assert.Equal(t, "Updated summary", found.Summary)
	assert.Equal(t, `{"new":"data"}`, found.Detail)
}

func TestReportRepoUpdateAddDetail(t *testing.T) {
	db := openTestDB(t)
	sr := NewSceneRepo(db)
	rr := NewReportRepo(db)
	ctx := context.Background()

	scene := &model.Scene{Name: "add-detail-test", Status: "draft"}
	require.NoError(t, sr.Create(ctx, scene))

	now := time.Now().UTC()
	report := &model.Report{
		SceneID:    scene.ID,
		RunID:      db.NextID(),
		Status:     "success",
		Summary:    "No detail initially",
		Detail:     "", // 初始没有 detail
		StartedAt:  &now,
		FinishedAt: &now,
	}
	require.NoError(t, rr.Create(ctx, report))

	// 验证初始没有 detail
	found, err := rr.GetByID(ctx, report.ID)
	require.NoError(t, err)
	assert.Equal(t, "", found.Detail)

	// 更新，添加 detail
	report.Detail = `{"added":"detail"}`
	require.NoError(t, rr.Update(ctx, report))

	// 验证 detail 已添加
	found, err = rr.GetByID(ctx, report.ID)
	require.NoError(t, err)
	assert.Equal(t, `{"added":"detail"}`, found.Detail)
}

func TestReportRepoDeleteCascade(t *testing.T) {
	db := openTestDB(t)
	sr := NewSceneRepo(db)
	rr := NewReportRepo(db)
	ctx := context.Background()

	scene := &model.Scene{Name: "delete-cascade-test", Status: "draft"}
	require.NoError(t, sr.Create(ctx, scene))

	now := time.Now().UTC()
	report := &model.Report{
		SceneID:    scene.ID,
		RunID:      db.NextID(),
		Status:     "success",
		Summary:    "To be deleted",
		Detail:     `{"data":"will be deleted"}`,
		StartedAt:  &now,
		FinishedAt: &now,
	}
	require.NoError(t, rr.Create(ctx, report))

	// 验证数据存在
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM reports WHERE id = ? AND deleted_at IS NULL`, report.ID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	err = db.QueryRow(`SELECT COUNT(*) FROM report_details WHERE report_id = ?`, report.ID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// 删除报告
	require.NoError(t, rr.Delete(ctx, report.ID))

	// 验证 reports 表已被软删除
	err = db.QueryRow(`SELECT COUNT(*) FROM reports WHERE id = ? AND deleted_at IS NULL`, report.ID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "reports table should be soft-deleted")

	// 验证 report_details 表已被删除
	err = db.QueryRow(`SELECT COUNT(*) FROM report_details WHERE report_id = ?`, report.ID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "report_details should be deleted")

	// 验证 GetByID 返回错误
	_, err = rr.GetByID(ctx, report.ID)
	assert.Equal(t, sql.ErrNoRows, err)
}

// --- UserRepo tests ---

func TestUserRepoCRUD(t *testing.T) {
	db := openTestDB(t)
	ur := NewUserRepo(db)
	ctx := context.Background()

	user := &model.User{
		Email:        "test@example.com",
		PasswordHash: "$2a$10$hash",
		Nickname:     "Test User",
		RoleID:       snowflake.ID(100),
		Status:       "active",
	}
	err := ur.Create(ctx, user)
	require.NoError(t, err)
	assert.NotZero(t, user.ID)

	found, err := ur.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.Email, found.Email)
	assert.Equal(t, user.Nickname, found.Nickname)
	assert.Equal(t, user.RoleID, found.RoleID)
	assert.Equal(t, user.Status, found.Status)
}

func TestUserRepoGetByEmail(t *testing.T) {
	db := openTestDB(t)
	ur := NewUserRepo(db)
	ctx := context.Background()

	user := &model.User{
		Email:        "findme@example.com",
		PasswordHash: "$2a$10$hash",
		Nickname:     "Find Me",
		RoleID:       snowflake.ID(100),
		Status:       "active",
	}
	require.NoError(t, ur.Create(ctx, user))

	found, err := ur.GetByEmail(ctx, "findme@example.com")
	require.NoError(t, err)
	assert.Equal(t, user.ID, found.ID)
	assert.Equal(t, "Find Me", found.Nickname)

	// Not found
	_, err = ur.GetByEmail(ctx, "notfound@example.com")
	assert.Equal(t, sql.ErrNoRows, err)
}

func TestUserRepoList(t *testing.T) {
	db := openTestDB(t)
	ur := NewUserRepo(db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		u := &model.User{
			Email:        "user" + string(rune('A'+i)) + "@example.com",
			PasswordHash: "$2a$10$hash",
			Nickname:     "User " + string(rune('A'+i)),
			RoleID:       snowflake.ID(100),
			Status:       "active",
		}
		require.NoError(t, ur.Create(ctx, u))
	}

	users, err := ur.List(ctx, repo.Filter{Limit: 3})
	require.NoError(t, err)
	assert.Len(t, users, 3)

	all, err := ur.List(ctx, repo.Filter{Limit: 10})
	require.NoError(t, err)
	assert.Len(t, all, 5)
}

func TestUserRepoListByStatus(t *testing.T) {
	db := openTestDB(t)
	ur := NewUserRepo(db)
	ctx := context.Background()

	require.NoError(t, ur.Create(ctx, &model.User{Email: "a@example.com", PasswordHash: "h", Nickname: "A", RoleID: 1, Status: "active"}))
	require.NoError(t, ur.Create(ctx, &model.User{Email: "b@example.com", PasswordHash: "h", Nickname: "B", RoleID: 1, Status: "disabled"}))
	require.NoError(t, ur.Create(ctx, &model.User{Email: "c@example.com", PasswordHash: "h", Nickname: "C", RoleID: 1, Status: "active"}))

	active, err := ur.List(ctx, repo.Filter{Status: "active", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, active, 2)

	disabled, err := ur.List(ctx, repo.Filter{Status: "disabled", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, disabled, 1)
}

func TestUserRepoUpdate(t *testing.T) {
	db := openTestDB(t)
	ur := NewUserRepo(db)
	ctx := context.Background()

	user := &model.User{
		Email:        "update@example.com",
		PasswordHash: "$2a$10$hash",
		Nickname:     "Old Name",
		RoleID:       snowflake.ID(100),
		Status:       "active",
	}
	require.NoError(t, ur.Create(ctx, user))

	user.Nickname = "New Name"
	user.Status = "disabled"
	user.PasswordHash = "$2a$10$newhash"
	err := ur.Update(ctx, user)
	require.NoError(t, err)

	found, err := ur.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "New Name", found.Nickname)
	assert.Equal(t, "disabled", found.Status)
	assert.Equal(t, "$2a$10$newhash", found.PasswordHash)
}

func TestUserRepoDelete(t *testing.T) {
	db := openTestDB(t)
	ur := NewUserRepo(db)
	ctx := context.Background()

	user := &model.User{
		Email:        "delete@example.com",
		PasswordHash: "$2a$10$hash",
		Nickname:     "Delete Me",
		RoleID:       snowflake.ID(100),
		Status:       "active",
	}
	require.NoError(t, ur.Create(ctx, user))

	err := ur.Delete(ctx, user.ID)
	require.NoError(t, err)

	_, err = ur.GetByID(ctx, user.ID)
	assert.Equal(t, sql.ErrNoRows, err)
}

// --- RoleRepo tests ---

func TestRoleRepoCRUD(t *testing.T) {
	db := openTestDB(t)
	rr := NewRoleRepo(db)
	ctx := context.Background()

	role := &model.Role{
		Name:        "admin",
		Description: "Administrator role",
		IsBuiltin:   false,
	}
	err := rr.Create(ctx, role)
	require.NoError(t, err)
	assert.NotZero(t, role.ID)

	found, err := rr.GetByID(ctx, role.ID)
	require.NoError(t, err)
	assert.Equal(t, "admin", found.Name)
	assert.Equal(t, "Administrator role", found.Description)
	assert.False(t, found.IsBuiltin)
}

func TestRoleRepoGetByName(t *testing.T) {
	db := openTestDB(t)
	rr := NewRoleRepo(db)
	ctx := context.Background()

	role := &model.Role{Name: "tester", Description: "Tester role", IsBuiltin: true}
	require.NoError(t, rr.Create(ctx, role))

	found, err := rr.GetByName(ctx, "tester")
	require.NoError(t, err)
	assert.Equal(t, role.ID, found.ID)
	assert.True(t, found.IsBuiltin)

	_, err = rr.GetByName(ctx, "notfound")
	assert.Equal(t, sql.ErrNoRows, err)
}

func TestRoleRepoList(t *testing.T) {
	db := openTestDB(t)
	rr := NewRoleRepo(db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		r := &model.Role{Name: "role" + string(rune('A'+i)), Description: "Role " + string(rune('A'+i))}
		require.NoError(t, rr.Create(ctx, r))
	}

	roles, err := rr.List(ctx, repo.Filter{Limit: 3})
	require.NoError(t, err)
	assert.Len(t, roles, 3)

	all, err := rr.List(ctx, repo.Filter{Limit: 10})
	require.NoError(t, err)
	assert.Len(t, all, 5)
}

func TestRoleRepoUpdate(t *testing.T) {
	db := openTestDB(t)
	rr := NewRoleRepo(db)
	ctx := context.Background()

	role := &model.Role{Name: "old-name", Description: "Old description", IsBuiltin: false}
	require.NoError(t, rr.Create(ctx, role))

	role.Name = "new-name"
	role.Description = "New description"
	role.IsBuiltin = true
	err := rr.Update(ctx, role)
	require.NoError(t, err)

	found, err := rr.GetByID(ctx, role.ID)
	require.NoError(t, err)
	assert.Equal(t, "new-name", found.Name)
	assert.Equal(t, "New description", found.Description)
	assert.True(t, found.IsBuiltin)
}

func TestRoleRepoDelete(t *testing.T) {
	db := openTestDB(t)
	rr := NewRoleRepo(db)
	ctx := context.Background()

	role := &model.Role{Name: "to-delete", Description: "Delete me"}
	require.NoError(t, rr.Create(ctx, role))

	err := rr.Delete(ctx, role.ID)
	require.NoError(t, err)

	_, err = rr.GetByID(ctx, role.ID)
	assert.Equal(t, sql.ErrNoRows, err)
}

// --- PermissionRepo tests ---

func TestPermissionRepoCRUD(t *testing.T) {
	db := openTestDB(t)
	pr := NewPermissionRepo(db)
	ctx := context.Background()

	perm := &model.Permission{
		Resource:    "scene",
		Action:      "read",
		Description: "Read scene permission",
	}
	err := pr.Create(ctx, perm)
	require.NoError(t, err)
	assert.NotZero(t, perm.ID)

	found, err := pr.GetByID(ctx, perm.ID)
	require.NoError(t, err)
	assert.Equal(t, "scene", found.Resource)
	assert.Equal(t, "read", found.Action)
	assert.Equal(t, "Read scene permission", found.Description)
}

func TestPermissionRepoGetByResourceAction(t *testing.T) {
	db := openTestDB(t)
	pr := NewPermissionRepo(db)
	ctx := context.Background()

	perm := &model.Permission{Resource: "user", Action: "write", Description: "Write user"}
	require.NoError(t, pr.Create(ctx, perm))

	found, err := pr.GetByResourceAction(ctx, "user", "write")
	require.NoError(t, err)
	assert.Equal(t, perm.ID, found.ID)
	assert.Equal(t, "Write user", found.Description)

	_, err = pr.GetByResourceAction(ctx, "user", "delete")
	assert.Equal(t, sql.ErrNoRows, err)
}

func TestPermissionRepoList(t *testing.T) {
	db := openTestDB(t)
	pr := NewPermissionRepo(db)
	ctx := context.Background()

	require.NoError(t, pr.Create(ctx, &model.Permission{Resource: "scene", Action: "read"}))
	require.NoError(t, pr.Create(ctx, &model.Permission{Resource: "scene", Action: "write"}))
	require.NoError(t, pr.Create(ctx, &model.Permission{Resource: "user", Action: "read"}))

	perms, err := pr.List(ctx)
	require.NoError(t, err)
	assert.Len(t, perms, 3)
}

func TestPermissionRepoListByRoleID(t *testing.T) {
	db := openTestDB(t)
	pr := NewPermissionRepo(db)
	rpr := NewRolePermissionRepo(db)
	ctx := context.Background()

	perm1 := &model.Permission{Resource: "scene", Action: "read"}
	perm2 := &model.Permission{Resource: "scene", Action: "write"}
	require.NoError(t, pr.Create(ctx, perm1))
	require.NoError(t, pr.Create(ctx, perm2))

	roleID := snowflake.ID(100)
	require.NoError(t, rpr.Assign(ctx, roleID, perm1.ID))
	require.NoError(t, rpr.Assign(ctx, roleID, perm2.ID))

	perms, err := pr.ListByRoleID(ctx, roleID)
	require.NoError(t, err)
	assert.Len(t, perms, 2)
}

// --- RolePermissionRepo tests ---

func TestRolePermissionRepoAssign(t *testing.T) {
	db := openTestDB(t)
	pr := NewPermissionRepo(db)
	rpr := NewRolePermissionRepo(db)
	ctx := context.Background()

	perm := &model.Permission{Resource: "scene", Action: "read"}
	require.NoError(t, pr.Create(ctx, perm))

	roleID := snowflake.ID(100)
	err := rpr.Assign(ctx, roleID, perm.ID)
	require.NoError(t, err)

	perms, err := rpr.ListPermissions(ctx, roleID)
	require.NoError(t, err)
	assert.Len(t, perms, 1)
	assert.Equal(t, perm.ID, perms[0].ID)
}

func TestRolePermissionRepoRevoke(t *testing.T) {
	db := openTestDB(t)
	pr := NewPermissionRepo(db)
	rpr := NewRolePermissionRepo(db)
	ctx := context.Background()

	perm := &model.Permission{Resource: "scene", Action: "read"}
	require.NoError(t, pr.Create(ctx, perm))

	roleID := snowflake.ID(100)
	require.NoError(t, rpr.Assign(ctx, roleID, perm.ID))

	perms, _ := rpr.ListPermissions(ctx, roleID)
	assert.Len(t, perms, 1)

	err := rpr.Revoke(ctx, roleID, perm.ID)
	require.NoError(t, err)

	perms, _ = rpr.ListPermissions(ctx, roleID)
	assert.Len(t, perms, 0)
}

func TestRolePermissionRepoRevokeAll(t *testing.T) {
	db := openTestDB(t)
	pr := NewPermissionRepo(db)
	rpr := NewRolePermissionRepo(db)
	ctx := context.Background()

	perm1 := &model.Permission{Resource: "scene", Action: "read"}
	perm2 := &model.Permission{Resource: "scene", Action: "write"}
	require.NoError(t, pr.Create(ctx, perm1))
	require.NoError(t, pr.Create(ctx, perm2))

	roleID := snowflake.ID(100)
	require.NoError(t, rpr.Assign(ctx, roleID, perm1.ID))
	require.NoError(t, rpr.Assign(ctx, roleID, perm2.ID))

	perms, _ := rpr.ListPermissions(ctx, roleID)
	assert.Len(t, perms, 2)

	err := rpr.RevokeAll(ctx, roleID)
	require.NoError(t, err)

	perms, _ = rpr.ListPermissions(ctx, roleID)
	assert.Len(t, perms, 0)
}

// --- DataSourceRepo tests ---

func TestDataSourceRepoCRUD(t *testing.T) {
	db := openTestDB(t)
	sr := NewSceneRepo(db)
	dsr := NewDataSourceRepo(db)
	ctx := context.Background()

	scene := &model.Scene{Name: "ds-test", Status: "draft"}
	require.NoError(t, sr.Create(ctx, scene))

	ds := &model.DataSource{
		SceneID:  scene.ID,
		Name:     "users.csv",
		FileName: "users.csv",
		Columns:  `["id","name","email"]`,
		Rows:     `[["1","Alice","alice@example.com"]]`,
		RowCount: 1,
		Source:   "csv",
	}
	err := dsr.Create(ctx, ds)
	require.NoError(t, err)
	assert.NotZero(t, ds.ID)

	found, err := dsr.GetByID(ctx, ds.ID)
	require.NoError(t, err)
	assert.Equal(t, "users.csv", found.Name)
	assert.Equal(t, 1, found.RowCount)
	assert.Equal(t, "csv", found.Source)
}

func TestDataSourceRepoGetBySceneIDAndName(t *testing.T) {
	db := openTestDB(t)
	sr := NewSceneRepo(db)
	dsr := NewDataSourceRepo(db)
	ctx := context.Background()

	scene := &model.Scene{Name: "ds-find-test", Status: "draft"}
	require.NoError(t, sr.Create(ctx, scene))

	require.NoError(t, dsr.Create(ctx, &model.DataSource{SceneID: scene.ID, Name: "data.csv", FileName: "data.csv", Columns: "[]", Rows: "[]", RowCount: 0, Source: "csv"}))
	require.NoError(t, dsr.Create(ctx, &model.DataSource{SceneID: scene.ID, Name: "data.yaml", FileName: "data.yaml", Columns: "[]", Rows: "[]", RowCount: 0, Source: "yaml"}))

	results, err := dsr.GetBySceneIDAndName(ctx, scene.ID, "data.csv")
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "csv", results[0].Source)
}

func TestDataSourceRepoGetBySceneIDAndNameAndSource(t *testing.T) {
	db := openTestDB(t)
	sr := NewSceneRepo(db)
	dsr := NewDataSourceRepo(db)
	ctx := context.Background()

	scene := &model.Scene{Name: "ds-source-test", Status: "draft"}
	require.NoError(t, sr.Create(ctx, scene))

	require.NoError(t, dsr.Create(ctx, &model.DataSource{SceneID: scene.ID, Name: "data", FileName: "data.csv", Columns: "[]", Rows: "[]", RowCount: 0, Source: "csv"}))
	require.NoError(t, dsr.Create(ctx, &model.DataSource{SceneID: scene.ID, Name: "data", FileName: "data.yaml", Columns: "[]", Rows: "[]", RowCount: 0, Source: "yaml"}))

	csv, err := dsr.GetBySceneIDAndNameAndSource(ctx, scene.ID, "data", "csv")
	require.NoError(t, err)
	assert.Equal(t, "data.csv", csv.FileName)

	yaml, err := dsr.GetBySceneIDAndNameAndSource(ctx, scene.ID, "data", "yaml")
	require.NoError(t, err)
	assert.Equal(t, "data.yaml", yaml.FileName)
}

func TestDataSourceRepoListBySceneID(t *testing.T) {
	db := openTestDB(t)
	sr := NewSceneRepo(db)
	dsr := NewDataSourceRepo(db)
	ctx := context.Background()

	scene := &model.Scene{Name: "ds-list-test", Status: "draft"}
	require.NoError(t, sr.Create(ctx, scene))

	require.NoError(t, dsr.Create(ctx, &model.DataSource{SceneID: scene.ID, Name: "a.csv", FileName: "a.csv", Columns: "[]", Rows: "[]", RowCount: 0, Source: "csv"}))
	require.NoError(t, dsr.Create(ctx, &model.DataSource{SceneID: scene.ID, Name: "b.yaml", FileName: "b.yaml", Columns: "[]", Rows: "[]", RowCount: 0, Source: "yaml"}))

	sources, err := dsr.ListBySceneID(ctx, scene.ID)
	require.NoError(t, err)
	assert.Len(t, sources, 2)
}

func TestDataSourceRepoUpdate(t *testing.T) {
	db := openTestDB(t)
	sr := NewSceneRepo(db)
	dsr := NewDataSourceRepo(db)
	ctx := context.Background()

	scene := &model.Scene{Name: "ds-update-test", Status: "draft"}
	require.NoError(t, sr.Create(ctx, scene))

	ds := &model.DataSource{
		SceneID:  scene.ID,
		Name:     "update.csv",
		FileName: "update.csv",
		Columns:  `["id"]`,
		Rows:     `[]`,
		RowCount: 0,
		Source:   "csv",
	}
	require.NoError(t, dsr.Create(ctx, ds))

	ds.Columns = `["id","name"]`
	ds.Rows = `[["1","Alice"],["2","Bob"]]`
	ds.RowCount = 2
	err := dsr.Update(ctx, ds)
	require.NoError(t, err)

	found, err := dsr.GetByID(ctx, ds.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, found.RowCount)
	assert.Contains(t, found.Columns, "name")
}

func TestDataSourceRepoDelete(t *testing.T) {
	db := openTestDB(t)
	sr := NewSceneRepo(db)
	dsr := NewDataSourceRepo(db)
	ctx := context.Background()

	scene := &model.Scene{Name: "ds-delete-test", Status: "draft"}
	require.NoError(t, sr.Create(ctx, scene))

	ds := &model.DataSource{SceneID: scene.ID, Name: "delete.csv", FileName: "delete.csv", Columns: "[]", Rows: "[]", RowCount: 0, Source: "csv"}
	require.NoError(t, dsr.Create(ctx, ds))

	err := dsr.Delete(ctx, ds.ID)
	require.NoError(t, err)

	_, err = dsr.GetByID(ctx, ds.ID)
	assert.Equal(t, sql.ErrNoRows, err)
}

// --- SOPluginRepo tests ---

func TestSOPluginRepoCRUD(t *testing.T) {
	db := openTestDB(t)
	spr := NewSOPluginRepo(db)
	ctx := context.Background()

	plugin := &model.SOPlugin{
		Name:     "test-plugin",
		Version:  "1.0.0",
		FilePath: "/path/to/plugin.so",
		Status:   "enabled",
		Config:   `{"key":"value"}`,
	}
	err := spr.Create(ctx, plugin)
	require.NoError(t, err)
	assert.NotZero(t, plugin.ID)

	found, err := spr.GetByID(ctx, plugin.ID)
	require.NoError(t, err)
	assert.Equal(t, "test-plugin", found.Name)
	assert.Equal(t, "1.0.0", found.Version)
	assert.Equal(t, "enabled", found.Status)
}

func TestSOPluginRepoList(t *testing.T) {
	db := openTestDB(t)
	spr := NewSOPluginRepo(db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		p := &model.SOPlugin{
			Name:     "plugin-" + string(rune('A'+i)),
			Version:  "1.0.0",
			FilePath: "/path/to/plugin.so",
			Status:   "enabled",
		}
		require.NoError(t, spr.Create(ctx, p))
	}

	plugins, err := spr.List(ctx, repo.Filter{Limit: 3})
	require.NoError(t, err)
	assert.Len(t, plugins, 3)

	all, err := spr.List(ctx, repo.Filter{Limit: 10})
	require.NoError(t, err)
	assert.Len(t, all, 5)
}

func TestSOPluginRepoListByStatus(t *testing.T) {
	db := openTestDB(t)
	spr := NewSOPluginRepo(db)
	ctx := context.Background()

	require.NoError(t, spr.Create(ctx, &model.SOPlugin{Name: "p1", Version: "1.0.0", FilePath: "/p1.so", Status: "enabled"}))
	require.NoError(t, spr.Create(ctx, &model.SOPlugin{Name: "p2", Version: "1.0.0", FilePath: "/p2.so", Status: "disabled"}))
	require.NoError(t, spr.Create(ctx, &model.SOPlugin{Name: "p3", Version: "1.0.0", FilePath: "/p3.so", Status: "enabled"}))

	enabled, err := spr.List(ctx, repo.Filter{Status: "enabled", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, enabled, 2)

	disabled, err := spr.List(ctx, repo.Filter{Status: "disabled", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, disabled, 1)
}

func TestSOPluginRepoUpdateStatus(t *testing.T) {
	db := openTestDB(t)
	spr := NewSOPluginRepo(db)
	ctx := context.Background()

	plugin := &model.SOPlugin{Name: "status-test", Version: "1.0.0", FilePath: "/test.so", Status: "enabled"}
	require.NoError(t, spr.Create(ctx, plugin))

	err := spr.UpdateStatus(ctx, plugin.ID, "disabled")
	require.NoError(t, err)

	found, err := spr.GetByID(ctx, plugin.ID)
	require.NoError(t, err)
	assert.Equal(t, "disabled", found.Status)
}

func TestSOPluginRepoUpdateConfig(t *testing.T) {
	db := openTestDB(t)
	spr := NewSOPluginRepo(db)
	ctx := context.Background()

	plugin := &model.SOPlugin{Name: "config-test", Version: "1.0.0", FilePath: "/test.so", Status: "enabled", Config: `{"old":"value"}`}
	require.NoError(t, spr.Create(ctx, plugin))

	err := spr.UpdateConfig(ctx, plugin.ID, `{"new":"value"}`)
	require.NoError(t, err)

	found, err := spr.GetByID(ctx, plugin.ID)
	require.NoError(t, err)
	assert.Equal(t, `{"new":"value"}`, found.Config)
}

func TestSOPluginRepoDelete(t *testing.T) {
	db := openTestDB(t)
	spr := NewSOPluginRepo(db)
	ctx := context.Background()

	plugin := &model.SOPlugin{Name: "delete-test", Version: "1.0.0", FilePath: "/test.so", Status: "enabled"}
	require.NoError(t, spr.Create(ctx, plugin))

	err := spr.Delete(ctx, plugin.ID)
	require.NoError(t, err)

	_, err = spr.GetByID(ctx, plugin.ID)
	assert.Equal(t, sql.ErrNoRows, err)
}

func TestSOPluginRepoDedup(t *testing.T) {
	db := openTestDB(t)
	spr := NewSOPluginRepo(db)
	ctx := context.Background()

	// Create first version
	plugin1 := &model.SOPlugin{Name: "dedup-plugin", Version: "1.0.0", FilePath: "/v1.so", Status: "enabled"}
	require.NoError(t, spr.Create(ctx, plugin1))

	// Create same name+version - should soft-delete the old one
	plugin2 := &model.SOPlugin{Name: "dedup-plugin", Version: "1.0.0", FilePath: "/v2.so", Status: "enabled"}
	require.NoError(t, spr.Create(ctx, plugin2))

	// Only the new one should be active
	plugins, err := spr.List(ctx, repo.Filter{Limit: 10})
	require.NoError(t, err)
	assert.Len(t, plugins, 1)
	assert.Equal(t, "/v2.so", plugins[0].FilePath)
}
