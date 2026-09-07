package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/api/dto"
	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
)

// --- Me / Logout ---

func TestMe(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/auth/me", nil)
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	data, err := json.Marshal(result.Data)
	require.NoError(t, err)

	var meResp struct {
		User        dto.UserDTO `json:"user"`
		Permissions []string    `json:"permissions"`
	}
	require.NoError(t, json.Unmarshal(data, &meResp))
	assert.Equal(t, "admin@salvo.local", meResp.User.Email)
	assert.NotEmpty(t, meResp.Permissions)
}

func TestMeNotAuthenticated(t *testing.T) {
	srv := newTestServer(t)

	resp := postJSON(t, srv, "/api/v1/auth/me", nil)
	result := decodeResponse(t, resp)
	assert.Equal(t, 401, result.Code)
}

func TestLogout(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/auth/logout", nil)
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
}

// --- ChangePassword ---

func TestChangePasswordSuccess(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/auth/change-password", dto.ChangePasswordRequest{
		OldPassword: "admin",
		NewPassword: "newpass123",
	})
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
}

func TestChangePasswordWrongOld(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/auth/change-password", dto.ChangePasswordRequest{
		OldPassword: "wrong",
		NewPassword: "newpass123",
	})
	result := decodeResponse(t, resp)
	assert.Equal(t, 401, result.Code)
}

func TestChangePasswordEmptyFields(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/auth/change-password", dto.ChangePasswordRequest{})
	result := decodeResponse(t, resp)
	assert.Equal(t, 400, result.Code)
}

// --- ResetPassword ---

func TestResetPasswordSuccess(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	// Get admin user ID
	resp := postJSONAuth(t, srv, token, "/api/v1/auth/me", nil)
	result := decodeResponse(t, resp)
	require.Equal(t, 0, result.Code)
	data, _ := json.Marshal(result.Data)
	var meResp struct {
		User dto.UserDTO `json:"user"`
	}
	require.NoError(t, json.Unmarshal(data, &meResp))

	// Reset password
	resp = postJSONAuth(t, srv, token, "/api/v1/auth/reset-password", dto.ResetPasswordRequest{
		UserID:      meResp.User.ID,
		NewPassword: "resetpass123",
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
}

func TestResetPasswordUserNotFound(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/auth/reset-password", dto.ResetPasswordRequest{
		UserID:      999999,
		NewPassword: "newpass123",
	})
	result := decodeResponse(t, resp)
	assert.Equal(t, 404, result.Code)
}

func TestResetPasswordEmptyFields(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/auth/reset-password", dto.ResetPasswordRequest{})
	result := decodeResponse(t, resp)
	assert.Equal(t, 400, result.Code)
}

// --- ListUsers ---

func TestListUsers(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/users/list", dto.ListUsersRequest{})
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	data, err := json.Marshal(result.Data)
	require.NoError(t, err)

	var listResp dto.ListResponse[[]dto.UserDTO]
	require.NoError(t, json.Unmarshal(data, &listResp))
	assert.NotEmpty(t, listResp.Items)
	assert.Equal(t, "admin@salvo.local", listResp.Items[0].Email)
}

// --- CreateUser ---

func TestCreateUserSuccess(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/users/create", dto.CreateUserRequest{
		Email:    "testuser@example.com",
		Password: "testpass123",
		Nickname: "Test User",
	})
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	data, err := json.Marshal(result.Data)
	require.NoError(t, err)

	var user dto.UserDTO
	require.NoError(t, json.Unmarshal(data, &user))
	assert.Equal(t, "testuser@example.com", user.Email)
	assert.Equal(t, "Test User", user.Nickname)
}

func TestCreateUserDefaultNickname(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/users/create", dto.CreateUserRequest{
		Email:    "noreply@example.com",
		Password: "testpass123",
	})
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	data, _ := json.Marshal(result.Data)
	var user dto.UserDTO
	require.NoError(t, json.Unmarshal(data, &user))
	assert.Equal(t, "noreply", user.Nickname)
}

