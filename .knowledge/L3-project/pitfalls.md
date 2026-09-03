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

---

## Lesson 4: DAG AND-join 阻塞导致 if-else 分支后流程永久停顿 (2026-07-20)

### 现象
运行 if-else 分支场景时，仅执行部分步骤即停止（如 22 步停止，预期 60+ 步），状态为 `completed`（0 失败），但流程未到达终点。汇合点之后的节点均未执行。

### 根因分析（3 层叠加）

| # | 问题层 | 影响 | 修复 |
|---|--------|------|------|
| **1** | **trace.go 缺少 parentFailed 检测** | `ExecuteWithTrace` 是实际运行路径（runner 调用它而非 `Execute`），但 trace.go 没有检查父节点是否被跳过，导致被跳过的父节点的子节点仍然执行 | 在 trace.go 的 inEdges 循环中添加 `parentFailed` / `hasActiveParent` 检测逻辑 |
| **2** | **条件边不满足时立即 return** | 当条件边评估为 false 时，代码立即 `close(sig) + return`，不考虑其他入边。多入边汇合节点只因为一条条件边不满足就被整体跳过 | 条件边不满足时仅标记该边为非活跃，继续处理其他边；所有边处理完后统一判断 |
| **3** | **普通边不区分"跳过"和"失败"** | 当父节点被条件跳过（正常行为，非失败），其下游的普通边子节点误将父节点无结果视为"失败"。实际上父节点是"条件跳过"而非"执行失败" | 引入 `skipped` map 记录被条件跳过的节点，普通边检查时跳过 skipped 父节点 |

### OR-join 语义设计

修复后 DAG 引擎采用隐式 OR-join 语义：

| 入边类型 | 父节点状态 | 处理方式 |
|---------|-----------|---------|
| 条件边 (`EdgeCondition`) | 父节点被跳过（无结果） | 豁免：该边非活跃，不算失败 |
| 条件边 (`EdgeCondition`) | 父节点成功，条件满足 | `hasActiveParent = true` |
| 条件边 (`EdgeCondition`) | 父节点成功，条件不满足 | 该边非活跃，不算失败 |
| 普通边 (`EdgeNormal`) | 父节点成功 | `hasActiveParent = true` |
| 普通边 (`EdgeNormal`) | 父节点被跳过（在 skipped map 中） | 豁免：该边非活跃，不算失败 |
| 普通边 (`EdgeNormal`) | 父节点执行失败 | `parentFailed = true` |

**汇合节点判定**：`len(inEdges) > 0 && (parentFailed || !hasActiveParent)` → 跳过

这等同于 OR-join：只要有至少一条活跃路径到达，汇合节点就执行。

### 排查方法

```bash
# 1. 检查条件边评估结果
grep "evalCondition" salvo-stdout.log

# 2. 检查节点跳过原因
grep "skipping node\|no active parent path\|condition not met" salvo.log

# 3. 检查执行路径是否完整
grep "node execution" salvo.log | grep -v "started" | head -30
```

### 最终修复方案

1. **trace.go**: 重写 inEdges 循环，条件边评估移入信号接收分支，条件不满足时不立即 return
2. **executor.go**: 同步相同逻辑，增加 `skipped` map 和 `hasActiveParent` 判断
3. **Executor.skipped map**: 新增字段，记录被条件跳过的节点 ID
4. **单元测试**: 添加 4 个 OR-join 测试覆盖双边/三边/全跳过场景

### 效果对比

| 指标 | 修复前 | 修复后 |
|------|--------|--------|
| if-else 双边汇合 | AND-join 阻塞，流程停止 | OR-join，活跃路径到达即执行 |
| if-else 三边汇合 | 同上 | 同上 |
| 条件跳过的子节点级联 | 子节点仍执行（trace.go 无检测） | 子节点被正确跳过 |
| 被跳过父节点的普通边子节点 | 误判为失败 | 正确识别为条件跳过 |

### Lessons Learned

