# 测试报告系统增强设计文档 (Enhanced Test Report System)

**版本**: 1.0  
**日期**: 2026-05-07  
**状态**: 已批准  
**作者**: AI Assistant  

---

## 1. 项目概述

### 1.1 背景与动机

当前 Salvo 测试平台的报告系统存在以下关键限制：

#### ❌ 现有问题

1. **数据粒度不足**
   - 仅支持全局级别的统计指标（QPS、P50/P95/P99）
   - 缺少每个节点（Node）的独立性能指标
   - 无法分析特定 API 端点的瓶颈

2. **时序数据缺失**
   - 无实时趋势图（QPS、延迟随时间变化曲线）
   - Dashboard 在测试运行结束后丢失实时数据
   - 无法进行历史对比和回溯分析

3. **报告质量有限**
   - HTML 导出使用简单模板，视觉效果差
   - 图标和 UI 不够专业
   - 不符合企业级测试报告标准

4. **可扩展性差**
   - 无法支持长时间压测（>1 小时）
   - 大并发场景下内存占用过高
   - 数据无法持久化，程序崩溃会丢失所有数据

### 1.2 项目目标

构建一个**生产级、可扩展、持久化**的测试报告系统，具备以下能力：

✅ **细粒度采集**: 全局 + 每个节点的独立 QPS/延迟统计  
✅ **时序可视化**: 实时趋势图 + 历史数据查询  
✅ **专业级报告**: 基于 report-preview.html 的精美 HTML 模板  
✅ **高可用架构**: 内存 + SQLite 双层存储，支持崩溃恢复  
✅ **Dashboard 增强**: 运行后数据不丢失，支持历史查看  
✅ **可扩展性**: 支持 50+ 节点、8+ 小时长时间运行  

### 1.3 技术选型决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| **存储策略** | 方案 C: 混合模式 | 平衡实时性与持久化，最稳健 |
| **采样频率** | 1 秒间隔 | 精度足够，性能开销小 |
| **刷写频率** | 10 秒批量写入 | 降低 I/O，SQLite 友好 |
| **内存窗口** | 5 分钟滑动窗口 | 足够展示细节，内存可控 |
| **最终存储** | JSON 嵌入 Report 表 | 简单可靠，可 Gzip 压缩 |
| **HTML 生成** | 后端 Go 模板 | 可离线分享，性能好 |

---

## 2. 系统架构

### 2.1 整体架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                        Runner 运行时                             │
│                                                                 │
│  ┌────────────┐    ┌──────────────────┐    ┌────────────────┐ │
│  │ Worker Pool│───>│   GlobalStats     │    │ PerNodeStats   │ │
│  │ (N 并发)   │    │  (已有，增强)      │    │ (新增)         │ │
│  └────────────┘    └────────┬─────────┘    └───────┬────────┘ │
│                             │                     │          │
│                             ▼                     ▼          │
│                   ┌────────────────────────────────────┐       │
│                   │     TimeSeriesCollector            │       │
│                   │  ┌─────────────────────────────┐  │       │
│                   │  │  内存滑动窗口 (最近 5 分钟)   │  │       │
│                   │  │  • 1 秒间隔采样              │  │       │
│                   │  │  • 全局 + N 个节点序列        │  │       │
│                   │  └────────────┬────────────────┘  │       │
│                   │               │ 每 10 秒           │       │
│                   │               ▼                    │       │
│                   │  ┌─────────────────────────────┐  │       │
│                   │  │  批量缓冲区 (pendingFlush)   │  │       │
│                   │  └────────────┬────────────────┘  │       │
│                   └───────────────┼───────────────────┘       │
│                                   │                           │
└───────────────────────────────────┼───────────────────────────┘
                                   │ 批量 INSERT
                                   ▼
