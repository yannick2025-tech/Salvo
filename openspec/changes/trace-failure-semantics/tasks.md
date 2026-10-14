# Tasks: trace-failure-semantics

## 1. 修改点 4：sceneNode 断言软失败（runner.go）

- [x] 1.1 写失败测试：expect_body 断言失败 + block_on_error=false → Execute 返回 nil、Output.Error 含断言详情、Response 保留
- [x] 1.2 写失败测试：expect_body 断言失败 + block_on_error=true → Execute 返回 err（现状回归保护）
- [x] 1.3 写失败测试：HTTP 非 2xx + block_on_error=false → Output.Error 含 HTTP 错误
- [x] 1.4 实现：expect_body 三处断言失败（field not found / path not found / value mismatch）重构为记录 assertionErr 并继续流程，结尾按 block_on_error 判定返回
- [x] 1.5 实现：HTTP 非 2xx 软失败路径补 non2xx 错误到 Output.Error
- [x] 1.6 实现：软失败路径 nodeStats.RecordLatency(latency, false) 对齐（含 ASSERT-FAIL 错误码）
- [x] 1.7 跑测试确认全绿

## 2. 修改点 1：executor span 感知 Output.Error（core/dag/trace.go）

- [x] 2.1 写失败测试：节点返回 Output{Error} + nil err → span 状态 error、下游照常执行
- [x] 2.2 实现：executeTraced 的 span.Finish(string(outputJSON), lastOutput.Error)
- [x] 2.3 跑测试确认全绿

## 3. 修改点 2：while 容器吞错记首次失败（while_node.go）

- [x] 3.1 写失败测试：step expect_body 断言失败（block_on_error=false）+ while 正常退出（max_iterations 路径）→ Output.Error 含首次失败 step 详情
- [x] 3.2 写测试：多 step 失败 → Output.Error 为首次失败（非最后）
- [x] 3.3 实现：主循环外声明 firstStepErr，HTTP step 与 generator step 失败分支记录首次失败
- [x] 3.4 实现：while 三个正常退出路径（exit met / max_iterations / max_duration）统一设置 Output.Error
- [x] 3.5 跑测试确认全绿

## 4. 修改点 3：Trace 聚合 error span（internal/trace/trace.go）

- [x] 4.1 写失败测试：span error + FinishTrace()（无显式错误）→ Trace.Status=error、Error 含节点摘要
- [x] 4.2 写测试：FinishTraceWithCanceled + error span → Status 保持 canceled
- [x] 4.3 实现：Context.Finish 在默认 OK 状态时扫描 spans 聚合
- [x] 4.4 跑测试确认全绿

## 5. 验证与文档

- [x] 5.1 go test ./... 全量回归（30 包全绿）+ go vet
- [x] 5.2 codegraph sync
- [x] 5.3 评估并更新 salvo-yaml-guide.md 的 block_on_error 语义（软失败描述）与 trace 行为说明（8.1 更新 + 新增 8.2 软失败与 trace 链路成败 + 参考行号修正）
- [x] 5.4 评估 .knowledge/ 知识库是否需要更新（pitfalls.md 新增 Lesson 9：软失败被 trace 吞没；debugging-playbook.md 触发条件表补索引行）

## 6. 补充：group 容器吞子节点软失败（A→B→LOOPS(C→D→E)→F 场景验证暴露）

- [x] 6.1 写链路级测试（chain_soft_fail_test.go，9 个用例）：A 硬失败 SKIP / A 软失败继续 / A 无断言成功 / D 软失败（红）/ D 硬失败中断 / D 无断言 / block 在 A 上（红）/ block 在 LOOPS 上 + D 软失败（红）/ block 在 LOOPS 上 + D 硬失败
- [x] 6.2 实现：executeGroup 记录首个子节点软失败（firstChildSoftErr），正常退出写入 Output.Error；nodeStats RecordLatency success 参数对齐
- [x] 6.3 确认 block_on_error 在 group 上仅对 group 自身失败生效（子节点硬失败/配置错误），对子节点软失败无效（无触发时机）
- [x] 6.4 go test ./... 全量回归零失败 + 更新 salvo-yaml-guide.md 8.2 group 说明 + Lesson 9 补充
