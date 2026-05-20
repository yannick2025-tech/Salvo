package auth

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
	"github.com/yannick2025-tech/Salvo/internal/store/model"
	"github.com/yannick2025-tech/Salvo/internal/store/repo"
)

type SeedConfig struct {
	AdminEmail    string
	AdminPassword string
}

func DefaultSeedConfig() SeedConfig {
	return SeedConfig{
		AdminEmail:    "admin@salvo.local",
		AdminPassword: "admin",
	}
}

type Seeders struct {
	users  repo.UserRepo
	roles  repo.RoleRepo
	perms  repo.PermissionRepo
	rp     repo.RolePermissionRepo
	config SeedConfig
}

func NewSeeders(users repo.UserRepo, roles repo.RoleRepo, perms repo.PermissionRepo, rp repo.RolePermissionRepo, cfg SeedConfig) *Seeders {
	return &Seeders{users: users, roles: roles, perms: perms, rp: rp, config: cfg}
}

func (s *Seeders) Seed(ctx context.Context) error {
	users, err := s.users.List(ctx, repo.Filter{Limit: 1})
	if err != nil {
		return fmt.Errorf("check users: %w", err)
	}
	isFresh := len(users) == 0

	permDefs := []struct {
		Resource string
		Action   string
		Desc     string
	}{
		{"dashboard", "read", "View dashboard"},
		{"scene", "read", "View scenes"},
		{"scene", "write", "Create/edit scenes"},
		{"scene", "run", "Start/stop scenes"},
		{"report", "read", "View reports"},
		{"report", "export", "Export reports"},
		{"trace", "read", "View traces"},
		{"runner", "read", "View runner status"},
		{"runner", "write", "Control runner"},
		{"user", "read", "View users"},
		{"user", "write", "Create/edit users"},
		{"role", "read", "View roles"},
		{"role", "write", "Create/edit roles"},
		{"settings", "read", "View settings"},
		{"settings", "write", "Edit settings"},
	}

	permIDs := make(map[string]snowflake.ID)
	for _, pd := range permDefs {
		existing, err := s.perms.List(ctx)
		if err == nil {
			for _, p := range existing {
				permIDs[p.Resource+":"+p.Action] = p.ID
			}
		}
		key := pd.Resource + ":" + pd.Action
		if _, exists := permIDs[key]; exists {
			continue
		}
		p := &model.Permission{
			Resource:    pd.Resource,
			Action:      pd.Action,
			Description: pd.Desc,
		}
		if err := s.perms.Create(ctx, p); err != nil {
			return fmt.Errorf("create permission %s:%s: %w", pd.Resource, pd.Action, err)
		}
		permIDs[key] = p.ID
	}

	roleDefs := []struct {
		Name        string
		Desc        string
		IsBuiltin   bool
		Permissions []string
	}{
		{
			Name: "admin", Desc: "Administrator with full access", IsBuiltin: true,
			Permissions: []string{
				"dashboard:read", "scene:read", "scene:write", "scene:run",
				"report:read", "report:export", "trace:read",
				"runner:read", "runner:write",
				"user:read", "user:write", "role:read", "role:write",
				"settings:read", "settings:write",
			},
		},
		{
			Name: "operator", Desc: "Can run tests and view results", IsBuiltin: true,
			Permissions: []string{
				"dashboard:read", "scene:read", "scene:write", "scene:run",
				"report:read", "report:export", "trace:read",
				"runner:read", "runner:write",
				"settings:read",
			},
		},
		{
			Name: "viewer", Desc: "Read-only access to all resources", IsBuiltin: true,
			Permissions: []string{
				"dashboard:read", "scene:read",
				"report:read", "trace:read",
				"runner:read", "settings:read",
			},
		},
	}

	roleIDs := make(map[string]snowflake.ID)
	for _, rd := range roleDefs {
		existingRoles, err := s.roles.List(ctx, repo.Filter{Limit: 200})
		if err != nil {
			return fmt.Errorf("list roles: %w", err)
		}
		var existingRole *model.Role
		for _, r := range existingRoles {
			if r.Name == rd.Name {
				existingRole = r
				break
			}
		}

		if existingRole != nil {
			roleIDs[rd.Name] = existingRole.ID
			if err := s.syncRolePermissions(ctx, existingRole.ID, rd.Permissions, permIDs); err != nil {
				return fmt.Errorf("sync role %s permissions: %w", rd.Name, err)
			}
			continue
		}

		r := &model.Role{
			Name:        rd.Name,
			Description: rd.Desc,
			IsBuiltin:   rd.IsBuiltin,
		}
		if err := s.roles.Create(ctx, r); err != nil {
			return fmt.Errorf("create role %s: %w", rd.Name, err)
		}
		roleIDs[rd.Name] = r.ID

		for _, permKey := range rd.Permissions {
			pid, ok := permIDs[permKey]
			if !ok {
				return fmt.Errorf("permission %s not found", permKey)
			}
			if err := s.rp.Assign(ctx, r.ID, pid); err != nil {
				return fmt.Errorf("assign %s to %s: %w", permKey, rd.Name, err)
			}
		}
	}

	if !isFresh {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(s.config.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}

	adminUser := &model.User{
		Email:        s.config.AdminEmail,
		PasswordHash: string(hash),
		Nickname:     "Admin",
		RoleID:       roleIDs["admin"],
		Status:       model.UserStatusActive,
		LastLoginAt:  nil,
	}
	if err := s.users.Create(ctx, adminUser); err != nil {
		return fmt.Errorf("create admin user: %w", err)
	}

	return nil
}

func (s *Seeders) syncRolePermissions(ctx context.Context, roleID snowflake.ID, wantPerms []string, permIDs map[string]snowflake.ID) error {
	wantSet := make(map[string]bool)
	for _, k := range wantPerms {
		wantSet[k] = true
	}

	current, err := s.rp.ListPermissions(ctx, roleID)
	if err != nil {
		return fmt.Errorf("list role permissions: %w", err)
	}

	currentSet := make(map[string]bool)
	for _, p := range current {
		key := p.Resource + ":" + p.Action
		currentSet[key] = true
	}

	for _, permKey := range wantPerms {
		if currentSet[permKey] {
			continue
		}
		pid, ok := permIDs[permKey]
		if !ok {
			continue
		}
		if err := s.rp.Assign(ctx, roleID, pid); err != nil {
			return fmt.Errorf("assign %s: %w", permKey, err)
		}
	}

	return nil
}
