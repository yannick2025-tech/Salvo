## Context

### 当前状态
Salvo 测试平台已具备完整的业务指标采集体系（QPS、延迟、错误率等），通过 `TimeSeriesCollector` 以 1 秒间隔采样并持久化。Runner 结构体管理测试生命周期，包含 `stats`、`nodeStats`、`collector` 等核心组件。

**现有架构约束：**
- 时序数据通过 `TimeSeriesStore` 接口存储（当前为内存实现）
- ReportDetail JSON 通过 `GenerateEnhancedHTML()` 渲染为导出报告
- Dashboard 通过 WebSocket 或轮询获取实时数据
- 前端使用 ECharts 统一图表渲染，遵循项目规范（平滑/阶梯切换、统一样式）

**技术栈：**
- 后端：Go 1.21+，标准库 runtime/debug 包
- 前端：Vue 3 + ECharts 5 + TypeScript
- 数据格式：JSON 序列化的 ReportDetail

## Goals / Non-Goals

**Goals:**
1. 在测试运行期间低开销地采集 Go runtime 和系统级性能指标
2. 将系统指标无缝集成到现有的 Dashboard 实时展示和报告分析流程
3. 提供直观的可视化（仪表盘+曲线图），帮助用户快速定位资源瓶颈
4. 确保新增功能不影响现有业务指标的采集精度和性能

**Non-Goals:**
- 不实现 Prometheus/Grafana 集成（未来扩展）
- 不做分布式监控（仅单机进程级别）
- 不修改数据库 schema（系统指标随 report JSON 存储）
- 不支持自定义指标采集（固定预设指标集）

## Decisions

### Decision 1: 采集器架构 - 独立模块 vs 扩展 TimeSeriesCollector

**选择：独立模块 `RuntimeMetricsCollector`**

**理由：**
- 职责分离：业务指标 vs 系统指标数据源不同、采样策略不同
- 可独立启停：用户可选择是否开启系统监控（默认开启）
- 降低耦合：不影响现有 TimeSeriesCollector 的稳定性和测试覆盖

**替代方案考虑：**
- ❌ 扩展 TimeSeriesCollector：会导致接口膨胀，Sample 结构体字段混杂
- ✅ 独立模块：清晰的边界，独立的配置和生命周期

### Decision 2: 指标数据结构设计

```go
type RuntimeMetricsSnapshot struct {
    Timestamp time.Time `json:"timestamp"`

    // Go Runtime
    GoroutineCount   int64   `json:"goroutine_count"`
    HeapAllocMB      float64 `json:"heap_alloc_mb"`
    HeapSysMB        float64 `json:"heap_sys_mb"`
    HeapIdleMB       float64 `json:"heap_idle_mb"`
    GCPauseTotalNs   uint64  `json:"gc_pause_total_ns"`
    GCPauseLastNs    uint64  `json:"gc_pause_last_ns"`
    GCCount          uint32  `json:"gc_count"`
    NextGC           uint64  `json:"next_gc"`

    // System (Linux/Mac)
    CPUUsagePercent float64 `json:"cpu_percent"`
    RSSMemoryMB     float64 `json:"rss_mb"`
    ThreadCount     int     `json:"thread_count"`

    // Runner Internal
    ActiveWorkers   int     `json:"active_workers"`
    PendingQueueLen int     `json:"pending_queue_len"`

    // Task Wait Time (P50/P95/P99)
    TaskWaitAvgMs    float64 `json:"task_wait_avg_ms"`    // 平均等待时间
    TaskWaitP50Ms    float64 `json:"task_wait_p50_ms"`    // P50 等待时间
    TaskWaitP95Ms    float64 `json:"task_wait_p95_ms"`    // P95 等待时间
    TaskWaitP99Ms    float64 `json:"task_wait_p99_ms"`    // P99 等待时间
    TaskWaitMaxMs    float64 `json:"task_wait_max_ms"`    // 最大等待时间
    TaskWaitSampleCount int64 `json:"task_wait_samples"`  // 统计样本数
}
```

