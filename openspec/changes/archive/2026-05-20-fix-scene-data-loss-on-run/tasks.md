## 1. Add `UpdateStatus` method to `SceneRepo`

- [x] 1.1 Add `UpdateStatus(ctx context.Context, id snowflake.ID, status string) error` to `SceneRepo` interface in `internal/store/repo/repo.go`
- [x] 1.2 Implement `UpdateStatus` in `internal/store/sqlite/sqlite.go` with targeted SQL that only updates `status` and `updated_at` columns
- [x] 1.3 Run existing tests to confirm no regression

## 2. Fix Runner lifecycle hooks

- [x] 2.1 Replace `r.scenes.Update()` call in `HookSceneSetup` with `r.scenes.UpdateStatus()` to only update status to "running"
- [x] 2.2 Replace `r.scenes.Update()` call in `HookSceneTeardown` with `r.scenes.UpdateStatus()` to only update status to "completed"

## 3. Verify fix

- [x] 3.1 Run `go build ./...` to ensure compilation passes
- [x] 3.2 Run `go test ./internal/store/sqlite/...` to confirm tests pass
- [x] 3.3 Follow reproduction steps in `proposal.md` to verify data is preserved after a scene run