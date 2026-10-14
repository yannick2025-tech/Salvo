---
layer: L1
maturity: proven
last-verified: 2026-05-25
source: .trae/rules/project_rules.md
tags: [echarts, chart, tooltip, line-chart, smooth, step, grid, datazoom]
---

# ECharts 图表样式规范

> 所有 ECharts 曲线图必须遵循 Dashboard Node 节点详情 (`renderNodeDetailChart`) 的配置模式。

## 核心原则

- **全局一致性**：Dashboard、ReportDetailPage、未来新页面的曲线图样式必须完全统一
- **预计算变量**：`smooth`/`step` 必须使用预计算的布尔变量，禁止在 series 中直接写表达式
- **完全替换模式**：`setOption` 必须传第二个参数 `true`

## smooth / step 控制规范 ⭐ 关键

```javascript
// ✅ 正确写法（必须遵循）
function renderChart() {
  const isSmooth = chartType.value === 'smooth'  // ① 预计算布尔变量

  chart.setOption({
    series: [{
      name: 'P50',
      type: 'line',
      smooth: isSmooth,                    // ② 直接使用布尔值
      step: isSmooth ? false : 'middle',   // ③ 用 false 而非 undefined
    }]
  }, true)  // ④ 必须传 true
}
```

```javascript
// ❌ 错误写法（禁止）
series: [{
  smooth: chartType.value === 'smooth',        // 表达式：每次都重新计算
  step: chartType.value === 'step' ? 'middle' : undefined,  // undefined 行为不确定
}]
```

**为什么这很重要？**
- ECharts 内部对 `smooth` 属性的类型有优化处理
- `false` 明确禁用平滑，`undefined` 可能使用默认行为导致不一致
- 预计算变量确保所有 series 使用相同的布尔值引用

## Series 配置模板

### 单一 Y 轴图表 (Latency Trend、QPS Trend)

```javascript
const isSmooth = chartType.value === 'smooth'

series: [
  {
    name: 'P50',
    type: 'line',
    smooth: isSmooth,
    step: isSmooth ? false : 'middle',
    data: p50Data,
    lineStyle: { width: 2, color: latencyColors[0] },
    itemStyle: { color: latencyColors[0] },  // 必须包含
    symbol: 'none',                           // 必须为 'none'
  }
]
```

### 双 Y 轴图表 (Node Charts - QPS + Latency)

```javascript
const isSmooth = nodeChartType.value === 'smooth'

series: [
  // QPS (左轴)
  {
    name: 'QPS',
    type: 'line',
    smooth: isSmooth,
    step: isSmooth ? false : 'middle',
    data: qpsData,
    yAxisIndex: 0,
    lineStyle: { width: 2, color: colors[0] },
    itemStyle: { color: colors[0] },
    symbol: 'none',
    areaStyle: {  // 仅 QPS 有面积填充
      color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
        { offset: 0, color: `rgba(${isDark ? '88,166,255' : '14,165,233'}, 0.12)` },
        { offset: 1, color: `rgba(${isDark ? '88,166,255' : '14,165,233'}, 0.01)` }
      ])
    }
  },
  // P50/P90/P95/P99 (右轴)
  {
    name: 'P50',
    type: 'line',
    smooth: isSmooth,
    step: isSmooth ? false : 'middle',
    data: p50Data,
    yAxisIndex: 1,
    lineStyle: { width: 2, color: latencyColors[0] },
    itemStyle: { color: latencyColors[0] },
    symbol: 'none',
  }
]
```

## 轴标签精度

- 所有百分比轴（错误率、成功率）：`formatter: (v) => v.toFixed(3) + '%'`
- 所有 QPS 轴：`formatter: (v) => v.toFixed(3)`
- 所有延迟轴(ms)：`formatter: (v) => v.toFixed(3)`
- 时间轴 interval：`Math.floor(dataLength / 8)` 避免标签重叠

## Tooltip 精度

- 错误率/成功率：`.toFixed(3)` + `%`
- QPS：`.toFixed(3)`
- 延迟：`.toFixed(3)` + `ms`
- **必须使用 `.toFixed(3)` 统一精度**

## 通用配置规范

### Grid 布局
```javascript
// 标准（与 Dashboard 一致）
grid: { top: 30, right: 20, bottom: 50, left: 50 }

// Node Charts（紧凑型）
grid: { top: 28, right: hasQPS ? 48 : 12, bottom: 36, left: 48 }
```

