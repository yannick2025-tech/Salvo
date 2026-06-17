package model

import "time"

// SOPlugin represents a registered .so plugin in the database.
type SOPlugin struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	Version   string     `json:"version"`
	FilePath  string     `json:"file_path"`
	Status    string     `json:"status"` // enabled, disabled
	Config    string     `json:"config"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

const (
	SOPluginStatusEnabled  = "enabled"
	SOPluginStatusDisabled = "disabled"
)