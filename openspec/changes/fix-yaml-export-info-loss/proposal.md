## Why

YAML 导出/复制存在严重信息丢失：前端 `DagFlow.generateYaml()` 只能访问 `nodes/edges/dataSources`，导致 `name`、`description`、`default_timeout`、`variables`、`config_params`、`derived_params`、`block_on_error`、`setup/teardown` 分节全部丢失（注释也因模型不存而无法保留）。而后端早已存在完整的 `/scenes/export` 接口（`yaml.Marshal`，含上述绝大部分字段），前端却未使用，反而维护一套重复且有 bug 的本地序列化（上一轮已修缩进）。同时后端导入把 setup/teardown 节点按真实 type 合并存储、未记录分节，导出按 `Type=="setup"` 分节无法重建，是数据模型层缺陷。本次一次性根治：前端改调后端导出，后端补齐 `block_on_error` 与节点 lifecycle 分节，彻底消除信息丢失。

## What Changes

- 前端"复制 YAML"/"导出 YAML"改为调用后端 `/api/v1/scenes/export`，删除本地 `generateYaml`/`toYamlLines` 及相关类型。
- **BREAKING**（数据模型）：`Node` 新增 `Lifecycle string` 字段（值 `setup`/`teardown`/空=main），并新增 DB 迁移 `ALTER TABLE nodes ADD COLUMN lifecycle`。
- 后端 YAML 导入：按 YAML 节点来源切片（`setup`/`nodes`/`teardown`）给 `Node.Lifecycle` 赋值。
- 后端 YAML 导出：按 `Node.Lifecycle` 分节（替代当前按 `Type` 分节）；导出时补 `yn.BlockOnError = n.BlockOnError`。
- 后端 `NodeDTO` + `toNodeDTO` 增加 `lifecycle` 字段；前端 `NodeDTO` 类型增加 `lifecycle`、`SceneDTO` 类型增加 `default_timeout`。
- 前端 `scene.ts` 增加 `exportYAML(id)`；`SceneDetailPage` 向 `DagFlow` 传入 `sceneId`。
- 导入端节点遍历原 `Setup+Nodes+Teardown` 合并逻辑保留，但每个节点记录其来源 lifecycle。

## Capabilities

### New Capabilities
- `yaml-export-fidelity`: 以后端 `/scenes/export` 为单一可信源的 YAML 导出/复制，要求无信息丢失（含 name、description、default_timeout、variables、config_params、derived_params、data_sources、setup/teardown 分节、block_on_error、edges），且 import→export 往返一致。

### Modified Capabilities
- `scene-data-integrity`: 现有"YAML export includes new node types and data sources"需求（指明 from DagFlow.vue）改为后端驱动；并新增"YAML 导入记录节点 lifecycle 分节"需求。

## Impact

- 后端：`internal/store/model/model.go`（Node 加 Lifecycle）、`internal/store/migration/migration.go`（新增迁移）、`internal/store/sqlite/sqlite.go`（INSERT/SELECT 加 lifecycle 列）、`internal/api/handler.go`（ImportYAML 赋 lifecycle、ExportYAML 按 lifecycle 分节 + 补 block_on_error）、`internal/api/dto/dto.go`（NodeDTO 加 lifecycle）。
- 前端：`web/app/src/api/scene.ts`（加 exportYAML）、`web/app/src/types/index.ts`（NodeDTO 加 lifecycle、SceneDTO 加 default_timeout、加 ExportYAMLResponse）、`web/app/src/views/scenes/DagFlow.vue`（复制/导出改调后端、删 generateYaml/toYamlLines、props 加 sceneId）、`web/app/src/views/scenes/SceneDetailPage.vue`（传 sceneId 给 DagFlow）。
- 数据兼容：已存在的 scene 在迁移后 lifecycle 为空（=main），其节点原属 setup/teardown 的历史信息无法回填，导出时归入 `nodes`（可接受；新生成或重新导入的场景才具备完整分节）。
- 测试：扩展 YAML 往返测试覆盖 setup/teardown 分节、block_on_error、default_timeout、variables/config_params/derived_params。