func TestCreateUserEmptyFields(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/users/create", dto.CreateUserRequest{})
	result := decodeResponse(t, resp)
	assert.Equal(t, 400, result.Code)
}

func TestCreateUserInvalidEmail(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/users/create", dto.CreateUserRequest{
		Email:    "not-an-email",
		Password: "testpass123",
	})
	result := decodeResponse(t, resp)
	assert.Equal(t, 400, result.Code)
}

// --- UpdateUser ---

func TestUpdateUserSuccess(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	// Create user first
	resp := postJSONAuth(t, srv, token, "/api/v1/users/create", dto.CreateUserRequest{
		Email:    "update@example.com",
		Password: "pass123",
	})
	result := decodeResponse(t, resp)
	require.Equal(t, 0, result.Code)
	data, _ := json.Marshal(result.Data)
	var user dto.UserDTO
	require.NoError(t, json.Unmarshal(data, &user))

	// Update nickname
	resp = postJSONAuth(t, srv, token, "/api/v1/users/update", dto.UpdateUserRequest{
		ID:       user.ID,
		Nickname: "Updated Name",
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	data, _ = json.Marshal(result.Data)
	var updated dto.UserDTO
	require.NoError(t, json.Unmarshal(data, &updated))
	assert.Equal(t, "Updated Name", updated.Nickname)
}

func TestUpdateUserNotFound(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/users/update", dto.UpdateUserRequest{
		ID:       999999,
		Nickname: "Ghost",
	})
	result := decodeResponse(t, resp)
	assert.Equal(t, 404, result.Code)
}

func TestUpdateUserMissingID(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/users/update", dto.UpdateUserRequest{})
	result := decodeResponse(t, resp)
	assert.Equal(t, 400, result.Code)
}

// --- DeleteUser ---

func TestDeleteUserSuccess(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	// Create user first
	resp := postJSONAuth(t, srv, token, "/api/v1/users/create", dto.CreateUserRequest{
		Email:    "delete@example.com",
		Password: "pass123",
	})
	result := decodeResponse(t, resp)
	require.Equal(t, 0, result.Code)
	data, _ := json.Marshal(result.Data)
	var user dto.UserDTO
	require.NoError(t, json.Unmarshal(data, &user))

	// Delete
	resp = postJSONAuth(t, srv, token, "/api/v1/users/delete", dto.IDRequest{ID: user.ID})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
}

func TestDeleteUserMissingID(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/users/delete", dto.IDRequest{})
	result := decodeResponse(t, resp)
	assert.Equal(t, 400, result.Code)
}

// --- ListRoles ---

func TestListRoles(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/roles/list", dto.ListRolesRequest{})
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	data, err := json.Marshal(result.Data)
	require.NoError(t, err)

	var listResp dto.ListResponse[[]dto.RoleDTO]
	require.NoError(t, json.Unmarshal(data, &listResp))
	assert.NotEmpty(t, listResp.Items)

	// At least admin role should exist
	foundAdmin := false
	for _, r := range listResp.Items {
		if r.Name == "admin" {
			foundAdmin = true
			assert.True(t, r.IsBuiltin)
		}
	}
	assert.True(t, foundAdmin, "admin role should exist")
}

// --- CreateRole ---

func TestCreateRoleSuccess(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/roles/create", dto.CreateRoleRequest{
		Name:        "custom-role",
		Description: "A custom role for testing",
	})
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	data, err := json.Marshal(result.Data)
	require.NoError(t, err)

	var role dto.RoleDTO
	require.NoError(t, json.Unmarshal(data, &role))
	assert.Equal(t, "custom-role", role.Name)
	assert.False(t, role.IsBuiltin)
}

func TestCreateRoleEmptyName(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/roles/create", dto.CreateRoleRequest{})
	result := decodeResponse(t, resp)
	assert.Equal(t, 400, result.Code)
}

// --- UpdateRole ---

