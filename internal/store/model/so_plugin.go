package model

import (
	"time"

	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
)

// SOPlugin represents a registered .so plugin in the database.
type SOPlugin struct {
	ID        snowflake.ID `json:"id"`
	Name      string       `json:"name"`
	Version   string       `json:"version"`
	FilePath  string       `json:"file_path"`
	Status    string       `json:"status"` // enabled, disabled
	Config    string       `json:"config"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
	DeletedAt *time.Time   `json:"deleted_at,omitempty"`
}

const (
	SOPluginStatusEnabled  = "enabled"
	SOPluginStatusDisabled = "disabled"
)