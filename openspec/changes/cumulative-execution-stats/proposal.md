## Why

实时进度页面的节点徽章计数（`N✓ N✗ N>> N⟳`）是"各链最新状态"的分布统计而非累计值：节点某轮失败、下轮成功后失败数即刻回落；用户离开页面再返回后，失败数/运行中数还会因重建数据不全而进一步减少。对性能测试场景，失败数/通过数应具备"只增不减"的累计语义，且页面往返后不丢失。

## What Changes

- 节点徽章统计模型从"每链最新状态"改为累计制：pass/fail/skip 为该节点自运行开始以来的累计次数，只增不减；running 表示当前正在执行的迭代数（实时增减）。
- 后端 WS 消息去重键从 `(run_id, chain_id, node_id)` 扩展为包含 `loop_index`，保证同一节点每一轮迭代的事件（含连续多轮失败）都能独立送达，不被慢网络合并丢弃。
- WS 消息与 REST trace span 补充稳定的幂等标识（`span_id`），前端对 REST 初始化、WS 订阅快照、WS 增量事件三源数据做幂等去重，消除双计。
- 前端 `initFromSpans` 基于全量已完成 span 重建累计值；REST 初始化与 WS 事件流交错时的覆盖竞态通过幂等键解决。
- 修复返回页面后 in-flight 节点 running 状态丢失：后端快照回调补充进行中（running）状态，订阅快照可重建当前并发。
- 节点徽章判定优先级与展示格式随累计语义调整（如 fail>0 即标红，即使后续有 pass）。

## Capabilities

### New Capabilities
- `realtime-execution-stats`: 实时进度页节点执行统计的累计语义、WS 事件传输保真（逐迭代送达）、多源数据（REST trace / WS 快照 / WS 增量）幂等合并、页面往返后的状态重建。

### Modified Capabilities

（无——现有 specs 不涉及实时进度统计）

## Impact

- 后端 `internal/ws/hub.go`：`Message.dedupKey()` 加入 loop_index / span 维度；快照推送逻辑。
- 后端 `internal/ws/sendqueue.go`：dedup 语义注释与容量上界说明更新（消息量从 O(节点×链) 变为 O(节点×链×迭代)）。
- 后端 `internal/trace/trace.go`：Span/广播携带 span_id；快照回调（`internal/api/server.go`）补充 in-flight running 状态。
- 后端 `internal/core/dag/trace.go`：Running 广播路径确认携带足够标识。
- 前端 `web/app/src/composables/useExecutionWs.ts`：`SpanUpdateEvent` 增加 span_id。
- 前端 `web/app/src/composables/useExecutionStatus.ts`：`processEvent`/`initFromSpans` 改累计制 + 幂等去重；徽章判定逻辑调整。
- 前端 `web/app/src/views/scenes/DagFlow.vue`、`DagSceneNode.vue`：徽章展示适配（label 格式、badge.status 判定）。
- 测试：`internal/ws`、`internal/trace`、`internal/api` 相关测试更新；前端类型检查。
