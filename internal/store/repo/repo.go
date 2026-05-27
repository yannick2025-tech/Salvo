// Package repo defines the Repository interfaces for the Salvo persistence layer.
//
// Each entity has its own Repository interface following the same CRUD
// pattern: Create, GetByID, List, Update, Delete (soft). All List
// methods accept a Filter for pagination and conditional queries.
// Soft-deleted records are automatically excluded from all queries.
package repo

import (
	"context"

	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
	"github.com/yannick2025-tech/Salvo/internal/store/model"
)

// Filter holds common pagination and filtering parameters for List queries.
type Filter struct {
	// Offset is the number of records to skip.
	Offset int
	// Limit is the maximum number of records to return.
	Limit int
	// Status filters by the status field (if applicable).
	Status string
	// SceneID filters by scene_id (for scene-scoped entities).
	SceneID snowflake.ID
}

// DefaultFilter returns a Filter with sensible defaults.
func DefaultFilter() Filter {
	return Filter{
		Offset: 0,
		Limit:  50,
	}
}

// SceneRepo provides persistence operations for Scene entities.
type SceneRepo interface {
	Create(ctx context.Context, scene *model.Scene) error
	GetByID(ctx context.Context, id snowflake.ID) (*model.Scene, error)
	List(ctx context.Context, filter Filter) ([]*model.Scene, error)
	Update(ctx context.Context, scene *model.Scene) error
	UpdateStatus(ctx context.Context, id snowflake.ID, status string) error
	Delete(ctx context.Context, id snowflake.ID) error
}

// NodeRepo provides persistence operations for Node entities.
type NodeRepo interface {
	Create(ctx context.Context, node *model.Node) error
	GetByID(ctx context.Context, id snowflake.ID) (*model.Node, error)
	List(ctx context.Context, filter Filter) ([]*model.Node, error)
	Update(ctx context.Context, node *model.Node) error
	Delete(ctx context.Context, id snowflake.ID) error
}

// EdgeRepo provides persistence operations for Edge entities.
type EdgeRepo interface {
	Create(ctx context.Context, edge *model.Edge) error
	GetByID(ctx context.Context, id snowflake.ID) (*model.Edge, error)
	List(ctx context.Context, filter Filter) ([]*model.Edge, error)
	Update(ctx context.Context, edge *model.Edge) error
	Delete(ctx context.Context, id snowflake.ID) error
}

// VariableRepo provides persistence operations for Variable entities.
type VariableRepo interface {
	Create(ctx context.Context, v *model.Variable) error
	GetByID(ctx context.Context, id snowflake.ID) (*model.Variable, error)
	List(ctx context.Context, filter Filter) ([]*model.Variable, error)
	Update(ctx context.Context, v *model.Variable) error
	Delete(ctx context.Context, id snowflake.ID) error
}

// PluginConfigRepo provides persistence operations for PluginConfig entities.
type PluginConfigRepo interface {
	Create(ctx context.Context, pc *model.PluginConfig) error
	GetByID(ctx context.Context, id snowflake.ID) (*model.PluginConfig, error)
	List(ctx context.Context, filter Filter) ([]*model.PluginConfig, error)
	Update(ctx context.Context, pc *model.PluginConfig) error
	Delete(ctx context.Context, id snowflake.ID) error
}

// ReportRepo provides persistence operations for Report entities.
type ReportRepo interface {
	Create(ctx context.Context, report *model.Report) error
	GetByID(ctx context.Context, id snowflake.ID) (*model.Report, error)
	GetByRunID(ctx context.Context, runID snowflake.ID) (*model.Report, error)
	List(ctx context.Context, filter Filter) ([]*model.Report, error)
	Update(ctx context.Context, report *model.Report) error
	Delete(ctx context.Context, id snowflake.ID) error
}

// RunRecordRepo provides persistence operations for RunRecord entities.
type RunRecordRepo interface {
	Create(ctx context.Context, record *model.RunRecord) error
	GetByID(ctx context.Context, id snowflake.ID) (*model.RunRecord, error)
	List(ctx context.Context, filter Filter) ([]*model.RunRecord, error)
	Update(ctx context.Context, record *model.RunRecord) error
	Delete(ctx context.Context, id snowflake.ID) error
}

type UserRepo interface {
	Create(ctx context.Context, user *model.User) error
	GetByID(ctx context.Context, id snowflake.ID) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	List(ctx context.Context, filter Filter) ([]*model.User, error)
	Update(ctx context.Context, user *model.User) error
	Delete(ctx context.Context, id snowflake.ID) error
}

type RoleRepo interface {
	Create(ctx context.Context, role *model.Role) error
	GetByID(ctx context.Context, id snowflake.ID) (*model.Role, error)
	GetByName(ctx context.Context, name string) (*model.Role, error)
	List(ctx context.Context, filter Filter) ([]*model.Role, error)
	Update(ctx context.Context, role *model.Role) error
	Delete(ctx context.Context, id snowflake.ID) error
}

type PermissionRepo interface {
	Create(ctx context.Context, perm *model.Permission) error
	GetByID(ctx context.Context, id snowflake.ID) (*model.Permission, error)
	GetByResourceAction(ctx context.Context, resource, action string) (*model.Permission, error)
	List(ctx context.Context) ([]*model.Permission, error)
	ListByRoleID(ctx context.Context, roleID snowflake.ID) ([]*model.Permission, error)
}

type RolePermissionRepo interface {
	Assign(ctx context.Context, roleID, permissionID snowflake.ID) error
	Revoke(ctx context.Context, roleID, permissionID snowflake.ID) error
	RevokeAll(ctx context.Context, roleID snowflake.ID) error
	ListPermissions(ctx context.Context, roleID snowflake.ID) ([]*model.Permission, error)
}

// DataSourceRepo provides persistence operations for DataSource entities.
type DataSourceRepo interface {
	Create(ctx context.Context, ds *model.DataSource) error
	GetByID(ctx context.Context, id snowflake.ID) (*model.DataSource, error)
	GetBySceneIDAndName(ctx context.Context, sceneID snowflake.ID, name string) (*model.DataSource, error)
	ListBySceneID(ctx context.Context, sceneID snowflake.ID) ([]*model.DataSource, error)
	Delete(ctx context.Context, id snowflake.ID) error
}
