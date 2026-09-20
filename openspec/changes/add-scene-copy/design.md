# 设计：场景完整复制

## Context

- 前端"复制"按钮现只调 `createScene({name, description})`，产出空场景；复制成功后仍停留在源场景编辑页。
- 一个"完整场景"由 4 类持久化实体构成：`scenes`（本体，含 variables/config_params/derived_params/plugins/default_timeout）、`nodes`（DAG 节点，config 为 JSON）、`edges`（连线，from/to 为节点 ID）、`data_sources`（CSV/内置数据源，含全部 rows）。
- 节点间引用关系：`edges.from/to` 存节点 ID；**group 节点的 `config.node_ids` 也存节点 ID 字符串数组**（[handler.go:265-289](file:../../../internal/api/handler.go) YAML 导入时从名称解析为 ID）。变量与数据源均按**名称**在节点 config 中引用（`${var}` / `dsName`），复制不改名，无需重写。
- 场景编辑的所有修改即时落库（节点/边增删改、变量 blur 保存、数据源上传），复制读库即为最新状态。
- SQLite 层已有事务惯例：repo 方法内 `BeginTx` + `defer tx.Rollback()` + `tx.Commit()`（如 `ReportRepo.Create` [sqlite.go:505-533](file:../../../internal/store/sqlite/sqlite.go)）。
- snowflake ID 全局唯一，新实体必须生成新 ID；JSON 大数到前端必须以 string 序列化（既有约定，`SceneDTO.ID` 已如此）。

## Goals / Non-Goals

**Goals:**
- 复制产出与源场景内容完全一致的可运行新场景（DAG、参数、变量、CSV 数据源）
- 复制原子性：任一步失败全部回滚，不留半成品场景
- 复制后前端跳转场景列表，消除"误以为在编辑新场景"的歧义
- 新场景与源场景完全解耦：删除/修改任一方不影响另一方

**Non-Goals:**
- 不复制 run_records 与测试报告（运行历史归源场景）
- 不做"场景模板"或跨项目复制等泛化能力
- 不改动 YAML 导入/导出链路（exportYAML→importYAML 的前端串联方案已被否决：字段往返丢失风险 + CSV 过前端内存）
- 不处理源场景不存在/重名等之外的新校验规则（重名沿用 createScene 现有错误语义）

## Decisions

### D1：复制逻辑整体下沉为 `SceneRepo.CopyTx` 仓库方法，而非 handler 编排

**选择**：在 `internal/store/sqlite/sqlite.go` 新增 `SceneRepo.CopyTx(ctx, srcID, newName, newDesc) (*model.Scene, error)`，单事务内完成全部 4 类实体的复制；handler 只做参数校验 + 调用 + DTO 转换。

**理由**：
- 原子性是数据层职责；handler 层无事务句柄，逐 repo Create 编排（ImportYAML 的做法）无法回滚——ImportYAML 中途失败会留半成品场景，复制功能不能复刻这个缺陷。
- 遵循仓库既有惯例（ReportRepo.Create 的事务模式），不引入新的抽象（如 `WithTx(fn)` 跨层传播事务句柄）。
- ID 映射（oldNodeID→newNodeID）是复制纯内部细节，封闭在仓库方法内不外泄。

**备选（否决）**：handler 逐条调用现有 `nodes.Create/edges.Create/dataSources.Create`——无事务、约 4N 次往返、中途失败留脏数据；`INSERT INTO ... SELECT` 单语句原生复制——无法为每行生成新 snowflake ID 并构建映射，需逐行处理。

### D2：ID 映射与 group 节点 config 重写

复制顺序固定为：`scenes → nodes（建映射）→ edges（映射重写 from/to）→ group 节点 config.node_ids（映射重写）→ data_sources`。

