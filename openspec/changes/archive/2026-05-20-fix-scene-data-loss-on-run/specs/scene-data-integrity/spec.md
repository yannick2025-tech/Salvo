## ADDED Requirements

### Requirement: Scene data integrity on status update

When the Runner updates a scene's status via lifecycle hooks, the system SHALL preserve all other scene fields (description, dag_json, variables, plugins) unchanged.

#### Scenario: HookSceneSetup preserves scene data
- **WHEN** `Runner.Run()` executes `HookSceneSetup` and calls `SceneRepo.UpdateStatus()` to set scene status to "running"
- **THEN** the scene's `description`, `dag_json`, `variables`, and `plugins` columns SHALL remain unchanged from their values before the update

#### Scenario: HookSceneTeardown preserves scene data
- **WHEN** `Runner.Run()` completes and executes `HookSceneTeardown` to set scene status to "completed"
- **THEN** the scene's `description`, `dag_json`, `variables`, and `plugins` columns SHALL remain unchanged from their values before the update

#### Scenario: UpdateScene API unchanged
- **WHEN** the `UpdateScene` API handler calls `SceneRepo.Update()` with a full Scene struct
- **THEN** all provided fields SHALL be updated as before — the existing `Update()` method behavior MUST remain unchanged

#### Scenario: UpdateStatus only modifies status and updated_at
- **WHEN** `SceneRepo.UpdateStatus()` is called with a scene ID and a new status
- **THEN** the SQL UPDATE SHALL only set the `status` and `updated_at` columns, leaving all other columns (`name`, `description`, `dag_json`, `variables`, `plugins`) untouched