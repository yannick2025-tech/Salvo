## ADDED Requirements

### Requirement: 节点执行统计为累计语义

实时进度页面的节点徽章统计 SHALL 采用累计语义：pass/fail/skip 为该节点自运行开始以来所有已结束迭代的累计次数，只增不减；running 为该节点当前正在执行的迭代数（随迭代开始增加、结束减少）。任一迭代结束后，其计入的结果 MUST NOT 因后续迭代的结果而减少。

#### Scenario: 后续成功不冲销先前失败
- **WHEN** 节点 A 在某链中第 1 轮迭代失败、第 2 轮迭代成功
- **THEN** 节点 A 徽章显示 fail=1、pass=1（而非 fail=0、pass=1）

#### Scenario: 连续多轮失败全部计入
- **WHEN** 节点 A 在某链中连续 3 轮迭代均失败
- **THEN** 节点 A 徽章显示 fail=3

#### Scenario: running 随迭代生命周期增减
- **WHEN** 节点 A 的第 5 轮迭代开始（收到 running 事件，loop_index=5）
- **THEN** 该链该节点 running 计 1；**WHEN** 第 5 轮迭代结束（收到终态事件，loop_index=5）
- **THEN** 该链该节点 running 归 0，pass/fail/skip 之一 +1

### Requirement: 每轮迭代事件独立送达

WebSocket 传输链路 SHALL 保证同一 (chain_id, node_id) 不同轮迭代的 span_update 事件互不去重合并：去重键 MUST 包含 loop_index。同一轮迭代内的 running 事件被后续终态事件覆盖 SHALL 是允许的（瞬态语义）。

#### Scenario: 同节点连续两轮事件不被合并
- **WHEN** 节点 A 第 1 轮失败事件（loop_index=0）尚未被 WritePump 刷出，第 2 轮成功事件（loop_index=1）进入发送队列
- **THEN** 两条消息均被送达客户端，第 1 条不被第 2 条覆盖

#### Scenario: 同轮 running 被终态覆盖
- **WHEN** 节点 A 第 i 轮的 running 事件（loop_index=i）尚未刷出，同轮终态事件（loop_index=i）进入发送队列
- **THEN** 客户端最终只收到该轮终态事件，running 不残留

### Requirement: 节点级终态广播携带迭代轮次

Span 的 Finish/Skip/FinishCanceled 广播事件 SHALL 携带该 span 最近一次 Running 广播的 loop_index（无 Running 调用时为 0），使前端能以 (chain_id, node_id, loop_index) 幂等去重，避免与每轮迭代事件双计。

#### Scenario: 正常完成后 Finish 与最后一轮事件不双计
- **WHEN** 节点 A 执行 3 轮迭代，每轮均广播终态事件（loop_index=0,1,2），随后 span Finish 广播终态（loop_index=2）
- **THEN** 前端累计 pass=3，Finish 广播因 loop_index=2 已应用而被跳过

#### Scenario: 取消中断计为该轮终态
- **WHEN** 节点 A 第 4 轮迭代进行中被手动取消，FinishCanceled 广播 canceled 事件（loop_index=4）
- **THEN** 前端该轮 running 清除且计入 skip，无 running 残留

### Requirement: 订阅快照重建累计状态

客户端订阅 run 后，服务端 SHALL 推送聚合快照（span_stats 消息）：每个 (chain_id, node_id) 一条，包含累计 pass/fail/skip、当前 in-flight 迭代索引集合、已应用的最大迭代索引。快照 MUST 能完整重建页面往返前的累计值与当前 running 并发。

#### Scenario: 返回页面后累计值恢复
- **WHEN** 节点 A 已累计 fail=2、pass=5，用户离开实时进度页后重新进入并订阅
- **THEN** 收到的 span_stats 显示该节点 fail=2、pass=5，徽章恢复相同数值

#### Scenario: 返回页面后 running 并发恢复
- **WHEN** 节点 A 在 3 条链中各有一个迭代正在执行，用户离开页面后重新进入并订阅
- **THEN** 快照包含各链该节点的 in-flight 迭代索引，徽章 running 恢复为 3

### Requirement: 三源数据幂等合并

前端 SHALL 对 REST trace 初始化、WS 订阅快照、WS 增量事件三源数据按 (chain_id, node_id, loop_index) 幂等合并：同一迭代的事件重复到达 MUST 只计一次；快照仅在比本地已应用状态更新（last_index ≥ 本地 applied_index）时应用；REST 初始化仅对本地无状态（applied_index=-1）的节点写入保守初值，MUST NOT 覆盖快照或增量已建立的状态。

#### Scenario: 增量先于快照到达不回退
- **WHEN** 客户端订阅后，增量事件（loop_index=3，pass）先于快照（last_index=2）被处理
- **THEN** 快照因 last_index=2 < 本地 applied_index=3 被忽略，本地累计值不回退

#### Scenario: REST 初始化不覆盖快照状态
- **WHEN** WS 快照已建立节点 A 状态（fail=2），随后 REST getTraceByRun 返回（span 最终态为 ok）
- **THEN** 节点 A 保持 fail=2，不被 REST 保守初值覆盖

#### Scenario: WS 不可用时 REST 兜底
- **WHEN** run 已结束且 WS 未连接，前端仅通过 REST trace 初始化
- **THEN** 节点徽章按 span 最终状态显示（至少 1 次计数），无空白状态

### Requirement: 累计徽章状态判定

节点徽章的整体状态（颜色语义）SHALL 按累计值判定，优先级为 running > fail > pass > skip > idle：任一链存在进行中迭代时显示 running；累计 fail>0 时显示 fail（即使后续有 pass）；仅当无 fail 且 pass>0 时显示 pass。

#### Scenario: 失败后有成功仍标红
- **WHEN** 节点 A 累计 fail=1、pass=5，无进行中迭代
- **THEN** 徽章状态为 fail（红色），label 显示 `5✓ 1✗`

### Requirement: run 结束清空运行中状态

Trace 完成时，服务端 SHALL 将该 run 全部节点的 in-flight 迭代集合清空，保证 run 结束后订阅快照与增量流不再残留 running 计数。

#### Scenario: 手动停止后 running 归零
- **WHEN** 运行被手动停止，部分链的节点迭代被取消（FinishCanceled 已按轮清理），随后 trace 完成
- **THEN** 最终快照中所有节点 running 集为空，前端无残留 ⟳ 计数
