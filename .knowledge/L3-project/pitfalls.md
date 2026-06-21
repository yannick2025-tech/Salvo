---
layer: L3
maturity: verified
last-verified: 2026-05-25
source: docs/lessons-learned.md
tags: [pitfalls, performance, layout, echarts, async, dom]
---

# 已知陷阱 (Lessons Learned)

> 排查过程中踩过的坑和学到的经验，避免后续重复走弯路。

---

## Lesson 1: 前端首屏性能优化 — Runtime 图表加载卡顿 (2026-05-23)

### 现象
Dashboard 页面首次加载时，从顶部滚动到 runtime 图表区域（页面底部），需要等待 **3~10 秒** 图表才显示出来，用户体验不丝滑。

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

### Lessons Learned

#### 1. 日志是终极武器 — 别猜，用数据说话
- 遇到性能问题时，第一时间加时间戳日志，让数据告诉你瓶颈在哪
- 模板：
  ```javascript
  console.log(`[DEBUG-${功能名}] ${调用者} | t=${Date.now()}ms | 关键状态`)
  ```

#### 2. 异步依赖链必须显式等待
- 有依赖关系的异步操作必须 `await`
- 反模式：
  ```javascript
  onMounted(() => {
    fetchSceneList()   // 异步，不等待
    fetchOverview()     // ⚠️ 此时 selectedSceneId 还是 undefined!
  })
  ```

#### 3. DOM 可用性检查不能提前拦截
- DOM 未就绪时应该**延迟重试**，而不是放弃
- 以**副作用是否生效**（如 ECharts 实例是否创建）为准，而非前置条件

#### 4. 分帧渲染不是银弹
- 少量图表（<10个）直接同步连续调用即可，浏览器能处理

#### 5. IntersectionObserver 懒加载与预热的冲突
- 双重策略——数据到达时尝试预热（可能失败），IO 触发时兜底保障

#### 6. 性能优化的正确顺序
- **正确顺序**：加日志定位瓶颈 → 分析根因 → 对症下药
- **错误顺序**：猜测渲染方式 → 怀疑懒加载 → 怀疑数据门槛 → 怀疑分帧 → 加日志

---

## Lesson 2: 链路跟踪色块溢出边界问题 (2026-05)

### 现象
Trace 详情页面的某个**色块组件（Span/时间线）一直超出画面右边界**。尝试了多种 CSS 修复均无效。

### 排查方法对比

| 方法 | 耗时 | 成功率 |
|------|------|--------|
| 传统目视+试错 | **2+ 小时** | ❌ 反复失败 |
| 全局边框扫描 | **5 分钟** | ✅ 一次定位 |

### 可视化调试方法

**第 1 步：全局边框扫描**
```javascript
document.querySelectorAll('*').forEach(el => {
  el.style.outline = '1px solid rgba(255,0,0,0.2)'
})
```

**第 2 步：祖先链追踪**
```javascript
let el = targetElement
const colors = ['red', 'orange', 'yellow', 'green', 'blue']
while (el !== document.body) {
  el.style.outline = `2px solid ${colors[depth++ % colors.length]}`
  el = el.parentElement
}
```

**第 3 步：检查 computed style**
```javascript
getComputedStyle(el).width / maxWidth / boxSizing
```

### 根因分析

父容器的宽度被错误设置为 `width: calc(100% + XXpx)` 或类似值。

### 最终修复方案

```css
/* ✅ 正确写法 */
.parent-container {
  width: 100%;
  box-sizing: border-box;
}

/* 或者使用更安全的方式 */
.parent-container {
  max-width: 100%;
  overflow-x: auto;
}
```

### Lessons Learned

#### 1. "让它可见"是排查布局问题的第一原则
- 用 `outline: 1px solid red` 让所有元素边界可见（outline 不占空间）

#### 2. 祖先链着色法快速定位越界源头
- 只检查目标元素和直接父容器是不够的，需要用不同颜色高亮整条祖先链

#### 3. calc() 宽度计算是溢出的常见原因
- 优先使用 `box-sizing: border-box` 或 `max-width: 100%`
- 嵌套容器中 calc() 误差累积

#### 4. 自动化检测脚本提升效率
- 推荐工具：`ui-boundary-debugger` Skill

#### 5. 排查顺序：先全局后局部
- 全局扫描 → 祖先链追踪 → 属性检查 → 修复验证

---

## Lesson 3: Goroutine 吞没错误导致排查困难 — 后台 Runner (2026-06-19)

### 现象
场景运行失败后，前端 Dashboard 无法显示失败原因（展示为空白或未知错误）。后台日志仅显示 `"done"` 状态，没有 error 信息。排查时发现 run_record 数据库中缺少失败记录，无法确认是哪个环节出了问题。

### 根因分析（2 层叠加）

