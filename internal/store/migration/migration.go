// Package migration provides database schema creation and versioned
// migration support for the Salvo persistence layer.
package migration

import (
	"database/sql"
	"fmt"
)

// Migrate runs all migrations on the given database connection.
// For SQLite, it creates all tables if they do not exist.
func Migrate(db *sql.DB) error {
	migrations := []struct {
		name string
		sql  string
	}{
		{"scenes", createScenesTable},
		{"nodes", createNodesTable},
		{"edges", createEdgesTable},
		{"variables", createVariablesTable},
		{"plugin_configs", createPluginConfigsTable},
		{"reports", createReportsTable},
		{"report_details", createReportDetailsTable},
		{"run_records", createRunRecordsTable},
		{"traces", createTracesTable},
		{"spans", createSpansTable},
		{"roles", createRolesTable},
		{"permissions", createPermissionsTable},
		{"role_permissions", createRolePermissionsTable},
		{"users", createUsersTable},
		{"time_series_samples", createTimeSeriesSamplesTable},
		{"data_sources", createDataSourcesTable},
		{"so_plugins", createSoPluginsTable},
	}

	for _, m := range migrations {
		if _, err := db.Exec(m.sql); err != nil {
			return fmt.Errorf("migration %s: %w", m.name, err)
		}
	}

	alterMigrations := []string{
		`ALTER TABLE run_records ADD COLUMN p90_latency REAL DEFAULT 0`,
		`ALTER TABLE time_series_samples ADD COLUMN p90_latency_ms REAL DEFAULT 0`,
		`ALTER TABLE run_records ADD COLUMN count INTEGER DEFAULT 0`,
		`ALTER TABLE run_records ADD COLUMN run_id INTEGER DEFAULT 0`,
		`ALTER TABLE scenes ADD COLUMN default_timeout INTEGER DEFAULT 0`,
		`ALTER TABLE nodes ADD COLUMN block_on_error BOOLEAN DEFAULT FALSE`,
		`ALTER TABLE data_sources ADD COLUMN source TEXT NOT NULL DEFAULT 'csv'`,
		// Migration for report_details table
		`INSERT INTO report_details (report_id, detail) SELECT id, detail FROM reports WHERE detail IS NOT NULL AND detail != ''`,
		`ALTER TABLE reports DROP COLUMN detail`,
		// Add indexes for better query performance
		`CREATE INDEX IF NOT EXISTS idx_reports_run_id ON reports(run_id)`,
		`CREATE INDEX IF NOT EXISTS idx_reports_status ON reports(status)`,
		`CREATE INDEX IF NOT EXISTS idx_reports_started_at ON reports(started_at)`,
	}
	for _, sql := range alterMigrations {
		db.Exec(sql)
	}

	return ensureSchemaVersion(db)
}

const createScenesTable = `
CREATE TABLE IF NOT EXISTS scenes (
	id              INTEGER PRIMARY KEY,
	name            TEXT    NOT NULL,
	description     TEXT    DEFAULT '',
	dag_json        TEXT    DEFAULT '',
	variables       TEXT    DEFAULT '',
	plugins         TEXT    DEFAULT '',
	status          TEXT    NOT NULL DEFAULT 'draft',
	default_timeout INTEGER DEFAULT 0,
	created_at      DATETIME NOT NULL,
	updated_at      DATETIME NOT NULL,
	deleted_at      DATETIME DEFAULT NULL
);
CREATE INDEX IF NOT EXISTS idx_scenes_status ON scenes(status);
CREATE INDEX IF NOT EXISTS idx_scenes_deleted_at ON scenes(deleted_at);
`

const createNodesTable = `
CREATE TABLE IF NOT EXISTS nodes (
	id              INTEGER PRIMARY KEY,
	scene_id        INTEGER NOT NULL,
	name            TEXT    NOT NULL,
	type            TEXT    NOT NULL,
	config          TEXT    DEFAULT '',
	position        TEXT    DEFAULT '',
	loop_count      INTEGER DEFAULT 1,
	created_at      DATETIME NOT NULL,
	updated_at      DATETIME NOT NULL,
	deleted_at      DATETIME DEFAULT NULL,
	FOREIGN KEY (scene_id) REFERENCES scenes(id)
);
CREATE INDEX IF NOT EXISTS idx_nodes_scene_id ON nodes(scene_id);
CREATE INDEX IF NOT EXISTS idx_nodes_deleted_at ON nodes(deleted_at);
`

