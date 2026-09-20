# 提案：场景完整复制

## Why

场景编辑页的"复制"按钮当前只调用 `createScene({name, description})` 创建了一个空场景——DAG、节点、连线、变量、CSV 数据源全部丢失，用户复制后拿到的是一张白纸，需要手工重建整个测试场景，与"复制"的语义完全相悖。同时复制成功后仍停留在老场景编辑页，用户容易误以为自己正在编辑的是新场景。

## What Changes

- 新增后端 API `POST /api/v1/scenes/copy`（请求 `{scene_id, name}`），在**单个 SQLite 事务内**完整复制场景：
  - 场景本体：description、variables、config_params、derived_params、plugins、default_timeout（name 用用户输入的新名，status 固定为 `draft`）
  - nodes 表：全部节点（type/config/position/lifecycle 等），生成新节点 ID 并维护 oldID→newID 映射
  - edges 表：全部连线，from/to 通过映射重写，保留 condition/priority
  - data_sources 表：全部数据源（含 CSV 完整 rows）
  - **不复制**：run_records（运行记录）、测试报告——它们属于源场景的运行历史
- 新场景 description 与源场景完全一致（原样继承，不添加“复制自”等标记）
- 前端 `SceneDetailPage` 的"复制"按钮改调新 `copyScene` API；复制成功后跳转到**场景列表**页（用户如需编辑新场景，从列表重新进入）
- 前端场景编辑的所有修改均已确认即时落库（节点/边/变量/数据源变更即刻调 API），复制直接读数据库最新状态，无需"先保存老场景"的额外动作

## Capabilities

### New Capabilities
- `scene-copy`: 场景完整复制能力——后端事务性复制 API 的行为契约（复制范围、ID 映射、状态与描述语义、事务原子性、错误处理）及前端复制入口的交互契约（跳转行为、错误提示）

### Modified Capabilities

（无——现有 spec 的需求均不变。`scene-data-integrity` 的 UpdateStatus 需求与今日"停止不更新修改时间"修复的冲突属于另一个已完成变更，不在本次范围）

## Impact

- **后端**：
  - `internal/api/server.go`：注册 `/api/v1/scenes/copy` 路由
  - `internal/api/handler.go`：新增 `CopyScene` handler（事务编排：场景→nodes→edges→data_sources）
  - `internal/api/dto/dto.go`：新增 `CopySceneRequest`
  - `internal/store/sqlite/sqlite.go`：可能需为 nodes/edges/data_sources 补充批量查询/创建的事务化方法（取决于现有 repo 方法签名）
  - `internal/api/server_test.go`：新增 CopyScene 集成测试（TDD）
- **前端**：
  - `web/app/src/api/scene.ts`：新增 `copyScene` 函数
  - `web/app/src/views/scenes/SceneDetailPage.vue`：`handleCopyScene` 改调新 API + 成功后 `router.push('/scenes')`
- **不受影响**：YAML 导入/导出、Dashboard、运行记录、报告系统、RBAC（copy 归入普通场景写权限）
- **兼容性**：纯新增 API，无破坏性变更；老前端调用 `createScene` 的行为不变