func TestUpdateRoleSuccess(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	// Create role first
	resp := postJSONAuth(t, srv, token, "/api/v1/roles/create", dto.CreateRoleRequest{
		Name:        "to-update",
		Description: "Original",
	})
	result := decodeResponse(t, resp)
	require.Equal(t, 0, result.Code)
	data, _ := json.Marshal(result.Data)
	var role dto.RoleDTO
	require.NoError(t, json.Unmarshal(data, &role))

	// Update
	resp = postJSONAuth(t, srv, token, "/api/v1/roles/update", dto.UpdateRoleRequest{
		ID:          role.ID,
		Name:        "updated-name",
		Description: "Updated description",
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	data, _ = json.Marshal(result.Data)
	var updated dto.RoleDTO
	require.NoError(t, json.Unmarshal(data, &updated))
	assert.Equal(t, "updated-name", updated.Name)
	assert.Equal(t, "Updated description", updated.Description)
}

func TestUpdateRoleNotFound(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/roles/update", dto.UpdateRoleRequest{
		ID:   999999,
		Name: "ghost",
	})
	result := decodeResponse(t, resp)
	assert.Equal(t, 404, result.Code)
}

func TestUpdateRoleBuiltinForbidden(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	// Find the built-in admin role
	resp := postJSONAuth(t, srv, token, "/api/v1/roles/list", dto.ListRolesRequest{})
	result := decodeResponse(t, resp)
	require.Equal(t, 0, result.Code)
	data, _ := json.Marshal(result.Data)
	var listResp dto.ListResponse[[]dto.RoleDTO]
	require.NoError(t, json.Unmarshal(data, &listResp))

	var builtinRoleID snowflake.ID
	found := false
	for _, r := range listResp.Items {
		if r.IsBuiltin {
			builtinRoleID = r.ID
			found = true
			break
		}
	}
	require.True(t, found, "should find a builtin role")

	resp = postJSONAuth(t, srv, token, "/api/v1/roles/update", dto.UpdateRoleRequest{
		ID:   builtinRoleID,
		Name: "hacked",
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 403, result.Code)
}

func TestUpdateRoleMissingID(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/roles/update", dto.UpdateRoleRequest{})
	result := decodeResponse(t, resp)
	assert.Equal(t, 400, result.Code)
}

// --- DeleteRole ---

func TestDeleteRoleSuccess(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	// Create role first
	resp := postJSONAuth(t, srv, token, "/api/v1/roles/create", dto.CreateRoleRequest{
		Name: "to-delete",
	})
	result := decodeResponse(t, resp)
	require.Equal(t, 0, result.Code)
	data, _ := json.Marshal(result.Data)
	var role dto.RoleDTO
	require.NoError(t, json.Unmarshal(data, &role))

	// Delete
	resp = postJSONAuth(t, srv, token, "/api/v1/roles/delete", dto.IDRequest{ID: role.ID})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
}

func TestDeleteRoleBuiltinForbidden(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	// List roles to find builtin
	resp := postJSONAuth(t, srv, token, "/api/v1/roles/list", dto.ListRolesRequest{})
	result := decodeResponse(t, resp)
	require.Equal(t, 0, result.Code)
	data, _ := json.Marshal(result.Data)
	var listResp dto.ListResponse[[]dto.RoleDTO]
	require.NoError(t, json.Unmarshal(data, &listResp))

	for _, r := range listResp.Items {
		if r.IsBuiltin {
			resp = postJSONAuth(t, srv, token, "/api/v1/roles/delete", dto.IDRequest{ID: r.ID})
			result = decodeResponse(t, resp)
			assert.Equal(t, 403, result.Code)
			return
		}
	}
	t.Fatal("no builtin role found")
}

func TestDeleteRoleNotFound(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/roles/delete", dto.IDRequest{ID: 999999})
	result := decodeResponse(t, resp)
	assert.Equal(t, 404, result.Code)
}

func TestDeleteRoleMissingID(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/roles/delete", dto.IDRequest{})
	result := decodeResponse(t, resp)
	assert.Equal(t, 400, result.Code)
}

// --- isValidEmail ---

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		email string
		want  bool
	}{
		{"user@example.com", true},
		{"admin@salvo.local", true},
		{"test.user+tag@domain.co.uk", true},
		{"not-an-email", false},
		{"@domain.com", false},
		{"user@", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			assert.Equal(t, tt.want, isValidEmail(tt.email))
		})
	}
}