const createEdgesTable = `
CREATE TABLE IF NOT EXISTS edges (
	id              INTEGER PRIMARY KEY,
	scene_id        INTEGER NOT NULL,
	from_node       INTEGER NOT NULL,
	to_node         INTEGER NOT NULL,
	condition       TEXT    DEFAULT '',
	priority        INTEGER DEFAULT 0,
	created_at      DATETIME NOT NULL,
	updated_at      DATETIME NOT NULL,
	deleted_at      DATETIME DEFAULT NULL,
	FOREIGN KEY (scene_id) REFERENCES scenes(id),
	FOREIGN KEY (from_node) REFERENCES nodes(id),
	FOREIGN KEY (to_node) REFERENCES nodes(id)
);
CREATE INDEX IF NOT EXISTS idx_edges_scene_id ON edges(scene_id);
CREATE INDEX IF NOT EXISTS idx_edges_deleted_at ON edges(deleted_at);
`

const createVariablesTable = `
CREATE TABLE IF NOT EXISTS variables (
	id              INTEGER PRIMARY KEY,
	scene_id        INTEGER NOT NULL,
	scope           TEXT    NOT NULL,
	key             TEXT    NOT NULL,
	value           TEXT    DEFAULT '',
	created_at      DATETIME NOT NULL,
	updated_at      DATETIME NOT NULL,
	deleted_at      DATETIME DEFAULT NULL,
	FOREIGN KEY (scene_id) REFERENCES scenes(id)
);
CREATE INDEX IF NOT EXISTS idx_variables_scene_id ON variables(scene_id);
CREATE INDEX IF NOT EXISTS idx_variables_scope_key ON variables(scope, key);
CREATE INDEX IF NOT EXISTS idx_variables_deleted_at ON variables(deleted_at);
`

const createPluginConfigsTable = `
CREATE TABLE IF NOT EXISTS plugin_configs (
	id              INTEGER PRIMARY KEY,
	scene_id        INTEGER NOT NULL,
	name            TEXT    NOT NULL,
	type            TEXT    NOT NULL,
	config          TEXT    DEFAULT '',
	phase           TEXT    NOT NULL DEFAULT 'before',
	priority        INTEGER DEFAULT 0,
	enabled         BOOLEAN DEFAULT 1,
	created_at      DATETIME NOT NULL,
	updated_at      DATETIME NOT NULL,
	deleted_at      DATETIME DEFAULT NULL,
	FOREIGN KEY (scene_id) REFERENCES scenes(id)
);
CREATE INDEX IF NOT EXISTS idx_plugin_configs_scene_id ON plugin_configs(scene_id);
CREATE INDEX IF NOT EXISTS idx_plugin_configs_deleted_at ON plugin_configs(deleted_at);
`

const createReportsTable = `
CREATE TABLE IF NOT EXISTS reports (
	id              INTEGER PRIMARY KEY,
	scene_id        INTEGER NOT NULL,
	run_id          INTEGER NOT NULL,
	status          TEXT    NOT NULL,
	summary         TEXT    DEFAULT '',
	detail          TEXT    DEFAULT '',
	started_at      DATETIME DEFAULT NULL,
	finished_at     DATETIME DEFAULT NULL,
	created_at      DATETIME NOT NULL,
	updated_at      DATETIME NOT NULL,
	deleted_at      DATETIME DEFAULT NULL,
	FOREIGN KEY (scene_id) REFERENCES scenes(id)
);
CREATE INDEX IF NOT EXISTS idx_reports_scene_id ON reports(scene_id);
CREATE INDEX IF NOT EXISTS idx_reports_deleted_at ON reports(deleted_at);
`

const createRunRecordsTable = `
CREATE TABLE IF NOT EXISTS run_records (
	id              INTEGER PRIMARY KEY,
	scene_id        INTEGER NOT NULL,
	status          TEXT    NOT NULL,
	worker_count    INTEGER DEFAULT 1,
	run_mode        TEXT    DEFAULT 'count',
	duration        REAL    DEFAULT 0,
	count           INTEGER DEFAULT 0,
	total_reqs      INTEGER DEFAULT 0,
	success_reqs    INTEGER DEFAULT 0,
	failed_reqs     INTEGER DEFAULT 0,
	avg_latency     REAL    DEFAULT 0,
	p50_latency     REAL    DEFAULT 0,
	p90_latency     REAL    DEFAULT 0,
	p95_latency     REAL    DEFAULT 0,
	p99_latency     REAL    DEFAULT 0,
	error_msg       TEXT    DEFAULT '',
	started_at      DATETIME DEFAULT NULL,
	finished_at     DATETIME DEFAULT NULL,
	created_at      DATETIME NOT NULL,
	updated_at      DATETIME NOT NULL,
	deleted_at      DATETIME DEFAULT NULL,
	FOREIGN KEY (scene_id) REFERENCES scenes(id)
);
CREATE INDEX IF NOT EXISTS idx_run_records_scene_id ON run_records(scene_id);
CREATE INDEX IF NOT EXISTS idx_run_records_status ON run_records(status);
CREATE INDEX IF NOT EXISTS idx_run_records_deleted_at ON run_records(deleted_at);
`

