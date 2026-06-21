## Context

Salvo 的设计规范定义了四级追踪体系（Scene → Chain → API → Function），但在实际实现中存在多个缺口：

1. **Runner 异常被吞没**：`Manager.Start` 以 `_ = r.Run(ctx)` 启动 goroutine，任何初始化失败（buildDAG、buildScope）仅在内部日志输出，不会被前端感知。更严重的是，在本次 Bug 前，内部日志也没有输出足够的上下文信息（缺失 trace_id、场景信息等）。

2. **早期失败无运行记录**：buildDAG/buildScope 失败发生在创建 runRecord 之前，导致数据库中没有任何失败记录。前端 Dashboard 显示为空，用户完全无法判断发生了什么。

3. **Goroutine 缺少 Panic 恢复**：Manager.Start 的后台 goroutine、worker pool 的任务执行、RuntimeMetricsCollector、TimeSeriesCollector 的后台采样 goroutine 均没有 defer/recover。

4. **TraceID 未贯穿所有日志**：虽然节点执行日志携带 trace_id/chain_id/node_id，但 setup/teardown 阶段、pool 任务调度、generator 函数执行等路径缺少这些上下文。

5. **Function Trace 完全缺失**：设计规范中的第四级（Function/Generator 追踪）从未实现。

6. **日志级别使用不统一**：部分错误场景用了 Info（如 DAG 执行失败用 Info）、部分正常流程用了 Debug、关键路径缺少 Warn 级别提示。

### 当前日志架构

```
Logger (interface) → zapLogger (zap backend)
  ├── With(fields)  → 子 Logger（预绑定字段）
  ├── WithContext(ctx) → 从 context 提取 trace_id 注入
  ├── 输出到文件(lumberjack) + stdout(io.MultiWriter)
  └── 级别: debug / info / warn / error / fatal
```

### 当前追踪架构

```
Tracer (in-memory ring buffer)
  ├── Start(ctx, sceneID, runID) → TraceContext
  │     ├── StartSpan(nodeID) → SpanContext
  │     └── FinishTrace() / FinishTraceWithError()
  └── List() → []*Trace (API 查询用)
```

## Goals / Non-Goals

**Goals:**
- 四级链路日志（Scene/Chain/API/Function）全部实现，关键路径日志携带完整的 trace_id/chain_id/node_id
- Manager.Start 和 Runner.Run 的异常能被前端和运行记录感知
- 所有后台 goroutine 有 panic 恢复机制，panic 时输出完整堆栈
- 日志级别使用规范化，error/warn/info/debug 各司其职
- 建立日志知识和排查文档，沉淀到 .knowledge/L3-project/

**Non-Goals:**
- 不引入新的日志框架或外部依赖（继续使用 zap）
- 不改变现有的 Trace/Span 数据模型（仅补充缺失的环节）
- 不涉及前端日志采集（前端 Vue 应用的日志是独立问题）
- 不实现分布式追踪（非分布式架构）
- 不改变现有的日志配置格式和输出方式

## Decisions

### D1: Runner 失败运行记录

**决策**：在 `Runner.Run()` 中将 runRecord 创建提前到 buildDAG/buildScope/lifecycle setup 之前。

**理由**：
- 当前创建 runRecord 的时机在生命周期 setup 之后、execute 之前（第 362 行），而 buildScope 失败在第 323 行
- 提前创建 runRecord 可以记录 Status=Failed + ErrorMsg，前端可通过运行记录列表直接看到失败原因
- 不改变 runRecord 表结构（已有 ErrorMsg 字段，见 dto.RunRecordDTO）

**代价**：如果生命周期 setup 失败，需要更新 runRecord 状态。需要确保 runRecord 创建后，任何后续失败路径都能更新它。

### D2: Manager.Start 错误传播

**决策**：Manager.Start 的后台 goroutine 中记录 error 到 Runner 的某个字段，同时 Manager 暴露 Error() 方法供 API 查询。

**替代方案**：
- **使用 channel 传递错误**：过于复杂，API 侧需要轮询或 select
- **回调通知**：侵入性强，破坏 Manager 的简洁接口

**选择理由**：atomic.Value 存储 error 最简单，API 查询时通过 Manager.List() 返回的 Runner 快照即可获取。