**设计要点：**
- 使用标准 `runtime.ReadMemStats()` 读取堆信息（< 1μs 开销）
- CPU 使用率通过 `/proc/self/stat`（Linux）或 `ps` 命令（Mac）计算
- 所有数值类型统一：整数用 int64/uint64，浮点用 float64
- 时间戳与业务指标 Sample 对齐，便于关联分析
- **Task Wait Time 字段**：从 Pool 的 WaitTimeTracker 获取滑动窗口统计值

### Decision 2.5: Task Wait Time 实现方案

**核心挑战：** Go channel 无法直接测量"在队列中等待的时间"

**解决方案：包装 Task 函数注入时间戳**

```go
// 在 Pool.Submit() 中改造
func (p *Pool) Submit(task Task) error {
    submitTime := time.Now()  // ← 记录提交时刻

    wrappedTask := func(ctx context.Context) error {
        waitDuration := time.Since(submitTime)  // ← 计算等待时间
        p.waitTracker.Record(waitDuration)       // ← 记录到统计器
        return task(ctx)                         // ← 执行原始任务
    }

    select {
    case p.tasks <- wrappedTask:
        p.submitted.Add(1)
        return nil
    case <-p.ctx.Done():
        return p.ctx.Err()
    }
}

// WaitTimeTracker: 滑动窗口统计器
type WaitTimeTracker struct {
    mu      sync.Mutex
    samples []time.Duration  // 固定大小环形缓冲区 (默认 1000)
    pos     int
    count   int64
    totalNs int64            // 用于计算 avg
}

func (t *WaitTimeTracker) Record(d time.Duration) {
    t.mu.Lock()
    defer t.mu.Unlock()

    if len(t.samples) == 0 {
        t.samples = make([]time.Duration, 1000)
    }

    t.samples[t.pos] = d
    t.pos = (t.pos + 1) % len(t.samples)
    t.count++
    t.totalNs += int64(d)
}

func (t *WaitTimeTracker) Stats() WaitTimeStats {
    t.mu.Lock()
    defer t.mu.Unlock()

    sorted := make([]time.Duration, len(t.samples))
    copy(sorted, t.samples)
    sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

    return WaitTimeStats{
        Avg: time.Duration(t.totalNs / max(t.count, 1)),
        P50: percentile(sorted, 50),
        P95: percentile(sorted, 95),
        P99: percentile(sorted, 99),
        Max: sorted[len(sorted)-1],
        SampleCount: t.count,
    }
}
```

**为什么这个方案最优：**
1. **非侵入性**：不修改原始 Task 函数签名和行为
2. **高精度**：使用 `time.Now()` 纳秒级精度
3. **低开销**：仅增加 2 次 `time.Now()` 调用 + 1 次 `time.Since()`
4. **线程安全**：使用 mutex 保护共享状态
5. **内存可控**：固定大小环形缓冲区（1000 样本），不会无限增长

**集成方式：**
```
Pool 结构体新增字段：
├── waitTracker *WaitTimeTracker  (在 New() 时初始化)
├── Submit() 改造：包装 task + 注入 submitTime
└── RuntimeMetricsCollector 读取：调用 p.waitTracker.Stats()
```

### Decision 3: 采样频率与生命周期

**采样频率：2 秒一次**
- 平衡精度与开销：runtime.ReadMemStats() 触发 STW（Stop-The-World），过于频繁会影响测试
- 与业务指标 1 秒采样错开，避免同时触发 GC

**生命周期管理：**
```
测试启动 → 创建 RuntimeMetricsCollector → 启动采样 goroutine
         ↓
测试运行中 → 每 2s 采集一次 → 追加到内存切片
         ↓
测试结束 → 停止采样 → 计算聚合统计 → 写入 ReportDetail.SystemMetrics
         ↓
报告生成 → JSON 序列化 → 前端渲染
```

### Decision 4: 前端 UI 设计

