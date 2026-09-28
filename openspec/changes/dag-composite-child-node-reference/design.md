## Context

DAG 编辑页当前对复合节点的子节点有两种互不兼容的模型：

- **group**：`config.node_ids` 引用场景内真实节点（构建期解析为 childNodes，子节点从主 DAG 拓扑排除）。子节点配置存在子节点自身，改配置即生效。前端可勾选添加/移除子节点，子节点游离渲染在画布上可点击编辑——但配置面板内无直接编辑入口，需到画布找游离节点。
- **while/loop**：`config.steps` 内嵌完整步骤定义（stepConfig：request/condition/think_time/retry/aes 等），与场景节点无关。前端无任何步骤管理入口，步骤也不渲染到画布——循环体配置完全无法修改。

用户决策（已确认）：**统一改成 group 式引用模型**——单节点配置放在单节点自身，group/while/loop 只做勾选/去勾选子节点 + 自身属性配置。执行时按 NODEID 引用找到子节点再取其属性，修改子节点天然同步。

已探明的两个附带 bug 必须一并修复：
1. **前端保存丢字段**：`saveNodeConfig` 对 while/loop 直接 `JSON.stringify(whileConfig/loopConfig)`，而这两个 reactive 未加载 `steps`/`fail_on_max_iterations`/`fail_on_max_duration`——打开面板一保存，这些字段全部被抹掉。
2. **group 导出闭环既有 bug**：导出器不把 group config 里的 node_ids（snowflake ID）转回节点名，而导入校验按名字匹配（`nodeNames[child]`）——group 场景导出后再导入会报 "child node not found"。

兼容性约束：现有 YAML（card-recharge.yaml 等）的 while 使用内嵌 steps 语法（generator 签名步骤、aes_decrypt、request 格式），不能废弃——采用双模式，`node_ids` 引用模式优先，`steps` 内嵌模式保留兜底。

### 当前实施进度

| 步骤 | 状态 |
|---|---|
| `dag.SnapshotVariables()` | ✅ 已完成 |
| buildDAG 泛化（isCompositeType、收集/排除/挂载三类、buildCompositeChildSN 递归、嵌套校验） | ✅ 已完成 |
| `runChildChain` 抽取 + executeGroup 重构 | ✅ 已完成 |
| while_node.go 引用模式入口（`len(n.childNodes) > 0` 分支） | ✅ 已加，但 `executeWhileRefMode` 未实现 |
| while_node.go 语法错误（executeWhileStepGenerator 签名被误删） | ❌ 待修复 |
| loop_node.go 引用分支 | ❌ 待完成 |
| YAML 导入/导出 | ❌ 待完成 |
| 前端全部 | ❌ 待完成 |

## Goals / Non-Goals

**Goals:**
- while/loop 支持 `node_ids` 引用模式，与 group 统一：构建期排除出主 DAG、挂 childNodes、执行期按序跑子链
- 变量快照刷新：子链内每步 extract 的变量通过 `SnapshotVariables()` 即时可见（修复 group 既有缺陷，while 退出条件依赖）
- 双模式并存：node_ids 优先、steps 兜底，存量 YAML 不破坏
- 嵌套禁止双处校验（构建报错 + 导入 400）
- YAML 导出 node_ids ID→名字，导出→再导入闭环
- 前端：保存浅合并修丢字段 bug；while/loop 面板子节点勾选/排序/移除；面板内编辑子节点 + 返回；画布渲染 while/loop 子节点

**Non-Goals:**
- 不提供 steps 内嵌模式的编辑 UI（用户已选引用模式；steps 编辑需求如后续出现再议）
- 不改变 while 既有骨架语义（退出条件/max_iterations/max_duration/fail_on_max_* 全保留）
- 不支持复合节点无限嵌套（group → while/loop → 普通节点，深度上限固定）

## Decisions

### D1：双模式优先级——node_ids 优先、steps 兜底