#### 1. 两条代码路径必须保持同步
- `Execute()` 和 `executeTraced()` 是独立的两条执行路径
- runner 调用的是 `ExecuteWithTrace()` → `executeTraced()`，不是 `Execute()`
- **任何逻辑变更必须同时修改两处**

#### 2. 条件边评估不应阻断其他入边的处理
- 旧逻辑：条件边不满足 → 立即 `return`（跳过整个节点）
- 新逻辑：条件边不满足 → 标记该边非活跃，继续处理其他边
- **多入边节点的任何一条边不满足，都不应阻止其他边的评估**

#### 3. "跳过"和"失败"是不同的语义
- 节点因条件边不满足被跳过 → 预期行为，不应阻塞下游
- 节点因执行错误而失败 → 异常行为，应阻塞下游普通边
- **必须用独立的数据结构（如 skipped map）区分这两种状态**

#### 4. 调试日志要加在实际执行的代码路径上
- 首次调试时将日志加在 `executor.go` 的 `Execute()` 方法中
- 但实际运行走的是 `trace.go` 的 `executeTraced()`
- **添加调试日志前先确认实际代码路径**

#### 5. OR-join 是 if-else DAG 的必要语义
- 所有 if-else 分支后汇合的场景都需要 OR-join
- AND-join 在 if-else 场景下必然阻塞（非活跃分支永不执行）
- **YAML 编排灵活性依赖于 OR-join 支持**

---

## Lesson 5: dagre 固定节点尺寸导致 DAG 一键美化后重叠 (2026-07-21)

### 现象
DAG 编辑器一键美化后，展开的 group/while 节点与周围节点和连线重叠，放大后仍需手动拖动才能看清。

### 根因分析
dagre 布局引擎不支持变长节点尺寸。所有节点统一使用 300×90 的固定尺寸计算布局，但展开的 group 节点（含子节点）和 while 节点（含步骤）实际渲染高度远超 90px，导致布局计算出的位置与实际渲染冲突。

### 修复方案
替换 dagre 为 ELK (Eclipse Layout Kernel)：
- ELK 原生支持变长节点尺寸，每个节点传入实际 width/height
- 展开的 group 节点根据子节点数量动态计算高度
- 展开的 while 节点根据步骤数量动态计算高度
- ELK 的 layered 算法 + BRANDES_KOEPF 策略减少边交叉

### Lessons Learned

#### 1. 布局引擎必须支持变长节点
- DAG 中节点尺寸不统一时（尤其是可展开/折叠节点），固定尺寸布局引擎必然产生重叠
- ELK 和 dagre 的 API 模式不同：ELK 是 async（返回 Promise），dagre 是 sync

#### 2. 异步布局需要调整调用方式
- `buildLayout()` 从同步变为异步（`async/await`）
- `applyLayout()` 和 `autoLayout()` 也需要 await
- VueFlow 的 watch 回调中调用 async 函数不会阻塞，无需特殊处理

---

## Lesson 6: WebSocket 实时状态推送注意事项 (2026-07-21)

### 注意事项

#### 1. gorilla/websocket 的 ReadPump/WritePump 必须分离 goroutine
- 每个连接需要独立的 ReadPump 和 WritePump goroutine
- WritePump 负责 ticker 心跳（30s），ReadPump 负责 pong 检测
- 不能在同一个 goroutine 中同时读写

#### 2. Hub 必须通过 channel 管理 Client 生命周期
- Register/Unregister 通过 channel 传递，避免 map 并发访问
- Client 断连时必须在 Hub 中清理订阅，否则内存泄漏
- maxClients 限制防止连接风暴

#### 3. 广播消息格式要统一
- span_update 事件格式：`{ type, run_id, chain_id, node_id, status, duration_ns, error, loop_index }`
- 循环节点的中间事件（status=running + loop_index）需要在每次迭代开始时广播
- 前端需要同时处理 running 中间事件和最终 completion 事件

#### 4. 前端 WS 重连要指数退避
- 断线后 1s → 2s → 4s → ... → 最大 30s
- 重连后必须重新发送 subscribe 消息
- onUnmounted 时断开连接，防止内存泄漏

---

