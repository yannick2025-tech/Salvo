# Lessons Learned

> 排查过程中踩过的坑和学到的经验，避免后续重复走弯路。

---

## Lesson 1: 前端首屏性能优化 — Runtime 图表加载卡顿 (2026-05-23)

### 现象
Dashboard 页面首次加载时，从顶部滚动到 runtime 图表区域（页面底部），需要等待 **3~10 秒** 图表才显示出来，用户体验不丝滑。而同页面的 Node 详情图（QPS/Latency/Error）却能即时显示。

### 排查过程（5 轮迭代）

#### 第 1 轮：怀疑是主题切换问题
- **假设**：切换深浅主题时图表重建慢
- **排查**：发现 `MutationObserver` dispose 了 5 个 runtime 小图但只重渲染了 3 个 Node 详情图
- **修复**：补全 5 个 runtime 图表的重渲染调用
- **结果**：❌ 问题依旧，只是修复了主题切换的子问题

#### 第 2 轮：怀疑是 IntersectionObserver 懒加载延迟
- **假设**：`rootMargin: 200px` 不够大，触发太晚
- **优化**：增大到 `800px` + 数据到达后 `nextTick()` 预热渲染
- **结果**：❌ 问题依旧，但方向对了

#### 第 3 轮：怀疑是数据量门槛过高
- **假设**：`sysMetricsHistory.length < 2` 导致首次只有 1 条数据时不渲染
- **日志证据**：
  ```
  sysMetricsHistory = [1条] → length < 2 → return ❌
  ```
- **修复**：改为 `length === 0` 才跳过
- **结果**：❌ 还是卡顿，但排除了这个因素

#### 第 4 轮：怀疑是分帧渲染延迟
- **假设**：4 层 `requestAnimationFrame` 嵌套导致 ~66ms+ 延迟
- **对比**：Node 详情图是同步连续调用，runtime 是 rAF 分帧
- **修复**：去掉分帧，改为同步连续调用
- **结果**：❌ 还是卡顿，说明瓶颈不在渲染本身

#### 第 5 轮：添加详细时间戳日志定位真因 ✅
在所有关键节点添加 `[SYS-DEBUG]` 日志：

```
[SYS-DEBUG] onMounted | t=xxxms
[SYS-DEBUG] fetchOverview start | t=xxxms
[SYS-DEBUG] fetchOverview done | hasRunning=... | sysTimeSeriesLen=0/300
[SYS-DEBUG] prewarmSysCharts | start | DOM not ready / no data / rendering...
[SYS-DEBUG] prewarmSysCharts | done in xx.xms
```

**关键发现（日志铁证）**：

```log
T=0s      onMounted → fetchOverview(第1次)
T=21ms    fetchOverview 返回: sysTimeSeriesLen=0 ❌ (无数据!)
T=5s      pollTimer → fetchOverview(第2次): sysTimeSeriesLen=300 ✅
T=5.1s    尝试渲染 → DOM not ready (ref=false) ❌
T=5.2s    100ms 重试 → DOM ready! 渲染成功 (49ms) 🚀
```

### 根因分析（3 层叠加）

| # | 问题层 | 影响 | 修复 |
|---|--------|------|------|
| **1** | **请求顺序错误** | `fetchOverview()` 在 `fetchSceneList()` 完成前就发出，`selectedSceneId` 为空 → 后端不加载数据 | `await fetchSceneList()` |
| **2** | **DOM 未就绪** | 数据到了但 `sysGoroutineChartRef.value=false`（Vue 还没挂载 DOM） | 100ms 自动重试机制 |
| **3** | **提前标记完成** | DOM 未就绪时 `sysChartsRendered=true`，后续不再重试 | 以 ECharts 实例创建成功为准 |

### 最终修复方案

```typescript
// 1️⃣ 请求顺序：等场景列表加载完再请求数据
onMounted(async () => {
  await fetchSceneList()    // ← 关键！确保 selectedSceneId 已设置
  fetchOverview()
})

// 2️⃣ 预热渲染：自动重试直到 DOM 就绪
let sysChartsRendered = false
function prewarmSysCharts(caller: string) {
  if (sysChartsRendered) return
  if (noData) return
  
  renderSysGoroutineChart()
  if (!sysGoroutineChart) {           // DOM 未就绪？
    setTimeout(() => prewarmSysCharts(`${caller}-retry`), 100)  // 100ms 后重试
    return
  }
  
  // 只有真正创建了 ECharts 实例才标记完成
  sysChartsRendered = true
  renderSysHeapChart()  // 同步连续调用其余 4 个
  renderSysCpuChart()
  renderSysTaskWaitChart()
  renderSysQueueChart()
}
```

