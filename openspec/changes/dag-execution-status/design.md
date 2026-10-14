## Context

Salvo 当前使用 VueFlow + dagre 渲染和布局 DAG。dagre 的固定节点尺寸 (300×90) 导致展开的 group/while 节点与周围节点重叠。执行状态方面，后端 trace 系统已完善（Span 含 chain_id、node_id、status、duration），但前端仅在执行结束后通过 /traces API 查看，无实时能力。当前项目无 WebSocket 基础设施。

**关键约束**：
- VueFlow 框架不变，仅替换布局算法
- trace 系统的 Span 写入路径需保持兼容
- 前端需要在"编辑态"和"执行态"间切换，编辑态行为不变
- 多 worker 并发执行时，同一 Trace 下多个 chain 的 Span 通过 chain_id 区分

## Goals / Non-Goals

**Goals:**
- 一键美化后 DAG 节点和边不再重叠，包括展开的 group/while 节点
- 场景执行时实时看到每个 DAG 节点的执行状态
- 支持聚合视图（多 chain 状态汇总）和单链路视图（单个 chain 执行路径）
- 循环节点显示当前循环进度

**Non-Goals:**
- 不改变 DAG 编辑功能（增删节点/边）
- 不改变 trace 持久化逻辑，WS 推送是瞬态通道
- 不实现执行暂停/恢复控制（仅展示）
- 不实现边（Edge）上的执行状态（仅节点）

## Decisions

### D1: 用 ELK 替换 dagre

**选择**: elkjs (Eclipse Layout Kernel)
**替代方案**: (A) 优化 dagre 参数 — dagre 不支持变长节点尺寸，参数调优效果有限；(B) dagre + 后处理碰撞检测 — 复杂度高，效果不如专业布局引擎
**理由**: ELK 是 VS Code/xtext 使用的分层布局引擎，原生支持变长节点、正交边路由、边交叉最小化。elkjs 是其 JS 移植，API 稳定。

**关键配置**:
```
algorithm: layered
direction: DOWN
nodePlacement.strategy: BRANDES_KOEPF
crossingMinimization.strategy: LAYER_SWEEP
spacing.nodeNode: 70
spacing.nodeNodeBetweenLayers: 110
edgeRouting: ORTHOGONAL
```

每个节点传入实际尺寸（group/while 展开后动态计算高度）。

### D2: WebSocket 实时推送

**选择**: gorilla/websocket + Hub 模式
**替代方案**: (A) 轮询 — 延迟 1-2s，不满足实时需求；(B) SSE — 单向推送够用，但项目后续可能需要双向通信
**理由**: WS 延迟最低（<100ms），Hub 模式简洁，且前端需要 subscribe 特定 run_id。

**架构**:
```
executor Span变更 → trace.go 广播 → ws.Hub → ws.Client → 前端
```

**WS 消息格式**:
- 客户端发送: `{ "type": "subscribe", "run_id": "xxx" }`
- 服务端推送: `{ "type": "span_update", "run_id": "xxx", "chain_id": "yyy", "node_id": "zzz", "status": "ok|error|skip|canceled", "duration_ns": 12345, "error": "", "loop_index": 0 }`

### D3: DAG 执行态双视图

**聚合视图**: 节点右上角显示圆角 badge 组（✓3 ✗1 ⚠1 ⟳1），各状态计数
**单链路视图**: 节点整体样式变化（PASS=绿色边框+淡绿背景，FAILED=红色，SKIP=黄色+半透明，RUNNING=蓝色脉冲），未执行节点半透明。边也跟随：活跃路径高亮，非活跃路径半透明

**切换**: DAG 上方 tab 切换 + 链路下拉选择器

### D4: 执行态 vs 编辑态

场景运行状态由前端已有的场景状态 API 判断：
- pending/running → 执行态（DAG 只读，显示状态）
- done/failed/canceled → 执行态保持最终状态
- 未运行 → 编辑态（当前行为不变）

## Risks / Trade-offs

- **[ELK 布局性能]** → ELK 比 dagre 稍慢（大图 ~50ms vs ~10ms），但节点数 <100 时差异可忽略。若后续超大 DAG 可考虑 web worker
- **[WS 连接管理]** → 客户端断连后重连需重新 subscribe，前端需实现重连逻辑
- **[内存占用]** → Hub 维护活跃连接列表，连接数上限设为 100，超出拒绝
- **[循环进度追踪]** → 当前 Span 只有单次执行记录，循环进度需 executor 在每次 loop iteration 时额外推送中间事件（而非仅最终 Finish）
