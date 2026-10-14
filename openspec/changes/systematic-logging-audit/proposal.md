## Why

Salvo 在近期 Bug 排查中暴露了一个严重问题：当场景运行异常时（如 ImportYAML 变量格式不兼容导致 runner 启动即失败），后台日志没有任何有效信息帮助定位问题。场景"什么都没跑就结束"，没有 dashboard 数据，没有测试报告，而日志只记录了常规的 HTTP 请求，完全没有 runner 初始化失败的报错。

这反映出日志系统存在多个系统性缺陷：异常被 goroutine 吞没、早期初始化失败不创建运行记录、缺少 panic 恢复、TraceID 未贯穿所有日志等。随着项目复杂度增长，缺乏可观测性将严重阻碍开发效率和线上问题排查。

## What Changes

- **四级链路日志标准化**：确保 Scene Trace → Chain Trace → API Trace → Function Trace 四级日志全部实现，每级日志携带完整的 trace_id / chain_id / node_id 上下文
- **Runner 全流程日志增强**：从 Manager.Start → Run → buildDAG/buildScope → execute 的每个关键阶段，失败时输出 actionable 的错误日志并创建失败运行记录
- **Goroutine Panic 恢复**：所有后台 goroutine（Manager.Start、worker pool、metrics collector 等）添加 defer/recover 和日志记录
- **错误传播机制**：后台 goroutine 失败时，错误可被主流程或 API 发现并展示（包括跑记录 status 和 error_msg）
- **日志级别规范**：定义并实施各场景的日志级别使用规则（error/warn/info/debug）
- **TraceID 自动注入**：所有业务日志自动携带 trace_id、chain_id、node_id 等上下文
- **日志知识文档**：将日志规范沉淀为 .knowledge/ 知识，便于 AI Agent 和开发者参考

## Capabilities

### New Capabilities
- `logging-4layer-chain`: 四级链路日志标准化，确保 Scene/Chain/API/Function 四级在 runner、DAG executor、generator 中完整实现
- `logging-panic-recovery`: 全局 Goroutine Panic 恢复机制，覆盖所有后台 goroutine
- `logging-error-propagation`: 后台错误传播机制，失败可被 API/前端感知
- `logging-level-standards`: 日志级别使用规范，定义 error/warn/info/debug 各层的使用场景

### Modified Capabilities
- `runtime-metrics-collector`: 增加 RuntimeMetricsCollector goroutine 的生命周期日志和 panic 恢复
- `html-report-export`: 报告中对失败场景标注错误原因

## Impact

- **internal/runner/**: Manager.Start、Runner.Run、pool 执行路径增加日志和 panic 恢复
- **internal/trace/**: 检查并完善四级追踪实现，可能扩展 Span 数据结构
- **internal/core/dag/**: executor 增加节点执行、条件评估等关键路径日志
- **internal/core/pool/**: worker pool 增加任务级日志和 panic 恢复
- **internal/logger/**: 可能增加 WithContext 的 trace_id 自动注入能力
- **internal/generator/**: generator 函数增加 Function Trace 日志
- **internal/api/**: API handler 层完善错误日志、运行记录错误处理
- **.knowledge/**: 新增 L1-conventions/logging-standards.md