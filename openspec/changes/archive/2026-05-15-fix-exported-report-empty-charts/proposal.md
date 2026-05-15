## Why

导出的 HTML 测试报告中所有 ECharts 图表（QPS 趋势、延迟趋势、错误率、节点图表等）显示为空数据，但指标卡片数据正常。原因是 Go 后端将 JSON 数据以 `string` 类型嵌入 HTML `<script>` 标签，导致浏览器端将数据解析为 JavaScript 字符串而非对象，图表函数无法读取数据。

## What Changes

- 修改 `EnhancedReportContext.JSONData` 字段类型，从 `string` 改为 `template.JS`，使 JSON 数据直接嵌入为 JavaScript 对象字面量
- 所有图表函数（overview、error rate、latency、QPS trend、latency trend、node charts、error breakdown）将能正确读取数据并渲染

## Capabilities

### New Capabilities
- `html-report-export`: 导出与在线页面一致的 HTML 测试报告，包含完整的指标卡片和 ECharts 图表

### Modified Capabilities
- 无

## Impact

- **internal/api/report_generator_enhanced.go**: 修改 `EnhancedReportContext` 结构体的 `JSONData` 字段类型
- 仅影响导出 HTML 报告的生成逻辑，不影响其他功能