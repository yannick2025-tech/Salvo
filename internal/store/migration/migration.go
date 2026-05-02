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
		{"run_records", createRunRecordsTable},
		{"traces", createTracesTable},
		{"spans", createSpansTable},
	}

	for _, m := range migrations {
		if _, err := db.Exec(m.sql); err != nil {
			return fmt.Errorf("migration %s: %w", m.name, err)
		}
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
	total_reqs      INTEGER DEFAULT 0,
	success_reqs    INTEGER DEFAULT 0,
	failed_reqs     INTEGER DEFAULT 0,
	avg_latency     REAL    DEFAULT 0,
	p50_latency     REAL    DEFAULT 0,
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

const currentVersion = 2

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

// CurrentVersion returns the current schema version number.
func CurrentVersion() int {
	return currentVersion
}