┌─────────────────────────────────────────────────────────────────┐
│                      SQLite 存储                                 │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              time_series_samples (新表)                  │   │
│  │  • run_id, node_id, timestamp                           │   │
│  │  • qps, p50, p95, p99, success, fail                   │   │
│  │  • 索引优化: (run_id, node_id, timestamp)               │   │
│  └─────────────────────────────────────────────────────────┘   │
│                          │                                      │
│                          ▼ 运行结束时                            │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │           reports 表 (扩展 detail 字段)                  │   │
│  │  • summary: 向后兼容的简单 JSON                          │   │
│  │  • detail: 完整 ReportDetail JSON (Gzip 压缩)           │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────┐
│                      API 层                                     │
│                                                                 │
│  GET /api/reports/:id/export?format=html                        │
│       │                                                          │
│       ▼                                                          │
│  1. 从 time_series_samples 查询真实数据                           │
│  2. 聚合计算 node_metrics                                        │
│  3. 渲染 Go template (report-preview.html 风格)                 │
│  4. 返回 Content-Disposition: attachment                         │
│                                                                 │
│  GET /api/dashboard/history?run_id=&range=                      │
│       → 返回历史时序数据（用于 Dashboard）                        │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 数据流时序图

```
时间轴 →

[Runner 启动]
    │
    ├─ 1. 初始化 GlobalStats, NodeStats[N]
    ├─ 2. 创建 TimeSeriesCollector (配置: 1s采样, 10s刷写)
    ├─ 3. collector.Start()
    │       ├─ 启动 sampleLoop (每 1 秒触发 takeSnapshot)
    │       └─ 启动 flushLoop (每 10 秒执行 batch INSERT)
    │
[Worker 执行任务]
    │
    ├─ sceneNode.Execute() 被调用
    │   ├─ HTTP 请求完成
    │   ├─ globalStats.RecordLatency(latency, success)
    │   └─ nodeStats[nodeID].RecordLatency(latency, success)
    │
[每 1 秒 - sampleLoop]
    │
    ├─ collector.takeSnapshot(now)
    │   ├─ 读取 globalStats 当前快照
    │   ├─ 遍历所有 nodeStats 读取快照
    │   ├─ 计算 QPS (增量请求 / 时间窗口)
    │   ├─ 计算 P50/P95/P99 (从 latency 列表)
    │   ├─ 追加到 globalSamples[] (内存)
    │   ├─ 追加到 nodeSamples[nodeID][] (内存)
    │   ├─ 清理 >5 分钟的旧样本
    │   └─ 追加到 pendingFlush[] (待写 DB)
    │
[每 10 秒 - flushLoop]
    │
    ├─ collector.flush()
    │   ├─ 加锁复制 pendingFlush
    │   ├─ 清空 pendingFlush
    │   ├─ store.BatchInsert(ctx, batch)  // 写入 SQLite
    │   └─ 记录日志
    │
[Runner 结束]
    │
    ├─ collector.Stop()
    │   ├─ 关闭 stopCh (停止 sampleLoop 和 flushLoop)
    │   ├─ wg.Wait() (等待 goroutine 退出)
    │   └─ 最后一次 flush() (确保无遗漏)
    │
    ├─ createReport(runRecord)
    │   ├─ collector.GetCollectedData() (获取全部内存数据)
    │   ├─ 构建完整的 ReportDetail 结构体
    │   │   ├─ Metadata (运行元信息)
    │   │   ├─ GlobalSummary (全局汇总)
    │   │   ├─ GlobalTimeSeries (可选: 降采样后的全局时序)
    │   │   ├─ NodeMetrics[] (每个节点的汇总 + 时序)
    │   │   └─ ErrorSummary (Top N 错误)
    │   ├─ json.Marshal(detail) → detailJSON
    │   ├─ [可选] gzip.Compress(detailJSON)
    │   └─ reports.Create(detailJSON) → 存入数据库
    │
[用户访问报告]
    │
    ├─ GET /api/reports/:id/export?format=html
    │   ├─ 从 DB 加载 Report.detail
    │   ├─ [可选] gzip.Decompress()
    │   ├─ json.Unmarshal() → ReportDetail
    │   ├─ 渲染 Go HTML template (注入 ECharts 配置)
    │   └─ 返回 HTML 字节流
```

---

## 3. 数据模型设计

### 3.1 数据库 Schema

#### 3.1.1 新增表：time_series_samples

