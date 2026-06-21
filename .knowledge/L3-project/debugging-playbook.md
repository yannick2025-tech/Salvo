---
layer: L3
maturity: proven
last-verified: 2026-05-25
source: .trae/rules/project_rules.md
tags: [debugging, troubleshooting, performance, layout, workflow]
---

# 问题排查手册

> 遇到已知类型的问题时，**必须先查阅 Lessons Learned**，复用已验证的方法论，避免重复踩坑。

## 核心原则

**不要猜！用数据和可视化说话。**

## 触发条件 — 何时自动查阅

在以下场景开始排查前，**必须先读取** [L3-project/pitfalls.md](./pitfalls.md)：

| 问题类型 | 触发关键词 | 对应 Lesson |
|---------|-----------|-------------|
| **性能/加载慢** | "卡顿"、"慢"、"延迟"、"加载时间"、"首屏"、"白屏" | Lesson 1: Runtime 图表优化 |
| **CSS/布局异常** | "溢出"、"超出边界"、"色块"、"定位错位"、"被遮挡" | Lesson 2: 链路跟踪溢出 |
| **ECharts 图表问题** | "图表不显示"、"颜色变化"、"曲线异常" | Lesson 1 + ECharts 配置规范 |
| **异步/时序问题** | "数据没到"、"顺序错误"、"竞态"、"依赖" | Lesson 1: 异步依赖链 |

## 排查顺序（强制执行）

```
Step 1: 📖 查阅 Lessons Learned
   ↓ 读取 pitfalls.md
   ↓ 找到匹配的 Lesson
   ↓ 应用其中的方法论（日志/可视化/追踪等）

Step 2: 🔍 定位瓶颈
   ↓ 性能问题 → 加时间戳日志（Lesson 1 教训 #1）
   ↓ 布局问题 → 全局边框扫描（Lesson 2 教训 #1）
   ↓ 图表问题 → 检查配置是否符合 chart-style.md

Step 3: 🎯 分析根因
   ↓ 对照 Lesson 中的"根因分析"表格
   ↓ 确认是否是相同或类似的问题模式

Step 4: ✅ 对症修复
   ↓ 使用 Lesson 中的"最终修复方案"
   ↓ 或基于教训推导新方案
```

## 经验模板

### 性能问题模板（参考 Lesson 1）

```javascript
// 1️⃣ 第一步：加日志（别猜！）
console.log(`[DEBUG-${功能}] ${调用点} | t=${Date.now()}ms | 关键状态`)

// 2️⃣ 第二步：分析日志时间线
//    - 数据什么时候到的？
//    - DOM 什么时候就绪的？
//    - 渲染函数什么时候被调用的？

// 3️⃣ 第三步：检查异步依赖链
//    - 是否有 fetchA() 在 fetchB() 之前但 B 依赖 A？
//    - 改为 await fetchA(); fetchB()

// 4️⃣ 第四步：DOM 可用性重试机制
//    - 不要 if (!ref) return 就放弃
//    - 用 setTimeout/retry 直到副作用生效
```

### 布局问题模板（参考 Lesson 2）

```javascript
// 1️⃣ 第一步：全局边框扫描（让它可见！）
document.querySelectorAll('*').forEach(el => {
  el.style.outline = '1px solid rgba(255,0,0,0.2)'
})

// 2️⃣ 第二步：祖先链着色法
let el = targetElement
const colors = ['red', 'orange', 'yellow', 'green', 'blue']
while (el !== document.body) {
  el.style.outline = `2px solid ${colors[depth++ % colors.length]}`
  el = el.parentElement
}

// 3️⃣ 第三步：检查 computed style
getComputedStyle(el).width / maxWidth / boxSizing
```

### 后台问题排查模板（四级链路日志法）

当后台测试执行异常时，按照以下 SOP 利用四级链路日志定位问题：

```bash
# 1️⃣ 检查 Runner 生命周期日志（Scene 级别）
grep "scene run started\|scene run completed\|failed to" /var/log/salvo.log

# 2️⃣ 检查 DAG 构建日志（Chain 级别）
grep "DAG built\|buildDAG\|building DAG edges\|DAG built successfully" /var/log/salvo.log

# 3️⃣ 检查节点执行日志（Node 级别）
grep "node execution started\|node execution completed\|node execution failed" /var/log/salvo.log

# 4️⃣ 检查 Generator 和内部函数日志（Function 级别）
grep "generator:" /var/log/salvo.log
```

**排查流程**：

```
Step 1: 定位失败场景
   ↓ 搜索 error 级别日志 → 找到第一个错误
   ↓ 提取 trace_id / scene_id / chain_id / node_id

Step 2: 追踪链路
   ↓ 用 trace_id 过滤该场景所有日志
   ↓ 按时间线排列：scene run → DAG → node execution → function

Step 3: 判断失败原因类型
   ↓ 初始化失败 → 检查 buildDAG/buildScope 日志
   ↓ 执行失败 → 检查节点 error 日志中的 error 字段
   ↓ Panic → 搜索 "goroutine panicked" + stacktrace
   ↓ Worker 层面 → 检查 "worker pool" 相关日志

Step 4: 验证修复
   ↓ 修复后检查 error 日志是否消失
   ↓ 检查 run_record 中 status 是否正确更新
```

**关键字段索引**：

| 字段 | 用途 | 示例值 |
|------|------|--------|
| `trace_id` | 关联同一场景所有日志 | `"trace_id": "123456789"` |
| `scene_id` | 过滤特定场景 | `"scene_id": "10001"` |
| `chain_id` | 关联同一轮 DAG 执行 | `"chain_id": "chain-001"` |
| `node_id` | 定位具体节点 | `"node_id": "node-42"` |
| `goroutine` | 定位 goroutine panic | `"goroutine": "scene-runner"` |
| `stacktrace` | Panic 时包含完整调用栈 | `"stacktrace": "goroutine 46 [running]..."` |

## 反模式警告 ⚠️

以下做法已被证明低效，**禁止作为首选方法**：

| ❌ 低效做法 | ✅ 正确替代 |
|------------|-------------|
| 目视猜测哪个元素有问题 | 全局边框扫描 + 日志 |
| 逐个调整 CSS 属性试错 | 先用可视化工具定位 |
| 不加日志直接优化代码 | 先加日志找到瓶颈 |
| 同时发起多个不相关的请求 | 梳理依赖链，按序执行 |
| DOM 未就绪时就放弃并标记完成 | 自动重试直到成功 |
| 从目标元素逐层往上查祖先 | 先全局扫描再局部聚焦 |

## 新增 Lesson 的时机

当遇到以下情况时，**必须在 [L3-project/pitfalls.md](./pitfalls.md) 中新增一个 Lesson**：

1. 排查耗时超过 30 分钟的问题
2. 发现了非直觉的根因（容易再次踩坑）
3. 总结出了可复用的方法论（日志/可视化/追踪等）
4. 解决了用户反馈的"体验问题"

**格式要求**：
- 标题格式：`## Lesson N: 简短描述 (YYYY-MM-DD)`
- 必须包含：现象、排查过程、根因、修复方案、Lessons Learned（5条以上）
- 必须包含：效果对比表格（优化前 vs 优化后）
- 参考现有 Lesson 的结构保持一致
