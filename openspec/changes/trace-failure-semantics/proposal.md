# Proposal: trace-failure-semantics

## Why

HTTP 200 但业务失败（body 中 errorCode 不为 0）的场景，节点层断言已正确判定失败（nodeStats/聚合视图），但 trace 链路显示成功——span 状态只取决于 `Execute()` 返回的 err，而节点失败通过 `Output.Error`（网络失败、HTTP 非 2xx 软失败）或容器吞错（while step 断言失败）传递时，executor 无法感知。用户无法通过 trace 链路发现失败，与统计口径不一致。

## What Changes

- executor trace 层感知 `Output.Error`：span 完成时以 `lastOutput.Error` 判定状态（覆盖网络失败路径，现状 span 误报 OK）
- while 容器吞错时将**首次** step 失败写入返回的 `Output.Error`（默认 `block_on_error=false` 场景，现状失败信息完全丢失）
- Trace 整体状态聚合：存在 error span 且未显式设置状态时，Trace 标 error，链路列表页可直接看出失败
- **普通节点断言失败遵循 block_on_error 语义**（**BREAKING**）：
  - `block_on_error=false`（默认）：expect_body 断言失败从"硬失败中断（errCh 报错 + 下游 skip）"改为**软失败**——记录 `Output.Error`，流程继续（AES/变量提取照常），下游节点照常执行，trace 标失败
  - `block_on_error=true`：保持硬失败中断语义不变
  - HTTP 非 2xx 软失败路径补齐 `Output.Error`（现状为空，trace 误报 OK）
  - 软失败路径 nodeStats 记 `RecordLatency(latency, false)`，节点级统计与 trace 口径对齐（现状断言失败发生在 nodeStats 记录之前，节点统计漏记）
- 与 while step 级 `block_on_error` 语义统一：false = 吞错不阻断，true = 中断

## Capabilities

### New Capabilities
- `trace-failure-semantics`: trace 链路成败以节点真实执行结果为准的完整语义——span 判定、容器失败传递、Trace 聚合、节点软失败（block_on_error 软失败语义）

### Modified Capabilities

（无现有 spec 目录下的对应能力，`openspec/specs/` 未建立该领域 spec）

## Impact

- `internal/core/dag/trace.go`：executeTraced 的 span.Finish 改为感知 Output.Error
- `internal/trace/trace.go`：Context.Finish 聚合 error span
- `internal/runner/runner.go`：sceneNode.Execute 的 expect_body 三处断言失败重构为软失败模式（block_on_error 判定）；HTTP 非 2xx 补 Output.Error；nodeStats 记录对齐
- `internal/runner/while_node.go`：吞错路径记录首次 step 失败到 Output.Error
- **行为变更**：默认（block_on_error=false）普通节点断言失败后下游从 skip 变为照常执行；依赖"断言失败即中断"的现有场景需显式配置 `block_on_error: true`
- 文档：`salvo-yaml-guide.md` 中 block_on_error 语义需更新（软失败描述）