// --- ListGenerators ---

func TestListGenerators(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/generators/list", nil)
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	data, err := json.Marshal(result.Data)
	require.NoError(t, err)

	var resp2 struct {
		Categories any `json:"categories"`
	}
	require.NoError(t, json.Unmarshal(data, &resp2))
	assert.NotNil(t, resp2.Categories)
}

// --- ListRunRecords / GetRunRecord ---

func TestListRunRecordsEmpty(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/runs/list", dto.ListRunRecordsRequest{})
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	data, err := json.Marshal(result.Data)
	require.NoError(t, err)

	var listResp dto.ListResponse[[]dto.RunRecordDTO]
	require.NoError(t, json.Unmarshal(data, &listResp))
	assert.Empty(t, listResp.Items)
}

func TestGetRunRecordNotFound(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/runs/get", dto.IDRequest{ID: 999999})
	result := decodeResponse(t, resp)
	assert.Equal(t, 404, result.Code)
}

func TestGetRunRecordMissingID(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/runs/get", dto.IDRequest{})
	result := decodeResponse(t, resp)
	assert.Equal(t, 400, result.Code)
}

// --- ListTraces ---

func TestListTracesEmpty(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/traces/list", dto.ListTracesRequest{})
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	data, err := json.Marshal(result.Data)
	require.NoError(t, err)

	var listResp dto.ListResponse[[]dto.TraceDTO]
	require.NoError(t, json.Unmarshal(data, &listResp))
	assert.Empty(t, listResp.Items)
	assert.Equal(t, 0, listResp.Pagination.Total)
}

// --- GetTrace / GetTraceByRun ---

func TestGetTraceNotFound(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/traces/get", dto.GetTraceRequest{ID: 999999})
	result := decodeResponse(t, resp)
	assert.Equal(t, 404, result.Code)
}

func TestGetTraceByRunNotFound(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/traces/get-by-run", dto.GetTraceByRunRequest{RunID: 999999})
	result := decodeResponse(t, resp)
	assert.Equal(t, 404, result.Code)
}

// --- UpdateDataSource ---

func TestUpdateDataSourceMissingID(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/datasources/update", dto.UpdateDataSourceRequest{})
	result := decodeResponse(t, resp)
	assert.Equal(t, 400, result.Code)
}

func TestUpdateDataSourceMissingContent(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/datasources/update", dto.UpdateDataSourceRequest{ID: 1})
	result := decodeResponse(t, resp)
	assert.Equal(t, 400, result.Code)
}

func TestUpdateDataSourceNotFound(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/datasources/update", dto.UpdateDataSourceRequest{
		ID:      999999,
		Content: "a,b\n1,2",
	})
	result := decodeResponse(t, resp)
	assert.Equal(t, 404, result.Code)
}

// --- SceneStatus ---

func TestSceneStatusNotRunning(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	// Create scene first
	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/create", dto.CreateSceneRequest{
		Name:   "status-test",
		Status: "draft",
	})
	result := decodeResponse(t, resp)
	require.Equal(t, 0, result.Code)
	data, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(data, &scene))

	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/status", dto.SceneStatusRequest{SceneID: scene.ID})
	result = decodeResponse(t, resp)
	// Scene is not running, returns 404
	assert.Equal(t, 404, result.Code)
}

func TestSceneStatusMissingID(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	// SceneID=0 won't be found in runner manager, returns 404
	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/status", dto.SceneStatusRequest{})
	result := decodeResponse(t, resp)
	assert.Equal(t, 404, result.Code)
}

// --- StopScene ---

func TestStopSceneNotRunning(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/stop", dto.StopSceneRequest{SceneID: 999999})
	result := decodeResponse(t, resp)
	assert.NotEqual(t, 0, result.Code)
}

// --- DashboardHistory ---

func TestDashboardHistoryEmpty(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/dashboard/history", dto.DashboardHistoryRequest{})
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
}