### D3: Panic 恢复的统一模式

**决策**：所有后台 goroutine 统一使用 `safeGo(ctx, fn)` 工具函数替代裸 `go fn()`。

```go
func safeGo(ctx context.Context, log logger.Logger, name string, fn func()) {
    go func() {
        defer func() {
            if r := recover(); r != nil {
                log.Error("goroutine panicked",
                    logger.F("goroutine", name),
                    logger.F("panic", r),
                )
            }
        }()
        fn()
    }()
}
```

**理由**：
- 统一模式避免每个地方写重复的 defer/recover
- 强制要求提供 goroutine name 便于问题定位
- 可集中控制 panic 时的输出格式（堆栈、trace_id 等）

### D4: TraceID 自动注入

**决策**：增强 `logger.WithContext(ctx)` 实现，自动从 context 中提取 trace_id、chain_id、node_id、scene_id。

**实现方式**：
- 定义 context key 常量（已有 `dag.ChainIDKey` 等）
- `WithContext` 自动提取这些值并 `With()` 绑定到子 Logger
- runner、node executor、generator 等统一使用 `log.WithContext(ctx)` 而非 `log.With(logger.F("trace_id", ...))`

**理由**：
- 消除手动构造 logger.F("trace_id", ...) 的重复代码和遗漏风险
- context 传递是 Go 的惯用模式，与 cascade 包的设计一致
- 新的代码路径只要传递了 context，自动获得日志上下文

### D5: Function Trace 实现

**决策**：在 generator 执行时通过 Tracer 记录 Function 级别的 Span。

**实现方式**：
- Generator Executor（如 number、uuid、csv 等）在执行时调用 `tctx.StartSpan(functionName)`
- Function Span 的 ParentNodeID 指向当前执行的 node_id
- 不持久化到数据库（仅内存），通过 Tracer.List() 供 Dashboard 查询

**理由**：
- Function 级别日志量极大（每次变量替换都触发），不适合全部持久化
- 内存保留 + 可查询的折衷方案满足调试需求

### D6: 日志级别规范

| 级别 | 使用场景 | 示例 |
|------|---------|------|
| **Error** | 确凿的系统故障，需要人工介入 | buildScope 失败、HTTP 请求超时、panic |
| **Warn** | 非致命异常，可能影响部分结果 | DataSource 解析失败、降级路径、未知节点类型 |
| **Info** | 关键生命周期事件、可追踪的业务流程 | 场景启动/完成、节点执行开始/结束、DAG 构建成功 |
| **Debug** | 诊断细节，仅在开发/排查时有用 | 变量值快照、数据源行注入、节点统计快照 |

## Risks / Trade-offs

| 风险 | 缓解措施 |
|------|---------|
| 增加大量日志 IO 影响性能 | Function Trace 仅在内存中记录；Debug 级别在生产环境建议关闭 |
| runRecord 创建提前后，createReport 可能在 runRecord 未完成时读取 | execute 完成后才触发 createReport；中间状态不可生成报告 |
| safeGo 工具函数可能被绕过 | 代码审查强制要求所有新 goroutine 使用 safeGo；可添加 linter 规则 |
| TraceID 自动注入增加 context 取值开销 | context.Value 是 O(1) 操作，开销可忽略 |

## Migration Plan

1. **Phase 1**：实现 safeGo 工具函数 + Manager.Start/Runner.Run 错误传播 + runRecord 提前创建
2. **Phase 2**：实现 TraceID 自动注入 + 补全 runner/DAG executor 关键路径日志
3. **Phase 3**：实现 Function Trace + generator 日志
4. **Phase 4**：补全 setup/teardown/pool/metrics collector 的 panic 恢复和日志
5. **Phase 5**：沉淀.logging-standards.md + debugging 知识到 .knowledge/

各阶段可独立发布，无强依赖关系。

## Open Questions

- Function Trace 的 Span 数据需要保留多久？是否需要一个独立的 TTL 配置？
- sendGo 工具函数是否应该继承调用者的 logger（通过闭包捕获）还是通过参数传递？
- 日志输出中 TraceID 长度（Snowflake ID 最长 19 位数字）是否需要在 Logger 层做格式化优化？