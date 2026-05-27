## Why

当前 Salvo 测试平台只关注业务层面的性能指标（QPS、延迟、错误率等），但缺乏对测试引擎自身运行状态的监控。在高并发压测场景下，无法及时发现：
- Goroutine 泄漏导致资源耗尽
- GC 暂停影响测试精度
- 内存持续增长导致 OOM
- **Worker 协程调度瓶颈**（任务排队等待时间过长）
- **任务吞吐量不达标**（实际 QPS 远低于目标值）

这些系统级指标的缺失使得问题排查困难，用户无法判断性能下降是来自被测系统还是测试工具本身。

## What Changes

- 新增 **Runtime Metrics Collector** 独立模块，在测试运行期间定期采集 Go runtime 和系统级指标
- 扩展 `ReportDetail` 数据结构，新增 `SystemMetrics` 字段存储时序性能数据
- **改造 Pool 组件**，支持 Task Wait Time（任务排队等待时间）统计
- Dashboard 新增**系统资源监控区域**，包含仪表盘和实时曲线图
- 测试报告页新增**系统性能分析板块**，展示完整生命周期内的资源变化
- 导出的 HTML 报告同步包含所有新增图表和指标

### 最终确认的 9 个核心指标（P0 + P1）

#### 🔴 P0 - 核心必选指标（5 个）

| # | 指标名称 | 技术含义 | 采集方式 | 展示形式 | 阈值 |
|---|---------|---------|---------|---------|------|
| 1 | **Goroutine Count** | 当前活跃协程数，检测协程泄漏 | `runtime.NumGoroutine()` | 曲线图 + 仪表盘 | >10k ⚠️ / >50k 🚨 |
| 2 | **Heap Alloc (MB)** | 堆内存已分配量，监控内存压力 | `ReadMemStats().Alloc / 1024 / 1024` | 双线曲线(Alloc+Sys) + 仪表盘 | >512MB ⚠️ / >1GB 🚨 |
| 3 | **GC Pause Last (ms)** | 最近一次 GC 暂停时间，评估对测试精度的影响 | `memstats.PauseNs[255] / 1e6` | 曲线图 + 统计卡片 | >5ms ⚠️ / >20ms 🚨 |
| 4 | **Active Workers** | 当前正在执行任务的 Worker 数，反映 Worker 利用率 | Pool 内部 atomic 计数器 | 实时仪表盘 | <Workers×80% ⚠️ |
| **5** | **⭐ Task Wait Time (P99)** | **任务从提交到开始执行的排队等待时间，检测调度瓶颈** | **Pool.Submit() 注入时间戳，worker() 开始时计算差值** | **曲线图(P50/P95/P99) + 仪表盘** | **>10ms ⚠️ / >100ms 🚨** |

#### 🟡 P1 - 强烈推荐指标（4 个）

| # | 指标名称 | 技术含义 | 采集方式 | 展示形式 | 阈值 |
|---|---------|---------|---------|---------|------|
| 6 | **CPU Usage (%)** | 进程 CPU 占用率，判断是否达到 CPU 瓶颈 | Linux: `/proc/self/stat`; Mac: `sysctl/ps` | 三色区域曲线(0-70-90-100%) | >70% ⚠️ / >90% 🚨 |
| 7 | **Pending Queue Len** | 任务队列积压数，背压检测 | `len(tasks)` channel 或 `submitted-completed` | 实时数值 + 趋势图 | >workers ⚠️ |
| 8 | **QPS Achievement (%)** | 实际 QPS / 目标 QPS × 100%，吞吐量达标率 | 业务指标计算衍生 | 百分比仪表盘 | <90% ⚠️ / <70% 🚨 |
| 9 | **RSS Memory (MB)** | 物理常驻内存集大小，比 Heap 更准确反映实际占用 | `/proc/self/status` (VmRSS) 或 syscall | 数值卡片 | >1GB ⚠️ / >2GB 🚨 |

### 展示形式设计

