## 1. 后端 trace 包：迭代广播与累计 stats

- [x] 1.1 `internal/trace/trace.go`：SpanBuilder 增加 `lastIterIndex` 字段，`BroadcastRunning` 更新之；`Finish/Skip/FinishCanceled` 的 `emitBroadcast` 改传 `lastIterIndex`（Skip 无 Running 保持 0）
- [x] 1.2 `internal/trace/trace.go`：新增 `NodeStats` 结构（Pass/Fail/Skip、RunningIdx map、LastIndex）与 `Trace.stats map[chain]map[node]*NodeStats`（复用 trace.mu），`emitBroadcast` 中同步更新计数（running→RunningIdx.Add；ok/error/skip→计数+1、RunningIdx.Delete、LastIndex=max）
- [x] 1.3 `internal/trace/trace.go`：`Trace.Finish` 清空全部 NodeStats.RunningIdx；新增 `SnapshotStats()` 深拷贝导出
- [x] 1.4 `internal/core/dag/trace.go`：SpanContext 接口新增 `IterationResult(loopIndex int, failed bool)`，spanAdapter 转发到 `SpanBuilder.emitBroadcast`
- [x] 1.5 `internal/core/dag/trace.go`：节点执行循环内每轮 `n.Execute` 返回后调用 `IterationResult(i, err != nil || output.Error != nil)`；硬失败（err != nil）return 前同样补发该轮事件
- [x] 1.6 `internal/trace/trace_test.go`：新增测试——多轮迭代累计计数、Finish 与最后一轮事件双发时 stats 不双计（LastIndex 幂等）、FinishCanceled 清 running、SnapshotStats 并发安全（-race）

## 2. 后端 ws 包：去重键与快照消息

- [x] 2.1 `internal/ws/hub.go`：`Message.dedupKey()` 对 span_update 加入 loop_index（`run/chain/node/loopIndex`）；确认 span_stats 类型返回 "" 不去重
- [x] 2.2 `internal/ws/hub.go`：Message 结构增加 span_stats 所需字段（pass/fail/skip、running_idx、last_index），或定义独立消息组装方式
- [x] 2.3 `internal/api/server.go`：SpanStateFunc 回调改为基于 `tr.SnapshotStats()` 输出 per (chain,node) 的 `span_stats` 消息（run 不存在或无 stats 时回退现状：SnapshotSpans 生成最终态 span_update）
- [x] 2.4 `internal/ws` 测试更新：dedupKey 含 loop_index 的行为验证（同轮覆盖、异轮保留）；sendqueue 注释同步

## 3. 前端：useExecutionWs 消息类型

- [x] 3.1 `useExecutionWs.ts`：新增 `SpanStatsEvent` 接口（type: 'span_stats'，chain_id/node_id/pass/fail/skip/running_idx/last_index），handleMessage 分发两个类型分别入队（spanUpdates 数组元素联合类型或独立 statsEvents 数组）

## 4. 前端：useExecutionStatus 累计制重构

- [x] 4.1 数据结构改造：chainStatuses 值改为 per-node `{pass, fail, skip, runningIdx: Set<number>, appliedIndex: number}`（初始 -1），aggregateStatus 派生逻辑改为累计求和（running = 各链 runningIdx.size 之和）
- [x] 4.2 `processEvent`（span_update）幂等累计：终态事件 loop_index ≤ appliedIndex 跳过，否则计数 +1、runningIdx.delete、appliedIndex=max；running 事件 loop_index > appliedIndex 时加入 runningIdx
- [x] 4.3 新增 `processStatsEvent`（span_stats）：last_index ≥ appliedIndex 时重置该 (chain,node) 全部字段，否则忽略（防乱序回退）
- [x] 4.4 `initFromSpans` 改合并语义：仅对 appliedIndex=-1（无状态）的 (chain,node) 写入保守初值（最终 span 状态计 1 次），已有状态不覆盖
- [x] 4.5 `computeAggregateStatus` 徽章判定调整：running > fail > pass > skip > idle；label 数值含义随累计语义（格式不变）
- [x] 4.6 DagFlow.vue：span_stats 事件流接入（watch 队列长度触发处理），确认 REST init 与 wsConnect 并行时序下不再互相覆盖
- [x] 4.7 `vue-tsc -b` 类型检查通过，无新增错误

## 5. 回归验证与收尾

- [x] 5.1 后端全量测试：`go test ./internal/trace/... ./internal/ws/... ./internal/api/... ./internal/core/...`（含 -race）
- [ ] 5.2 手动场景验证：多轮失败后成功（fail 不回落）、离开/返回页面（累计值与 running 恢复）、手动停止（running 清零、canceled 计 skip）、并发多链聚合视图（需用户实际运行场景验证；注：`TestExecutorWithLoggers` 的 -race 失败为预存在问题，基线可复现，与本变更无关）
- [x] 5.3 codegraph sync + 变更后自检清单 + 更新 tasks.md 勾选状态

## 6. 实施后修复（5.2 手动验证发现：页面重进累计值从 1 重涨）

- [x] 6.1 根因：runner 持续生成新链（每链独立 Trace，同 runID 在 active map 相互覆盖），订阅快照 `ByRunID` 只能拿到最新一条链的 stats。修复：stats 从 Trace 迁移到 Tracer 级 per-runID 聚合（`RunStats`），`SpanStateFunc` 回调改用 `tracer.SnapshotRunStats(id)`（不再依赖 ByRunID 取 stats）
- [x] 6.2 runner 在 `p.Wait()` 后调用 `tracer.FinishRun(runID)`：run 结束清 running 标记、保留累计计数（结束后进入页面仍可见最终总数）
- [x] 6.3 内存有界：单 run 保留最近 32 个 run 的 stats；单 run 链数超 2048 时最老链折叠进 `_merged` 伪链（聚合视图总数精确，单链视图丢失最老链明细）
- [x] 6.4 新增回归测试 `TestRunStatsAggregateAcrossChains`（45 链同 runID 聚合）；`TestNodeStatsFinishClearsAllRunning` 调整为 `TestFinishRunClearsAllRunning`（run 级语义）；trace/ws/api -race、runner、core/dag 全量通过