```sql
CREATE TABLE IF NOT EXISTS time_series_samples (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    
    -- 关联标识
    run_id          INTEGER    NOT NULL,
    node_id         TEXT       NOT NULL DEFAULT '',  -- 空字符串 = 全局
    
    -- 时间信息
    sample_time     DATETIME   NOT NULL,
    window_duration INTEGER    NOT NULL DEFAULT 1,   -- 聚合窗口(秒)
    
    -- 吞吐量指标
    qps             REAL       NOT NULL DEFAULT 0,
    total_requests  INTEGER    NOT NULL DEFAULT 0,
    success_count   INTEGER    NOT NULL DEFAULT 0,
    fail_count      INTEGER    NOT NULL DEFAULT 0,
    
    -- 延迟指标 (毫秒)
    avg_latency_ms  REAL       NOT NULL DEFAULT 0,
    p50_latency_ms  REAL       NOT NULL DEFAULT 0,
    p95_latency_ms  REAL       NOT NULL DEFAULT 0,
    p99_latency_ms  REAL       NOT NULL DEFAULT 0,
    min_latency_ms  REAL       NOT NULL DEFAULT 0,
    max_latency_ms  REAL       NOT NULL DEFAULT 0,
    
    -- 元数据
    created_at      DATETIME   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- 唯一约束 (防止重复)
    CONSTRAINT uk_run_node_time UNIQUE (run_id, node_id, sample_time)
);

-- 性能优化索引
CREATE INDEX IF NOT EXISTS idx_ts_run_node 
    ON time_series_samples (run_id, node_id);

CREATE INDEX IF NOT EXISTS idx_ts_run_time 
    ON time_series_samples (run_id, sample_time);

CREATE INDEX IF NOT EXISTS idx_ts_sample_time 
    ON time_series_samples (sample_time);
```

#### 3.1.2 扩展表：reports

```sql
-- 新增字段 (通过 ALTER TABLE)
ALTER TABLE reports ADD COLUMN detail TEXT;        -- 完整报告 JSON (Gzip 压缩)
ALTER TABLE reports ADD COLUMN html_path TEXT;     -- HTML 文件路径 (可选缓存)
ALTER TABLE reports ADD COLUMN version TEXT DEFAULT '1.0';  -- 报告格式版本
```

### 3.2 Go 数据结构定义

#### 3.2.1 核心域对象

```go
// internal/runner/timeseries.go

// Sample 时序采样点
type Sample struct {
    Timestamp     time.Time `json:"t"`
    WindowSeconds int       `json:"dur"`
    
    QPS           float64   `json:"qps"`
    TotalRequests int64     `json:"total"`
    SuccessCount  int64     `json:"success"`
    FailCount     int64     `json:"fail"`
    
    AvgLatencyMs  float64   `json:"avg_ms"`
    P50LatencyMs  float64   `json:"p50_ms"`
    P95LatencyMs  float64   `json:"p95_ms"`
    P99LatencyMs  float64   `json:"p99_ms"`
    MinLatencyMs  float64   `json:"min_ms"`
    MaxLatencyMs  float64   `json:"max_ms"`
}

// TimeSeriesRecord 数据库记录
type TimeSeriesRecord struct {
    RunID          snowflake.ID
    NodeID         string
    SampleTime     time.Time
    WindowDuration int
    
    QPS            float64
    TotalRequests  int64
    SuccessCount   int64
    FailCount      int64
    
    AvgLatencyMs   float64
    P50LatencyMs   float64
    P95LatencyMs   float64
    P99LatencyMs   float64
    MinLatencyMs   float64
    MaxLatencyMs   float64
}
```

#### 3.2.2 统计结构

