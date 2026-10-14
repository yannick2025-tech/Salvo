# Design: trace-failure-semantics

## Context

trace span 状态由 `executeTraced`（internal/core/dag/trace.go）的 `span.Finish(outputJSON, nil)` 决定，只感知 `n.Execute()` 返回的 err。但节点失败有三条传递路径：

1. **Execute err**（断言失败、block_on_error=true）：span 正确标 error，但 errCh 报错导致下游 skip / 链路中断
2. **Output.Error**（网络失败 `return &dag.Output{Error: err}, nil`；HTTP 非 2xx 软失败现状不设 Error）：executor 不读该字段，span 误报 OK；已验证全代码库该字段无消费者
3. **容器吞错**（while step 断言失败 + `block_on_error=false`）：while_node.go L523-L560 仅 Warn + 连续失败计数，失败信息不进入 Output，span 误报 OK

链路模型：runner Count 模式每次迭代生成独立 chainID，各执行一次 `ExecuteWithTrace` → 独立 Trace。场景 A→B→LOOPS(C→D→E)→F 迭代 5 次 = 5 条链路，每条含 A/B/LOOPS/F 四个 span。

## Goals / Non-Goals

**Goals:**
- trace 链路成败与节点真实执行结果（nodeStats 口径）一致：节点判失败 → span error → Trace error
- 普通节点断言失败支持软失败（`block_on_error=false`）：下游照常执行，链路标失败
- while 容器吞错时保留首次失败信息（用户确认：记首次，非最后一次）
- 失败链路在 trace 列表页直接可见（Trace.Status 聚合）

**Non-Goals:**
- 不改变 loop 容器 steps 的断言支持现状（loop step 无 expect_body，属独立功能补充）
- 不修改前端 traces 页面（已有 error 状态渲染）
- 不改变场景级 r.stats 的请求统计口径（HTTP 层为准）
- 不为软失败新增 YAML 配置项（复用现有 block_on_error，语义对齐 while step 级）

## Decisions

1. **span 判定统一为 `err != nil || Output.Error != nil`**（executeTraced L373）：`span.Finish(outputJSON, lastOutput.Error)`。Output.Error 无其他消费者，零侵入。
2. **软失败走 Output.Error 通道而非 executor 分支**：sceneNode 断言失败时记录 `assertionErr`，流程继续（AES/变量提取照常，保证下游变量可用），结尾按 block_on_error 判定——true 返回 err（硬失败，现状不变），false 设 `Output.Error` 返回。下游因 results 有值而照常执行，executor/容器（LOOPS/WHILE/GROUP/普通节点）无需任何改动。
3. **while 记首次失败**：循环外声明 `firstStepErr`，step 失败被吞时 `if firstStepErr == nil { firstStepErr = stepErr }`；正常退出路径统一在返回的 Output 上设置。generator step 失败分支同样处理。
4. **Trace 聚合在 `Context.Finish` 兜底**：仅当 Status 为空（未被 FinishWithError 显式设置）时扫描 spans，取第一个 error span 设置 `Trace.Status=error` + Error 摘要。显式状态（canceled/error）优先，不覆盖。
5. **nodeStats 对齐**：软失败路径执行 `nodeStats.RecordLatency(latency, false)`（现状断言失败 return 在记录之前，节点统计漏记断言失败）。
6. **HTTP 非 2xx 软失败补 Error**：1796 行后记录 non2xx 错误，随 Output.Error 一起带出（现状 recordFailedNode 已记录 UI 明细，但 Output.Error 为空导致 trace 误报 OK）。

## Risks / Trade-offs

- **BREAKING：默认语义变更**——`block_on_error` 未配置（默认 false）的普通节点，断言失败后下游从 skip 变为照常执行。现有场景若依赖隐式中断行为，需显式配置 `block_on_error: true`。与 while step 级语义统一是明确的平台决策（用户已确认）。
- **软失败链路 ExecuteWithTrace 返回 nil**：runner 不再记链路级 `RecordLatency(0,false)`，链路失败转移至 Trace.Status 体现；场景成功率统计以 HTTP 层为准，两层口径各自自洽。
- **下游数据级联**：B 软失败时 Response 保留（HTTP 200 场景），变量提取照常；若 B 网络失败 Response 为空，下游引用 B 变量可能取空——失败节点无有效输出的自然结果。
- **Trace 聚合的顺序依赖**：spans 按 AddSpan 顺序（完成序）扫描，Trace.Error 摘要取第一个 error span（拓扑上先完成者），非严格时间序——可接受（同一链路内节点完成顺序即执行顺序的近似）。
