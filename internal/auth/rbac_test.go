package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
	"github.com/yannick2025-tech/Salvo/internal/store/model"
)

// mockPermissionRepo is a mock implementation of PermissionRepo
type mockPermissionRepo struct {
	permissions []*model.Permission
}

func (m *mockPermissionRepo) Create(ctx context.Context, perm *model.Permission) error {
	perm.ID = snowflake.ID(len(m.permissions) + 1)
	m.permissions = append(m.permissions, perm)
	return nil
}

func (m *mockPermissionRepo) GetByID(ctx context.Context, id snowflake.ID) (*model.Permission, error) {
	for _, p := range m.permissions {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, nil
}

func (m *mockPermissionRepo) GetByResourceAction(ctx context.Context, resource, action string) (*model.Permission, error) {
	for _, p := range m.permissions {
		if p.Resource == resource && p.Action == action {
			return p, nil
		}
	}
	return nil, nil
}

func (m *mockPermissionRepo) List(ctx context.Context) ([]*model.Permission, error) {
	return m.permissions, nil
}

func (m *mockPermissionRepo) ListByRoleID(ctx context.Context, roleID snowflake.ID) ([]*model.Permission, error) {
	return m.permissions, nil
}

// mockRolePermissionRepo is a mock implementation of RolePermissionRepo
// It stores assignments as roleID -> []permissionID, and looks up full
// Permission objects from the linked mockPermissionRepo.
type mockRolePermissionRepo struct {
	permRepo   *mockPermissionRepo
	assignments map[snowflake.ID][]snowflake.ID // roleID -> []permissionID
}

func newMockRolePermissionRepo(permRepo *mockPermissionRepo) *mockRolePermissionRepo {
	return &mockRolePermissionRepo{
		permRepo:    permRepo,
		assignments: make(map[snowflake.ID][]snowflake.ID),
	}
}

func (m *mockRolePermissionRepo) Assign(ctx context.Context, roleID, permissionID snowflake.ID) error {
	m.assignments[roleID] = append(m.assignments[roleID], permissionID)
	return nil
}

func (m *mockRolePermissionRepo) Revoke(ctx context.Context, roleID, permissionID snowflake.ID) error {
	ids := m.assignments[roleID]
	for i, id := range ids {
		if id == permissionID {
			m.assignments[roleID] = append(ids[:i], ids[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockRolePermissionRepo) RevokeAll(ctx context.Context, roleID snowflake.ID) error {
	m.assignments[roleID] = nil
	return nil
}

func (m *mockRolePermissionRepo) ListPermissions(ctx context.Context, roleID snowflake.ID) ([]*model.Permission, error) {
	ids := m.assignments[roleID]
	var perms []*model.Permission
	for _, id := range ids {
		for _, p := range m.permRepo.permissions {
			if p.ID == id {
				perms = append(perms, p)
				break
			}
		}
	}
	return perms, nil
}

func TestNewRBACChecker(t *testing.T) {
	permRepo := &mockPermissionRepo{}
	rpRepo := newMockRolePermissionRepo(permRepo)

	checker := NewRBACChecker(permRepo, rpRepo)
	require.NotNil(t, checker)
}

func TestRBACChecker_HasPermission_True(t *testing.T) {
	ctx := context.Background()
	permRepo := &mockPermissionRepo{}
	rpRepo := newMockRolePermissionRepo(permRepo)

	// Create permissions
	perm1 := &model.Permission{Resource: "scene", Action: "read"}
	perm2 := &model.Permission{Resource: "scene", Action: "write"}
	_ = permRepo.Create(ctx, perm1)
	_ = permRepo.Create(ctx, perm2)

	// Assign to role
	roleID := snowflake.ID(100)
	_ = rpRepo.Assign(ctx, roleID, perm1.ID)
	_ = rpRepo.Assign(ctx, roleID, perm2.ID)

	checker := NewRBACChecker(permRepo, rpRepo)

	// Check permission
	has, err := checker.HasPermission(ctx, roleID, "scene", "read")
	require.NoError(t, err)
	assert.True(t, has)

	has, err = checker.HasPermission(ctx, roleID, "scene", "write")
	require.NoError(t, err)
	assert.True(t, has)
}

func TestRBACChecker_HasPermission_False(t *testing.T) {
	ctx := context.Background()
	permRepo := &mockPermissionRepo{}
	rpRepo := newMockRolePermissionRepo(permRepo)

	roleID := snowflake.ID(100)
	checker := NewRBACChecker(permRepo, rpRepo)

	// Check permission without any assignments
	has, err := checker.HasPermission(ctx, roleID, "scene", "read")
	require.NoError(t, err)
	assert.False(t, has)
}

func TestRBACChecker_HasPermission_WrongAction(t *testing.T) {
	ctx := context.Background()
	permRepo := &mockPermissionRepo{}
	rpRepo := newMockRolePermissionRepo(permRepo)

	perm := &model.Permission{Resource: "scene", Action: "read"}
	_ = permRepo.Create(ctx, perm)

	roleID := snowflake.ID(100)
	_ = rpRepo.Assign(ctx, roleID, perm.ID)

	checker := NewRBACChecker(permRepo, rpRepo)

	// Wrong action
	has, err := checker.HasPermission(ctx, roleID, "scene", "delete")
	require.NoError(t, err)
	assert.False(t, has)

	// Wrong resource
	has, err = checker.HasPermission(ctx, roleID, "user", "read")
	require.NoError(t, err)
	assert.False(t, has)
}

func TestRBACChecker_Revoke(t *testing.T) {
	ctx := context.Background()
	permRepo := &mockPermissionRepo{}
	rpRepo := newMockRolePermissionRepo(permRepo)

	perm := &model.Permission{Resource: "scene", Action: "read"}
	_ = permRepo.Create(ctx, perm)

	roleID := snowflake.ID(100)
	_ = rpRepo.Assign(ctx, roleID, perm.ID)

	checker := NewRBACChecker(permRepo, rpRepo)

	has, _ := checker.HasPermission(ctx, roleID, "scene", "read")
	assert.True(t, has)

	// Revoke permission
	_ = rpRepo.Revoke(ctx, roleID, perm.ID)

	has, _ = checker.HasPermission(ctx, roleID, "scene", "read")
	assert.False(t, has)
}

func TestRBACChecker_RevokeAll(t *testing.T) {
	ctx := context.Background()
	permRepo := &mockPermissionRepo{}
	rpRepo := newMockRolePermissionRepo(permRepo)

	perm1 := &model.Permission{Resource: "scene", Action: "read"}
	perm2 := &model.Permission{Resource: "scene", Action: "write"}
	_ = permRepo.Create(ctx, perm1)
	_ = permRepo.Create(ctx, perm2)

	roleID := snowflake.ID(100)
	_ = rpRepo.Assign(ctx, roleID, perm1.ID)
	_ = rpRepo.Assign(ctx, roleID, perm2.ID)

	checker := NewRBACChecker(permRepo, rpRepo)

	// Revoke all permissions
	_ = rpRepo.RevokeAll(ctx, roleID)

	has1, _ := checker.HasPermission(ctx, roleID, "scene", "read")
	assert.False(t, has1)

	has2, _ := checker.HasPermission(ctx, roleID, "scene", "write")
	assert.False(t, has2)
}

func TestContextUserID(t *testing.T) {
	ctx := context.Background()
	userID := snowflake.ID(12345)

	ctx = WithUserID(ctx, userID)
	retrievedID := UserIDFromCtx(ctx)
	assert.Equal(t, userID, retrievedID)
}

func TestContextUserID_NotSet(t *testing.T) {
	ctx := context.Background()
	retrievedID := UserIDFromCtx(ctx)
	assert.Equal(t, snowflake.ID(0), retrievedID)
}

func TestContextRoleID(t *testing.T) {
	ctx := context.Background()
	roleID := snowflake.ID(100)

	ctx = WithRoleID(ctx, roleID)
	retrievedID := RoleIDFromCtx(ctx)
	assert.Equal(t, roleID, retrievedID)
}

func TestContextRoleID_NotSet(t *testing.T) {
	ctx := context.Background()
	retrievedID := RoleIDFromCtx(ctx)
	assert.Equal(t, snowflake.ID(0), retrievedID)
}
