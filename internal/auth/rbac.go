package auth

import (
	"context"

	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
	"github.com/yannick2025-tech/Salvo/internal/store/repo"
)

type RBACChecker struct {
	perms repo.PermissionRepo
	rp    repo.RolePermissionRepo
}

func NewRBACChecker(perms repo.PermissionRepo, rp repo.RolePermissionRepo) *RBACChecker {
	return &RBACChecker{perms: perms, rp: rp}
}

func (c *RBACChecker) HasPermission(ctx context.Context, roleID snowflake.ID, resource, action string) (bool, error) {
	rolePerms, err := c.rp.ListPermissions(ctx, roleID)
	if err != nil {
		return false, err
	}
	for _, p := range rolePerms {
		if p.Resource == resource && p.Action == action {
			return true, nil
		}
	}
	return false, nil
}

type ctxKey string

const (
	ctxKeyUserID ctxKey = "user_id"
	ctxKeyRoleID ctxKey = "role_id"
)

func WithUserID(ctx context.Context, id snowflake.ID) context.Context {
	return context.WithValue(ctx, ctxKeyUserID, id)
}

func UserIDFromCtx(ctx context.Context) snowflake.ID {
	v, _ := ctx.Value(ctxKeyUserID).(snowflake.ID)
	return v
}

func WithRoleID(ctx context.Context, id snowflake.ID) context.Context {
	return context.WithValue(ctx, ctxKeyRoleID, id)
}

func RoleIDFromCtx(ctx context.Context) snowflake.ID {
	v, _ := ctx.Value(ctxKeyRoleID).(snowflake.ID)
	return v
}
