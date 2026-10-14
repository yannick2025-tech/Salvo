## Context

When `Runner.Run()` calls `HookSceneSetup` and `HookSceneTeardown`, the scene status is updated via `SceneRepo.Update()`. This method executes a full-column UPDATE SQL that overwrites ALL fields (`name`, `description`, `dag_json`, `variables`, `plugins`, `status`), but the lifecycle hooks only populate `Model.ID`, `Name`, and `Status` — leaving `Description`, `DAGJSON`, `Variables`, and `Plugins` as empty strings. The result is permanent data loss after every scene run.

There are exactly 3 callers of `SceneRepo.Update()`:

| Caller | Fields Set | Safe? |
|--------|-----------|-------|
| `handler.go:291` (UpdateScene API) | Loads existing scene first, then selectively overrides | ✅ Yes |
| `runner.go:305` (HookSceneSetup) | Only `ID`, `Name`, `Status` | ❌ Wipes data |
| `runner.go:312` (HookSceneTeardown) | Only `ID`, `Name`, `Status` | ❌ Wipes data |

## Goals / Non-Goals

**Goals:**
- Scene `description`, `dag_json`, `variables`, and `plugins` must be preserved when `HookSceneSetup` or `HookSceneTeardown` updates the scene status
- No behavior change for the `UpdateScene` API handler (caller 1)
- Existing tests must continue to pass

**Non-Goals:**
- Recovery of already-lost data (impossible without a backup)
- Changes to the `Scene` model or database schema
- API contract changes

## Decisions

### Decision 1: Add `UpdateStatus` method rather than modifying `Update`

**Chosen: Add `SceneRepo.UpdateStatus(ctx, id snowflake.ID, status string) error`**

Options considered:
- **A — Dynamic SQL in `Update`**: Detect non-empty fields and build SET clause dynamically. Rejected because it's fragile — some fields (like `description`) can legitimately be empty strings, making it impossible to distinguish "clear the field" from "don't update it."
- **B — Add `UpdateStatus` method** ✅: Clean separation of concerns. The `Update` method continues to work as before for full scene updates (API handler). The new method handles the specific use case of status-only updates with a simple, targeted SQL statement.
- **C — Pointer fields (`*string`)**: Use pointers to distinguish nil (don't update) from empty (clear). Rejected as over-engineering — it would require changing the `Scene` model and all callers.

**Rationale**: The three callers have fundamentally different semantics (full update vs. status-only), so separate methods provide clearer intent and safer defaults.

### Decision 2: HookSceneTeardown sets status to "completed" unconditionally

The existing teardown hook always sets status to `SceneStatusCompleted`. This is preserved. No change to business logic — only the persistence mechanism changes.

## Risks / Trade-offs

- **[Low] Existing corrupted data not recoverable**: Scenes that have already been run have lost their fields permanently. The fix only prevents future data loss. Users must re-enter lost data manually.
- **[Low] API handler future change**: If the `UpdateScene` handler ever needs to clear a field to empty (e.g., removing all variables), the current approach wouldn't support it. This is a pre-existing limitation, not introduced by this fix.
- **[Low] Test gap**: The existing test for `SceneRepo.Update` (`sqlite_test.go`) doesn't verify partial updates. A new test for `UpdateStatus` will be added to close this gap.