- nodes 逐行读出：生成新 snowflake ID、写新行、记 `map[oldID]newID`
- edges 的 `from_node_id/to_node_id` 经映射重写；映射缺失（脏数据悬空边）→ 事务失败回滚，报 500，**不静默跳过**（悬空边在源场景本身就是异常，复制时暴露优于掩盖）
- group 节点 `config.node_ids`：JSON 反序列化 → 逐个映射替换 → 重新序列化后写入。node_ids 含未知 ID 时同样失败回滚
- 变量/数据源引用按名称，名称不变，节点 config 其余部分**逐字节原样复制**，不做任何反序列化改写（最小干预，避免未知 config 结构的意外损伤）

### D3：新场景的元语义

| 字段 | 取值 | 理由 |
|------|------|------|
| name | 用户输入 | 交互契约 |
| status | `draft` | 复制件是新场景，须走完整启动校验；沿用源场景的 running/completed 会造成"复制即已结束"的错误语义 |
| description | 原样继承源场景描述 | 用户最终确认：描述是场景配置的一部分，复制件应与源完全一致 |
| dag_json | 置空（新场景由 nodes/edges 表重建） | 与 createScene/ImportYAML 建场景的初始态一致；dag_json 属遗留冗余字段 |
| 其余字段（variables/config_params/derived_params/plugins/default_timeout） | 逐字段原样复制 | 完整复制契约 |

**不复制**：run_records、reports（运行历史归源场景）。

### D4：API 契约

```
POST /api/v1/scenes/copy
请求:  { "scene_id": "7245...", "name": "新场景名" }
成功:  200 { "code": 0, "data": { ...新场景 SceneDTO... } }
失败:  400 名称为空/场景不存在；409 名称已存在（沿用 createScene 语义）；500 事务失败
```

- `scene_id` 为 string（snowflake 防 JS 精度丢失，既有约定）
- 路由注册在 `server.go`，鉴权与 UpdateScene 同组（登录 + 场景写权限），无新权限项
- 响应返回完整 SceneDTO（含新 ID），为将来"复制后直接跳新场景"留口子，但本次前端仍跳列表（用户已确认）

### D5：前端交互

`SceneDetailPage.handleCopyScene`：
1. 调 `copyScene(sceneId, newName)`（`web/app/src/api/scene.ts` 新增）
2. 成功 → toast 提示 → `router.push('/scenes')`
3. 失败（重名等）→ toast 错误、**停留在当前页**，弹窗不强制关闭，用户可改名重试

不做"复制前保存老场景"动作——所有编辑已即时落库，复制读库即最新（提案已确认）。

## Risks / Trade-offs

- [复制中途失败留脏数据] → 单事务整体回滚（D1），测试覆盖失败路径（源含悬空边/未知 group 引用时断言无残留行）
- [group 节点 node_ids 漏映射 → 新场景 group 引用源场景节点] → D2 固定顺序 + 专门测试用例：复制后断言新 group 节点 node_ids 全部指向新场景节点
- [大 CSV（rows JSON 数 MB）复制慢/撑大事务] → 单事务内 `INSERT ... SELECT` 式直插（rows 字符串整体搬运，不经过 Go 反序列化）；SQLite 单写者下本就串行写，复制是低频管理操作，可接受；若未来成为瓶颈再评估文件级引用
- [名称并发冲突] → 409 语义与 createScene 一致，前端可改名重试（D5）
- [dag_json 置空是否影响现有读取路径] → 已核对：编辑器/运行时以 nodes/edges 表为准（ImportYAML 建场景同样置空起步），风险低；实现时以 `grep dag_json` 全量复核一次

## Migration Plan

纯新增能力，无 schema 变更、无数据迁移：部署即生效，回滚 = 下线路由。前端与后端可独立发布（老前端不知新 API，新前端对老后端会得到 404 + 错误 toast，行为等同今天的失败路径，无恶化）。

## Open Questions

（无——复制范围、跳转行为、status/description 语义均已在提案阶段与用户确认）
