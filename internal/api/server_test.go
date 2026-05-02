package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/api/dto"
	"github.com/yannick2025-tech/Salvo/internal/auth"
	"github.com/yannick2025-tech/Salvo/internal/logger"
	"github.com/yannick2025-tech/Salvo/internal/store/migration"
	"github.com/yannick2025-tech/Salvo/internal/store/sqlite"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sqlite.Open(dbPath, 1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	err = migration.Migrate(db.DB)
	require.NoError(t, err)

	log, err := logger.New(logger.Config{Level: "error"})
	require.NoError(t, err)

	jwtManager := auth.NewJWTManager("test-secret", 24*60*60*1e9)

	users := sqlite.NewUserRepo(db)
	roles := sqlite.NewRoleRepo(db)
	perms := sqlite.NewPermissionRepo(db)
	rp := sqlite.NewRolePermissionRepo(db)

	rbacChecker := auth.NewRBACChecker(perms, rp)

	seedCfg := auth.DefaultSeedConfig()
	seed := auth.NewSeeders(users, roles, perms, rp, seedCfg)
	require.NoError(t, seed.Seed(context.Background()))

	return New(Config{
		Addr:   ":0",
		DB:     db,
		Logger: log,
		JWT:    jwtManager,
		RBAC:   rbacChecker,
	})
}

func getAdminToken(t *testing.T, srv *Server) string {
	t.Helper()
	resp := postJSON(t, srv, "/api/v1/auth/login", dto.LoginRequest{
		Email:    "admin@salvo.local",
		Password: "admin",
	})
	result := decodeResponse(t, resp)
	require.Equal(t, 0, result.Code)

	loginData, err := json.Marshal(result.Data)
	require.NoError(t, err)

	var loginResp dto.LoginResponse
	require.NoError(t, json.Unmarshal(loginData, &loginResp))
	return loginResp.Token
}

func postJSON(t *testing.T, srv *Server, path string, body any) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(w, req)
	return w.Result()
}

func postJSONAuth(t *testing.T, srv *Server, token, path string, body any) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(w, req)
	return w.Result()
}

func decodeResponse(t *testing.T, resp *http.Response) dto.Response {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	_ = resp.Body.Close()

	var result dto.Response
	require.NoError(t, json.Unmarshal(body, &result))
	return result
}

func TestLogin(t *testing.T) {
	srv := newTestServer(t)

	resp := postJSON(t, srv, "/api/v1/auth/login", dto.LoginRequest{
		Email:    "admin@salvo.local",
		Password: "admin",
	})
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	loginData, err := json.Marshal(result.Data)
	require.NoError(t, err)

	var loginResp dto.LoginResponse
	require.NoError(t, json.Unmarshal(loginData, &loginResp))
	assert.NotEmpty(t, loginResp.Token)
	assert.Equal(t, "admin@salvo.local", loginResp.User.Email)
}

func TestLoginInvalidPassword(t *testing.T) {
	srv := newTestServer(t)

	resp := postJSON(t, srv, "/api/v1/auth/login", dto.LoginRequest{
		Email:    "admin@salvo.local",
		Password: "wrong",
	})
	result := decodeResponse(t, resp)
	assert.Equal(t, 401, result.Code)
}

func TestUnauthorizedAccess(t *testing.T) {
	srv := newTestServer(t)

	resp := postJSON(t, srv, "/api/v1/scenes/list", dto.ListScenesRequest{})
	result := decodeResponse(t, resp)
	assert.Equal(t, 401, result.Code)
}

func TestCreateAndGetScene(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/create", dto.CreateSceneRequest{
		Name:        "login-test",
		Description: "Login perf test",
		Status:      "draft",
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	sceneData, err := json.Marshal(result.Data)
	require.NoError(t, err)

	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData, &scene))
	assert.Equal(t, "login-test", scene.Name)
	assert.Equal(t, "draft", scene.Status)
	assert.NotZero(t, scene.ID)

	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/get", dto.IDRequest{ID: scene.ID})
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
}

func TestGetSceneNotFound(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/get", dto.IDRequest{ID: 999999})
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := decodeResponse(t, resp)
	assert.Equal(t, 404, result.Code)
}

func TestUpdateScene(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/create", dto.CreateSceneRequest{
		Name:   "original",
		Status: "draft",
	})
	result := decodeResponse(t, resp)
	sceneData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData, &scene))

	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/update", dto.UpdateSceneRequest{
		ID:     scene.ID,
		Name:   "updated",
		Status: "ready",
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	updatedData, _ := json.Marshal(result.Data)
	var updated dto.SceneDTO
	require.NoError(t, json.Unmarshal(updatedData, &updated))
	assert.Equal(t, "updated", updated.Name)
	assert.Equal(t, "ready", updated.Status)
}

func TestDeleteScene(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/create", dto.CreateSceneRequest{
		Name: "to-delete", Status: "draft",
	})
	result := decodeResponse(t, resp)
	sceneData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData, &scene))

	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/delete", dto.IDRequest{ID: scene.ID})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/get", dto.IDRequest{ID: scene.ID})
	result = decodeResponse(t, resp)
	assert.Equal(t, 404, result.Code)
}

