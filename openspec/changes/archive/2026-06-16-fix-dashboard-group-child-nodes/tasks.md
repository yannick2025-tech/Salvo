## 1. 核心逻辑修改

- [x] 1.1 在 `aggregateNodeMetricsWithSceneID()` 函数末尾（返回 result 前），新增 Report Fallback 逻辑：收集 tracer 中已有的 nodeID 集合
- [x] 1.2 查询目标 scene 最近一次已完成运行的 report detail（通过 `h.reports.GetByRunID` 或 `h.reports.List` + 过滤）
- [x] 1.3 遍历 report 的 `node_metrics`，对每个不在 tracer nodeID 集合中的节点，构建 `dto.NodeMetricDTO` 并追加到 result
- [x] 1.4 处理 report 查询失败/为空的边界情况：静默跳过，不影响原有返回值

## 2. 数据映射与格式适配

- [x] 2.1 实现 `reportNodeMetricToDTO()` 辅助方法：将 `runner.NodeMetricDetail` 映射为 `dto.NodeMetricDTO`（字段对应：TotalRequests→TotalReqs, AvgLatencyMs→AvgLatency 等）
- [x] 2.2 确保补充节点的 name/type 字段从 nodeNameMap/nodesByType 正确填充
- [x] 2.3 补充节点的时间序列字段设为空数组，Dashboard 前端需兼容无时序数据的节点展示

## 3. 验证

- [x] 3.1 编译通过：`go build ./...`
- [x] 3.2 功能验证：启动服务，运行含 Group 子节点的场景，确认 Dashboard 节点数 = HTML 报告节点数（20 个）
- [x] 3.3 回归验证：运行不含 Group 节点的场景，确认 Dashboard 行为不变

## 4. 后续修复：节点级统计缺失

- [x] f1: Fix Group child nodes QPS=0 — 从报告 metadata duration 计算 avgQPS 填充 TSQPS
- [x] f2: Fix Timer nodes not recording stats — 重构 executeTimer 全路径 RecordLatency + 实际耗时
- [x] f3: Fix all non-HTTP node types missing n.nodeStats — Delay/Condition/IfElse/Group/Timer 全部补写 nodeStats