### 效果对比

| 指标 | 优化前 | 优化后 |
|------|--------|--------|
| 数据到达→渲染完成 | ~10 秒 | **~116 毫秒** 🚀 |
| 首屏可见时间 | ~10 秒 | **< 2 秒** |
| 用户感知 | 卡顿明显 | 丝滑无感 |

### Lessons Learned

#### 1. 日志是终极武器 — 别猜，用数据说话
- **教训**：前 4 轮都是"猜测→验证→失败"，浪费大量时间
- **经验**：遇到性能问题时，第一时间加时间戳日志，让数据告诉你瓶颈在哪
- **模板**：
  ```javascript
  console.log(`[DEBUG-${功能名}] ${调用者} | t=${Date.now()}ms | 关键状态`)
  ```

#### 2. 异步依赖链必须显式等待
- **教训**：`fetchA(); fetchB()` 同时发起，B 可能依赖 A 的结果（如 sceneId）
- **经验**：有依赖关系的异步操作必须 `await`
- **反模式**：
  ```javascript
  onMounted(() => {
    fetchSceneList()   // 异步，不等待
    fetchOverview()     // ⚠️ 此时 selectedSceneId 还是 undefined!
  })
  ```
- **正模式**：
  ```javascript
  onMounted(async () => {
    await fetchSceneList()   // 确保 selectedSceneId 已设置
    fetchOverview()          // 现在有有效参数了
  })
  ```

#### 3. DOM 可用性检查不能提前拦截
- **教训**：`if (!ref.value) return` + 标记已完成 → 永远不再重试
- **经验**：DOM 未就绪时应该**延迟重试**，而不是放弃
- **原则**：以**副作用是否生效**（如 ECharts 实例是否创建）为准，而非前置条件

#### 4. 分帧渲染不是银弹
- **教训**：4 层 `requestAnimationFrame` 嵌套导致额外 66ms+ 延迟
- **经验**：少量图表（<10个）直接同步连续调用即可，浏览器能处理
- **何时用分帧**：大量 DOM 操作或复杂计算时才需要

#### 5. IntersectionObserver 懒加载与预热的冲突
- **教训**：IO 懒加载是为了节省资源，但如果数据已经到达，应该立即预热
- **经验**：双重策略——数据到达时尝试预热（可能失败），IO 触发时兜底保障
- **代码模式**：
  ```javascript
  // 数据驱动（快路径）
  fetchData().then(() => prewarmCharts())
  
  // IO 兜底（慢路径）
  new IntersectionObserver(() => prewarmCharts())
  ```

#### 6. 性能优化的正确顺序
- **错误顺序**（我们走过的弯路）：
  1. 怀疑渲染方式 → 2. 怀疑懒加载 → 3. 怀疑数据门槛 → 4. 怀疑分帧 → 5. 加日志
- **正确顺序**：
  1. **加日志定位瓶颈** → 2. 分析根因 → 3. 对症下药

---

## Lesson 2: 链路跟踪色块溢出边界问题 (2026-05)

### 现象
Trace 详情页面的某个**色块组件（Span/时间线）一直超出画面右边界**。尝试了多种 CSS 修复均无效：
- 设置 `overflow: hidden` → 色块被裁剪，但根因未解决
- 调整 `max-width: 100%` → 无效
- 检查父容器 `padding/margin` → 未发现异常

### 排查过程

#### 传统方法（❌ 低效）
1. **目视检查**：逐个查看 DOM 元素的 computed style → 无法定位
2. **DevTools 盒模型**：检查每个元素的 width/padding/border → 正常
3. **试错修改**：随机调整 CSS 属性 → 2+ 小时无果

#### 可视化调试方法（✅ 高效）

**第 1 步：全局边框扫描**

在浏览器控制台执行：

```javascript
/* 一键全量诊断 */
document.querySelectorAll('*').forEach(el => {
  el.style.outline = '1px solid rgba(255,0,0,0.2)'
})
```