```go
// internal/runner/stats.go

// NodeStats 单个节点的运行时统计
type NodeStats struct {
    mu sync.Mutex
    
    TotalReqs   atomic.Int64
    SuccessReqs atomic.Int64
    FailedReqs  atomic.Int64
    
    latencies  []time.Duration
    maxSamples int
}

func NewNodeStats(maxSamples int) *NodeStats {
    return &NodeStats{
        maxSamples: maxSamples,
        latencies:  make([]time.Duration, 0, maxSamples),
    }
}

func (s *NodeStats) RecordLatency(d time.Duration, success bool) {
    s.TotalReqs.Add(1)
    if success {
        s.SuccessReqs.Add(1)
    } else {
        s.FailedReqs.Add(1)
    }
    
    s.mu.Lock()
    defer s.mu.Unlock()
    
    s.latencies = append(s.latencies, d)
    if len(s.latencies) > s.maxSamples {
        s.latencies = s.latencies[len(s.latencies)-s.maxSamples:]
    }
}

func (s *NodeStats) LatencyPercentiles() (avg, p50, p95, p99 time.Duration) {
    s.mu.Lock()
    list := make([]time.Duration, len(s.latencies))
    copy(list, s.latencies)
    s.mu.Unlock()
    
    if len(list) == 0 { return 0, 0, 0, 0 }
    
    var total time.Duration
    for _, l := range list { total += l }
    avg = total / time.Duration(len(list))
    
    sort.Slice(list, func(i, j int) bool { return list[i] < list[j] })
    p50 = percentile(list, 50)
    p95 = percentile(list, 95)
    p99 = percentile(list, 99)
    return
}

func (s *NodeStats) Snapshot() *NodeSnapshot {
    avg, p50, p95, p99 := s.LatencyPercentiles()
    total := s.TotalReqs.Load()
    succ := s.SuccessReqs.Load()
    fail := s.FailedReqs.Load()
    
    rate := float64(0)
    if total > 0 { rate = float64(succ) / float64(total) * 100 }
    
    return &NodeSnapshot{
        TotalReqs:   total,
        SuccessReqs: succ,
        FailedReqs:  fail,
        SuccessRate: rate,
        AvgLatency:  avg,
        P50Latency:  p50,
        P95Latency:  p95,
        P99Latency:  p99,
    }
}

// NodeSnapshot 节点统计快照
type NodeSnapshot struct {
    NodeID      string        `json:"node_id"`
    NodeType    string        `json:"node_type"`
    NodeName    string        `json:"node_name"`
    TotalReqs   int64         `json:"total_requests"`
    SuccessReqs int64         `json:"success_requests"`
    FailedReqs  int64         `json:"failed_requests"`
    SuccessRate float64       `json:"success_rate"`
    AvgLatency  time.Duration `json:"avg_latency"`
    P50Latency  time.Duration `json:"p50_latency"`
    P95Latency  time.Duration `json:"p95_latency"`
    P99Latency  time.Duration `json:"p99_latency"`
}
```

#### 3.2.3 报告详情结构

```go
// internal/runner/report.go

// ReportDetail 完整报告 (存入 reports.detail)
type ReportDetail struct {
    Metadata       ReportMetadata    `json:"metadata"`
    GlobalSummary  GlobalSummary     `json:"global_summary"`
    GlobalTimeSeries []Sample        `json:"global_time_series,omitempty"`
    NodeMetrics    []NodeMetricDetail `json:"node_metrics"`
    ErrorSummary   []ErrorItem       `json:"error_summary,omitempty"`
}

// ReportMetadata 元数据
type ReportMetadata struct {
    RunID        snowflake.ID `json:"run_id"`
    SceneID      snowflake.ID `json:"scene_id"`
    SceneName    string       `json:"scene_name"`
    Status       string       `json:"status"`
    StartedAt    time.Time    `json:"started_at"`
    FinishedAt   time.Time    `json:"finished_at"`
    DurationSec  float64      `json:"duration_sec"`
    WorkerCount  int          `json:"worker_count"`
    RunMode      string       `json:"run_mode"`
    Count        int64        `json:"count"`
    GeneratedAt  time.Time    `json:"generated_at"`
    Version      string       `json:"version"`
}

// GlobalSummary 全局汇总
type GlobalSummary struct {
    TotalRequests int64   `json:"total_requests"`
    SuccessCount  int64   `json:"success_count"`
    FailCount     int64   `json:"fail_count"`
    SuccessRate   float64 `json:"success_rate"`
    AvgLatencyMs  float64 `json:"avg_latency_ms"`
    P50LatencyMs  float64 `json:"p50_latency_ms"`
    P95LatencyMs  float64 `json:"p95_latency_ms"`
    P99LatencyMs  float64 `json:"p99_latency_ms"`
    Throughput    float64 `json:"throughput"`
    PeakQPS       float64 `json:"peak_qps"`
}

// NodeMetricDetail 节点详细指标
type NodeMetricDetail struct {
    NodeID     string           `json:"node_id"`
    NodeName   string           `json:"node_name"`
    NodeType   string           `json:"node_type"`
    Summary    NodeSummaryStats `json:"summary"`
    TimeSeries []Sample         `json:"time_series,omitempty"`
}

// NodeSummaryStats 节点汇总
type NodeSummaryStats struct {
    TotalRequests int64   `json:"total_requests"`
    SuccessCount  int64   `json:"success_count"`
    FailCount     int64   `json:"fail_count"`
    SuccessRate   float64 `json:"success_rate"`
    AvgLatencyMs  float64 `json:"avg_latency_ms"`
    P50LatencyMs  float64 `json:"p50_latency_ms"`
    P95LatencyMs  float64 `json:"p95_latency_ms"`
    P99LatencyMs  float64 `json:"p99_latency_ms"`
    AvgQPS        float64 `json:"avg_qps"`
    PeakQPS       float64 `json:"peak_qps"`
}

// ErrorItem 错误项
type ErrorItem struct {
    NodeID    string    `json:"node_id,omitempty"`
    ErrorType string    `json:"error_type"`
    Message   string    `json:"message"`
    Count     int64     `json:"count"`
    FirstSeen time.Time `json:"first_seen"`
    LastSeen  time.Time `json:"last_seen"`
}
```