### DataZoom 滑块
```javascript
dataZoom: [{
  type: 'slider',
  height: 18,
  bottom: 4,
  borderColor: 'transparent',
  backgroundColor: theme.lineColor,
  fillerColor: `rgba(${theme.primaryColor}, 0.15)`,
  handleStyle: { color: theme.colors.primary },
  textStyle: { color: theme.textColor, fontSize: 10 },
  brushSelect: true
}]
```

### X 轴配置
```javascript
xAxis: {
  type: 'category',
  data: timeLabels,
  axisLine: { lineStyle: { color: theme.lineColor } },
  axisLabel: { color: theme.textColor, fontSize: 10 }
}
```

### Y 轴配置
```javascript
yAxis: {
  type: 'value',
  axisLine: { show: false },
  splitLine: { lineStyle: { color: theme.lineColor, type: 'dashed' } },
  axisLabel: { color: theme.textColor, fontSize: 10, formatter: '{value}ms' }
}

// 双 Y 轴示例
yAxis: [
  {   // 左轴 (QPS)
    type: 'value',
    position: 'left',
    axisLine: { show: false },
    splitLine: { lineStyle: { color: theme.lineColor, type: 'dashed' } },
    axisLabel: { color: theme.textColor, fontSize: 10 }
  },
  {   // 右轴 (Latency)
    type: 'value',
    position: 'right',
    axisLine: { show: false },
    splitLine: { show: false },
    axisLabel: { color: theme.textColor, fontSize: 10, formatter: '{value}ms' }
  }
]
```

### Tooltip 样式
```javascript
tooltip: {
  trigger: 'axis',
  confine: true,
  backgroundColor: isDark ? 'rgba(30,41,59,0.95)' : 'rgba(255,255,255,0.96)',
  borderColor: isDark ? 'rgba(71,85,105,0.3)' : 'rgba(148,163,184,0.2)',
  borderWidth: 1,
  borderRadius: 12,
  padding: [12, 16],
  textStyle: { fontSize: 11, color: labelColor },
  extraCssText: `
    border-radius: 12px;
    box-shadow: 0 1px 3px rgba(0,0,0,0.04), 0 8px 24px rgba(0,0,0,0.08);
    backdrop-filter: blur(8px);
  `,
  formatter: (params) => {
    let h = `<div style="font-size:11.5px;font-weight:600;margin-bottom:6px">${timeLabel}</div>`
    params.forEach(item => {
      h += `${item.marker} ${item.seriesName}: <strong>${value}</strong>${unit}<br/>`
    })
    return h
  }
}
```

### Legend 图例
```javascript
legend: {
  data: ['P50', 'P90', 'P95', 'P99'],
  textStyle: { color: theme.textColor },
  top: 0
  // 不要设置 itemWidth/itemHeight/itemGap（使用默认值）
}
```

## 🚫 禁止使用的属性

| 属性 | 禁止值 | 正确值 | 原因 |
|------|--------|--------|------|
| `sampling` | `'lttb'` | 不设置（删除） | Dashboard 未使用，会导致渲染差异 |
| `smooth` | 表达式 `type === 'smooth'` | 预计算布尔变量 `isSmooth` | ECharts 内部优化不同 |
| `step` | `undefined` | `false` | `undefined` 行为不确定 |
| `setOption` 第二参数 | 不传 | `true` | 必须完全替换配置 |

## ✅ 配置检查清单

创建或修改曲线图时，必须检查：

- [ ] 是否使用了 `const isSmooth = ...` 预计算变量？
- [ ] 所有 series 的 `smooth` 和 `step` 是否都使用 `isSmooth`？
- [ ] `step` 的假值是否为 `false` 而非 `undefined`？
- [ ] 是否包含了 `itemStyle` 属性？
- [ ] `symbol` 是否设置为 `'none'`？
- [ ] `setOption` 是否传了第二个参数 `true`？
- [ ] 是否移除了 `sampling` 属性？
- [ ] `backgroundColor` Node Charts 是否为 `'transparent'`？

## 💡 踩坑经验

### 问题 1：平滑度不一致
**现象**：测试报告曲线比 Dashboard 更"锯齿"
**原因**：使用了 `sampling: 'lttb'` + `smooth` 使用表达式
**解决**：完全对齐 Dashboard 的 `renderNodeDetailChart` 写法

### 问题 2：切换按钮失效
**现象**：点击"平滑/阶梯"按钮无反应
**原因**：将 `smooth` 写死为 `true`
**解决**：始终使用 `const isSmooth = ...` + 动态绑定

### 问题 3：阶梯模式残留
**现象**：切换到"平滑"后仍有轻微台阶感
**原因**：`step` 使用 `undefined` 而非 `false`
**解决**：`step: isSmooth ? false : 'middle'`
