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
	var count int
	users, err := s.users.List(ctx, repo.Filter{Limit: 1})
	if err != nil {
		return fmt.Errorf("check users: %w", err)
	}
	count = len(users)
	if count > 0 {
		return nil
	}

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
		p := &model.Permission{
			Resource:    pd.Resource,
			Action:      pd.Action,
			Description: pd.Desc,
		}
		if err := s.perms.Create(ctx, p); err != nil {
			return fmt.Errorf("create permission %s:%s: %w", pd.Resource, pd.Action, err)
		}
		permIDs[pd.Resource+":"+pd.Action] = p.ID
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
				"dashboard:read", "scene:read", "scene:run",
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
