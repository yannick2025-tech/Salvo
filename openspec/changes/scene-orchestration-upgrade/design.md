## Context

Salvo 当前场景编排系统基于 YAML 导入 + DAG 执行器，支持 HTTP/Delay/Condition/If-Else/Setup/Teardown 节点类型。变量系统已有三级作用域（Global→Scene→API）和 `${var}` 字符串插值，但存在以下限制：

- 前端无变量编辑 GUI，只能通过 YAML 导入
- `ResolveString` 不支持嵌套引用（变量 A 的值引用变量 B）
- 无大数据集参数化能力（无法上传 CSV 文件）
- `LoopCount` 字段已存在但前端未暴露配置入口
- `NodeTypeGroup` 常量已定义但未实现
- 无定时触发能力，delay 节点仅支持固定延迟且阻塞链路

现有代码基础：
- `internal/core/dag/` — DAG 数据结构 + Executor（支持 sync/async、conditional edges、loop count）
- `internal/core/variable/` — 三级作用域变量系统 + `ResolveString`
- `internal/core/timer/` — ThinkTime（固定延迟）+ Ticker（周期调用），已实现但未集成到 DAG
- `internal/store/model/model.go` — Node.LoopCount 字段、NodeTypeGroup 常量
- `internal/runner/runner.go` — sceneNode 执行逻辑、buildScope 变量构建

## Goals / Non-Goals

**Goals:**
- 前端 GUI 支持编辑场景变量（增删改）
- 变量支持嵌套引用和表达式拼接
- 支持上传 CSV 文件作为数据源，按行迭代参数化
- Group 分组节点实现子链路循环
- Timer 节点实现延迟触发和周期定时
- 所有新功能在 YAML 导入/导出中完整支持

**Non-Goals:**
- Node 级变量作用域（现有 extract 机制已够用）
- 子 DAG 嵌套执行器（Group 采用引用式，不引入子 DAG）
- 事件总线架构（当前规模不需要）
- JSON/XML 等其他文件格式（首期仅支持 CSV）
- 分布式数据源共享（单机文件存储即可）

## Decisions

### D1: 变量嵌套解析 — 递归替换而非 AST

**选择**：在 `ResolveString` 中增加递归解析，最多 10 层深度防止循环引用。

**替代方案**：构建表达式 AST（如 antlr）— 过度设计，当前需求仅是字符串拼接和变量替换。

**理由**：递归替换实现简单，与现有 `${var}` 语法兼容，10 层上限足以覆盖所有实际场景。

### D2: CSV DataSource — 文件存储 + 内存行迭代器

**选择**：
- CSV 上传后存储到 `data_sources` 表（文件名、列名列表、行数据 JSON）
- Runner 启动时为每个 DataSource 创建 `RowIterator`（线程安全，原子递增行索引，用完循环）
- 每次链路迭代开始时，将当前行数据注入变量作用域 `${文件名.列名}`

**替代方案**：运行时实时读文件 — 增加磁盘 I/O 和延迟，不适合高频压测。

**理由**：预加载到内存避免 I/O 瓶颈；行迭代器原子递增保证并发安全；用完循环满足长时间运行需求。

### D3: Group 分组 — 引用式 + Executor 展开

**选择**：
- Group 节点 config 中存 `node_ids: ["id1","id2","id3"]` 和 `loop_count: 3`
- Executor 遇到 Group 时，按拓扑序展开子节点链，重复执行 loop_count 次
- Group 的 sync/async 模式决定是否阻塞下游：sync 等所有循环完成，async 不等待

**替代方案**：子 DAG 嵌套 — 需要独立的子 DAG 执行器，数据模型变更大。

**理由**：引用式不需要改变 DAG 核心数据结构；子节点间的边已存在于主 DAG 中，Group 只是逻辑容器；展开执行复用现有 Executor 逻辑。

### D4: Timer 触发器 — 独立 goroutine + 信号通道

**选择**：
- Timer 节点 config 中存 `mode: "delay"|"interval"`、`seconds: 10`
- Executor 识别 timer 节点后，启动独立 goroutine：
  - delay 模式：`time.After(duration)` 后触发下游
  - interval 模式：`time.Ticker` 周期触发下游
- Timer 节点始终为 async，不阻塞 DAG 主链路
- 场景结束时 context cancel 自动停止 timer

**替代方案**：集成到 `internal/core/timer/` 包 — Ticker 已有但设计为阻塞式 Run()，不适合 DAG 非阻塞场景。

**理由**：独立 goroutine 更灵活，context 取消即停止，与 DAG executor 的信号通道机制一致。

### D5: 前端 Group 可视化 — 折叠节点 + 双击展开

**选择**：
- 默认渲染为折叠节点，显示名称和循环次数（如"登录流程 x3"）
- 双击展开后，内部子节点在 Group 边框内可见
- 展开状态下子节点仍可编辑和连线
- Group 节点有入口/出口端口，外部边连接到 Group 而非子节点

**替代方案**：始终展开 — 大场景下节点过多，视觉混乱。

**理由**：折叠/展开切换兼顾简洁性和可编辑性；与 VueFlow 的分组渲染能力匹配。

## Risks / Trade-offs

- **[循环引用风险]** 变量 A 引用 B、B 引用 A 导致无限递归 → 10 层深度上限 + 检测循环时报错
- **[内存占用]** 大 CSV 文件（10 万行）全部加载到内存 → 限制单文件最大 10MB（约 50 万行），超出拒绝上传
- **[文件名规范]** CSV 文件名必须是纯英文（字母+数字+下划线），避免 `${user-list.password}` 等歧义引用 → 上传时校验文件名格式，不合规直接拒绝
- **[Group 展开复杂度]** Group 内子节点有条件边时，展开执行逻辑复杂 → 首期 Group 内子节点仅支持顺序链路，不支持条件分支
- **[Timer 精度]** Go timer 精度约 1-10ms，不适合亚毫秒级定时 → 文档说明精度限制，建议最小间隔 1 秒
- **[并发安全]** RowIterator 原子递增在高并发下可能成为热点 → 使用 atomic.Int64，实测 10 万 QPS 无瓶颈
