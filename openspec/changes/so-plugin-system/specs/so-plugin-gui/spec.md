## ADDED Requirements

### Requirement: SO Plugin management page
The system SHALL provide a SO Plugin management page at route `/plugins` with label "SO 插件管理". The page SHALL only be visible to users with admin role (permission: `plugins:read`). Non-admin users SHALL NOT see this menu item.

#### Scenario: Admin sees plugin menu
- **WHEN** a user with role "admin" is logged in
- **THEN** the sidebar shows "SO 插件管理" menu item

#### Scenario: Non-admin cannot see plugin menu
- **WHEN** a user with role "user" is logged in
- **THEN** the sidebar does NOT show "SO 插件管理" menu item

### Requirement: SO Plugin upload
The page SHALL provide a file upload button that accepts `.so` files. On upload, the file is sent to `POST /api/v1/plugins/so/upload` as multipart form data. The server loads the plugin immediately, extracts Name/Version/Ops, and persists metadata. The upload response SHALL include the plugin's name, version, and supported operations.

#### Scenario: Successful upload
- **WHEN** admin uploads `shell-aes-1.0.0.so`
- **THEN** the server loads it, returns `{name: "shell-aes", version: "1.0.0", ops: ["encrypt", "decrypt"]}`, and the plugin appears in the list

#### Scenario: Upload incompatible .so
- **WHEN** admin uploads a .so compiled with a different Go version
- **THEN** the server returns an error, and the file is not persisted

### Requirement: SO Plugin list
The page SHALL display a table of all registered SO plugins with columns:
- Plugin Name
- Version
- Supported Operations (comma-separated)
- Status (enabled/disabled, shown as colored badge)
- Created At
- Actions (Disable/Enable, Delete)

The table SHALL be sorted by name ascending, then version descending. Style SHALL be consistent with existing pages (UsersPage.vue, SettingsPage.vue).

#### Scenario: List with multiple versions
- **WHEN** shell-aes@1.0.0 and shell-aes@1.1.0 are both registered
- **THEN** both rows are displayed, with 1.1.0 appearing above 1.0.0

### Requirement: SO Plugin disable/enable
Each plugin row SHALL have a "Disable" (废弃) button when status is enabled, and "Enable" button when status is disabled. Clicking calls `PUT /api/v1/plugins/so/:id/status` with `{status: "disabled"}` or `{status: "enabled"}`. A confirmation dialog SHALL appear before disabling.

#### Scenario: Disable plugin
- **WHEN** admin clicks "Disable" on shell-aes@1.0.0 and confirms
- **THEN** the status changes to "disabled" in the table, and a toast shows "插件已废弃，重启后生效"

### Requirement: SO Plugin delete
Each plugin row SHALL have a "Delete" (删除) button. Clicking calls `DELETE /api/v1/plugins/so/:id`. A confirmation dialog SHALL appear with the message "确定要删除此插件吗？此操作不可恢复，重启后生效。"

#### Scenario: Delete plugin
- **WHEN** admin clicks "Delete" on shell-aes@1.0.0 and confirms
- **THEN** the .so file is deleted from disk, the DB record is removed, and the row disappears from the table on next refresh

### Requirement: SO Plugin config editing
Each plugin row SHALL have a "Config" button that opens a modal with a JSON editor textarea. The config is saved via `PUT /api/v1/plugins/so/:id/config` with `{config: "..."}`. The config is passed to `Plugin.Init(config)` on next load (restart).

#### Scenario: Edit plugin config
- **WHEN** admin edits the config of shell-aes to `{"default_key": "AAA=", "default_iv": "BBB="}` and saves
- **THEN** the config is persisted, and on next restart `Init()` is called with the new config

### Requirement: Settings page rename
The current "系统设置" page at `/settings` SHALL be renamed to "个人设置". The page content (password change + personal info) remains unchanged. The sidebar menu label SHALL change from "系统设置" to "个人设置".

#### Scenario: Menu label change
- **WHEN** any user views the sidebar
- **THEN** the settings menu item shows "个人设置" instead of "系统设置"

### Requirement: SO Plugin management API endpoints
The system SHALL provide the following REST API endpoints under `/api/v1/plugins/so`:
- `POST /upload` — upload .so file (multipart), load plugin, persist metadata
- `GET /` — list all plugins
- `GET /:id` — get plugin details
- `PUT /:id/status` — update plugin status (enabled/disabled)
- `PUT /:id/config` — update plugin config
- `DELETE /:id` — delete plugin (file + DB record)

All endpoints SHALL require admin role authentication.

#### Scenario: Non-admin API access
- **WHEN** a non-admin user calls `GET /api/v1/plugins/so/`
- **THEN** the server returns 403 Forbidden
