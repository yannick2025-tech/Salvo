## Context

导出 HTML 报告时，Go 后端通过 `json.Marshal(ctx)` 将 `EnhancedReportContext` 序列化为 JSON 字符串，赋值给 `ctx.JSONData`（`string` 类型），然后在 HTML 模板中通过 `{{.JSONData}}` 嵌入到 `<script>` 标签中。

Go 的 `html/template` 在渲染 `string` 类型时，会自动将其作为 JavaScript 字符串字面量处理——加上双引号并转义内部字符。导致浏览器端 `reportData` 变量是一个字符串而非对象，所有图表函数无法读取数据。

## Goals / Non-Goals

**Goals:**
- 修复导出 HTML 报告中所有 ECharts 图表为空数据的问题
- 确保图表数据与在线报告页面显示一致
- 保持向后兼容，不影响现有功能

**Non-Goals:**
- 不修改图表样式或布局
- 不修改数据采集或存储逻辑
- 不修改在线报告页面的渲染逻辑

## Decisions

### 决策 1：使用 `template.JS` 类型替代 `string`

**方案对比：**

| 方案 | 描述 | 复杂度 | 风险 |
|------|------|--------|------|
| A: `template.JS` | 将 `JSONData` 字段类型从 `string` 改为 `template.JS` | 低（1 行改动） | 低 |
| B: `JSON.parse()` | 在 JS 中额外调用 `JSON.parse()` 解析字符串 | 中（需改模板） | 低 |
| C: 自定义模板函数 | 注册一个返回 `template.JS` 的模板函数 | 中（需改模板注册） | 低 |

**选择方案 A**，原因：
- 改动最小，仅需修改结构体字段类型
- `template.JS` 是 Go 标准库 `html/template` 提供的安全类型，标记内容为安全的 JavaScript 代码
- 渲染时不会加引号转义，JSON 对象直接作为 JS 对象字面量嵌入

### 决策 2：不修改模板中的 JavaScript 代码

模板中的 `const reportData = {{.JSONData}};` 保持不变。修改字段类型后，输出将从：
```javascript
// 修改前（string 类型）
const reportData = "{\"Metrics\":{...}}";  // JS 字符串
```
变为：
```javascript
// 修改后（template.JS 类型）
const reportData = {"Metrics":{...}};  // JS 对象
```

所有图表函数中的 `reportData.metrics`、`reportData.node_metrics`、`reportData.error_breakdown` 等访问将正常工作。

## Risks / Trade-offs

- **[低风险] XSS 安全**：`template.JS` 标记的内容不会被转义。但 JSON 数据由后端 `json.Marshal` 生成，不包含用户输入，风险可控。
- **[低风险] 模板渲染兼容性**：`html/template` 对 `template.JS` 类型的处理在 Go 1.x 中行为一致，无兼容性问题。