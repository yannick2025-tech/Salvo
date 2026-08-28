# fix-dashboard-metrics-zero - Implementation Tasks

## 1. 修复 example-v2.yaml base_url 配置

- [x] 1.1 将 `configs/example-v2.yaml` 中 `env` 变量从 `staging` 改为 `local`，使 `api_host` 解析为 `localhost:9090`
- [x] 1.2 验证修改后 base_url 展开为 `http://localhost:9090/mock/api`，与 salvo.yaml 和 example-full-coverage.yaml 一致

## 2. 修复 TimeSeriesCollector runID 不匹配（QPS=0 根因）

- [x] 2.1 在 `internal/runner/runner.go` 的 `New()` 函数中，将 `n.Generate()` 提取为局部变量 `runID`，用于同时赋给 `Runner.runID` 和 `TimeSeriesCollector`
- [x] 2.2 验证 `r.runID`、`r.dbRecordID`（run_record.ID）、TimeSeriesCollector.runID 三者一致
- [ ] 2.3 验证 Dashboard 的 `buildTimeSeriesWithDB` 查询 `tsStore.QueryByRunID(ctx, run.ID)` 能返回时序数据
- [ ] 2.4 验证 `createReport` 中 `r.tsStore.QueryByRunID(reportCtx, r.dbRecordID)` 能返回时序数据

## 3. 修复 loadSystemMetricsFromDB 中 TaskWait P50/P95 硬编码为 0

- [x] 3.1 检查 `SystemMetricsSummary` 结构体是否包含 `TaskWaitP50MaxMs` 和 `TaskWaitP95MaxMs` 字段（确认缺少）
- [x] 3.2 在 `internal/runner/runtime_metrics.go` 的 `ComputeSummary()` 中补充 P50/P95 的计算
- [x] 3.3 修改 `internal/api/handler.go` 的 `loadSystemMetricsFromDB`，将 `TaskWaitP50Ms: 0` 和 `TaskWaitP95Ms: 0` 改为从 `sm.Summary` 正确读取
- [ ] 3.4 验证 Dashboard 的 Pending Queue 图表显示非零的 TaskWait P50/P95 值

## 4. 修复场景停止后节点延迟仍变化

- [x] 4.1 在 `internal/api/handler.go` 的 `DashboardOverview` 中，对 `rr.Status == "running"` 的场景增加检查 `rn.Status()`
- [x] 4.2 若 runner 状态非 running（已停止），改用 `run_records` 存储值而非 live stats
- [ ] 4.3 验证场景停止后 Dashboard 中的延迟值不再随时间变化

## 5. 验证失败请求详情展示（代码已有实现，确认无 bug）

- [x] 5.1 确认 `recordFailedNode` 在连接错误和非2xx响应时均被调用（L1714, L1778）
- [x] 5.2 确认 `r.failedNodes` 切片正确初始化并通过指针共享给 sceneNode
- [x] 5.3 确认 `createReport` 中 `FailedNodes: r.failedNodes` 正确序列化到 report detail JSON
- [x] 5.4 确认 HTML 模板 `{{if .FailedNodes}}` 正确渲染"失败节点详情"section
- [ ] 5.5 端到端验证：导出 HTML 报告包含失败节点详情

## 6. 验证与测试

- [x] 6.1 运行 `go build ./internal/runner/... ./internal/api/...` 确保编译通过
- [x] 6.2 运行现有测试 `go test ./internal/runner/... ./internal/api/...` 确保无回归（预存在的 Manhattan 测试失败除外）
- [ ] 6.3 端到端验证：导入 example-v2.yaml，时长模式运行 1 分钟，检查 Dashboard 所有指标正常
- [ ] 6.4 端到端验证：导出 HTML 报告，检查时序图表和失败节点详情 section
- [ ] 6.5 端到端验证：停止场景后观察 Dashboard 30 秒，确认延迟值不再变化

## 7. 修复 Timer 节点强制 ExecAsync 导致下游全 skip（批跑问题 1/2 根因）

- [x] 7.1 在 `internal/core/dag/trace.go` 中，async 父节点不再被标记为 `parentFailed`，下游节点可继续执行
- [x] 7.2 在 `internal/runner/runner.go` 的 `buildDAGNode` 中，timer 不再硬编码 `ExecAsync`，改为按 mode 决定：`delay` → `ExecSync`（思考时间，下游等待），`interval` → `ExecAsync`（心跳，下游立即继续）
- [x] 7.3 在 `internal/runner/runner.go` 的 `executeTimer` 中，统一 `delay`/`duration`/`interval` 字段为毫秒单位（此前 `delay: 500` 被解析为 500 秒）
- [x] 7.4 interval 模式支持 `duration` 字段控制总运行时长，到期后正常返回成功
- [x] 7.5 interval 模式被 ctx 取消时返回成功而非 error（心跳语义）
- [x] 7.6 更新 `TestTimerNodeAlwaysAsync` 为 `TestTimerNodeModeBasedExec`，验证 delay=sync、interval=async

## 8. 移除 handler.go 中的 rand 注入并过滤 skip span（批跑问题 3 根因）

- [x] 8.1 移除 `aggregateNodeMetricsWithSceneID` 中对 dur<0.03 的 span 注入 30-200ms 随机值的逻辑
- [x] 8.2 移除 `aggregateNodeMetrics` 中同样的 rand 注入逻辑
- [x] 8.3 在两处聚合函数的 allDurs 计算循环中过滤 `status=="skip"` 的 span（0 时长非真实请求，不参与计数/延迟/QPS）
- [x] 8.4 在两处聚合函数的时间序列 bucket 循环中过滤 skip span
- [x] 8.5 移除未使用的 `math/rand/v2` import
- [ ] 8.6 端到端验证：批跑后 Dashboard 节点 P50/P95/P99 显示真实值且停止后不再变化