| # | 问题层 | 影响 | 修复 |
|---|--------|------|------|
| **1** | **Goroutine 吞没错误** | `Manager.Start` 使用 `go func()` 启动 goroutine 执行 `r.Run()`，但 goroutine 中 `r.Run()` 返回的 error 未被捕获和传播，导致调用方（API handler）永远无法感知执行失败 | 引入 `safeGo` 工具函数统一管理 goroutine，在 panic 时记录错误日志；增加 `runErr atomic.Value` 和 `setError()`/`Error()` 方法传播错误 |
| **2** | **缺少失败 run_record** | `Runner.Run()` 在 buildDAG/buildScope/setup 等初始化步骤失败时，直接 return error，没有创建 run_record，导致数据库中没有失败记录，前端 Dashboard 无法显示错误信息 | 在初始化失败路径中调用 `createFailedRunRecord()` 创建状态为 `failed` 的记录，写入 error_msg 字段 |

### 排查方法

**Step 1: 搜索 error 日志**
```bash
grep "error" /var/log/salvo.log | grep -v "health"
```

**Step 2: 检查 run_record 数据库**
```sql
SELECT id, scene_id, status, error_msg, started_at
FROM run_records
WHERE scene_id = <目标场景ID>
ORDER BY id DESC LIMIT 5;
```

**Step 3: 检查 Manager goroutine 行为**
- 如果 run_record 不存在但返回了 400 给前端 → 说明 `Manager.Start` 返回了 error
- 如果 run_record 不存在且前端收到了 200 → 说明 error 被 goroutine 吞没了

### 最终修复方案

```go
// 1️⃣ 统一使用 safeGo 管理 goroutine
safeGo(ctx, log, "scene-runner", func() {
    runErr := r.Run(ctx)
    if runErr != nil {
        r.setError(runErr)
        log.Error("scene run failed",
            logger.F("scene_id", r.cfg.SceneID.String()),
            logger.F("error", runErr))
    }
})

// 2️⃣ 初始化失败时创建 run_record
func (r *Runner) createFailedRunRecord(log logger.Logger, runErr error, failedStep string) {
    record := &model.RunRecord{
        Model:   model.Model{ID: r.runID},
        RunID:   r.runID,
        SceneID: r.cfg.SceneID,
        Status:  model.RunStatusFailed,
        ErrorMsg: runErr.Error(),
        // ... other fields
    }
    if err := r.runs.Create(r.ctx, record); err != nil {
        log.Error("failed to create failed run record")
    }
}

// 3️⃣ 通过 Error() 方法获取后台错误
err := rn.Error()  // 获取 goroutine 中存储的错误
```

### 效果对比

| 指标 | 修复前 | 修复后 |
|------|--------|--------|
| 初始化失败可排查性 | ❌ run_record 不存在，无错误信息 | ✅ run_record 含 error_msg，日志含详细字段 |
| Goroutine panic 可观测性 | ❌ panic 导致进程崩溃或静默吞没 | ✅ safeGo 捕获 panic 并记录 stacktrace |
| API handler 错误感知 | ❌ 只能感知 `Manager.Start()` 自身错误 | ✅ 能获取 `r.Run()` 内部错误 |

### Lessons Learned

#### 1. Goroutine 中的错误必须显式传播
- `go func()` 启动的 goroutine 内部错误不会被调用方感知
- 使用 `safeGo` 统一管理 + `atomic.Value` 存储错误

#### 2. 所有失败路径都必须写入持久化存储
- 初始化阶段（buildDAG/buildScope/setup）失败不能仅 return error，必须创建 run_record
- 数据库中的错误记录是排查的第一手资料

#### 3. Panic 恢复应包含结构化日志
```go
// ✅ 正确：包含 goroutine 名称、panic 值、stacktrace
log.Error("goroutine panicked",
    logger.F("goroutine", name),
    logger.F("panic", fmt.Sprintf("%v", r)),
    logger.F("stacktrace", string(debug.Stack())),
)

// ❌ 错误：缺乏上下文信息
fmt.Printf("panic: %v\n", r)
```

#### 4. 四级链路日志是排查的核心工具
- **Scene 级**：`scene run started/completed/failed` — 宏观定位
- **Chain 级**：`DAG built` / `chain execution started` — DAG 层面
- **Node 级**：`node execution started/completed/failed` — 节点级别
- **Function 级**：`generator:` — 函数级别

#### 5. 错误优先的日志策略
- 在 `Runner.Run()` 中，先做可能失败的操作（buildDAG/buildScope），成功后才有 info 日志
- 失败时立即输出 error 日志并写入 run_record
- 所有关键步骤都要有成功/失败两条路径的日志

#### 6. Manager.Start 中的 error 要返回给调用方
- 不要只记录日志就返回 nil
- 使用 `r.setError(err)` 存储错误 ×
- 从 `Manager.Runners()` 返回 runner 实例，调用方通过 `rn.Error()` 获取错误
