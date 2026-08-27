## Why

测试报告和导出 HTML 报告中的"失败节点详情"当前只有一个搜索框，使用 OR 逻辑跨字段模糊匹配。用户无法执行联合查询，例如"节点名包含'订单' **且** 错误码是 503"——单框 OR 逻辑会将包含任一关键字的记录都返回，无法精确定位。当失败记录数量较多时，用户难以快速过滤出特定场景的失败请求，排查效率低。

## What Changes

- 将失败节点详情的单搜索框拆分为 5 个独立查询字段：
  1. **节点名**（node_name）— 模糊匹配
  2. **NodeID**（node_id）— 模糊匹配
  3. **错误码**（error_code）— 模糊匹配，支持 504、timeout 等
  4. **错误信息**（error_message）— 模糊匹配
  5. **协议**（protocol）— 下拉选择（全部/http/db/mq/...）
- 字段间使用 **AND 逻辑**：只有同时满足所有已填字段条件的记录才显示
- 每个字段独立清空，整体布局紧凑（一行排列）
- 空字段不参与过滤（相当于该字段无约束）
- 同步更新前端 SPA（ReportDetailPage.vue）和导出 HTML 报告模板（report_generator_enhanced.go）
- 导出 HTML 报告的 JS 过滤逻辑同步支持多字段 AND 查询

## Capabilities

### New Capabilities
- `failed-nodes-multi-field-search`: 失败节点详情多字段独立查询能力，支持节点名、NodeID、错误码、错误信息、协议五个字段的独立 AND 逻辑联合过滤

### Modified Capabilities
（无现有 capability 的规格变更）

## Impact

- **前端**: `web/app/src/views/reports/ReportDetailPage.vue` — 替换单搜索框为多字段查询区，新增 5 个 ref + 更新 filteredFailedNodes computed
- **后端 HTML 模板**: `internal/api/report_generator_enhanced.go` — 替换搜索栏 HTML，重写 filterFailedNodes() JS 函数支持多字段 AND
- **CSS**: 两处新增多字段查询区样式（紧凑横排布局）
- **知识库**: 更新 `.knowledge/L1-conventions/` 相关约定（如有 UI 搜索模式约定）
- **文档**: 无外部 API 变更，纯前端/HTML 模板调整
