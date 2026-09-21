---
layer: L2
maturity: verified
last-verified: 2026-09-21
source: docs/biz-migration/salvo-yaml-guide.md（附：ID 体系与日志检索）
tags: [id, run_id, trace, log-search, naming]
---

# ID 体系速查表

> 全站 ID 命名与语义统一规范（2026-09-21 起）。**排查后端日志用"运行ID"，不要用"数据KEY"。**

## 三类核心 ID

| 名称 | 含义 | 页面展示 | 日志检索 |
|------|------|---------|---------|
| 数据KEY | 各数据库表的主键（scenes/runs/traces/users 等表的 `id`） | 主键列统一命名"数据KEY"，hover 显示"数据库主键：值" | ✗ 日志中不存在此值 |
| 运行ID（run_id） | 一次场景运行的业务标识（雪花ID），贯穿 run / trace / 后端日志 | 业务键列统一命名"运行ID"，hover 提示"检索后端日志用此 ID"，点击可复制 | ✓ 日志键 `run_id` |
| 链路ID（chain_id） | 一次 DAG 链路执行的业务标识 | trace span 的"链路ID" | ✓ 日志键 `chain_id` |

另有 `scene_id` / `node_id` 上下文日志键，用于定位场景级与节点级日志。

## 关键实现锚点

- 日志注入：`logger.ContextWithRunID`（internal/logger/zap.go）；runner 运行日志统一携带 `run_id`
- 智能匹配：`TraceFilter.TraceID` → `(t.id = ? OR t.run_id = ?)`（internal/trace/store/sqlite.go buildTraceWhere）
- 按运行ID取链路：`POST /traces/get-by-run`（参数 `run_id`）
- 前端关联跳转：场景列表"最后一次运行记录"→ 运行控制页；运行历史/报告列表场景ID → 场景列表；运行历史"链路" → `/traces?trace_id=<运行ID>` 自动预填过滤条件

## 禁止事项

- 禁止把 runID 写入日志键 `trace_id`（历史遗留已于 2026-09-21 修复）
- 新增页面的主键列必须命名"数据KEY"，运行业务键列必须命名"运行ID"
- 链路过滤/查询入口必须同时接受数据KEY与运行ID（OR 匹配）
