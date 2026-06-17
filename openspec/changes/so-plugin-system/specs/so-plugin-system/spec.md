## ADDED Requirements

### Requirement: SO Plugin interface contract
The system SHALL define a universal Plugin interface at `internal/plugin/so/contract.go` that .so files must implement:
- `Name() string` — unique plugin identifier (lowercase, alphanumeric + hyphens)
- `Version() string` — semver version string (e.g. "1.0.0")
- `Init(config string) error` — initialize with JSON config string from GUI
- `Ops() []string` — list of supported operations
- `Call(op string, args ...string) (string, error)` — universal entry point

The .so file MUST export a symbol named `New` of type `func() interface{}` that returns a Plugin instance.

#### Scenario: Valid plugin load
- **WHEN** a .so file exports `New() interface{}` returning a struct implementing all 5 methods
- **THEN** the plugin is loaded successfully and registered in the Loader

#### Scenario: Missing New symbol
- **WHEN** a .so file does not export a `New` symbol
- **THEN** load fails with error `symbol "New" not found`

#### Scenario: Wrong interface
- **WHEN** a .so file's `New()` returns a type that doesn't implement Plugin
- **THEN** load fails with error `does not implement so.Plugin`

### Requirement: SO Plugin Loader with version management
The system SHALL provide a Loader at `internal/plugin/so/loader.go` that manages .so plugin loading with version support. Multiple versions of the same plugin CAN coexist. The Loader SHALL:
- `Load(path string, config string) (Plugin, error)` — open .so, lookup New, init, register
- `Get(name string, version string) (Plugin, bool)` — retrieve by name+version; empty version returns latest
- `List() []Plugin` — list all loaded plugins sorted by name, then version descending

#### Scenario: Load same plugin different versions
- **WHEN** shell-aes@1.0.0 and shell-aes@1.1.0 are both loaded
- **THEN** `Get("shell-aes", "")` returns version 1.1.0 (latest)
- **AND** `Get("shell-aes", "1.0.0")` returns version 1.0.0 (explicit)

#### Scenario: Duplicate version load
- **WHEN** shell-aes@1.0.0 is already loaded and another .so with same name@version is loaded
- **THEN** the second load fails with error `already registered`

### Requirement: SO Plugin logging
The Loader SHALL log the following events at INFO level:
- Plugin loaded: `name`, `version`, `ops`, `path`
- Plugin resolved (on Get): `name`, `version`, `latest` or `explicit`
- Plugin call success: `name`, `version`, `op`
- Plugin call failure: `name`, `version`, `op`, `error`

Every plugin invocation via `__so()` expression SHALL produce a log entry with the actual version called.

#### Scenario: Log on load
- **WHEN** shell-aes@1.0.0 is loaded
- **THEN** log line: `[so] plugin loaded: name=shell-aes version=1.0.0 ops=[encrypt decrypt] path=/data/plugins/shell-aes-1.0.0.so`

#### Scenario: Log on call
- **WHEN** `${__so("shell-aes", "encrypt", "data")}` is resolved and version 1.1.0 is called
- **THEN** log line: `[so] call success: plugin=shell-aes version=1.1.0 op=encrypt`

### Requirement: SO Plugin persistence
The system SHALL persist .so files to `data/plugins/` directory on upload. Plugin metadata (name, version, file path, status, config) SHALL be stored in the `so_plugins` database table. On Salvo startup, all plugins with `status=enabled` SHALL be automatically loaded.

#### Scenario: Upload and persist
- **WHEN** user uploads shell-aes.so via GUI
- **THEN** the file is saved to `data/plugins/shell-aes-1.0.0.so` and a `so_plugins` record is inserted with `status=enabled`

#### Scenario: Auto-load on restart
- **WHEN** Salvo restarts and there are 3 plugins with `status=enabled`
- **THEN** all 3 plugins are loaded via `Loader.Load()` at startup, and log entries are produced for each

### Requirement: SO Plugin status management
The system SHALL support two status values for SO plugins:
- `enabled`: plugin is loaded and available for use
- `disabled`: plugin is not loaded (will not be loaded on next restart)

Status transitions:
- Upload → enabled
- Disable (废弃) → disabled (current process keeps the plugin until restart)
- Enable → enabled (takes effect on next restart)
- Delete (删除) → physical file deleted + DB record removed (takes effect on next restart)

#### Scenario: Disable plugin
- **WHEN** user disables shell-aes@1.0.0
- **THEN** `so_plugins.status` is set to `disabled`, and on next restart the plugin is not loaded

#### Scenario: Delete plugin
- **WHEN** user deletes shell-aes@1.0.0
- **THEN** the .so file at `data/plugins/shell-aes-1.0.0.so` is deleted, the `so_plugins` record is removed, and on next restart the plugin is gone

### Requirement: SO Plugin expression integration
The system SHALL register `__so` as a system function in the expression engine. The function signature is `__so(pluginRef, op, args...)` where `pluginRef` is `"name"` or `"name@version"`. The function calls `Loader.Get(name, version)` then `Plugin.Call(op, args...)`.

#### Scenario: Expression with SO plugin
- **WHEN** expression `${__so("shell-aes", "encrypt", "13312345674")}` is resolved
- **THEN** the latest version of shell-aes is called with `Call("encrypt", "13312345674")` and the result replaces the expression

### Requirement: SO Plugin database model
The system SHALL create a `so_plugins` table with columns:
- `id` (TEXT, primary key, snowflake ID)
- `name` (TEXT, not null)
- `version` (TEXT, not null)
- `file_path` (TEXT, not null, relative path under data/plugins/)
- `status` (TEXT, not null, default 'enabled')
- `config` (TEXT, JSON string, default '')
- `ops` (TEXT, JSON array string, default '[]')
- `created_at` (TEXT, ISO8601)
- `updated_at` (TEXT, ISO8601)

Unique constraint on `(name, version)`.

#### Scenario: Query enabled plugins
- **WHEN** Salvo starts up
- **THEN** `SELECT * FROM so_plugins WHERE status = 'enabled'` returns all plugins to load
