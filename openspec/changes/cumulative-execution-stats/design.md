## Context

实时进度页（DagFlow）节点徽章当前为"每链最新状态"语义：`useExecutionStatus.processEvent` 对每个 (chain_id, node_id) 只保留最后一次事件的状态，聚合计数是各链最新状态的分布。三个数据源（REST `getTraceByRun`、WS 订阅快照、WS 增量事件）在页面往返后重建状态时均存在缺陷：

1. REST trace 的 span 粒度是"节点级最终结果"——executor 中 `span := tctx.StartSpan(n.ID())` 在 loop 外只创建一次，`Finish` 只记录最后一轮迭代的结果，中间每轮迭代的成败不落 trace。
2. WS 增量事件经 `SendQueue` 按 `(run_id, chain_id, node_id)` 去重，同一节点多轮迭代的消息互相覆盖，慢网络下事件在服务端即丢失。
3. `Running()` 只广播 WS 事件不落 trace，in-flight 节点状态无法从任何持久数据重建。

此外 `Finish/Skip/FinishCanceled` 的广播硬编码 `loopIndex=0`，与 `BroadcastRunning(loopIndex)` 的迭代粒度不一致。

## Goals / Non-Goals

**Goals:**

- 节点徽章 pass/fail/skip 为自运行开始以来的累计次数，只增不减；running 表示当前正在执行的迭代数。
- 同一节点每一轮迭代的事件（含连续多轮失败）独立送达前端，不被 SendQueue 去重合并。
- 页面往返（离开再返回）后，已发生的累计值与当前 running 并发完整重建。
- REST / WS 快照 / WS 增量三源数据交错时通过幂等键合并，不双计、不回退。
- DB 中 trace/span 结构不变（历史数据兼容，无迁移）。

**Non-Goals:**

- 不改造 trace 持久化结构（span 仍为节点级最终结果，每轮迭代明细不落库）。
- 不改动链路视图（chain mode）以外的 UI 布局与样式。
- 不处理跨 run 的统计（每次订阅仅限单 run）。

## Decisions

### D1. 广播统一为迭代粒度，span 广播携带 lastIterIndex

`SpanBuilder` 增加 `lastIterIndex int` 字段，`BroadcastRunning(i)` 时更新。`Finish/Skip/FinishCanceled` 的 `emitBroadcast` 改用 `lastIterIndex`（skip 场景无 Running 调用，保持 0）。

**理由**：现状 `loopIndex=0` 硬编码使 Finish 广播与迭代事件无法区分轮次；统一后每条 span_update 都带准确轮次，前端幂等键才成立。

### D2. executor 每轮迭代广播终态事件

`dag/trace.go` 节点执行循环内，每轮 `n.Execute` 返回后广播该轮结果：`err != nil` 或 `output.Error != nil` → status=`error`，否则 `ok`，loop_index=i。通过 `SpanContext` 新增方法 `IterationResult(loopIndex int, failed bool)`（`spanAdapter` 转发到 `SpanBuilder.emitBroadcast`）。

**正常完成路径的 Finish 广播与最后一轮 IterationResult 重复**：无碍，由前端幂等键跳过（见 D5），且保证 cancel/硬失败路径无需特判补发。

**替代方案（否决）**：把每轮迭代落成独立 span —— 改动 trace/DB/REST DTO 全链路，且 REST 历史数据语义变化，成本远超收益。

### D3. WS dedupKey 加入 loop_index

`Message.dedupKey()` 从 `run/chain/node` 扩展为 `run/chain/node/loopIndex`。

**效果**：不同轮迭代互不覆盖（保真）；同轮内 running→终态仍合并（瞬态被终态覆盖，符合原设计意图且无累计损失）。

**SendQueue 容量语义变化**：上界从 O(链×节点) 变为 O(链×节点×迭代数)。见 Risks。

### D4. Trace 挂 per-run 累计 stats，emitBroadcast 同步更新

`trace` 包新增：

```go
// NodeStats holds cumulative counters for one (chain_id, node_id).
type NodeStats struct {
    Pass, Fail, Skip int
    RunningIdx       map[int]struct{} // in-flight loop indexes
    LastIndex        int              // highest applied iteration index
}
```

