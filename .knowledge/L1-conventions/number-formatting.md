---
layer: L1
maturity: proven
last-verified: 2026-05-25
source: .trae/rules/project_rules.md
tags: [formatting, percentage, qps, latency, tofixed, number]
---

# 数值格式化规范

> 所有数值显示必须遵循统一的精度和格式规则。

## 百分比 (错误率 / 成功率 / 占比)

- **必须**使用 `toFixed(3)` 显示百分比，保留 3 位小数
- 示例：`37.095%` ✅  vs `37.09%` ❌ vs `37.09483793517407%` ❌
- 适用场景：ECharts Y 轴 axisLabel、tooltip formatter、模板插值 `{{ }}`

```javascript
// ECharts axisLabel
formatter: (v: number) => v.toFixed(3) + '%'

// 模板中
{{ ((success / total) * 100).toFixed(3) }}%
```

## 数字 (QPS / Throughput / Count)

- 大数字用千分位格式化：`Number(n).toLocaleString()` → `1,234,567`
- 小数字（QPS）保留 3 位小数：`.toFixed(3)`
- 整数值（worker count, duration）**禁止**显示浮点，用 `Math.round()` 或 `parseInt()`

## 延迟 (Latency)

- 单位统一：ms 或 s，根据量级自动切换
- < 1s 显示为 ms（3位小数）：`456.789ms`
- ≥ 1s 显示为 s（3位小数）：`1.235s`

## 时间格式

### Datetime Display
- 完整时间：`YYYY-MM-DD HH:MM:SS` → `2026/05/11 15:28:43`
- 简短时间：`HH:MM:SS` → `15:28:43`
- 运行中的结束时间显示 `--`（不显示 "Now"）

### Duration (持续时间)
- 格式：`XX小时XX分XX秒` 或 `XX分XX秒`，根据时长自适应，**所有数值必须补零到 2 位**
- 示例：`3小时6分5秒` → `03小时06分05秒` ✅  vs `3小时6分5秒` ❌
- 短时长（< 1 小时）：`06分05秒`
- 长时长（≥ 1 小时）：`03小时06分05秒`
- 运行中的场景实时计算：`(Date.now() - startTime) / 1000`

```javascript
// 补零格式化函数
function formatDuration(totalSeconds: number): string {
  const h = Math.floor(totalSeconds / 3600)
  const m = Math.floor((totalSeconds % 3600) / 60)
  const s = totalSeconds % 60

  if (h > 0) {
    return `${String(h).padStart(2, '0')}小时${String(m).padStart(2, '0')}分${String(s).padStart(2, '0')}秒`
  }
  return `${String(m).padStart(2, '0')}分${String(s).padStart(2, '0')}秒`
}

// 示例：
// formatDuration(11005)  → "03小时06分05秒"
// formatDuration(365)    → "06分05秒"
```