const createSchemaVersionTable = `
CREATE TABLE IF NOT EXISTS schema_version (
	version     INTEGER PRIMARY KEY,
	applied_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);
`

const currentVersion = 6

func ensureSchemaVersion(db *sql.DB) error {
	if _, err := db.Exec(createSchemaVersionTable); err != nil {
		return fmt.Errorf("create schema_version: %w", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_version WHERE version = ?`, currentVersion).Scan(&count); err != nil {
		return fmt.Errorf("check schema version: %w", err)
	}

	if count == 0 {
		if _, err := db.Exec(`INSERT INTO schema_version (version) VALUES (?)`, currentVersion); err != nil {
			return fmt.Errorf("insert schema version: %w", err)
		}
	}

	return nil
}

const createTracesTable = `
CREATE TABLE IF NOT EXISTS traces (
	id              INTEGER PRIMARY KEY,
	scene_id        INTEGER NOT NULL,
	run_id          INTEGER NOT NULL,
	status          TEXT    NOT NULL DEFAULT 'ok',
	error           TEXT    DEFAULT '',
	started_at      DATETIME NOT NULL,
	finished_at     DATETIME DEFAULT NULL,
	duration_ns     INTEGER DEFAULT 0,
	created_at      DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_traces_scene_id ON traces(scene_id);
CREATE INDEX IF NOT EXISTS idx_traces_run_id ON traces(run_id);
CREATE INDEX IF NOT EXISTS idx_traces_status ON traces(status);
`

const createSpansTable = `
CREATE TABLE IF NOT EXISTS spans (
	id              INTEGER PRIMARY KEY,
	trace_id        INTEGER NOT NULL,
	node_id         TEXT    NOT NULL,
	status          TEXT    NOT NULL DEFAULT 'ok',
	error           TEXT    DEFAULT '',
	input           TEXT    DEFAULT '',
	output          TEXT    DEFAULT '',
	started_at      DATETIME NOT NULL,
	finished_at     DATETIME DEFAULT NULL,
	duration_ns     INTEGER DEFAULT 0,
	FOREIGN KEY (trace_id) REFERENCES traces(id)
);
CREATE INDEX IF NOT EXISTS idx_spans_trace_id ON spans(trace_id);
CREATE INDEX IF NOT EXISTS idx_spans_node_id ON spans(node_id);
`

const createUsersTable = `
CREATE TABLE IF NOT EXISTS users (
	id              INTEGER PRIMARY KEY,
	email           TEXT    NOT NULL UNIQUE,
	password_hash   TEXT    NOT NULL,
	nickname        TEXT    NOT NULL DEFAULT '',
	role_id         INTEGER NOT NULL,
	status          TEXT    NOT NULL DEFAULT 'active',
	last_login_at   DATETIME DEFAULT NULL,
	created_at      DATETIME NOT NULL,
	updated_at      DATETIME NOT NULL,
	deleted_at      DATETIME DEFAULT NULL,
	FOREIGN KEY (role_id) REFERENCES roles(id)
);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_role_id ON users(role_id);
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);
`

const createRolesTable = `
CREATE TABLE IF NOT EXISTS roles (
	id              INTEGER PRIMARY KEY,
	name            TEXT    NOT NULL UNIQUE,
	description     TEXT    DEFAULT '',
	is_builtin      BOOLEAN DEFAULT 0,
	created_at      DATETIME NOT NULL,
	updated_at      DATETIME NOT NULL,
	deleted_at      DATETIME DEFAULT NULL
);
CREATE INDEX IF NOT EXISTS idx_roles_name ON roles(name);
CREATE INDEX IF NOT EXISTS idx_roles_deleted_at ON roles(deleted_at);
`

const createPermissionsTable = `
CREATE TABLE IF NOT EXISTS permissions (
	id              INTEGER PRIMARY KEY,
	resource        TEXT    NOT NULL,
	action          TEXT    NOT NULL,
	description     TEXT    DEFAULT '',
	created_at      DATETIME NOT NULL,
	UNIQUE(resource, action)
);
`

const createRolePermissionsTable = `
CREATE TABLE IF NOT EXISTS role_permissions (
	role_id         INTEGER NOT NULL,
	permission_id   INTEGER NOT NULL,
	PRIMARY KEY (role_id, permission_id),
	FOREIGN KEY (role_id) REFERENCES roles(id),
	FOREIGN KEY (permission_id) REFERENCES permissions(id)
);
`

// CurrentVersion returns the current schema version number.
func CurrentVersion() int {
	return currentVersion
}

// RollbackReportDetailsMigration rolls back the report_details table migration.
// This function restores the original reports.detail column and removes the report_details table.
// WARNING: This function should only be used during development or for emergency rollbacks.
func RollbackReportDetailsMigration(db *sql.DB) error {
	rollbackMigrations := []string{
		// Add detail column back to reports table
		`ALTER TABLE reports ADD COLUMN detail TEXT DEFAULT ''`,
		// Restore data from report_details to reports
		`UPDATE reports SET detail = (SELECT detail FROM report_details WHERE report_id = reports.id)`,
		// Drop report_details table
		`DROP TABLE IF EXISTS report_details`,
	}

	for _, sql := range rollbackMigrations {
		if _, err := db.Exec(sql); err != nil {
			return fmt.Errorf("rollback: %w", err)
		}
	}

	return nil
}

const createTimeSeriesSamplesTable = `
CREATE TABLE IF NOT EXISTS time_series_samples (
	id              INTEGER PRIMARY KEY AUTOINCREMENT,
	run_id          INTEGER    NOT NULL,
	node_id         TEXT       NOT NULL DEFAULT '',
	sample_time     DATETIME   NOT NULL,
	window_duration INTEGER    NOT NULL DEFAULT 1,
	qps             REAL       NOT NULL DEFAULT 0,
	total_requests  INTEGER    NOT NULL DEFAULT 0,
	success_count   INTEGER    NOT NULL DEFAULT 0,
	fail_count      INTEGER    NOT NULL DEFAULT 0,
	avg_latency_ms  REAL       NOT NULL DEFAULT 0,
	p50_latency_ms  REAL       NOT NULL DEFAULT 0,
	p90_latency_ms  REAL       NOT NULL DEFAULT 0,
	p95_latency_ms  REAL       NOT NULL DEFAULT 0,
	p99_latency_ms  REAL       NOT NULL DEFAULT 0,
	min_latency_ms  REAL       NOT NULL DEFAULT 0,
	max_latency_ms  REAL       NOT NULL DEFAULT 0,
	created_at      DATETIME   NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uk_run_node_time UNIQUE (run_id, node_id, sample_time)
);
CREATE INDEX IF NOT EXISTS idx_ts_run_node ON time_series_samples (run_id, node_id);
CREATE INDEX IF NOT EXISTS idx_ts_run_time ON time_series_samples (run_id, sample_time);
CREATE INDEX IF NOT EXISTS idx_ts_sample_time ON time_series_samples (sample_time);
`

const createDataSourcesTable = `
CREATE TABLE IF NOT EXISTS data_sources (
	id              INTEGER PRIMARY KEY,
	scene_id        INTEGER NOT NULL,
	name            TEXT    NOT NULL,
	file_name       TEXT    NOT NULL,
	columns         TEXT    NOT NULL DEFAULT '[]',
	rows            TEXT    NOT NULL DEFAULT '[]',
	row_count       INTEGER NOT NULL DEFAULT 0,
	source          TEXT    NOT NULL DEFAULT 'csv',
	created_at      DATETIME NOT NULL,
	updated_at      DATETIME NOT NULL,
	deleted_at      DATETIME DEFAULT NULL,
	FOREIGN KEY (scene_id) REFERENCES scenes(id)
);
CREATE INDEX IF NOT EXISTS idx_data_sources_scene_id ON data_sources(scene_id);
CREATE INDEX IF NOT EXISTS idx_data_sources_name ON data_sources(name);
CREATE INDEX IF NOT EXISTS idx_data_sources_deleted_at ON data_sources(deleted_at);
`

const createSoPluginsTable = `
CREATE TABLE IF NOT EXISTS so_plugins (
	id              INTEGER PRIMARY KEY,
	name            TEXT    NOT NULL,
	version         TEXT    NOT NULL,
	file_path       TEXT    NOT NULL,
	status          TEXT    NOT NULL DEFAULT 'disabled',
	config          TEXT    DEFAULT '',
	created_at      DATETIME NOT NULL,
	updated_at      DATETIME NOT NULL,
	deleted_at      DATETIME DEFAULT NULL
);
CREATE INDEX IF NOT EXISTS idx_so_plugins_name ON so_plugins(name);
CREATE INDEX IF NOT EXISTS idx_so_plugins_status ON so_plugins(status);
CREATE INDEX IF NOT EXISTS idx_so_plugins_deleted_at ON so_plugins(deleted_at);
`

const createReportDetailsTable = `
CREATE TABLE IF NOT EXISTS report_details (
	report_id       INTEGER PRIMARY KEY,
	detail          TEXT,
	FOREIGN KEY (report_id) REFERENCES reports(id) ON DELETE CASCADE
);
`
