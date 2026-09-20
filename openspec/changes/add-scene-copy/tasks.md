# Tasks: add-scene-copy

## 1. 后端数据层：SceneRepo.CopyTx

- [x] 1.1 在 `internal/store/sqlite/sqlite_test.go` 新增 `TestSceneRepoCopyTx`（TDD 红灯先行）：创建含多类型节点（http/delay/group）、连线（含 condition/priority）、变量、CSV 数据源（含 rows）的源场景，断言复制后新场景 nodes/edges/data_sources 行数与逐字段内容一致、所有实体为新 snowflake ID、status=draft、dag_json 为空、description 为空
- [x] 1.2 扩展测试用例：group 节点 config.node_ids 重写（新 group 指向新子节点 ID）、edges from/to 映射重写、变量/数据源名称引用逐字节不变、run_records/报告不复制（源场景预置运行记录后断言新场景为 0 条）
- [x] 1.3 扩展测试用例：失败回滚——构造悬空边与 group 引用未知节点 ID 两种脏数据源场景，断言返回错误且数据库无新场景及任何关联行残留
- [x] 1.4 在 `internal/store/sqlite/sqlite.go` 实现 `SceneRepo.CopyTx(ctx, srcID, newName, newDesc) (*model.Scene, error)`：单事务（BeginTx + defer Rollback + Commit，参照 ReportRepo.Create 惯例）内按 scenes → nodes（建 oldID→newID 映射）→ edges（映射重写）→ group config.node_ids（映射重写）→ data_sources（rows 整体直插）顺序复制；映射缺失时返回错误触发回滚
- [x] 1.5 确认事务内查询源数据使用 `tx.QueryContext`（读旧写新同事务，避免复制期间源场景被并发修改产生不一致快照）

## 2. 后端 API 层：POST /api/v1/scenes/copy

- [x] 2.1 在 `internal/api/dto/dto.go` 新增 `CopySceneRequest{ SceneID string \`json:"scene_id"\`, Name string \`json:"name"\` }`
- [x] 2.2 在 `internal/api/server_test.go` 新增 `TestCopyScene` 集成测试（TDD 红灯先行）：成功路径（200 + 返回 SceneDTO，ID 为 string）+ 失败路径（名称空→400、scene_id 不存在→400、重名→409）
- [x] 2.3 在 `internal/api/handler.go` 新增 `handleCopyScene`：校验 name 非空 → GetByID 校验源场景存在（含重名冲突检查，语义对齐 CreateScene）→ 调 `SceneRepo.CopyTx`（description 固定为空）→ `toSceneDTO` 返回；错误统一走 `dto.ErrorResp`，禁重复 WriteHeader
- [x] 2.4 在 `internal/api/server.go` 注册路由 `POST /api/v1/scenes/copy`，鉴权与 UpdateScene 同组（handleAuth + 场景写权限）
- [x] 2.5 `go test ./internal/store/sqlite/ ./internal/api/ -count=1` 全绿 + `go vet ./...` 无告警

## 3. 前端：复制按钮接入新 API

- [x] 3.1 在 `web/app/src/api/scene.ts` 新增 `copyScene(sceneId: string, name: string): Promise<SceneDTO>`
- [x] 3.2 修改 `web/app/src/views/scenes/SceneDetailPage.vue` 的 `handleCopyScene`：改调 `copyScene`；成功 → toast 成功提示 + `router.push('/scenes')`；失败 → toast 展示后端错误信息、停留在当前页（弹窗保持可改名重试）
- [x] 3.3 `npx vite build` 通过；浏览器手工验证：复制含 group 节点 + CSV 数据源的场景 → 跳转列表 → 进入新场景编辑页确认 DAG/变量/数据源完整 → 直接启动测试可运行

## 4. 收尾

- [x] 4.1 `codegraph sync` 同步索引；按 code-review-checklist 自检（writeJSON 单次写入、错误小写开头、`fmt.Errorf("%w")` 包装、goroutine 无新增）
- [x] 4.2 文档影响评估：API 新增端点若 docs 有接口清单则补充；确认无需 OpenSpec 之外的文档变更
- [x] 4.3 生成 commit message（Conventional Commits，feat: add scene copy ...），待用户确认后提交
