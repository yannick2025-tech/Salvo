package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/api/dto"
	"github.com/yannick2025-tech/Salvo/internal/core/expr"
	"github.com/yannick2025-tech/Salvo/internal/generator/builtin"
	"github.com/yannick2025-tech/Salvo/internal/plugin/so"
	"github.com/yannick2025-tech/Salvo/internal/store/migration"
	"github.com/yannick2025-tech/Salvo/internal/store/model"
	"github.com/yannick2025-tech/Salvo/internal/store/repo"
	"github.com/yannick2025-tech/Salvo/internal/store/sqlite"
)

// =============================================================================
// Section 13.5: SO plugin full-chain integration tests.
//
// These tests verify the complete chain: Upload API → DB persistence →
// Loader registration → Expression engine invocation.
// =============================================================================

// ---------------------------------------------------------------------------
// SO Plugin API CRUD
// ---------------------------------------------------------------------------

// TestSOPlugin_UploadAndGet verifies uploading an SO plugin via API and
// retrieving it.
func TestSOPlugin_UploadAndGet(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/so-plugins/create", dto.UploadSOPluginRequest{
		Name:     "shell-aes",
		Version:  "1.0.0",
		FilePath: "/tmp/plugins/shell-aes.so",
		Status:   model.SOPluginStatusEnabled,
		Config:   `{"key_size":256}`,
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	data, err := json.Marshal(result.Data)
	require.NoError(t, err)

	var plugin dto.SOPluginDTO
	require.NoError(t, json.Unmarshal(data, &plugin))
	assert.Equal(t, "shell-aes", plugin.Name)
	assert.Equal(t, "1.0.0", plugin.Version)
	assert.Equal(t, model.SOPluginStatusEnabled, plugin.Status)
	assert.Equal(t, `{"key_size":256}`, plugin.Config)
	assert.NotZero(t, plugin.ID)
	assert.NotZero(t, plugin.CreatedAt)

	t.Logf("uploaded plugin ID=%d", plugin.ID)

	// Directly verify via repo that the plugin exists.
	repoPlugin, err := srv.handler.soPlugins.GetByID(context.Background(), plugin.ID)
	if err != nil {
		t.Logf("repo GetByID error: %v", err)
	} else {
		t.Logf("repo found plugin: id=%d name=%s status=%s", repoPlugin.ID, repoPlugin.Name, repoPlugin.Status)
	}

	// Verify we can retrieve it via API.
	resp = postJSONAuth(t, srv, token, "/api/v1/so-plugins/get", dto.GetSOPluginRequest{ID: plugin.ID})
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result = decodeResponse(t, resp)
	t.Logf("Get response code=%d message=%s", result.Code, result.Message)
	assert.Equal(t, 0, result.Code)
}

// TestSOPlugin_UploadValidation verifies that the upload API rejects
// invalid inputs.
func TestSOPlugin_UploadValidation(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	tests := []struct {
		name    string
		request dto.UploadSOPluginRequest
	}{
		{"empty name", dto.UploadSOPluginRequest{Version: "1.0.0", FilePath: "/tmp/test.so"}},
		{"empty version", dto.UploadSOPluginRequest{Name: "test", FilePath: "/tmp/test.so"}},
		{"empty file_path", dto.UploadSOPluginRequest{Name: "test", Version: "1.0.0"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := postJSONAuth(t, srv, token, "/api/v1/so-plugins/create", tc.request)
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			result := decodeResponse(t, resp)
			assert.Equal(t, 400, result.Code)
		})
	}
}