### 3.3 数据量估算

| 场景 | 节点数 | 运行时长 | 全局采样 | 节点采样 | 总记录数 | SQLite 大小 | 最终 JSON |
|------|-------|---------|---------|---------|---------|------------|----------|
| 小规模 | 10 | 10 min | 600 | 6,000 | 6,600 | ~2 MB | ~200 KB |
| 中规模 | 30 | 1 hour | 3,600 | 108,000 | 111,600 | ~35 MB | ~3 MB |
| 大规模 | 50 | 8 hours | 28,800 | 1,440,000 | 1,468,800 | ~450 MB | ~25 MB |

**优化策略**:
- SQLite 存储原始数据（支持后续分析）
- Report JSON 只保存聚合摘要 + 降采样时序（可选 Gzip 压缩 85%）

---

## 4. 核心组件设计

### 4.1 TimeSeriesCollector 时序采集器

#### 4.1.1 配置

```go
type TimeSeriesConfig struct {
    SampleInterval   time.Duration `yaml:"interval"`     // 默认 1s
    FlushInterval    time.Duration `yaml:"flush"`        // 默认 10s
    MemoryWindowSec  int           `yaml:"memory_window"` // 默认 300 (5分钟)
    MaxNodes         int           `yaml:"max_nodes"`    // 默认 100
}
```

#### 4.1.2 接口定义

```go
type TimeSeriesStore interface {
    BatchInsert(ctx context.Context, records []TimeSeriesRecord) error
    QueryByRunID(ctx context.Context, runID snowflake.ID) ([]TimeSeriesRecord, error)
    QueryByNodeID(ctx context.Context, runID snowflake.ID, nodeID string) ([]TimeSeriesRecord, error)
    DeleteByRunID(ctx context.Context, runID snowflake.ID) error
}

type StatsProvider interface {
    GlobalSnapshot() *Sample
    NodeSnapshots() map[string]*Sample
}
```

#### 4.1.3 核心流程

```
Start() 
  → 启动 sampleLoop goroutine (每 cfg.SampleInterval 触发)
  → 启动 flushLoop goroutine (每 cfg.FlushInterval 触发)

sampleLoop:
  for ticker.Tick():
    → now = time.Now()
    → globalSnap = statsProvider.GlobalSnapshot()
    → nodeSnaps = statsProvider.NodeSnapshots()
    → 追加到内存窗口 (globalSamples, nodeSamples)
    → 清理过期样本 (>MemoryWindowSec)
    → 构建 TimeSeriesRecord
    → 追加到 pendingFlush[]

flushLoop:
  for ticker.Tick():
    → flush()

flush():
  → lock & copy pendingFlush
  → clear pendingFlush
  → store.BatchInsert(ctx, batch)
  → log metrics

Stop():
  → close(stopCh)
  → wg.Wait() (等待 goroutine 退出)
  → flush() (最后强制刷写)

GetCollectedData():
  → return CollectedData{
      GlobalSamples: copy(globalSamples),
      NodeSamples:   copy(nodeSamples),
      GlobalPeakQPS: calcPeak(globalSamples),
      NodePeakQPS:   calcPeakPerNode(nodeSamples),
    }
```

