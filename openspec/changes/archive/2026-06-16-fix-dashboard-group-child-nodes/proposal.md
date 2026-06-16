## Why

Dashboard 页面的"节点指标"统计缺少 Group 子节点的数据。HTML 测试报告正确显示全部 20 个节点（含 5 个 Group 子节点），但 Dashboard 仅显示 15 个节点，缺失的恰好是 Group 下的子节点（如"订单处理子步骤1/2/3"等）。

根因：两个视图使用了不同的数据源——HTML 报告从 `r.nodeStats`（内存统计 map）读取，包含所有节点；而 Dashboard 从 `h.tracer.List()`（链路追踪 span）聚合，Group 子节点通过 `executeGroup` 直接调用执行，未经过 DAG executor 的 span 记录流程，导致 tracer 中不存在这些子节点的 span。

## What Changes

- **修复 Dashboard 节点指标数据源**：让 `aggregateNodeMetricsWithSceneID` 能够获取到 Group 子节点的统计数据
- **方案选择（推荐方案 B）**：在 `aggregateNodeMetricsWithSceneID` 中增加 fallback 逻辑，当 tracer 数据不足时，从最近一次运行的 report detail（`node_metrics` 字段）中补充 Group 子节点数据
- **不改变现有行为**：HTML 报告、时序数据存储等不受影响

## Capabilities

### New Capabilities
- `dashboard-group-child-metrics`: 确保 Dashboard 节点指标聚合逻辑能够覆盖 Group 子节点的执行统计

### Modified Capabilities
- （无，现有 spec 层面需求不变，仅实现缺陷修复）

## Impact

- **受影响代码**：
  - `internal/api/handler.go` — `aggregateNodeMetricsWithSceneID()` 函数（约 L2308）
  - 可能需要新增一个辅助方法从 report store 读取 node_metrics 作为补充数据源
- **不受影响**：
  - HTML 报告生成（`runner.go` 的 `createReport`）— 已正确包含子节点
  - 时序数据采集（`timeseries_collector.go`）— 已通过 `NodeSnapshots()` 覆盖全部节点
  - 前端展示层（`DashboardPage.vue`）— 数据驱动，无需改动