## Lesson 7: Go 1.26.2 运行时 FD.Write 返回错误字节数导致 mock server panic (2026-08-26)

### 现象
Salvo 运行测试场景期间，日志中出现 goroutine panic：
```
panic: invalid return from write: got 129 from a write of 7
```
panic 发生在 mock server 处理 `POST /mock/api/users` 返回 201 响应后，flush 阶段崩溃。

### 完整证据链

**Step 1: 定位 panic 日志**
```bash
grep -n "panic\|invalid return from write" logs/salvo.log
# 结果：3 处匹配（1 次原始 panic + 1 次 recovered panic + 1 次 finalFlush 重入）
```

**Step 2: 分析堆栈**
```
goroutine 241945 [running]:
internal/poll.(*FD).Write(0x..., {0x..., 0x109, 0x1000})    ← Go 标准库 poller
net.(*netFD).Write(0x..., {0x...})
net.(*conn).Write(0x..., {0x...})
net/http.checkConnErrorWriter.Write({0x...}, {0x...})
bufio.(*Writer).Flush(0x...)
net/http.(*response).finishRequest(0x...)   ← HTTP 响应结束 flush
net/http.(*conn).serve(0x..., {0x..., 0x...})
created by net/http.(*Server).Serve in goroutine 3
```

**Step 3: 确认无 Salvo 业务代码在堆栈中**
- 堆栈最深层为 `internal/poll.(*FD).Write`（Go 运行时）
- 无任何 `internal/runner/`、`internal/mock/`、`internal/api/` 路径
- panic 发生在 `net/http.(*response).finishRequest` → `bufio.Flush` → `FD.Write`

**Step 4: 确认 panic 被 recover**
```
panic: invalid return from write: got 129 from a write of 7 [recovered]
```
`net/http.(*conn).serve.func1()` 中的 defer recover 捕获了 panic，**不影响其他请求**。

**Step 5: 确认触发上下文**
```
[MOCK-REQ] 2026/08/26 14:19:55 POST /mock/api/users | 201 | 114.55ms
2026/08/26 14:19:55 http: panic serving [::1]:50136: invalid return from write: got 129 from a write of 7
```
- Mock server 正常处理请求并返回 201
- 客户端地址 `[::1]:50136` 是 Salvo runner 发出的请求
- 在 flush 响应体时，底层 write 系统调用返回 129（期望 7）

**Step 6: 确认 Go 版本**
```bash
go version
# go version go1.26.2 darwin/arm64
```

### 根因

这是 **Go 1.26.2 在 macOS (darwin/arm64) 上的运行时 bug**。`internal/poll.(*FD).Write` 在某些边界条件下（高并发 + 连接关闭时序竞争）返回了错误的字节数。`net/http` server 检测到 `n != len(p)` 后主动 panic 以保护数据一致性。

**关键细节**：堆栈中 `FD.Write` 的 buffer 参数为 `{ptr, 0x109, 0x1000}`（length=265, cap=4096），但 panic 消息说 "write of 7"。这说明 panic 来自更上层的某次小写入（可能是 HTTP chunked encoding 的 trailing chunk `0\r\n\r\n` = 5 bytes 或 headers flush），而非堆栈中显示的 265-byte buffer write。这是 Go HTTP server 在 response finish 阶段多次 flush 中的某一次失败了。

### 影响评估

| 维度 | 评估 |
|------|------|
| 数据完整性 | 该连接响应可能不完整，但 mock server 无状态，不影响测试数据 |
| 其他请求 | 不受影响，panic 被 per-connection goroutine 的 recover 捕获 |
| 正在运行的场景 | 不受影响，runner 的 HTTP client 会收到连接重置错误，按正常错误路径处理 |
| 进程稳定性 | 不受影响，仅该 goroutine 终止 |
| 复现频率 | 日志中仅出现 1 次（3 行匹配为同一事件的重入），属偶发 |

### 建议

1. **无需修复 Salvo 代码**：这不是应用层 bug，堆栈中无业务代码
2. **短期**：观察日志中该 panic 的出现频率，如果仅偶发（< 10 次/小时），可忽略
3. **中期**：关注 Go 1.26.3+ release notes，确认是否修复此 poller bug
4. **长期**：如频繁出现，考虑回退到 Go 1.25.x 或升级到修复版本