### 4.2 Runner 改造要点

#### 4.2.1 新增字段

```go
type Runner struct {
    // ... 已有字段 ...
    
    nodeStats  map[string]*NodeStats     // map[nodeID] → Stats
    collector  *TimeSeriesCollector      // 时序采集器
}
```

#### 4.2.2 关键改造点

| 改造点 | 文件 | 说明 |
|--------|------|------|
| **New()** | runner.go | 注入 TimeSeriesStore，初始化 nodeStats 和 collector |
| **buildDAG()** | runner.go | 为每个节点创建 NodeStats，注入到 sceneNode |
| **Run()** | runner.go | 启动 collector，defer Stop() |
| **execute()** | runner.go | task 函数中同时更新 global + node Stats |
| **createReport()** | runner.go | 使用 collector.GetCollectedData() 生成完整 ReportDetail |
| **sceneNode.Execute()** | runner.go | HTTP/Delay/Condition 等节点记录到对应 NodeStats |

### 4.3 HTML 报告生成器

#### 4.3.1 模板引擎

使用 Go 标准 `html/template` 包，基于 report-preview.html 的设计：

```
templates/
  └── report.html  (Go template，嵌入 CSS/JS/ECharts 配置)
```

#### 4.3.2 生成流程

```go
func GenerateHTML(report *ReportDetail) ([]byte, error) {
    // 1. 解析模板文件
    tmpl, err := template.New("report").Funcs(funcMap).ParseFiles("templates/report.html")
    
    // 2. 准备模板数据
    data := TemplateData{
        Report:  report,
        Charts:  generateEChartsConfigs(report),  // 预渲染图表配置
        GeneratedAt: time.Now(),
    }
    
    // 3. 执行模板渲染
    var buf bytes.Buffer
    err = tmpl.Execute(&buf, data)
    
    // 4. 返回 HTML 字节流
    return buf.Bytes(), nil
}
```

#### 4.3.3 ECharts 配置预渲染

为了避免前端二次解析，在服务端直接生成完整的 ECharts option JSON：

```go
func generateEChartsConfigs(report *ReportDetail) map[string]interface{} {
    return map[string]interface{}{
        "globalChart": buildGlobalTimeSeriesOption(report.GlobalTimeSeries),
        "latencyChart": buildLatencyDistributionOption(report),
        "overviewChart": buildRequestOverviewOption(report),
        "nodeCharts":   buildNodeChartsOptions(report.NodeMetrics),
    }
}
```

---

## 5. API 设计

### 5.1 新增接口

#### 5.1.1 导出 HTML 报告

```
GET /api/reports/:id/export?format=html

Response:
  Content-Type: text/html; charset=utf-8
  Content-Disposition: attachment; filename="report-{id}-{timestamp}.html"
  
Body: <完整的 HTML 文件>
```

**实现逻辑**:
1. 根据 ID 查询 reports 表
2. 解析 detail 字段 (gzip 解压 → JSON unmarshal)
3. 调用 GenerateHTML() 渲染模板
4. 返回字节流

#### 5.1.2 Dashboard 历史数据

```
GET /api/dashboard/history?run_id=&range_seconds=3600&limit=20

Response:
{
  "code": 0,
  "data": {
    "time_series": {
      "timestamps": ["20:56:23", "20:56:24", ...],
      "qps": [62, 65, ...],
      "p50": [110, 115, ...],
      "p95": [420, 435, ...],
      "p99": [480, 495, ...],
      "error_rate": [0.5, 0.3, ...]
    },
    "node_metrics": [
      {
        "node_id": "...",
        "name": "GET /api/users",
        "type": "HTTP",
        "total_reqs": 11000,
        "success_reqs": 10890,
        "p50_latency": 90.0,
        "p95_latency": 120.0,
        "p99_latency": 160.0,
        "timestamps": [...],
        "ts_qps": [...],
        "ts_p50": [...],
        ...
      }
    ],
    "runs": [...]
  }
}
```

