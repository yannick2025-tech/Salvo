## Context

YAML 是场景的唯一可移植载体（导入/备份/迁移/复制）。当前导出由前端 `DagFlow.generateYaml()` 本地拼接，它只能访问 `props.nodes/edges/dataSources`，丢失所有 scene 级字段与节点 `block_on_error`；且上一轮已暴露手写序列化的缩进 bug。后端虽有完整 `/scenes/export`（`yaml.Marshal`，输出 name/description/default_timeout/variables/config_params/derived_params/data_sources/setup/nodes/teardown/edges），但前端未使用。

更深层问题：后端导入把 YAML 的 `setup`/`nodes`/`teardown` 三段节点 append 到一起、按节点自身 `type` 存储（`handler.go:203-234`），未记录来源段；导出再按 `Type=="setup"/"teardown"` 分节（`handler.go:588-595`），对 YAML 导入的场景（其 setup 段节点真实 type 多为 generator/http）匹配不上 → 分节永久丢失。这是数据模型缺陷，需新增 `Node.Lifecycle` 字段才能根治。

现有数据：`Scene` 表已存 name/description/default_timeout/variables/plugins；`Node` 表已存 block_on_error；`DataSource` 独立表。后端 `SceneDTO`/`NodeDTO` 已含 default_timeout/block_on_error，但前端 TS 类型漏了这两字段。

## Goals / Non-Goals

**Goals:**
- "复制 YAML"/"导出 YAML"输出与原 YAML 在结构字段上一致（无信息丢失），缩进正确。
- 前端成为后端 `/scenes/export` 的薄客户端，删除本地序列化重复逻辑。
- setup/teardown 分节在 import→export 往返中保持。
- `block_on_error`、`default_timeout`、`variables`/`config_params`/`derived_params` 全程不丢。
- 老数据库平滑迁移，存量场景可用。

**Non-Goals:**
- 恢复 YAML 注释（模型层不存，不在本次范围；接受丢失）。
- 回填存量场景的 lifecycle（历史 setup/teardown 段信息已不可恢复，迁移后归入 main/nodes，仅新生成/重新导入的场景具备完整分节）。
- 改动 DAG 运行器对 setup/teardown 的执行语义（运行期已按 edges 拓扑执行，本次仅影响 YAML 序列化层）。
- 重写前端节点编辑表单（block_on_error 编辑 UI 不在本次范围，仅保证导出含该字段）。

## Decisions

### 决策1：前端改调后端 `/scenes/export`，删除本地 `generateYaml`
- **选择**：复制/导出均 `await exportYAML(sceneId)`，拿 `ExportYAMLResponse.YAML`。
- **备选**：方案 B（前端增强 generateYaml，传 scene 级数据 + 补类型）。否决理由：重复后端逻辑、上一轮刚因手写序列化出 bug、且无法重建 setup/teardown（前端无 lifecycle 标记）。
- **理由**：后端已是单一可信源且往返有测试；前端薄客户端化降低维护面，缩进由 `yaml.Marshal` 保证。

### 决策2：`Node` 新增 `Lifecycle string` 字段（非 nullable，空串=main）
- **选择**：新列 `lifecycle TEXT NOT NULL DEFAULT ''`。导入时按 `ys.Setup`/`ys.Nodes`/`ys.Teardown` 切片分别赋 `"setup"`/`""`/`"teardown"`。导出按 `n.Lifecycle` 分节，替代按 `Type` 分节。
- **备选 A**：复用 `Type` 存 `"setup"/"teardown"`。否决：会覆盖节点真实类型（generator/http），破坏运行期类型分发与前端编辑表单。
- **备选 B**：用独立 `node_sections` 表。否决：过度设计，单列足够表达三态。
- **理由**：lifecycle 与 type 正交（lifecycle=何时跑，type=跑什么），最小 schema 变更覆盖需求。

### 决策3：导出补 `block_on_error`
- **选择**：`ExportYAML` 构造 `yn` 时加 `yn.BlockOnError = n.BlockOnError`（`yamlNode` 已有该 yaml tag）。`omitempty` 保证 false 节点不输出，与原 YAML 一致。

### 决策4：variables/config_params/derived_params 复用后端既有启发式
- **选择**：沿用 `ExportYAML` 现有还原逻辑（`cfg_` 前缀→config_params，`${...}`→derived_params，其余→variables）。导入仍把三者合并存入 `Scene.Variables` JSON。
- **理由**：往返一致即可；不在本次重设计变量模型。已知限制：启发式可能与原始分类有偏差（如 key 非 `cfg_` 前缀但语义为配置参数），属既有行为，不在本次范围。

### 决策5：前端类型补全
- `SceneDTO` 加 `default_timeout: number`；`NodeDTO` 加 `lifecycle: string`（并补 `block_on_error: boolean` 以便后续编辑 UI 复用）。新增 `ExportYAMLResponse { yaml: string }`。
- **理由**：后端已返回这些字段，前端仅补类型对齐，无运行时行为变化。

### 决策6：`DagFlow` props 加 `sceneId`
- **选择**：`SceneDetailPage` 把 `route.params.id` 作为 `sceneId` 传入 `DagFlow`；`copyYaml`/`exportYaml` 用之调 API。
- **备选**：在 `DagFlow` 内 `useRoute()`。否决：保持组件受控、可测试。

## Risks / Trade-offs

- [存量场景 lifecycle 为空 → setup/teardown 归入 nodes] → 可接受；迁移文档注明，重新导入即可恢复分节。
- [variables 启发式分类偏差] → 既有行为，非本次引入；如需精确分类需重构变量模型（独立 epic）。
- [复制/导出变异步，需 loading/错误态] → 前端加 `exporting` 态 + toast；失败回退提示"导出失败，请重试"。
- [DB 迁移] → 新增列带默认值，向后兼容；回滚=删除列（SQLite 支持）。
- [前后端字段对齐遗漏] → tasks 中列出每处 struct/DTO/类型/SQL 同步点，逐项 checklist。

## Migration Plan

1. 后端先合：`model.Node` 加 Lifecycle → 迁移脚本 → `sqlite.go` INSERT/SELECT/Update 加列 → `ImportYAML` 赋 lifecycle → `ExportYAML` 按 lifecycle 分节 + 补 block_on_error → `NodeDTO`/`toNodeDTO` 加 lifecycle。
2. 后端测试：扩展往返测试覆盖 setup/teardown 分节、block_on_error、default_timeout、variables 三分类。
3. 前端：`types` 补字段 → `scene.ts` 加 `exportYAML` → `SceneDetailPage` 传 sceneId → `DagFlow` 复制/导出改调后端、删 `generateYaml`/`toYamlLines`。
4. 回滚：后端可独立回滚（删除迁移列）；前端如需回滚则恢复 `generateYaml`（git revert）。前后端解耦，可分批上线。