**结果**：立即看到所有元素的边界盒模型，发现某个**中间层父容器宽度异常偏大**。

**第 2 步：祖先链追踪**

```javascript
// 高亮目标元素及其所有祖先
let el = document.querySelector('.target-span')
const colors = ['red', 'orange', 'yellow', 'green', 'blue']

while (el && el !== document.body) {
  const depth = Math.min(getAncestorDepth(el), colors.length - 1)
  el.style.outline = `2px solid ${colors[depth]}`
  el = el.parentElement
}
```

**颜色含义**：
- 🔴 **红色**：目标元素本身
- 🟠 **橙色**：直接父容器 ← **发现问题所在！**
- 🟡 **黄色**：祖父容器
- ...

**第 3 步：定位根因**

检查橙色边框的父容器样式：

```javascript
const parentEl = targetEl.parentElement
console.log('Parent:', {
  width: getComputedStyle(parentEl).width,
  maxWidth: getComputedStyle(parentEl).maxWidth,
  boxSizing: getComputedStyle(parentEl).boxSizing,
})
```

**发现**：该父容器的宽度被错误设置为 `width: calc(100% + XXpx)` 或类似值。

### 根因分析

| 层级 | 问题 | 影响 |
|------|------|------|
| **CSS 计算错误** | 父容器使用了错误的 `calc(100% + XXpx)` | 容器比预期宽 |
| **嵌套传递** | 子元素继承父容器的 100% 宽度 | 错误宽度逐层放大 |
| **难以察觉** | 没有明显的 overflow 报错 | DevTools 显示"正常" |

### 最终修复方案

```css
/* ❌ 错误写法 */
.parent-container {
  width: calc(100% + 20px);  /* 多余的 padding/border 导致溢出 */
}

/* ✅ 正确写法 */
.parent-container {
  width: 100%;
  box-sizing: border-box;  /* 让 padding/border 包含在 width 内 */
}

/* 或者使用更安全的方式 */
.parent-container {
  max-width: 100%;
  overflow-x: auto;  /* 允许横向滚动而非溢出 */
}
```

### 效果对比

| 方法 | 耗时 | 成功率 |
|------|------|--------|
| 传统目视+试错 | **2+ 小时** | ❌ 反复失败 |
| 全局边框扫描 | **5 分钟** | ✅ 一次定位 |

### Lessons Learned

#### 1. "让它可见"是排查布局问题的第一原则
- **教训**：花大量时间猜测哪个元素有问题
- **经验**：用 `outline: 1px solid red` 让所有元素边界可见，问题一目了然
- **为什么用 outline 而非 border**：outline 不占空间，不影响布局

#### 2. 祖先链着色法快速定位越界源头
- **教训**：只检查目标元素和直接父容器，忽略了更高层祖先
- **经验**：用不同颜色高亮整条祖先链，哪一层有异常一目了然
- **代码模板**：
  ```javascript
  let el = targetElement
  const colors = ['red', 'orange', 'yellow', 'green', 'blue']
  while (el !== document.body) {
    el.style.outline = `2px solid ${colors[depth++ % colors.length]}`
    el = el.parentElement
  }
  ```

#### 3. calc() 宽度计算是溢出的常见原因
- **教训**：`width: calc(100% + XXpx)` 容易导致溢出
- **经验**：优先使用 `box-sizing: border-box` 或 `max-width: 100%`
- **常见陷阱**：
  - 忘记 padding/border 会增加实际尺寸
  - 嵌套容器中 calc() 误差累积
  - 百分比基于不同的 containing block

#### 4. 自动化检测脚本提升效率
- **教训**：手动逐个检查元素太慢
- **经验**：使用自动化脚本批量检测所有溢出元素
- **推荐工具**：[ui-boundary-debugger](../.trae/skills/ui-boundary-debugger/SKILL.md) Skill

#### 5. 排查顺序：先全局后局部
- **错误顺序**：先看目标元素 → 再看父容器 → 再看祖父...
- **正确顺序**：
  1. **全局扫描**：`* { outline: 1px solid red }` — 快速锁定异常区域
  2. **祖先链追踪**：着色法定位具体层级
  3. **属性检查**：对比正常/异常元素的 computed style
  4. **修复验证**：改一处→验证→移除调试代码

---

## Lesson 3: (待记录)

> 下次遇到值得记录的问题时在此处添加。