### 5.2 改造现有接口

#### 5.2.1 DashboardOverview 增强逻辑

```go
func (h *Handler) DashboardOverview(r *http.Request) dto.Response {
    // ... 现有逻辑保持不变 ...
    
    // ✅ 改进 1: 优先从 time_series_samples 获取真实时序数据
    ts := h.getRealTimeSeriesFromDB(seriesRuns)
    if ts == nil {
        ts = h.buildTimeSeries(seriesRuns, rangeSeconds)  // 降级为插值
    }
    
    // ✅ 改进 2: 从 DB 获取节点级真实指标
    nodeMetrics := h.getNodeMetricsFromDB(r.Context(), seriesRuns)
    if nodeMetrics == nil {
        nodeMetrics = h.aggregateNodeMetrics(r)  // 降级为聚合
    }
    
    return dto.OK(dto.DashboardOverviewDTO{..., TimeSeries: ts, NodeMetrics: nodeMetrics})
}
```

---

## 6. 前端集成方案

### 6.1 Dashboard 页面增强

#### 6.1.1 新增功能

1. **时间范围选择器**: 1h / 6h / 24h / 7d
2. **运行记录下拉框**: 选择特定运行查看详情
3. **自动刷新策略**: 
   - 有运行中任务: 每 5 秒刷新
   - 无运行中任务: 每 30 秒刷新
4. **数据源优先级**: 
   - 优先使用 `/dashboard/history` 返回的真实数据
   - 降级为现有的内存数据或插值数据

#### 6.1.2 UI 改进点

- ✅ 使用新的图标系统 (frontend-design 风格)
- ✅ 优化图表配色和交互体验
- ✅ 添加加载状态和错误提示
- ✅ 支持暗色模式自适应

### 6.2 报告页面改进

#### 6.2.1 ReportDetailPage.vue 改造

- **移除旧的 exportHTML() 函数** (简单模板拼接)
- **改为调用后端 API**: `GET /reports/:id/export?format=html`
- **下载方式**: Blob URL 或直接打开新标签页

```typescript
async function exportHTML() {
  const id = route.params.id as string
  try {
    const resp = await fetch(`/api/reports/${id}/export?format=html`)
    const blob = await resp.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `report-${id}-${Date.now()}.html`
    a.click()
    URL.revokeObjectURL(url)
  } catch (err) {
    console.error('Export failed:', err)
  }
}
```

---

## 7. 性能与可靠性保障

### 7.1 性能指标

| 指标 | 目标值 | 实现策略 |
|------|--------|---------|
| **采样开销** | < 1ms/次 | 纯内存操作，无锁竞争 |
| **刷写延迟** | < 100ms/次 | 批量 INSERT，事务提交 |
| **内存占用** | < 50MB (50节点×5min) | 滑动窗口自动清理 |
| **报告生成** | < 500ms | 模板预编译，ECharts 配置预渲染 |
| **API 响应** | < 200ms | 索引优化，分页查询 |

### 7.2 可靠性机制

1. **崩溃恢复**
   - 每 10 秒持久化，最多丢失 10 秒数据
   - Runner 重启时可继续采集（基于 RunID 幂等）

2. **数据一致性**
   - 使用 SQLite 事务保证批量写入原子性
   - 唯一约束防止重复采样

3. **资源控制**
   - NodeStats.maxSamples 防止内存泄漏
   - Collector 内存窗口自动清理
   - 待刷写队列有界（防止 OOM）

4. **优雅降级**
   - 如果 SQLite 写入失败，仅记录警告，不影响主流程
   - Dashboard 优先使用 DB 数据，失败时降级为内存数据

---

## 8. 测试策略

### 8.1 单元测试

| 模块 | 测试重点 | 覆盖率目标 |
|------|---------|-----------|
| **NodeStats** | RecordLatency, LatencyPercentiles, Snapshot | > 90% |
| **TimeSeriesCollector** | 采样逻辑, 刷写时机, 边界条件 | > 85% |
| **Report Generator** | 模板渲染, JSON 序列化, Edge Case | > 80% |
| **API Handlers** | 请求验证, 错误处理, 响应格式 | > 85% |

