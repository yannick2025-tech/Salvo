## Context

Salvo 的 Dashboard 和报告系统在导入默认 `example-v2.yaml` 以时长模式运行后，全局指标全部显示 0 或异常值。经系统性代码审查，确认 6 个独立 bug 叠加导致：

1. **example-v2.yaml base_url 指向不存在域名** — `env=staging` → `api_host=staging-api.example.com` → DNS 解析失败 → 100% 错误率，所有 HTTP 延迟记录为 0
2. **TimeSeriesCollector runID 与 run_record ID 不匹配** — `runner.go` 的 `New()` 函数中 `n.Generate()` 被调用两次（L294 和 L299），产生两个不同 ID；时序数据以 collector 的 runID 存储，但 Dashboard 和报告查询用 run_record 的 ID → 查询永远返回空
3. **loadSystemMetricsFromDB 硬编码 TaskWait P50/P95 = 0** — `handler.go` L2364-2365 将 P50 和 P95 硬编码为 0
4. **场景停止后 Dashboard 仍读 live stats** — runner 停止后若仍在 `runningMap` 中，Dashboard 继续从 live runner 读取延迟，in-flight 请求完成导致 `latencyList` 变化
5. **全局 P50 聚合逻辑** — 对于已完成的历史 run_record，只取 P50/P90/P99 三个点放入 `allLatencies`，而非原始延迟数据
6. **失败请求详情在报告中可能缺失** — 需验证 `failed_nodes` 字段的序列化、存储和渲染链路

## Goals / Non-Goals

**Goals:**
- 修复默认 YAML 配置使开箱即用体验正常
- 修复 runID 不匹配使时序数据（QPS/P50/P95/P99）在 Dashboard 和报告中正确显示
- 修复 PendingQueue TaskWait P50/P95 硬编码为 0
- 消除场景停止后延迟值变化的用户困惑
- 修复全局 P50 聚合准确性
- 确保失败请求详情在测试报告和 HTML 报告中正确展示

**Non-Goals:**
- 不重构 Dashboard 聚合架构（仅修复 bug）
- 不新增功能（仅修复已有功能的 bug）
- 不修改前端代码（除非轮询逻辑需要调整）

## Decisions

### Decision 1: runID 统一生成（根因 2 核心修复）

**选择**: 在 `Runner.New()` 中只调用一次 `n.Generate()`，用局部变量 `runID` 同时赋给 `Runner.runID` 和 `TimeSeriesCollector`。

**替代方案**: 在 collector 创建后修改其 runID 字段 → 需要暴露 setter 或公开字段，破坏封装性。

**理由**: 最小改动、最直观、不引入新 API。

### Decision 2: 停止后延迟变化的修复策略

**选择**: 在 `DashboardOverview` handler 中，对 `rr.Status == "running"` 的场景增加检查 runner 的 `Status()`。若 runner 状态非 `running`（已停止），改用 `run_records` 存储值而非 live stats。

**替代方案**: 在 runner 停止时立即从 `runningMap` 移除 → 可能导致最后一批 in-flight 请求的统计数据丢失。

**理由**: 保留 runner 在 runningMap 中便于最终数据采集，但 Dashboard 读取时判断 runner 实际状态。

### Decision 3: 全局 P50 聚合逻辑修复

**选择**: 对已完成的历史 run_record（非 running），保持现有逻辑（只取 P50/P90/P99 三个点），因为原始延迟数据不可用（已被百分位聚合）。对运行中的场景，已通过 `GetAllLatencies()` 获取完整原始延迟列表（L2098-2101），此路径已正确。

**理由**: 历史数据无法恢复原始延迟，只能用百分位近似。运行中场景的聚合逻辑已正确，无需修改。

### Decision 4: loadSystemMetricsFromDB P50/P95 修复

**选择**: 从 `sm.Summary` 结构中读取 `TaskWaitP50MaxMs` 和 `TaskWaitP95MaxMs`（需要确认 summary 是否包含这些字段；如不包含，在 `ComputeSummary` 中补充计算）。

**替代方案**: 从 time series 的最后一个数据点读取 P50/P95 → 不够代表性。

**理由**: Summary 应包含完整的统计摘要。

## Risks / Trade-offs

- **[Risk] runID 修改可能影响已有数据** → 仅影响新创建的 run，已有数据不受影响（已有数据的时序本就查询不到）
- **[Risk] 停止后延迟判断依赖 runner.Status() 可能有窗口期** → 可接受的 trade-off，窗口期极短（毫秒级）
- **[Risk] loadSystemMetricsFromDB 修复需要确认 Summary 结构** → 如缺少字段需同步修改 `ComputeSummary`
