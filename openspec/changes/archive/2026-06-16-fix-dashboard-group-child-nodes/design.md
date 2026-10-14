## Context

### 当前状态

Salvo 的 Dashboard 页面展示"节点指标"卡片，数据来自 `Handler.aggregateNodeMetricsWithSceneID()` 函数（`handler.go:2308`）。该函数遍历 `h.tracer.List()` 返回的链路追踪 span，按 `sp.NodeID` 聚合每个节点的请求数、延迟分布等指标。

Group 节点的子节点（如"订单处理子步骤1/2/3"）在 DAG 构建阶段被**排除**出 DAG 拓扑（`buildDAG` 中的 `groupChildIDs` 预扫描逻辑），改为由 Group 节点的 `executeGroup()` 方法直接调用 `child.Execute(ctx, input)` 执行。这意味着：

```
DAG Executor 执行路径（有 tracer span）:
  → 注册测试用户 ✅ 有 span
  → 用户登录获取Token ✅ 有 span
  → ...
  → 订单处理流程(Group) ✅ 有 span（Group 本身）
      └─ 子步骤1 ❌ 无 span（直接调用，绕过 executor）
      └─ 子步骤2 ❌ 无 span
      └─ 子步骤3 ❌ 无 span
```

### 数据源对比

| 视图 | 数据源 | 覆盖范围 | 结果 |
|------|--------|---------|------|
| HTML 报告 | `r.nodeStats` map | 全部节点（含子节点） | 20 个 ✅ |
| Dashboard | `h.tracer.List()` spans | 仅 DAG 层级节点 | 15 个 ❌ |
| 时序存储 | `TimeSeriesCollector.takeSnapshot` | `r.nodeStats` 全量快照 | 完整 ✅ |

## Goals / Non-Goals

**Goals:**
- Dashboard 节点指标正确显示 Group 子节点的统计数据（请求次数、延迟分位值、QPS 等）
- 保持现有 HTML 报告和时序存储行为不变
- 改动最小化，不侵入核心执行路径

**Non-Goals:**
- 不修改 `executeGroup()` 或 DAG 执行器的 span 记录机制
- 不改变前端组件结构或 API 响应格式
- 不引入新的外部依赖

## Decisions

### 决策 1：采用 Report Fallback 方案补充子节点数据

**选择**：在 `aggregateNodeMetricsWithSceneID()` 末尾，从最近一次完成的运行报告中读取 `node_metrics`，将 tracer 中缺失的子节点补充进结果集。

**备选方案对比**：

| 方案 | 描述 | 优点 | 缺点 |
|------|------|------|------|
| **A. Tracer 增强** | 在 `executeGroup()` 中为每次子节点执行手动创建 tracer span | 数据源头一致 | 侵入执行路径；需注入 tracer 依赖到 sceneNode；span 语义与 DAG span 不同 |
| **B. Report Fallback（推荐）** | 从已有 report detail 补充缺失节点 | 零侵入；report 已有完整数据 | 数据来源混合（tracer + report）；需一次额外 DB 查询 |
| **C. 时序存储查询** | 从 ts_store 按 run_id 查询全部节点时序 | 数据精确 | 时序是采样聚合非原始统计；实现复杂度高 |

**选择 B 的理由**：
1. report detail 的 `node_metrics` 字段已包含所有节点的完整统计（含子节点），数据准确
2. 仅在 `aggregateNodeMetricsWithSceneID` 一个函数内改动，不影响执行路径
3. report store 已有 `GetByRunID` 接口，无需新开发
4. 性能影响可控：仅对目标 scene 最近一次运行做一次 DB 查询

### 决策 2：补充数据的合并策略

**策略**：以 tracer 数据为主，report 数据为补充。具体逻辑：

```
1. 正常从 tracer 聚合得到 nodeMetrics（现有逻辑不变）
2. 收集 tracer 中已有的 nodeID 集合：tracerNodeIDs
3. 查询目标 scene 最近一次完成运行的 report detail
4. 遍历 report 的 node_metrics，对每个不在 tracerNodeIDs 中的节点：
   - 构建 NodeMetricDTO（从 report 的 summary 字段映射）
   - 时间序列设为空数组（Dashboard 对无时序的节点仍可显示汇总指标）
5. 将补充节点追加到结果集
```

## Risks / Trade-offs

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| report 与 tracer 数据时间窗口不完全一致 | 子节点可能显示与主节点不同时间范围的统计 | 可接受：Dashboard 本身就是近似概览；子节点的时间范围标注来自 report 自身的 startedAt/finishedAt |
| 场景无已完成运行时 report 为空 | 无法补充子节点 | 回退到纯 tracer 数据（与当前行为一致，不退化） |
| 额外一次 DB 查询 | Dashboard 加载增加 ~5-10ms | 仅查询一条记录（limit=1），索引覆盖 |

## Open Questions

- （无）