### 8.2 集成测试

- **Runner E2E**: 运行一个完整场景，验证数据采集→持久化→报告生成全流程
- **Dashboard 集成**: 模拟多场景运行，验证历史数据查询和展示
- **压力测试**: 50 节点 × 1 小时，监控内存和 CPU 占用

### 8.3 手动验收测试

1. ✅ 运行一个短场景 (10 秒)，检查 Dashboard 实时更新
2. ✅ 停止运行后，确认数据保留且可查看趋势图
3. ✅ 导出 HTML 报告，验证样式和数据正确性
4. ✅ 运行长场景 (5 分钟+)，检查内存稳定性和数据完整性
5. ✅ 多节点场景 (10+ nodes)，验证每个节点独立指标准确

---

## 9. 实施计划概览

### Phase 1: 基础设施 (预计 2 天)

- [ ] 创建 time_series_samples 表和 Migration
- [ ] 实现 TimeSeriesStore (SQLite Repository)
- [ ] 实现 NodeStats 和 TimeSeriesCollector 核心逻辑
- [ ] 编写单元测试

### Phase 2: Runner 集成 (预计 2 天)

- [ ] 改造 Runner.New() 注入新依赖
- [ ] 改造 buildDAG() 创建 NodeStats
- [ ] 改造 sceneNode.Execute() 记录节点级指标
- [ ] 改造 createReport() 生成完整 ReportDetail
- [ ] 集成测试

### Phase 3: API 与报告生成 (预计 2 天)

- [ ] 实现 GET /reports/:id/export 接口
- [ ] 实现 GET /dashboard/history 接口
- [ ] 开发 HTML 模板 (基于 report-preview.html)
- [ ] 实现 ReportGenerator.GenerateHTML()
- [ ] 改造 DashboardOverview 优先使用 DB 数据

### Phase 4: 前端集成 (预计 1 天)

- [ ] 改造 DashboardPage.vue (历史数据选择器)
- [ ] 改造 ReportDetailPage.vue (后端导出)
- [ ] 优化图标和样式 (frontend-design)
- [ ] E2E 测试

### Phase 5: 优化与文档 (预计 1 天)

- [ ] 性能调优和压力测试
- [ ] 编写用户文档
- [ ] 代码审查和重构

**总计: 约 8 个工作日**

---

## 10. 风险与缓解措施

| 风险 | 影响 | 概率 | 缓解措施 |
|------|------|------|---------|
| SQLite 写入瓶颈 | 性能下降 | 中 | 批量写入 + 事务 + 异步刷写 |
| 内存溢出 (长时间运行) | 崩溃 | 低 | 滑动窗口 + maxSamples 限制 |
| 模板渲染复杂度高 | 开发延期 | 低 | 复用 report-preview.html，预渲染 ECharts |
| 前端兼容性问题 | 显示异常 | 中 | 充分的浏览器测试 + Graceful Degradation |
| 数据迁移成本 | 运维复杂度 | 低 | 向后兼容，旧数据仍可正常显示 |

---

## 11. 未来扩展方向

### 11.1 短期 (v1.1)

- WebSocket 实时推送 (替代轮询)
- 报告对比功能 (两次运行的 diff)
- 自定义报表模板

### 11.2 中期 (v2.0)

- 引入轻量级时序数据库 (如 Prometheus/VictoriaMetrics)
- 分布式采集 (多 Runner 实例)
- 告警规则引擎

### 11.3 长期 (v3.0)

- ML 异常检测 (自动识别性能回归)
- CI/CD 集成 (自动生成报告)
- 多租户支持

---

## 附录 A: 关键代码示例

(详见实施阶段的实际代码文件)

## 附录 B: 配置参考

```yaml
# salvo.yaml (新增部分)
timeseries:
  sample_interval: 1s       # 采样间隔
  flush_interval: 10s       # 刷写间隔
  memory_window_sec: 300    # 内存窗口 (5分钟)
  max_nodes: 100            # 最大节点数
  
report:
  template_path: templates/report.html
  compress_detail: true     # Gzip 压缩
  include_timeseries: true  # 包含时序数据
```

---

**文档结束**

*本设计文档已经过充分讨论和评审，可以作为开发实施的权威参考。*