#### Dashboard 新增"系统监控"区域布局：
```
┌─────────────────────────────────────────────────────────────┐
│ 现有 Metrics Row (6 cards):                                  │
│ [总请求] [成功率] [P99] [QPS] [TTFB] [持续时间]              │
├──────────┬──────────┬──────────┬─────────────────────────────┤
│ QPS 图表 │ 延迟分布  │ 错误率   │                             │
├──────────┴──────────┴──────────┤  系统资源监控 (NEW)          │
│                                 │                             │
│ ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐              │
│ │Gorout│ │Heap  │ │GC    │ │Worker│ │CPU   │  ← 5个仪表盘   │
│ │1.2k  │ │256MB │ │0.5ms │ │18/20 │ │45%   │              │
│ └──────┘ └──────┘ └──────┘ └──────┘ └──────┘              │
│                                 │                             │
│ ┌──────────────┬───────────────┤                             │
│ │Goroutine曲线 │ Heap Memory    │  ← 2条主趋势曲线           │
│ │              │ (双线)        │                             │
│ ├──────────────┼───────────────┤                             │
│ │Task Wait Time│ Pending Queue │  ← 辅助曲线                 │
│ │(P50/P95/P99) │ (实时柱状)    │                             │
│ └──────────────┴───────────────┘                             │
├─────────────────────────────────────────────────────────────┤
│ Node 详情图 (现有)                                            │
└─────────────────────────────────────────────────────────────┘
```

**关键特性：**
- ✅ 遵循项目 ECharts 规范（smooth/step 切换、统一样式）
- ✅ 图表独立状态管理（避免联动刷新 bug）
- ✅ 响应式布局（移动端自动折叠）
- ✅ 颜色语义化：绿色(正常) / 橙色(警告) / 红色(危险)

## Capabilities

### New Capabilities
- `runtime-metrics-collector`: 后端 Go runtime 和系统指标采集模块，负责定时采样和数据持久化，包含 Task Wait Time 统计功能
- `system-monitoring-ui`: 前端系统监控 UI 组件，包括 Dashboard 实时展示和报告页面历史分析
- `exported-report-system-metrics`: 导出 HTML 报告中的系统指标渲染和交互逻辑

### Modified Capabilities
- `report-detail-schema`: 扩展 ReportDetail 结构体，新增 SystemMetrics 时序数据字段和 TaskWaitTimeStats 聚合统计
- `pool-component`: 改造 Pool.Submit() 支持 Task Wait Time 采集（非侵入式包装）

## Impact

### 后端代码
- `internal/runner/runtime_metrics.go`: **新增** RuntimeMetricsCollector 采集器模块
- `internal/runner/report.go`: **修改** 扩展 ReportDetail 结构体，新增 SystemMetrics 字段
- `internal/core/pool/pool.go`: **修改** 改造 Submit() 方法，注入时间戳以支持 Task Wait Time 统计
- `internal/api/handler.go`: 无需修改（通过已有 JSON 序列化自动支持新字段）

### 前端代码
- `web/app/src/views/dashboard/DashboardPage.vue`: **修改** 新增系统监控区域（5个仪表盘 + 4条曲线图）
- `web/app/src/views/reports/ReportDetailPage.vue`: **修改** 新增系统性能分析 section
- `internal/api/report_generator_enhanced.go`: **修改** 导出报告模板新增系统指标图表

### 性能影响
- **采样频率**: 2 秒一次（与业务指标 1s 错开，减少 STW 影响）
- **单次开销**: < 1ms（ReadMemStats ~100μs + 系统指标读取 ~500μs）
- **内存占用**: 1 小时测试约 1800 样本 × ~250 bytes = ~450 KB
- **Task Wait Time**: 仅增加 2 次 time.Now() 调用（纳秒级开销）

### 依赖
- ✅ 无新增外部依赖（纯标准库实现）
- ✅ 兼容 Linux/macOS/Windows（Windows 降级为仅 runtime 指标）
- 可选未来扩展：集成 `prometheus/client_golang` 或 `gopsutil`
