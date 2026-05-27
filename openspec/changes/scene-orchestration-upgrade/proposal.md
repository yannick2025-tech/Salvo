## Why

当前 Salvo 的场景编排系统基于 YAML 导入 + DAG 执行，已能完成基本的 HTTP 链路压测。但面对复杂业务场景（如电商全链路、多用户参数化压测），缺少 5 项关键能力：变量管理不便、无法参数化大数据集、无子链路循环、无节点分组、无定时触发。这些限制使得 Salvo 难以胜任真实业务压测需求。

## What Changes

- **变量增强**：前端 GUI 支持编辑多个场景变量；后端 `ResolveString` 支持嵌套引用（变量 A 引用变量 B）和表达式拼接（`${base_url}/api/v1/${path}`）
- **文件参数化（CSV DataSource）**：支持上传 CSV 文件（带表头），每条链路迭代取一行数据，用完循环；节点中通过 `${文件名.列名}` 引用当前行的列值
- **子链路循环**：Group 分组节点包裹子节点链（D1→D2→D3），Group 上配置 LoopCount 实现子链路循环执行；异步节点的循环不阻塞下游
- **节点分组（Group）**：新增 Group 节点类型，config 中引用子节点 ID 列表；DAG 图上默认折叠显示，双击展开查看内部子节点
- **定时触发器（Timer）**：新增 timer 节点类型，支持延迟触发（场景开始后 X 秒执行一次）和周期定时（每 X 秒执行一次直到场景结束），均不阻塞 DAG 主链路

## Capabilities

### New Capabilities
- `variable-enhancement`: 场景变量 GUI 编辑、嵌套引用解析、表达式拼接
- `csv-data-source`: CSV 文件上传、解析、行迭代、`${文件名.列名}` 引用
- `node-group`: Group 分组节点（引用式子节点列表、LoopCount 循环、折叠/展开可视化）
- `timer-trigger`: timer 节点（延迟触发 + 周期定时，不阻塞主链路）

### Modified Capabilities
- `scene-data-integrity`: YAML 导入/导出需扩展支持 variables、data_sources、group、timer 节点类型

## Impact

- **后端**：`internal/runner/runner.go`（新增 group/timer 节点执行逻辑）、`internal/core/variable/variable.go`（递归解析）、`internal/core/dag/executor.go`（Group 展开、Timer 触发）、`internal/store/model/model.go`（新增 DataSource 模型）、`internal/api/handler.go`（CSV 上传 API、变量 CRUD）
- **前端**：`SceneDetailPage.vue`（变量编辑区、LoopCount 配置）、`DagFlow.vue`（Group 折叠/展开、timer 节点渲染）
- **数据库**：新增 `data_sources` 表（文件存储、列名映射）
- **YAML 格式**：扩展 `data_sources`、`timer` 节点类型、`group` 节点类型