#### Dashboard 新增区域布局
```
┌─────────────────────────────────────────────────────┐
│ Metrics Row (现有)                                   │
│ [总请求] [成功率] [平均延迟] [P99] [QPS] [持续时间]    │
├──────────┬──────────┬──────────┬─────────────────────┤
│ QPS 图表 │ 延迟分布  │ 错误率   │                     │
├──────────┴──────────┴──────────┤ 系统监控区域 (新增)   │
│                                     ┌─────┐ ┌─────┐ │
│  Goroutine 曲线                    │内存 │ │CPU  │ │
│                                     │仪表│ │仪表│ │
│  Heap Memory 曲线                   └─────┘ └─────┘ │
│                                     ┌─────────────┐ │
│  GC Pause 曲线                       │ Worker 状态  │ │
│                                     └─────────────┘ │
├─────────────────────────────────────────────────────┤
│ Node 详情图 (现有)                                    │
└─────────────────────────────────────────────────────┘
```

**组件复用原则：**
- 复用现有 `.chart-card`、`.chart-body` 样式类
- 复用 ECharts 配置规范（smooth/step 切换、tooltip 格式）
- 新增 `.gauge-container` 用于仪表盘容器

#### 报告页面新增 Section
在"趋势分析"和"节点详情"之间插入"系统性能分析"板块：
- 4 个 Gauge 卡片（Goroutine、内存、CPU、GC）
- 3 个曲线图（Goroutine 趋势、Heap 趋势、CPU 趋势）
- 1 个汇总表格（峰值/均值/最小值）

### Decision 5: 阈值告警机制

**三级状态定义：**

| 指标 | 正常 (绿色) | 警告 (橙色) | 危险 (红色) |
|------|------------|------------|------------|
| Goroutine Count | < 10,000 | 10,000-50,000 | > 50,000 |
| Heap Alloc MB | < 512 MB | 512 MB - 1 GB | > 1 GB |
| CPU Usage % | < 70% | 70%-90% | > 90% |
| GC Pause Last ms | < 5ms | 5-20ms | > 20ms |

**实现方式：**
- 后端：RuntimeMetricsSnapshot 中不包含阈值逻辑（纯数据采集）
- 前端：ECharts visualMap 或 markLine 绘制阈值线
- 仪表盘：根据当前值动态设置颜色

## Risks / Trade-offs

### Risk 1: ReadMemStats() 触发 STW 影响测试精度
**影响：** 每次调用可能触发短暂的 GC 暂停（通常 < 100μs）
**缓解措施：**
- 采样间隔设为 2 秒，将影响降至最低
- 在高精度测试场景下提供关闭选项（配置项 `EnableSystemMetrics: false`）

### Risk 2: 跨平台兼容性（Linux/Mac/Windows）
**影响：** CPU 和 RSS 读取方式不同
**缓解措施：**
- 使用 `runtime.GOOS` 条件编译
- Linux: `/proc/self/stat` + `/proc/self/status`
- Mac: `sysctl` 或 `ps` 命令解析
- Windows: 使用 gopsutil 库或降级处理（仅采集 runtime 指标）

### Risk 3: 内存占用增长
**影响：** 1 小时测试约产生 1800 个采样点 × ~200 bytes = 360 KB
**缓解措施：**
- 数据量可控，远小于业务时序数据
- 测试结束后释放内存，仅在 ReportDetail 中保留聚合结果

### Trade-off: 实时性 vs 准确性
- **选择：准实时（2s 延迟）换取更高准确性**
- Dashboard 展示时使用最近一次采样值，非真正实时

## Open Questions

1. **是否需要持久化原始时序数据？**
   - 当前方案：仅保留在 ReportDetail JSON 中
   - 未来可考虑：写入 InfluxDB/Prometheus 做长期趋势分析

2. **是否需要 API 端点单独暴露系统指标？**
   - 当前方案：通过 `/api/v1/reports/:id` 返回完整 report（含系统指标）
   - 未来可考虑：`GET /api/v1/runs/:id/metrics/system` 实时查询

3. **Worker 内部指标（Active Workers、Pending Queue）的数据来源？**
   - 需要 Runner 暴露内部状态接口或回调函数
   - 建议：在 StatsProvider 接口中新增 `RuntimeMetrics() *RuntimeMetricsSnapshot` 方法