`executeWhile`/`executeLoop` 开头判断 `len(n.childNodes) > 0`（buildDAG 挂载结果）：有则走引用模式，否则走现有 steps 路径。
**备选**：废弃 steps 模式——被否决，card-recharge.yaml 等存量场景直接破坏。
**注意**：引用模式判断依据是 childNodes（构建期解析产物）而非 config.node_ids 非空——构建期解析失败（引用不存在）会在 buildDAG 报错，不会静默走到 steps。

### D2：引用模式复用 while 全部骨架

`executeWhileRefMode` 保留退出条件评估、max_iterations/max_duration 上限、fail_on_max_iterations/fail_on_max_duration 失败行为、interval_seconds 轮询间隔；每轮迭代改为 `runChildChain`，退出条件用 `expr.EvaluateConditionExpr(condExpr, input.Variables)`（变量已被刷新）。
**备选**：把 while 骨架抽成通用函数两种模式共享——被否决，steps 模式有 stepConfig 类型分派（request/generator/condition/aes），与引用模式的子链执行差异太大，强行共享会把 executeWhile 拆得支离破碎；两分支各自独立、骨架字段语义一致即可。

### D3：runChildChain 每步刷新变量快照

`buildInput` 从 initialVars **拷贝**快照（既有行为，不可改——DAG 并行分支共享变量会引入数据竞争），因此子链内后续子节点读不到新 extract 变量。修复方式：runChildChain 每步 `child.Execute` 后 `input.Variables = input.Executor.SnapshotVariables()`。
**备选**：子链直接读写共享 initialVars——被否决，绕过 varsMu 锁有并发风险。

### D4：嵌套规则与递归挂载

- group 不能含 group（既有规则保留）
- while/loop 子节点禁止 group/while/loop（复合类型不可进循环体）
- **例外**：group 可以含 while/loop（`buildCompositeChildSN` 递归解析其 node_ids，group → while → 普通节点是合法的两层结构）

`buildCompositeChildSN` 递归深度有界：while/loop 不可含复合节点，所以嵌套最多 group → while/loop → 普通节点。

### D5：YAML 导入按名字匹配、导出 ID→名字

导入校验与 name→ID 解析循环从 `n.Type == group` 扩为 `isCompositeType`；导出循环新增 `case "node_ids"`：ID 数组经 nodeNameMap（ID→名）转名字，查不到名字时原样保留（防御性，不阻塞导出）。

### D6：前端保存浅合并

while/loop 保存分支改为：`JSON.parse(selectedNode.config)` 得原对象 → 表单字段覆盖（node_ids/steps/退出条件等）→ 序列化。未知键全保留，彻底杜绝丢字段。保存成功后 `fetchNodes()` 并按 id 重同步 `selectedNode`。

### D7：面板编辑导航只存 id 不存引用

"编辑"按钮 → `selectNode(childNode)`，`returnToNodeId = 当前端 id`；"← 返回"按 id 现查节点后 selectNode。不存节点对象引用（fetchNodes 会整体替换数组导致陈旧引用）。

## Risks / Trade-offs

- [group 改用 runChildChain 后变量行为增强（子链内 extract 即时可见）] → 属缺陷修复方向，group 全量既有测试回归把关；若回归爆可临时回退 group 独立实现
- [双模式并存增加理解成本] → 前端同节点同时有 node_ids 与 steps 时显示提示"N 个 YAML 内嵌步骤（引用子节点优先生效）"
- [引用模式 while 退出条件依赖子节点 extract 写共享变量] → runChildChain 每步刷新快照保证；TDD 测试 TestExecuteWhileNodeIDsRefMode 验证此链路（无刷新则红）
- [导出 ID→名字查不到] → 原样保留 ID，导入端 name/ID 双匹配兜底（buildDAG 既有逻辑已支持双格式）
- [while_node.go 当前语法错误] → executeWhileStepGenerator 签名被误删，需先恢复签名再实现 executeWhileRefMode

## Migration Plan

纯代码变更，无数据库 schema 变更、无配置迁移。存量场景：
- while steps 场景：childNodes 为空 → 走 steps 兜底路径，行为不变
- group 场景：runChildChain 语义与原 executeGroup 内联循环一致 + 变量刷新增强
回滚：单 commit revert 即可，无状态残留。

## Open Questions

无——用户已确认统一引用模型方案与双模式兼容策略。
