## Context

当前系统中存在三个主要视图使用 ECharts 折线图展示数据：Dashboard 页面、测试报告详情页（ReportDetailPage）、以及导出的 HTML 离线报告。这些页面中的图表都提供了平滑曲线（smooth）和阶梯线（step）两种展示模式，但实现上存在两个核心问题：

**现状问题：**
1. **UI 不一致**：Dashboard 中只有错误率趋势图和节点详情图有切换按钮，QPS 趋势图和延迟趋势图缺少该功能；导出报告中部分图表定义了状态变量但未渲染切换按钮
2. **状态耦合**：所有图表共享单一的状态变量（如 `nodeChartType`），导致点击任意一个节点的切换按钮会同时改变所有节点的显示模式，用户无法针对单个图表进行精细化控制

**技术栈约束：**
- 前端框架：Vue 3 + Composition API + TypeScript
- 图表库：ECharts 5.x
- 导出报告：Go 模板引擎生成静态 HTML + 内嵌 JavaScript
- 状态管理：组件内 ref/reactive（无全局状态库）

## Goals / Non-Goals

**Goals:**
- 所有折线图统一具备平滑/阶梯切换能力，视觉一致性达到 100%
- 每个图表维护独立的 chartType 状态，互不干扰
- 节点详情图的切换状态按节点 ID 隔离，支持每个节点独立控制
- 导出的 HTML 报告与在线页面行为完全一致
- 遵循项目现有的 ECharts 配置规范（project_rules.md 中的统一样式标准）

**Non-Goals:**
- 不引入新的状态管理库（如 Pinia）或复杂架构
- 不修改图表的核心数据逻辑或后端 API
- 不改变 ECharts 的其他配置项（只涉及 smooth/step 属性）
- 不添加新的图表类型或数据可视化功能

## Decisions

### 决策 1：使用 Reactive Map 管理独立状态

**选择方案：** 采用 `reactive(new Map())` 或 `ref<Record<string, 'smooth' | 'step'>>()` 存储每个图表的独立状态

**理由：**
- Map/Record 结构天然支持键值对，适合"图表ID → 状态"的映射关系
- Vue 3 的 reactive 系统能深度追踪 Map 内部变化，触发响应式更新
- 相比创建多个独立 ref 变量（如 `errorRateChartType`, `qpsChartType`, `latTrendChartType`...），Map 方案更简洁且易于扩展

**替代方案考虑：**
- ❌ 多个独立 ref：代码冗余高，新增图表时需手动添加变量
- ❌ Composable 函数封装：过度抽象，当前场景复杂度不够
- ❌ 全局状态库：引入不必要的依赖

### 决策 2：图表 ID 命名规范

**命名规则：**
```
Dashboard:
  - "errorRate"      → 错误率趋势图
  - "qpsTrend"       → QPS 趋势图（新增）
  - "latTrend"       → 延迟趋势图（新增）
  - "node-{nodeId}"  → 节点详情图（动态 ID）

ReportDetailPage:
  - "errorRate"      → 错误率趋势图
  - "qpsTrend"       → QPS 趋势图
  - "latTrend"       → 延迟趋势图
  - "node-{idx}"     → 节点详情图（索引 ID）

Exported HTML Report:
  - "errorRate"      → 错误率趋势图
  - "qps"            → QPS 趋势图
  - "latTrend"       → 延迟趋势图
  - "node-{idx}"     → 节点详情图
```

**理由：** 语义化命名便于调试和维护，节点图使用动态 ID 支持独立控制

### 决策 3：导出报告使用 JavaScript 对象模拟响应式

**选择方案：** 在导出的 HTML 中使用普通 JavaScript 对象 `const chartTypes = {}` 存储状态，通过 DOM 操作更新按钮样式和重新渲染图表

**理由：**
- 导出报告是纯静态 HTML，无 Vue 运行时
- JavaScript 对象足够轻量，无需引入额外库
- 通过 `onclick` 事件处理器直接操作，简单直接

### 决策 4：统一的切换函数签名

**函数设计：**
```typescript
// Dashboard / ReportDetailPage
function switchChartType(chartId: string, type: 'smooth' | 'step') {
  chartTypes.value[chartId] = type
  // 重新渲染对应图表
}

// Exported HTML Report (JavaScript)
function switchChartType(chartId, type) {
  chartTypes[chartId] = type;
  // 重新渲染对应图表
}
```

**理由：** 统一的 API 设计降低认知负担，便于维护

## Risks / Trade-offs

**风险 1：性能影响 - 切换时重绘单个图表 vs 全量重绘**
→ **缓解措施：** 只重绘目标图表（通过 chartId 定位），不触发其他图表更新。ECharts 的 `setOption(option, true)` 已优化为增量更新

**风险 2：节点图数量较多时的内存占用**
→ **缓解措施：** Map 只存储字符串键值对（约 50 bytes/条），即使 20 个节点也仅占 1KB，可忽略不计

**风险 3：导出报告中 JavaScript 代码量增加**
→ **缓解措施：** 复用通用的 `switchChartType` 和 `renderChart` 函数，避免重复代码。最终增加 < 2KB

**权衡：独立性 vs 简洁性**
- 当前方案优先保证独立性（每个图表完全隔离），牺牲了一定的代码简洁性
- 如果未来需要"一键切换全部图表"，可轻松添加全局切换按钮调用所有 chartId

## Migration Plan

**实施步骤：**
1. 先改造 DashboardPage.vue（影响面最小，便于验证）
2. 再改造 ReportDetailPage.vue（逻辑类似）
3. 最后更新导出报告模板（report_generator_enhanced.go）
4. 每步完成后进行手动测试，确保：
   - 切换按钮可见性
   - 状态隔离性（点击 A 图表不影响 B 图表）
   - 视觉一致性（遵循 project_rules.md 样式规范）

**回滚策略：**
- Git 分支开发，可随时 revert
- 无数据库变更，无 API 变更，零风险回滚
