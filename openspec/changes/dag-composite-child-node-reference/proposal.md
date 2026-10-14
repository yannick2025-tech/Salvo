## Why

DAG 编辑页对复合节点的子节点管理存在两套互不兼容的模型：group 用 `config.node_ids` 引用场景内真实节点（可勾选增删、画布渲染、配置即改即生效），而 while/loop 用 `config.steps` 内嵌步骤定义——前端无任何步骤管理入口，循环体配置完全无法修改。此外前端保存 while/loop 配置时会把未加载进表单的字段（steps / fail_on_max_iterations / fail_on_max_duration）整体抹掉，YAML 导出 group 时不把 node_ids 的 ID 转回节点名导致导出后再导入必然失败。

统一为 group 式"引用子节点"模型后，单节点配置归单节点自身，复合节点只做勾选/排序子节点 + 自身属性配置，执行时按引用找到子节点取其属性，修改天然同步；同时修复上述保存丢字段与导出闭环两个既有 bug。

## What Changes

- **while/loop 支持 `node_ids` 引用模式**：buildDAG 将三类复合节点（group/while/loop）统一处理——子节点收集、排除出主 DAG 拓扑、挂载 childNodes；`node_ids` 引用优先，内嵌 `steps` 保留兜底（兼容 card-recharge.yaml 等存量场景，**不破坏现有 YAML**）。
- **新增公共子链执行 `runChildChain`**：group/while/loop 共享顺序执行子节点逻辑；每步执行后通过 `Executor.SnapshotVariables()` 刷新变量快照，修复 group 既有缺陷（子链内 extract 的变量下游子节点读不到），while 引用模式的退出条件也依赖此刷新。
- **嵌套规则**：group 不能含 group（既有）；while/loop 子节点禁止 group/while/loop（复合类型不可进循环体），构建期与 YAML 导入双处校验。
- **YAML 导入**：node_ids 校验与 name→ID 解析从仅 group 扩展到三类复合节点；新增嵌套禁止校验（400）。
- **YAML 导出（修既有 bug）**：导出时把三类复合节点 config.node_ids 的 snowflake ID 转回节点名，导出→再导入闭环可用。
- **前端保存丢字段修复**：while/loop 保存改为基于原始 config 浅合并，表单字段覆盖、未知键全保留，彻底杜绝 steps 等字段被抹掉。
- **前端 while/loop 面板新增"子节点"管理区**：复用 group 的勾选列表 + 有序列表（上移/下移/移除）交互；可选子节点过滤掉复合类型。
- **前端面板内直接编辑子节点**：group/while/loop 已选列表加"编辑"按钮跳到子节点配置面板，panel-header 加"← 返回"回到复合节点。
- **画布渲染**：while/loop 有 node_ids 时反查挂 childNodes 渲染（同 group），双击展开；loop 新增展开分支；while 内嵌 steps 的只读展示保留。

## Capabilities

### New Capabilities
- `composite-child-reference`: 复合节点（group/while/loop）统一"引用子节点"模型——node_ids 引用模式执行语义、变量快照刷新、嵌套规则、双模式优先级（node_ids 优先 / steps 兜底）、YAML 导入校验与导出 ID→名字转换、前端子节点管理（勾选/排序/编辑/返回）与画布渲染。

### Modified Capabilities
- `node-group`: group 执行重构为共享 runChildChain，行为增强——子链内每步 extract 的变量通过 SnapshotVariables 即时刷新，后续子节点与循环体可立即读取（原实现读旧快照）。

## Impact

- **后端**：
  - `internal/core/dag/executor.go`：新增 `SnapshotVariables()`（已完成）
  - `internal/runner/runner.go`：`isCompositeType` 辅助、buildDAG 泛化（收集/排除/挂载三类）、`buildCompositeChildSN` 递归挂载、`runChildChain` 公共子链执行、executeGroup 重构（已完成）
  - `internal/runner/while_node.go`：引用模式入口（已加）+ `executeWhileRefMode` 实现（待完成，当前有语法错误需修复）
  - `internal/runner/loop_node.go`：引用模式分支（待完成）
  - `internal/api/handler.go`：YAML 导入校验/解析扩展 + 导出 node_ids ID→名字（待完成）
- **前端**：
  - `web/app/src/views/scenes/SceneDetailPage.vue`：保存浅合并修 bug、while/loop 子节点区、编辑/返回导航（待完成）
  - `web/app/src/components/dag/DagFlow.vue` / `DagSceneNode.vue`：while/loop 子节点渲染（待完成）
- **测试**：`internal/runner/composite_ref_test.go`（已写，红）、YAML 导入/导出闭环测试（待写）、group 既有测试全量回归
- **兼容性**：存量 while steps YAML 不受影响（双模式兜底）；group 变量刷新增强属缺陷修复方向，全量回归把关
