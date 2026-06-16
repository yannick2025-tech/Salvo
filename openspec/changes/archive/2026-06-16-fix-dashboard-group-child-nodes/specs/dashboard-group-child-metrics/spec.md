## ADDED Requirements

### Requirement: Dashboard 节点指标包含 Group 子节点

`Handler.aggregateNodeMetricsWithSceneID()` 函数返回的节点指标列表 SHALL 包含场景中所有已执行节点的统计数据，包括 Group 节点下配置的子节点。

当 tracer span 数据中缺少某些节点的记录时（典型情况为 Group 子节点），系统 SHALL 从该场景最近一次已完成运行的报告详情（report detail）中补充这些缺失节点的指标数据。

#### Scenario: 场景含 Group 节点时 Dashboard 显示全部子节点
- **WHEN** 用户访问某场景的 Dashboard 页面，且该场景包含一个配置了 3 个子节点的 Group 节点
- **THEN** 返回的 `node_metrics` 列表 SHALL 包含这 3 个子节点的统计指标（总请求数、成功/失败数、延迟分位值等）
- **AND** 子节点的总节点数与 HTML 测试报告中显示的节点数一致

#### Scenario: 场景无已完成运行时回退到纯 tracer 数据
- **WHEN** 目标场景没有任何已完成（completed/cancelled）的运行记录
- **THEN** 系统 SHALL 仅返回 tracer 中已有的节点数据（与修改前行为一致）
- **AND** 不因 report 查询失败而报错或返回空结果

#### Scenario: 补充子节点的数据格式与现有节点一致
- **WHEN** 从 report detail 补充一个 Group 子节点到 node_metrics
- **THEN** 该子节点的 NodeMetricDTO 结构 SHALL 与 tracer 来源的节点完全一致
- **AND** 包含字段：node_id、name、type、total_reqs、success_reqs、avg_latency、p50_latency、p95_latency、p99_latency、sort_order
- **AND** 时间序列字段（timestamps、ts_p50 等）可为空数组

### Requirement: Report Fallback 查询范围限制

从 report store 查询补充数据时，系统 SHALL 仅查询目标 scene_id 最近一次有报告详情的运行记录，避免全表扫描。

#### Scenario: 多次运行时仅取最近一次
- **WHEN** 目标场景有 5 次历史运行记录
- **THEN** 仅查询最新的一条（按 startedAt 倒序）用于补充缺失节点