### Lessons Learned

#### 1. 堆栈分析是区分应用 bug 和运行时 bug 的关键
- 检查堆栈中是否包含业务代码路径
- 如果堆栈完全在标准库/运行时内 → 优先怀疑 Go 版本 bug
- 如果堆栈包含业务代码 → 检查业务代码逻辑

#### 2. `net/http` server 的 per-connection panic 不会杀死进程
- `serve.func1()` 有 defer recover，panic 仅终止该连接的 goroutine
- 其他连接和 goroutine 不受影响
- 这是 Go HTTP server 的设计特性，不是 bug

#### 3. "invalid return from write" 是 poller 层的经典问题
- 常见于 macOS kqueue / Linux epoll 的异步 I/O 实现
- 通常由连接关闭时序竞争引起（client 已关闭连接，server 仍在写）
- Go 运行时选择 panic 而非静默忽略，是为了防止数据损坏

#### 4. 排查 panic 的标准流程
```
日志 grep panic → 提取堆栈 → 判断业务/运行时 → 确认 recover 状态 → 评估影响 → 决定修复策略
```

---

## Lesson 8: 失败节点多字段 AND 查询 — 前端 computed 与导出 HTML JS 双实现模式 (2026-08-27)

### 现象
测试报告失败节点详情只有单搜索框（OR 逻辑），无法满足"节点名含'订单'且错误码是503"的联合查询需求。

### 根因
单搜索框跨字段 OR 模糊匹配，只要任一字段命中即返回，无法精确定位多条件组合的记录。

### 修复方案
拆分为 5 个独立查询字段（节点名、NodeID、错误码、错误信息、协议下拉），AND 逻辑联合过滤。

### 关键实现模式 — 双套实现

| 环境 | 实现方式 | 说明 |
|------|---------|------|
| 前端 SPA | Vue `ref` + `computed` | 5 个 ref 绑定 v-model，computed 响应式自动过滤 |
| 导出 HTML | 原生 JS + `data-*` 属性 | 5 个 input/select，`oninput`/`onchange` 触发过滤函数 |

**前端 computed 核心逻辑**：
```typescript
const filteredFailedNodes = computed(() => {
  return nodes.filter(fn => {
    if (qName && !fn.node_name.toLowerCase().includes(qName)) return false
    if (qProto && (fn.protocol || 'http') !== qProto) return false  // 协议精确匹配
    return true
  })
})
```

**导出 HTML JS 核心逻辑**：
```javascript
function filterFailedNodes() {
  cards.forEach(function(card) {
    var match = true;
    if (qName && !name.includes(qName)) match = false;
    if (qProto && proto !== qProto) match = false;
    card.style.display = match ? '' : 'none';
  });
}
```

### Lessons Learned

#### 1. 空字段不参与过滤
- 空字段视为"无约束"，只有用户主动填写的字段才作为 AND 条件
- 避免了"必须填满所有字段才能查询"的误区

#### 2. 文本用模糊匹配，协议用精确匹配
- node_name/node_id/error_code/error_message 用 `includes()` 模糊匹配 + 忽略大小写
- protocol 用 `===` 精确匹配（下拉选择，值固定）
- protocol 为空时默认按 "http" 处理

#### 3. 协议下拉选项从数据动态生成
- 前端：`computed` 从 failedNodes 提取 unique protocols
- 导出 HTML：`populateProtocolOptions()` 遍历 `.failed-node-card` 的 `data-protocol` 属性

#### 4. CSS Grid 网格布局自适应
- `grid-template-columns: repeat(auto-fit, minmax(160px, 1fr))` 实现一行排列 + 窄屏自动换行
- 避免五个输入框在窄屏拥挤

#### 5. 双套实现需保持逻辑一致
- 前端 Vue computed 和导出 HTML JS 各自独立实现，但过滤逻辑必须一致
- 修改时需同步更新两处，避免行为不一致