`Trace` 持有 `stats map[string]map[string]*NodeStats`（chain→node，`trace.mu` 保护），`SpanBuilder.emitBroadcast` 在广播的同时更新对应计数（running→RunningIdx.Add(i)；ok/error/skip→对应计数+1、RunningIdx.Delete(i)、LastIndex=max）。`Trace.Finish` 时清空全部 RunningIdx（run 结束后不再有并发）。

`SnapshotStats()` 返回深拷贝，供 WS 快照回调使用。

**理由**：与广播同线程更新，天然一致，无需分发器；生命周期跟随 Tracer buffer 淘汰，无需独立清理机制。

### D5. WS 快照改为聚合 span_stats 消息，前端 appliedIndex 幂等

- `server.go` 的 `SpanStateFunc` 回调改为基于 `tr.SnapshotStats()` 输出：每个 (chain, node) 一条 `span_stats` 消息（Type=`span_stats`，字段：chain_id、node_id、pass/fail/skip、running_idx 数组、last_index），`dedupKey()` 对非 `span_update` 类型返回 ""，天然不去重。
- `span_update` 增量事件沿用现结构（loop_index 已有字段，前后端均无新增字段需求）。

**前端幂等合并**（`useExecutionStatus` 重构核心）：

- per (chain, node) 维护 `{pass, fail, skip, runningIdx: Set, appliedIndex: int(-1 初始)}`。
- `span_update` 终态事件：`loop_index ≤ appliedIndex` → 跳过（重复）；否则对应计数 +1、`runningIdx.delete`、`appliedIndex = max(appliedIndex, loop_index)`。
- `span_update` running 事件：`loop_index > appliedIndex` 时加入 runningIdx（旧迭代重放忽略）。
- `span_stats` 快照：仅当 `last_index ≥ appliedIndex` 时应用（重置全部字段）——解决"快照与已处理增量乱序"（Subscribe 先加入订阅集合再推快照，并发增量可能先到）。
- REST `initFromSpans`：改合并语义——仅对前端尚无状态（appliedIndex=-1）的 (chain, node) 写入保守初值（最终 span 状态计 1 次），已被快照/增量建立的状态不覆盖。**消除 REST 后到覆盖竞态**。

### D6. 徽章判定与展示适配累计语义

`computeAggregateStatus` 的 badge.status 优先级调整为：running > fail（有失败即标红，即使后续成功）> pass > skip > idle。label 格式 `N✓ N✗ N>> N⟳` 不变，数值含义变为累计。running 计数 = 各链该节点 runningIdx 大小之和。

### D7. 前端类型与消息结构

`useExecutionWs.ts` 的 `SpanUpdateEvent` 不变；新增 `SpanStatsEvent` 接口（span_stats 消息）。`useExecutionStatus` 对外导出的 `AggregateCounts`/`NodeBadge` 结构不变（消费方 DagFlow/DagSceneNode 无感知）。

## Risks / Trade-offs

- [SendQueue 消息量增长至 O(迭代总数)，极端高频迭代场景（如 interval 100ms × 万级迭代）慢网络下出站队列堆积] → 单条消息约 200B，万级迭代约 2MB，可接受；SendQueue 本身 never-dropping 保证不丢。若后续出现实际堆积问题，可在此基础上加"同 key 仅保最新 + 定期聚合快照"的混合策略，本次不做。
- [Stats 内存量 = 活跃 run 数 × 链 × 节点的小结构，随 Tracer buffer 淘汰] → 单 run 通常 < 100KB，Buffer 上界已有约束，无需额外淘汰。
- [正常完成路径 Finish 广播与最后一轮 IterationResult 双发] → 前端 appliedIndex 幂等天然去重，双发仅多一条消息。
- [rest 兜底初值（最终态计 1 次）在 REST 先于快照到达时短暂低估计数] → 快照到达即校正；仅影响 WS 完全不可用且 run 进行中的边缘场景。
- [前端无单测框架，processEvent 幂等逻辑回归依赖类型检查 + 手动验证] → 后端 ws/trace/api 均有测试覆盖广播与快照语义；前端以 vue-tsc + 场景手动验证（离开/返回页面、多轮失败、手动停止）。

## Migration Plan

纯增量发布，无 DB/协议破坏性变更（span_stats 为新增消息类型，旧前端忽略未知 type 即跳过，增量 span_update 结构不变）。回滚 = 回退代码。

## Open Questions

无——广播粒度、幂等键、快照聚合方案均已定案。
