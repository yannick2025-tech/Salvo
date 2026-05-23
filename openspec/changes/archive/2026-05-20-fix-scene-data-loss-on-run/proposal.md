## Why

When `Runner.Run()` updates the scene status via `HookSceneSetup` and `HookSceneTeardown`, the `SceneRepo.Update()` SQL writes ALL columns (`name`, `description`, `dag_json`, `variables`, `plugins`, `status`), but the `Scene` struct passed to `Update()` only has `Model.ID`, `Name`, and `Status` populated. This causes `description`, `dag_json`, `variables`, and `plugins` to be overwritten with empty strings, resulting in **permanent data loss on every scene run**.

This means after the first run, the scene loses its description, DAG JSON, and variables. If the user later edits the scene or views the DAG, the data is gone.

## What Changes

- `internal/store/sqlite/sqlite.go`: Change `SceneRepo.Update()` to use dynamic SQL that only updates non-zero/non-empty fields (incremental update pattern)
- `internal/runner/runner.go`: Simplify `HookSceneSetup` and `HookSceneTeardown` to only pass the `status` field for update, relying on the new incremental update logic
- No breaking API changes — fixes existing behavior without changing interfaces

## Capabilities

### New Capabilities
- `scene-data-integrity`: Scene data (description, dag_json, variables, plugins) must be preserved when scene status is updated during run lifecycle

### Modified Capabilities
- *(none — this is a bug fix, not a requirement change)*

## Impact

- **Affected code**: `internal/store/sqlite/sqlite.go` (`SceneRepo.Update`), `internal/runner/runner.go` (lifecycle hooks)
- **Affected data**: All existing scenes that have been run (even once) have already lost their `description`, `dag_json`, `variables`, and `plugins`. These values are **not recoverable** from the database — user must re-enter them
- **No API changes**: All endpoints remain the same
- **No migration needed**: Fix prevents future data loss; existing corrupted data is not fixable by code

---

## Reproduction Steps

> Follow these steps to confirm the bug exists before applying the fix:

### Prerequisites
- Salvo server running locally
- A scene with complete data: name, description, DAG nodes/edges, and variables

### Step-by-step

1. **Create a scene** with description, variables, and DAG nodes (via "导入 YAML" or the frontend editor)

2. **Verify scene data is intact** — run the following SQL to confirm all fields are populated:
   ```bash
   sqlite3 -header /Users/xiongyang/Desktop/home/code/snailx/salvo.db \
     "SELECT id, name, description, status, dag_json, variables FROM scenes WHERE id=<SCENE_ID>;"
   ```
   **Expected**: `description`, `dag_json`, `variables` are non-empty.

3. **Start the scene** — click "启动测试" from the scene detail page or run control page. Let it run for a few seconds.

4. **Stop the scene** — click the stop button.

5. **Verify data loss** — re-run the SQL query:
   ```bash
   sqlite3 -header /Users/xiongyang/Desktop/home/code/snailx/salvo.db \
     "SELECT id, name, description, status, dag_json, variables FROM scenes WHERE id=<SCENE_ID>;"
   ```
   **Actual (bug)**: `description`, `dag_json`, `variables`, `plugins` are now **empty strings**.
   **Expected (after fix)**: `description`, `dag_json`, `variables`, `plugins` are **preserved** with their original values.

### Root Cause Code Path

```go
// runner.go:303-310 — HookSceneSetup passes incomplete Scene struct
lc.Register(lifecycle.HookSceneSetup, func(ctx context.Context) error {
    return r.scenes.Update(ctx, &model.Scene{
        Model:  scene.Model,                            // only ID, CreatedAt, UpdatedAt
        Name:   scene.Name,
        Status: model.SceneStatusRunning,                // Description/DAGJSON/Variables/Plugins are empty!
    })
})

// sqlite.go:115-122 — Update writes ALL fields unconditionally
func (r *SceneRepo) Update(ctx context.Context, scene *model.Scene) error {
    _, err := r.db.ExecContext(ctx, `
        UPDATE scenes SET name=?, description=?, dag_json=?, variables=?, plugins=?, status=?, updated_at=?
        WHERE id=? AND deleted_at IS NULL`,
        scene.Name, scene.Description, scene.DAGJSON, scene.Variables,
        scene.Plugins, scene.Status, scene.UpdatedAt, scene.ID)
    return err
}
```