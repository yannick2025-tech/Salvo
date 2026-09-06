package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
	"github.com/yannick2025-tech/Salvo/internal/store/model"
	"github.com/yannick2025-tech/Salvo/internal/store/repo"
)

func TestDefaultSeedConfig(t *testing.T) {
	cfg := DefaultSeedConfig()
	assert.Equal(t, "admin@salvo.local", cfg.AdminEmail)
	assert.Equal(t, "admin", cfg.AdminPassword)
}

func TestNewSeeders(t *testing.T) {
	users := &mockUserRepo{}
	roles := &mockRoleRepo{}
	perms := &mockPermissionRepo{}
	rp := newMockRolePermissionRepo(perms)
	cfg := DefaultSeedConfig()

	seeders := NewSeeders(users, roles, perms, rp, cfg)
	require.NotNil(t, seeders)
	assert.Equal(t, users, seeders.users)
	assert.Equal(t, roles, seeders.roles)
	assert.Equal(t, perms, seeders.perms)
	assert.Equal(t, rp, seeders.rp)
	assert.Equal(t, cfg, seeders.config)
}

func TestSeeders_Seed_FreshDatabase(t *testing.T) {
	ctx := context.Background()
	users := &mockUserRepo{}
	roles := &mockRoleRepo{}
	perms := &mockPermissionRepo{}
	rp := newMockRolePermissionRepo(perms)
	cfg := SeedConfig{
		AdminEmail:    "test@example.com",
		AdminPassword: "testpass",
	}

	seeders := NewSeeders(users, roles, perms, rp, cfg)
	err := seeders.Seed(ctx)
	require.NoError(t, err)

	// Verify permissions were created
	assert.Greater(t, len(perms.permissions), 0, "permissions should be created")

	// Verify roles were created
	assert.Greater(t, len(roles.roles), 0, "roles should be created")

	// Verify admin user was created (fresh database)
	assert.Equal(t, 1, len(users.users), "admin user should be created")
	assert.Equal(t, "test@example.com", users.users[0].Email)
	assert.Equal(t, "Admin", users.users[0].Nickname)
	assert.Equal(t, model.UserStatusActive, users.users[0].Status)
}

func TestSeeders_Seed_ExistingDatabase(t *testing.T) {
	ctx := context.Background()
	users := &mockUserRepo{}
	roles := &mockRoleRepo{}
	perms := &mockPermissionRepo{}
	rp := newMockRolePermissionRepo(perms)

	// Pre-populate with an existing user
	existingUser := &model.User{
		Email:    "existing@example.com",
		Nickname: "Existing User",
	}
	users.users = append(users.users, existingUser)

	cfg := DefaultSeedConfig()
	seeders := NewSeeders(users, roles, perms, rp, cfg)
	err := seeders.Seed(ctx)
	require.NoError(t, err)

	// Verify no new admin user was created
	assert.Equal(t, 1, len(users.users), "no new admin user should be created")
	assert.Equal(t, "existing@example.com", users.users[0].Email)
}

func TestSeeders_SyncRolePermissions(t *testing.T) {
	ctx := context.Background()
	perms := &mockPermissionRepo{}
	rp := newMockRolePermissionRepo(perms)

	// Create some permissions
	perm1 := &model.Permission{}
	perm1.Resource = "scene"
	perm1.Action = "read"
	perm2 := &model.Permission{}
	perm2.Resource = "scene"
	perm2.Action = "write"
	perms.permissions = append(perms.permissions, perm1, perm2)

	permIDs := map[string]snowflake.ID{
		"scene:read":  perm1.ID,
		"scene:write": perm2.ID,
	}

	seeders := &Seeders{perms: perms, rp: rp}
	roleID := snowflake.ID(100)

	// Sync permissions
	wantPerms := []string{"scene:read", "scene:write"}
	err := seeders.syncRolePermissions(ctx, roleID, wantPerms, permIDs)
	require.NoError(t, err)

	// Verify permissions were assigned
	assigned, err := rp.ListPermissions(ctx, roleID)
	require.NoError(t, err)
	assert.Len(t, assigned, 2)
}

func TestSeeders_SyncRolePermissions_AlreadyAssigned(t *testing.T) {
	ctx := context.Background()
	perms := &mockPermissionRepo{}
	rp := newMockRolePermissionRepo(perms)

	perm1 := &model.Permission{Resource: "scene", Action: "read"}
	perm1.ID = snowflake.ID(1)
	perms.permissions = append(perms.permissions, perm1)

	permIDs := map[string]snowflake.ID{
		"scene:read": perm1.ID,
	}

	roleID := snowflake.ID(100)
	// Pre-assign permission
	rp.Assign(ctx, roleID, perm1.ID)

	seeders := &Seeders{perms: perms, rp: rp}
	wantPerms := []string{"scene:read"}
	err := seeders.syncRolePermissions(ctx, roleID, wantPerms, permIDs)
	require.NoError(t, err)

	// Verify no duplicate assignment
	assigned, err := rp.ListPermissions(ctx, roleID)
	require.NoError(t, err)
	assert.Len(t, assigned, 1)
}

// Mock implementations

type mockUserRepo struct {
	users []*model.User
}

func (m *mockUserRepo) Create(ctx context.Context, user *model.User) error {
	m.users = append(m.users, user)
	return nil
}

func (m *mockUserRepo) GetByID(ctx context.Context, id snowflake.ID) (*model.User, error) {
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, nil
}

func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, nil
}

func (m *mockUserRepo) List(ctx context.Context, filter repo.Filter) ([]*model.User, error) {
	if filter.Status != "" {
		var filtered []*model.User
		for _, u := range m.users {
			if u.Status == filter.Status {
				filtered = append(filtered, u)
			}
		}
		return filtered, nil
	}
	return m.users, nil
}

func (m *mockUserRepo) Update(ctx context.Context, user *model.User) error {
	for i, u := range m.users {
		if u.ID == user.ID {
			m.users[i] = user
			return nil
		}
	}
	return nil
}

func (m *mockUserRepo) Delete(ctx context.Context, id snowflake.ID) error {
	for i, u := range m.users {
		if u.ID == id {
			m.users = append(m.users[:i], m.users[i+1:]...)
			return nil
		}
	}
	return nil
}

type mockRoleRepo struct {
	roles []*model.Role
}

func (m *mockRoleRepo) Create(ctx context.Context, role *model.Role) error {
	m.roles = append(m.roles, role)
	return nil
}

func (m *mockRoleRepo) GetByID(ctx context.Context, id snowflake.ID) (*model.Role, error) {
	for _, r := range m.roles {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, nil
}

func (m *mockRoleRepo) GetByName(ctx context.Context, name string) (*model.Role, error) {
	for _, r := range m.roles {
		if r.Name == name {
			return r, nil
		}
	}
	return nil, nil
}

func (m *mockRoleRepo) List(ctx context.Context, filter repo.Filter) ([]*model.Role, error) {
	return m.roles, nil
}

func (m *mockRoleRepo) Update(ctx context.Context, role *model.Role) error {
	for i, r := range m.roles {
		if r.ID == role.ID {
			m.roles[i] = role
			return nil
		}
	}
	return nil
}

func (m *mockRoleRepo) Delete(ctx context.Context, id snowflake.ID) error {
	for i, r := range m.roles {
		if r.ID == id {
			m.roles = append(m.roles[:i], m.roles[i+1:]...)
			return nil
		}
	}
	return nil
}