func TestListScenes(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	for i := 0; i < 3; i++ {
		postJSONAuth(t, srv, token, "/api/v1/scenes/create", dto.CreateSceneRequest{
			Name: "scene-" + string(rune('A'+i)), Status: "draft",
		})
	}

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/list", dto.ListScenesRequest{Limit: 10})
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
}

func TestCreateSceneValidation(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/create", dto.CreateSceneRequest{})
	result := decodeResponse(t, resp)
	assert.Equal(t, 400, result.Code)
	assert.Contains(t, result.Message, "name is required")
}

func TestAddAndListNode(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/create", dto.CreateSceneRequest{
		Name: "node-test", Status: "draft",
	})
	result := decodeResponse(t, resp)
	sceneData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData, &scene))

	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/nodes/add", dto.AddNodeRequest{
		SceneID: scene.ID,
		Name:    "Login",
		Type:    "http",
		Config:  `{"url":"/api/login"}`,
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/nodes/list", dto.ListNodesRequest{
		SceneID: scene.ID,
		Limit:   10,
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
}

func TestUpdateNode(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/create", dto.CreateSceneRequest{
		Name: "node-update-test", Status: "draft",
	})
	result := decodeResponse(t, resp)
	sceneData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData, &scene))

	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/nodes/add", dto.AddNodeRequest{
		SceneID: scene.ID, Name: "Old", Type: "http",
	})
	result = decodeResponse(t, resp)
	nodeData, _ := json.Marshal(result.Data)
	var node dto.NodeDTO
	require.NoError(t, json.Unmarshal(nodeData, &node))

	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/nodes/update", dto.UpdateNodeRequest{
		ID:   node.ID,
		Name: "New",
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
}

func TestDeleteNode(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/create", dto.CreateSceneRequest{
		Name: "node-delete-test", Status: "draft",
	})
	result := decodeResponse(t, resp)
	sceneData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData, &scene))

	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/nodes/add", dto.AddNodeRequest{
		SceneID: scene.ID, Name: "ToDelete", Type: "http",
	})
	result = decodeResponse(t, resp)
	nodeData, _ := json.Marshal(result.Data)
	var node dto.NodeDTO
	require.NoError(t, json.Unmarshal(nodeData, &node))

	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/nodes/delete", dto.DeleteNodeRequest{ID: node.ID})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
}

func TestAddAndDeleteEdge(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/create", dto.CreateSceneRequest{
		Name: "edge-test", Status: "draft",
	})
	result := decodeResponse(t, resp)
	sceneData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData, &scene))

	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/nodes/add", dto.AddNodeRequest{
		SceneID: scene.ID, Name: "A", Type: "http",
	})
	result = decodeResponse(t, resp)
	nodeDataA, _ := json.Marshal(result.Data)
	var nodeA dto.NodeDTO
	require.NoError(t, json.Unmarshal(nodeDataA, &nodeA))

	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/nodes/add", dto.AddNodeRequest{
		SceneID: scene.ID, Name: "B", Type: "delay",
	})
	result = decodeResponse(t, resp)
	nodeDataB, _ := json.Marshal(result.Data)
	var nodeB dto.NodeDTO
	require.NoError(t, json.Unmarshal(nodeDataB, &nodeB))

	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/edges/add", dto.AddEdgeRequest{
		SceneID:  scene.ID,
		FromNode: nodeA.ID,
		ToNode:   nodeB.ID,
		Priority: 1,
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	edgeData, _ := json.Marshal(result.Data)
	var edge dto.EdgeDTO
	require.NoError(t, json.Unmarshal(edgeData, &edge))
	assert.Equal(t, nodeA.ID, edge.FromNode)
	assert.Equal(t, nodeB.ID, edge.ToNode)

	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/edges/delete", dto.DeleteEdgeRequest{ID: edge.ID})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
}

func TestSetAndListVariable(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/create", dto.CreateSceneRequest{
		Name: "var-test", Status: "draft",
	})
	result := decodeResponse(t, resp)
	sceneData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData, &scene))

	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/variables/set", dto.SetVariableRequest{
		SceneID: scene.ID,
		Scope:   "global",
		Key:     "base_url",
		Value:   "http://localhost:8080",
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/variables/list", dto.ListVariablesRequest{
		SceneID: scene.ID,
		Limit:   10,
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
}

func TestListAndUpdatePlugin(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/create", dto.CreateSceneRequest{
		Name: "plugin-test", Status: "draft",
	})
	result := decodeResponse(t, resp)
	sceneData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData, &scene))

	resp = postJSONAuth(t, srv, token, "/api/v1/plugins/list", dto.SceneIDRequest{SceneID: scene.ID})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
}

func TestListReports(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/reports/list", dto.ListReportsRequest{Limit: 10})
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
}

func TestGetReportNotFound(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/reports/get", dto.GetReportRequest{ID: 999999})
	result := decodeResponse(t, resp)
	assert.Equal(t, 404, result.Code)
}

func TestCORSPreflight(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/scenes/list", nil)
	w := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestInvalidJSON(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/scenes/create", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)

	resp := w.Result()
	result := decodeResponse(t, resp)
	assert.Equal(t, 400, result.Code)
}