// TestSOPlugin_List verifies listing SO plugins with status filtering.
func TestSOPlugin_List(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	// Upload two plugins.
	for _, p := range []dto.UploadSOPluginRequest{
		{Name: "plugin-a", Version: "1.0.0", FilePath: "/tmp/a.so", Status: model.SOPluginStatusEnabled},
		{Name: "plugin-b", Version: "1.0.0", FilePath: "/tmp/b.so", Status: model.SOPluginStatusDisabled},
	} {
		resp := postJSONAuth(t, srv, token, "/api/v1/so-plugins/create", p)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		result := decodeResponse(t, resp)
		assert.Equal(t, 0, result.Code)
	}

	// List all.
	resp := postJSONAuth(t, srv, token, "/api/v1/so-plugins/list", dto.ListSOPluginsRequest{})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	data, err := json.Marshal(result.Data)
	require.NoError(t, err)

	var listResp dto.ListResponse[[]dto.SOPluginDTO]
	require.NoError(t, json.Unmarshal(data, &listResp))
	assert.Equal(t, 2, listResp.Pagination.Total)

	// List only enabled.
	resp = postJSONAuth(t, srv, token, "/api/v1/so-plugins/list", dto.ListSOPluginsRequest{
		Status: model.SOPluginStatusEnabled,
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	data, err = json.Marshal(result.Data)
	require.NoError(t, err)

	var enabledResp dto.ListResponse[[]dto.SOPluginDTO]
	require.NoError(t, json.Unmarshal(data, &enabledResp))
	assert.Equal(t, 1, enabledResp.Pagination.Total)
	assert.Equal(t, "plugin-a", enabledResp.Items[0].Name)
}

// TestSOPlugin_UpdateStatus verifies enabling and disabling an SO plugin.
func TestSOPlugin_UpdateStatus(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	// Upload a disabled plugin.
	resp := postJSONAuth(t, srv, token, "/api/v1/so-plugins/create", dto.UploadSOPluginRequest{
		Name:     "test-plugin",
		Version:  "1.0.0",
		FilePath: "/tmp/test.so",
		Status:   model.SOPluginStatusDisabled,
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	data, _ := json.Marshal(result.Data)
	var plugin dto.SOPluginDTO
	json.Unmarshal(data, &plugin)
	assert.Equal(t, model.SOPluginStatusDisabled, plugin.Status)
	require.NotZero(t, plugin.ID, "plugin ID should be non-zero after upload")
	t.Logf("uploaded plugin ID=%d", plugin.ID)

	// Enable it.
	t.Logf("about to enable plugin ID=%d", plugin.ID)
	resp = postJSONAuth(t, srv, token, "/api/v1/so-plugins/status", dto.UpdateSOPluginStatusRequest{
		ID:     plugin.ID,
		Status: model.SOPluginStatusEnabled,
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	result = decodeResponse(t, resp)
	t.Logf("UpdateStatus response code=%d message=%s", result.Code, result.Message)
	assert.Equal(t, 0, result.Code)

	// Verify enabled.
	resp = postJSONAuth(t, srv, token, "/api/v1/so-plugins/get", dto.GetSOPluginRequest{ID: plugin.ID})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	result = decodeResponse(t, resp)
	t.Logf("Get after update response code=%d message=%s", result.Code, result.Message)
	if result.Code == 0 {
		data, _ = json.Marshal(result.Data)
		json.Unmarshal(data, &plugin)
		t.Logf("plugin status after update: %s", plugin.Status)
	}
	assert.Equal(t, model.SOPluginStatusEnabled, plugin.Status)

	// Disable it.
	resp = postJSONAuth(t, srv, token, "/api/v1/so-plugins/status", dto.UpdateSOPluginStatusRequest{
		ID:     plugin.ID,
		Status: model.SOPluginStatusDisabled,
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
}

// TestSOPlugin_UpdateStatus_Invalid verifies that invalid status values
// are rejected.
func TestSOPlugin_UpdateStatus_Invalid(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/so-plugins/status", dto.UpdateSOPluginStatusRequest{
		ID:     1,
		Status: "invalid_status",
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	result := decodeResponse(t, resp)
	assert.Equal(t, 400, result.Code)
}

// TestSOPlugin_UpdateConfig verifies updating an SO plugin's config.
func TestSOPlugin_UpdateConfig(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/so-plugins/create", dto.UploadSOPluginRequest{
		Name:     "configurable",
		Version:  "1.0.0",
		FilePath: "/tmp/config.so",
		Config:   `{"key":"old"}`,
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	data, _ := json.Marshal(result.Data)
	var plugin dto.SOPluginDTO
	json.Unmarshal(data, &plugin)

	// Update config.
	resp = postJSONAuth(t, srv, token, "/api/v1/so-plugins/config", dto.UpdateSOPluginConfigRequest{
		ID:     plugin.ID,
		Config: `{"key":"new","timeout":30}`,
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	// Verify new config.
	resp = postJSONAuth(t, srv, token, "/api/v1/so-plugins/get", dto.GetSOPluginRequest{ID: plugin.ID})
	result = decodeResponse(t, resp)
	data, _ = json.Marshal(result.Data)
	json.Unmarshal(data, &plugin)
	assert.Equal(t, `{"key":"new","timeout":30}`, plugin.Config)
}

// TestSOPlugin_Delete verifies soft-deleting an SO plugin.
func TestSOPlugin_Delete(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/so-plugins/create", dto.UploadSOPluginRequest{
		Name:     "to-delete",
		Version:  "1.0.0",
		FilePath: "/tmp/delete.so",
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	data, _ := json.Marshal(result.Data)
	var plugin dto.SOPluginDTO
	json.Unmarshal(data, &plugin)

	// Delete it.
	resp = postJSONAuth(t, srv, token, "/api/v1/so-plugins/delete", dto.DeleteSOPluginRequest{ID: plugin.ID})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	// Verify deleted (404).
	resp = postJSONAuth(t, srv, token, "/api/v1/so-plugins/get", dto.GetSOPluginRequest{ID: plugin.ID})
	result = decodeResponse(t, resp)
	assert.Equal(t, 404, result.Code)
}

// TestSOPlugin_GetNonExistent verifies 404 for non-existent plugins.
func TestSOPlugin_GetNonExistent(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/so-plugins/get", dto.GetSOPluginRequest{ID: 99999})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	result := decodeResponse(t, resp)
	assert.Equal(t, 404, result.Code)
}

// ---------------------------------------------------------------------------
// SO Plugin Repo CRUD
// ---------------------------------------------------------------------------

// TestSOPluginRepo_CRUD verifies direct SO plugin repository operations.
func TestSOPluginRepo_CRUD(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "so_test.db")
	db, err := sqlite.Open(dbPath, 1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	err = migration.Migrate(db.DB)
	require.NoError(t, err)

	ctx := context.Background()

	soRepo := sqlite.NewSOPluginRepo(db)

	// Create.
	p := &model.SOPlugin{
		Name:     "repo-test",
		Version:  "2.0.0",
		FilePath: "/tmp/repo-test.so",
		Status:   model.SOPluginStatusDisabled,
		Config:   `{"enabled":false}`,
	}
	err = soRepo.Create(ctx, p)
	require.NoError(t, err)
	assert.NotZero(t, p.ID)
	assert.NotZero(t, p.CreatedAt)

	// GetByID.
	found, err := soRepo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "repo-test", found.Name)
	assert.Equal(t, "2.0.0", found.Version)
	assert.Equal(t, model.SOPluginStatusDisabled, found.Status)

	// List.
	plugins, err := soRepo.List(ctx, repo.Filter{})
	require.NoError(t, err)
	assert.Len(t, plugins, 1)

	// List with status filter.
	disabled, err := soRepo.List(ctx, repo.Filter{Status: model.SOPluginStatusDisabled})
	require.NoError(t, err)
	assert.Len(t, disabled, 1)

	enabled, err := soRepo.List(ctx, repo.Filter{Status: model.SOPluginStatusEnabled})
	require.NoError(t, err)
	assert.Len(t, enabled, 0)

	// UpdateStatus.
	err = soRepo.UpdateStatus(ctx, p.ID, model.SOPluginStatusEnabled)
	require.NoError(t, err)

	found, err = soRepo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, model.SOPluginStatusEnabled, found.Status)

	// UpdateConfig.
	err = soRepo.UpdateConfig(ctx, p.ID, `{"enabled":true,"timeout":60}`)
	require.NoError(t, err)

	found, err = soRepo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, `{"enabled":true,"timeout":60}`, found.Config)

	// Delete (soft).
	err = soRepo.Delete(ctx, p.ID)
	require.NoError(t, err)

	_, err = soRepo.GetByID(ctx, p.ID)
	require.Error(t, err) // should be sql.ErrNoRows

	// Deleted should not appear in list.
	all, err := soRepo.List(ctx, repo.Filter{})
	require.NoError(t, err)
	assert.Len(t, all, 0)
}

// ---------------------------------------------------------------------------
// Full chain: Upload API → Expression engine call
// ---------------------------------------------------------------------------

// TestSOPlugin_FullChain simulates the complete chain:
// 1. Upload plugin record via API (persists to DB)
// 2. Manually register an in-memory plugin with the Loader
// 3. Register __so function in expression registry
// 4. Call the plugin via expression engine
//
// Note: Step 2 simulates what InitFromDB does (Load .so → Register),
// but uses Register() since we cannot compile .so files in a test.
func TestSOPlugin_FullChain(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	// Step 1: Upload plugin record via API.
	resp := postJSONAuth(t, srv, token, "/api/v1/so-plugins/create", dto.UploadSOPluginRequest{
		Name:     "shell-aes",
		Version:  "1.0.0",
		FilePath: "/tmp/plugins/shell-aes.so",
		Status:   model.SOPluginStatusEnabled,
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	data, err := json.Marshal(result.Data)
	require.NoError(t, err)
	var plugin dto.SOPluginDTO
	require.NoError(t, json.Unmarshal(data, &plugin))
	assert.Equal(t, "shell-aes", plugin.Name)

	// Step 2: Create expression registry with builtin functions.
	reg := expr.NewFunctionRegistry()
	builtin.RegisterAll(reg)

	// Step 3: Create loader and register an in-memory plugin
	// (simulating InitFromDB's Load step with an in-memory plugin).
	loader := so.NewLoader()
	aesPlugin := &testShellAESPlugin{}
	err = loader.Register(aesPlugin)
	require.NoError(t, err)

	// Step 4: Register __so function (same as InitFromDB does).
	err = so.RegisterSO(reg, loader)
	require.NoError(t, err)

	// Step 5: Call the plugin via expression engine.
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	iv := "MTIzNDU2Nzg5MDEyMzQ1Ng==" // base64 of "1234567890123456"
	plaintext := "sensitive-data"

	encExpr := fmt.Sprintf(`${__so("shell-aes", "encrypt", "%s", "%s", "%s")}`, key, iv, plaintext)
	encResult, err := expr.Resolve(encExpr, nil, reg)
	require.NoError(t, err)
	require.NotEmpty(t, encResult)

	// Decrypt and verify round-trip.
	decExpr := fmt.Sprintf(`${__so("shell-aes", "decrypt", "%s", "%s", "%s")}`, key, iv, encResult)
	decResult, err := expr.Resolve(decExpr, nil, reg)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decResult)

	// Verify the plugin record is still in DB.
	resp = postJSONAuth(t, srv, token, "/api/v1/so-plugins/get", dto.GetSOPluginRequest{ID: plugin.ID})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
}

// ---------------------------------------------------------------------------
// Multi-version plugin support
// ---------------------------------------------------------------------------

// TestSOPlugin_MultiVersionAPIAndLoader verifies that multiple versions of
// the same plugin can be uploaded via API and called via expression engine
// with versioned references.
func TestSOPlugin_MultiVersionAPIAndLoader(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	// Upload two versions of the same plugin.
	for _, v := range []string{"1.0.0", "2.0.0"} {
		resp := postJSONAuth(t, srv, token, "/api/v1/so-plugins/create", dto.UploadSOPluginRequest{
			Name:     "multi",
			Version:  v,
			FilePath: fmt.Sprintf("/tmp/multi-%s.so", v),
		})
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		result := decodeResponse(t, resp)
		assert.Equal(t, 0, result.Code)
	}

	// Verify both versions in list.
	resp := postJSONAuth(t, srv, token, "/api/v1/so-plugins/list", dto.ListSOPluginsRequest{})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	data, _ := json.Marshal(result.Data)
	var listResp dto.ListResponse[[]dto.SOPluginDTO]
	json.Unmarshal(data, &listResp)
	assert.Equal(t, 2, listResp.Pagination.Total)

	// Verify the Loader resolves versions correctly.
	reg := expr.NewFunctionRegistry()
	builtin.RegisterAll(reg)
	loader := so.NewLoader()

	// Register both versions.
	for _, plugin := range []so.Plugin{
		&multiVersionPlugin{ver: "1.0.0"},
		&multiVersionPlugin{ver: "2.0.0"},
	} {
		err := loader.Register(plugin)
		require.NoError(t, err)
	}

	err := so.RegisterSO(reg, loader)
	require.NoError(t, err)

	// Call latest version (should be 2.0.0).
	resultLatest, err := expr.Resolve(`${__so("multi", "version")}`, nil, reg)
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", resultLatest)

	// Call specific version.
	resultV1, err := expr.Resolve(`${__so("multi@1.0.0", "version")}`, nil, reg)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", resultV1)
}

// ---------------------------------------------------------------------------
// Unauthorized access
// ---------------------------------------------------------------------------

// TestSOPlugin_UnauthorizedAccess verifies that SO plugin APIs require
// authentication.
func TestSOPlugin_UnauthorizedAccess(t *testing.T) {
	srv := newTestServer(t)

	endpoints := []struct {
		name string
		path string
		body any
	}{
		{"upload", "/api/v1/so-plugins/create", dto.UploadSOPluginRequest{Name: "x", Version: "1.0.0", FilePath: "/tmp/x.so"}},
		{"list", "/api/v1/so-plugins/list", dto.ListSOPluginsRequest{}},
		{"get", "/api/v1/so-plugins/get", dto.GetSOPluginRequest{ID: 1}},
		{"status", "/api/v1/so-plugins/status", dto.UpdateSOPluginStatusRequest{ID: 1, Status: "enabled"}},
		{"config", "/api/v1/so-plugins/config", dto.UpdateSOPluginConfigRequest{ID: 1, Config: "{}"}},
		{"delete", "/api/v1/so-plugins/delete", dto.DeleteSOPluginRequest{ID: 1}},
	}

	for _, ep := range endpoints {
		t.Run(ep.name, func(t *testing.T) {
			resp := postJSON(t, srv, ep.path, ep.body)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			result := decodeResponse(t, resp)
			assert.Equal(t, 401, result.Code)
		})
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// testShellAESPlugin implements so.Plugin for AES-CBC encryption/decryption.
type testShellAESPlugin struct{}

func (p *testShellAESPlugin) Name() string    { return "shell-aes" }
func (p *testShellAESPlugin) Version() string { return "1.0.0" }

func (p *testShellAESPlugin) Call(op string, args []string) (string, error) {
	switch op {
	case "encrypt":
		return p.encrypt(args)
	case "decrypt":
		return p.decrypt(args)
	default:
		return "", fmt.Errorf("unknown operation %q", op)
	}
}

func (p *testShellAESPlugin) encrypt(args []string) (string, error) {
	if len(args) < 3 {
		return "", fmt.Errorf("encrypt requires 3 args: key, iv, plaintext")
	}
	return fmt.Sprintf("encrypted:%s", args[2]), nil
}

func (p *testShellAESPlugin) decrypt(args []string) (string, error) {
	if len(args) < 3 {
		return "", fmt.Errorf("decrypt requires 3 args: key, iv, ciphertext")
	}
	return strings.TrimPrefix(args[2], "encrypted:"), nil
}

// multiVersionPlugin returns its version on the "version" operation.
type multiVersionPlugin struct {
	ver string
}

func (p *multiVersionPlugin) Name() string    { return "multi" }
func (p *multiVersionPlugin) Version() string { return p.ver }

func (p *multiVersionPlugin) Call(op string, args []string) (string, error) {
	if op == "version" {
		return p.ver, nil
	}
	return "", fmt.Errorf("unknown operation %q", op)
}