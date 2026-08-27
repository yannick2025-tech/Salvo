## Context

当前测试报告（ReportDetailPage.vue）和导出 HTML 报告（report_generator_enhanced.go）的失败节点详情已移至报告末尾，并带有一个单搜索框，使用 OR 逻辑跨 node_name / node_id / error_code / error_message 四个字段模糊匹配。

实际排查场景中，用户往往需要精确定位"某类节点 + 特定错误码"的失败记录，例如：
- "节点名包含'订单'且错误码是 503"
- "NodeID 含 'node_5' 且错误信息含 'timeout'"

单框 OR 逻辑无法满足此类联合查询需求。失败节点数据模型已在前序变更中扩展了 Protocol/ErrorCode/Attributes 字段，为多字段独立查询提供了数据基础。

## Goals / Non-Goals

**Goals:**
- 提供节点名、NodeID、错误码、错误信息、协议五个独立查询字段
- 字段间使用 AND 逻辑，支持精确定位
- 前端 SPA 和导出 HTML 报告同步实现
- 布局紧凑，不显著增加报告垂直空间
- 每个字段独立清空，操作直观

**Non-Goals:**
- 不实现服务端搜索/分页（失败节点数据量通常 < 1000 条，前端过滤足够）
- 不实现保存查询条件功能
- 不实现正则/通配符匹配（简单 includes 即满足需求）
- 不实现跨报告搜索
- 不修改 FailedNodeDetail 数据模型（前序变更已完成扩展）

## Decisions

### D1: 五个独立查询字段，AND 逻辑

**选择**: 拆分为 5 个独立输入框 + 协议下拉框
- 节点名（text）、NodeID（text）、错误码（text）、错误信息（text）、协议（select）

**备选方案**:
- A. 单框多关键字语法（`name:订单 code:503`）— 紧凑但需学习语法，可发现性差
- B. 混合方案（常用字段独立 + 通用框）— 实现复杂，用户需判断用哪个框

**理由**: 独立查询框最直观，用户无需学习语法即可使用。五个字段覆盖了主要查询维度。AND 逻辑符合"逐步缩小范围"的排查思路。

### D2: 空字段不参与过滤

空字段视为"无约束"，不参与 AND 条件。只有用户主动填写的字段才作为过滤条件。这避免了"必须填满所有字段才能查询"的误区。

### D3: 布局采用网格横排

查询区使用 CSS Grid 一行排列（5 列），在小屏下自动换行（minmax + auto-fit）。每个输入框带 placeholder 提示字段含义，无独立 label 以节省空间。

### D4: 前端 computed + 导出 HTML JS 两套实现

- **前端 SPA**: 用 Vue `computed` 实现，5 个 `ref` + 1 个 `filteredFailedNodes` computed，响应式自动过滤
- **导出 HTML**: 用原生 JS，5 个 `<input>` + 1 个 `<select>`，`oninput`/`onchange` 触发 `filterFailedNodes()` 函数，通过 `data-*` 属性读取字段值

**理由**: 两套环境约束不同（Vue 响应式 vs 静态 HTML + JS），各自用最自然的方式实现。过滤逻辑保持一致（AND + includes）。

### D5: 错误码用 text 而非 number 输入

错误码字段用 `type="text"` 而非 `type="number"`，因为错误码可能是非数字（如 "timeout"、"connection_refused"）。模糊匹配（includes）比精确匹配更灵活。

## Risks / Trade-offs

- **风险**: 五个输入框在小屏上可能拥挤 → **缓解**: CSS Grid `auto-fit + minmax(140px, 1fr)` 自动换行
- **风险**: 用户期望 OR 逻辑但实际是 AND → **缓解**: 在查询区顶部显示提示文案"多个字段同时匹配（AND）"
- **权衡**: 独立框比单框多占垂直空间（约 40-60px）→ 可接受，失败节点区在报告末尾，空间充足
- **权衡**: 前端和导出 HTML 各维护一套过滤逻辑 → 可接受，逻辑简单且一致，后续若需调整可同步修改
