## MODIFIED Requirements

### Requirement: Scene data integrity on status update

When the Runner updates a scene's status via lifecycle hooks, the system SHALL preserve all other scene fields (description, dag_json, variables, plugins, data_sources) unchanged.

#### Scenario: HookSceneSetup preserves scene data
- **WHEN** `Runner.Run()` executes `HookSceneSetup` and calls `SceneRepo.UpdateStatus()` to set scene status to "running"
- **THEN** the scene's `description`, `dag_json`, `variables`, `plugins`, and `data_sources` columns SHALL remain unchanged from their values before the update

#### Scenario: HookSceneTeardown preserves scene data
- **WHEN** `Runner.Run()` completes and executes `HookSceneTeardown` to set scene status to "completed"
- **THEN** the scene's `description`, `dag_json`, `variables`, `plugins`, and `data_sources` columns SHALL remain unchanged from their values before the update

#### Scenario: UpdateScene API unchanged
- **WHEN** the `UpdateScene` API handler calls `SceneRepo.Update()` with a full Scene struct
- **THEN** all provided fields SHALL be updated as before — the existing `Update()` method behavior MUST remain unchanged

#### Scenario: UpdateStatus only modifies status and updated_at
- **WHEN** `SceneRepo.UpdateStatus()` is called with a scene ID and a new status
- **THEN** the SQL UPDATE SHALL only set the `status` and `updated_at` columns, leaving all other columns (`name`, `description`, `dag_json`, `variables`, `plugins`, `data_sources`) untouched

## ADDED Requirements

### Requirement: YAML import supports new node types and data sources
The YAML import handler SHALL support parsing `group` and `timer` node types, and a top-level `data_sources` section. Unknown node types SHALL produce a validation error.

#### Scenario: Import YAML with group node
- **WHEN** YAML contains a node with `type: group` and `config: {node_ids: ["A","B"], loop_count: 2}`
- **THEN** the import creates a Node with type="group" and resolves node names to IDs in the config

#### Scenario: Import YAML with timer node
- **WHEN** YAML contains a node with `type: timer` and `config: {mode: "interval", seconds: 10}`
- **THEN** the import creates a Node with type="timer" and the specified config

#### Scenario: Import YAML with data sources
- **WHEN** YAML contains `data_sources: [{name: users, file: users.csv}]`
- **THEN** the import links the existing data source named "users" to the scene

### Requirement: YAML export includes new node types and data sources
The YAML export (from DagFlow.vue) SHALL serialize group and timer nodes with their full config, and include a `data_sources` section if the scene has linked data sources.

#### Scenario: Export scene with group node
- **WHEN** user exports YAML from a scene containing a group node
- **THEN** the YAML includes the group node with type, node_ids (as names), and loop_count

#### Scenario: Export scene with data sources
- **WHEN** user exports YAML from a scene with linked data sources
- **THEN** the YAML includes a `data_sources` section listing each data source name and file reference
