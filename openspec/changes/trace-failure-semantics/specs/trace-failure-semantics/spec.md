# Spec Delta: trace-failure-semantics

## ADDED Requirements

### Requirement: span 状态以节点真实执行结果为准
span 的成败必须同时反映 `Execute()` 返回的 error 与 `Output.Error` 字段——任一非空即标 error。

#### Scenario: 网络失败的 span 标记 error
- **WHEN** 节点 HTTP 请求网络失败（返回 `Output.Error` 非空，Execute 不返回 err）
- **THEN** 该节点 span 状态为 error，span.Error 含网络错误详情

#### Scenario: 正常执行的 span 标记 ok
- **WHEN** 节点执行成功且 `Output.Error` 为空
- **THEN** span 状态为 ok

### Requirement: 普通节点断言失败遵循 block_on_error 软失败语义
expect_body 断言失败（如 HTTP 200 但 body 中 errorCode 不为 0）时，按节点 block_on_error 配置区分硬失败与软失败。

#### Scenario: 断言失败且 block_on_error=false 时软失败
- **WHEN** 节点配置 expect_body 断言失败且 `block_on_error=false`（默认）
- **THEN** 节点不返回 error，`Output.Error` 含断言详情（期望值与实际值），Response 与变量提取照常
- **THEN** 下游节点（含 loop/while/group/普通节点）照常执行，不 skip
- **THEN** 该节点 span 状态为 error，链路 Trace 状态为 error

#### Scenario: 断言失败且 block_on_error=true 时硬失败
- **WHEN** 节点配置 expect_body 断言失败且 `block_on_error=true`
- **THEN** 节点返回 error，链路中断（下游 skip、全链 cancel），行为与现状一致

#### Scenario: HTTP 非 2xx 软失败携带错误信息
- **WHEN** 节点 HTTP 响应非 2xx 且 `block_on_error=false`
- **THEN** `Output.Error` 含 `HTTP <code>` 错误信息，流程继续（现状语义），span 状态为 error

#### Scenario: 断言软失败时节点统计记失败
- **WHEN** 节点断言软失败（block_on_error=false）
- **THEN** nodeStats 以 success=false 记录该次执行，聚合视图 fail 计数与 trace 一致

### Requirement: while 容器吞错时向 trace 传递首次失败
while step 失败被吞（`block_on_error=false` 默认路径）时，容器最终返回的 Output 必须携带首次 step 失败信息。

#### Scenario: while step 断言失败后容器正常退出
- **WHEN** while 某次迭代中 step expect_body 断言失败（未达 fail_after_consecutive 阈值）且 while 按退出条件正常结束
- **THEN** while 节点 span 状态为 error，span.Error 含**首次**失败 step 的名称与断言详情
- **THEN** while 执行语义不变（继续迭代、后续 step 照常）

#### Scenario: while 无 step 失败
- **WHEN** while 所有迭代所有 step 成功
- **THEN** while 节点 span 状态为 ok，Output.Error 为空

### Requirement: Trace 整体状态聚合 error span
Trace 未显式设置状态（默认 OK）时，若存在 error span，Trace 整体状态必须为 error。

#### Scenario: 存在 error span 的链路聚合
- **WHEN** 链路执行完成（ExecuteWithTrace 未返回 error）且 spans 中存在状态为 error 的 span
- **THEN** Trace.Status 为 error，Trace.Error 含第一个 error span 的节点 ID 与错误摘要

#### Scenario: 显式 canceled 状态不被覆盖
- **WHEN** 链路手动停止（FinishTraceWithCanceled）且存在 error span
- **THEN** Trace.Status 保持 canceled（显式状态优先于聚合）

#### Scenario: 全成功链路保持 ok
- **WHEN** 链路所有 span 状态为 ok/skip
- **THEN** Trace.Status 为 ok
