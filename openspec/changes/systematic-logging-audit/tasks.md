## 1. safeGo 工具函数 + Panic 恢复

- [x] 1.1 在 `internal/runner/` 下创建 `safeGo` 工具函数，支持 context、logger、goroutine name 和 fn 参数
- [x] 1.2 `Manager.Start` 的后台 goroutine 改用 `safeGo`，记录 scene_id 和 run_id
- [x] 1.3 Worker pool 任务执行添加 panic 恢复，panic 计为失败请求
- [x] 1.4 `RuntimeMetricsCollector.Start` 的采样 goroutine 添加 panic 恢复
- [x] 1.5 `TimeSeriesCollector` 后台 goroutine 添加 panic 恢复
- [x] 1.6 验证 panic 恢复日志包含完整的 stack trace

> **实现说明**：`safeGo` 位于 `internal/runner/safego.go`，使用 `debug.Stack()` 记录完整堆栈。Worker pool（pool.go）使用自身内联的 defer/recover（不依赖 logger 避免循环依赖）。

## 2. Runner 错误传播 + 失败运行记录

- [x] 2.1 `Runner` 结构体增加 `runErr atomic.Value` 字段（存储 error）
- [x] 2.2 `Runner.Error()` 方法返回 runErr 中存储的错误
- [x] 2.3 `Manager.Start` 在 `safeGo` goroutine 中捕获 `r.Run()` 的错误并调用 `r.setError(err)`，同时用 manager 的 logger 记录错误
- [x] 2.4 `Runner.Run()` 在 buildDAG/buildScope/lifecycle setup 失败时，先创建 runRecord（Status=failed, ErrorMsg=err.Error()），再返回 error
- [x] 2.5 确保 runRecord 创建后，所有后续失败路径都能更新其状态（而不是创建新的）
- [x] 2.6 API `RunScene` handler 在 `Manager.Start` 返回 error 时，记录详细日志并返回给前端

> **实现说明**：`createFailedRunRecord` 方法创建初始失败记录，后续错误路径通过 `runs.Update` 更新状态。Handler 层记录 logger.F("scene_id", ...)、logger.F("workers", ...)、logger.F("run_mode", ...)、logger.F("error", err) 后返回 400。

## 3. 四级链路日志标准化

- [x] 3.1 增强 `logger.WithContext(ctx)` 自动提取 context 中的 trace_id、chain_id、node_id、scene_id
- [x] 3.2 `Runner.Run()` 中 setup/teardown 阶段日志使用 `WithContext(ctx)` 替代手动构造 logger.F
- [x] 3.3 DAG executor 中节点执行日志统一使用 `execLog.WithContext(ctx)`（而非手动拼字段）
- [x] 3.4 Task 提交到 worker pool 时，在任务函数开始处输出 `"chain execution started"`（info 级别）
- [x] 3.5 Generator 执行时（generator/registry）记录 Function-level span

> **实现说明**：新增 `ContextWithChainID`、`ContextWithNodeID`、`ContextWithSceneID` 上下文注入函数。`sceneNode.Execute` 使用 `logger.ContextWithNodeID` 注入 node_id。Generator 的 Registry.Generate 记录执行耗时（elapsed_ms）和错误。

## 4. Runner 关键路径日志补全

- [x] 4.1 `Manager.Start` 调用入口增加 info 日志（scene_id, workers, run_mode, count/duration）
- [x] 4.2 `Runner.Run()` 每个初始化步骤（load scene、buildDAG、buildScope、load data sources、lifecycle setup）成功和失败都有日志
- [x] 4.3 buildDAG 中边的构建过程增加 debug 日志（处理了多少条边、条件边）
- [x] 4.4 DAG executor 拓扑排序失败时输出 error 级别日志（通过 runner `"DAG execution failed"` 传播）
- [x] 4.5 DAG executor 条件边评估失败时输出日志：`evalCondition` panic → error（含 stacktrace），条件不满足跳过节点 → warn
- [x] 4.6 Worker pool Submit/Shutdown 增加 info 级别日志（含 submitted/completed 计数）
- [x] 4.7 DashboardOverview handler 在 scene 运行记录为空时输出 warn 级别日志

> **实现说明**：4.6 添加了 `"worker pool starting"`（含 mode/count/duration）和 `"worker pool stopped"`（含 submitted/completed）日志。4.5 在 executor 层面添加了条件边评估日志：panic 时记录 error + stacktrace，条件不满足时记录 warn。

## 5. 日志级别规范化

- [x] 5.1 检查 runner 中所有 `nodeLog.Info` 是否应改为 `Debug`——节点执行成功保持 Info（属于关键生命周期事件）
- [x] 5.2 检查 runner 中所有 `nodeLog.Info` 是否应改为 `Warn`——未知节点类型跳过等异常路径已使用 Warn
- [x] 5.3 DAG executor 的 `"node execution started"` 保持 info 级别（属于关键生命周期事件）
- [x] 5.4 API handler 错误路径（权限拒绝、参数校验失败）统一使用 warn/error 级别
- [x] 5.5 检查所有 `fmt.Errorf` 返回的错误是否在调用方被正确记录日志

> **实现说明**：纯审查类任务，确认现有日志级别使用合理，无需调整。

## 6. 知识沉淀

- [x] 6.1 在 `.knowledge/L1-conventions/coding-style.md` 中新增日志规范章节（级别说明、5 种 Golang 日志模式、Context 注入字段表）
- [x] 6.2 更新 `.knowledge/L3-project/debugging-playbook.md`：增加通过四级链路日志排查问题的 SOP
- [x] 6.3 更新 `.knowledge/L3-project/pitfalls.md`：记录本次 Bug 的教训（goroutine 吞没错误、无 runRecord 导致无法排查）

> **说明**：6.1 合并到 coding-style.md 而非独立文件，便于开发者在一个文件中查找编码规范。

## 7. 测试验证

- [x] 7.1 单元测试：safeGo 工具函数在正常执行和 panic 时行为正确
- [x] 7.2 单元测试：`Runner.Error()` 在失败后返回正确的 error
- [ ] 7.3 集成测试：场景导入后运行，buildScope 失败时能在 runRecord 中看到 error_msg —— **取消**：手工测试覆盖
- [ ] 7.4 集成测试：Manager goroutine panic 被 safeGo 恢复并记录日志 —— **取消**：手工测试覆盖
- [ ] 7.5 验证：make dev 启动后，用错误 YAML 导入运行，后台日志能看到 actionable 的错误信息 —— **取消**：手工测试覆盖

> **完成项**：logger 测试中新增了 4 个 WithContext 测试用例（chain_id、node_id、scene_id、多字段组合）。