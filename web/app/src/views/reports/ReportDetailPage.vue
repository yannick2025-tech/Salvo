<template>
  <div class="report-detail">
    <div class="page-header">
      <button class="btn-back" @click="$router.push('/reports')">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M19 12H5M12 19l-7-7 7-7"/></svg>
        返回
      </button>
      <h2>测试报告</h2>
      <button class="btn-login-primary" @click="exportHTML" :disabled="!report">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
        导出 HTML
      </button>
    </div>

    <div v-if="report && metrics">

      <!-- Metrics Row: expanded with Peak QPS / Throughput / TTFB / Min -->
      <div class="metrics-row">
        <div class="metric-card">
          <div class="metric-label tooltip-wrapper" data-tooltip="成功请求数占总请求数的百分比">成功率</div>
          <div class="metric-value" :style="{ color: successRateColor }">{{ formatSuccessRate(metrics.success_rate) }}</div>
          <div class="metric-sub">{{ metrics.success_reqs }} / {{ metrics.total_reqs }} 次请求</div>
        </div>

        <div class="metric-card">
          <div class="metric-label tooltip-wrapper" data-tooltip="测试期间发送的 HTTP 请求总数">总请求数</div>
          <div class="metric-value" style="color: var(--accent-primary)">{{ Number(metrics.total_reqs || 0).toLocaleString() }}</div>
          <div class="metric-sub">{{ metrics.duration_s ? Number(metrics.duration_s).toFixed(3) + 's' : '-' }} 持续时间</div>
        </div>

        <div class="metric-card">
          <div class="metric-label tooltip-wrapper" data-tooltip="测试期间达到的最大每秒查询数">峰值QPS</div>
          <div class="metric-value" style="color: var(--accent-info)">{{ fmtPeakQPS }}</div>
          <div class="metric-sub">{{ calcQPS }} 平均QPS</div>
        </div>

        <div class="metric-card">
          <div class="metric-label tooltip-wrapper" data-tooltip="所有 worker 平均每秒处理的请求数">吞吐量</div>
          <div class="metric-value" style="color: var(--accent-success)">{{ fmtThroughput }}</div>
          <div class="metric-sub">{{ metrics.worker_count || '-' }} 工作线程</div>
        </div>

        <div class="metric-card">
          <div class="metric-label tooltip-wrapper" data-tooltip="所有请求的平均响应时间（算术平均值）">平均延迟</div>
          <div class="metric-value" style="color: var(--accent-info)">{{ fmtLatency3(metrics.avg_latency_s) }}</div>
          <div class="metric-sub">P50 {{ fmtLatency3(metrics.p50_latency_s) }}</div>
        </div>

        <div class="metric-card">
          <div class="metric-label tooltip-wrapper" data-tooltip="第90百分位延迟 - 90% 的请求在此时间内完成">P90延迟</div>
          <div class="metric-value" style="color: var(--accent-warning)">{{ fmtLatency3(metrics.p90_latency_s) }}</div>
          <div class="metric-sub">P95 {{ fmtLatency3(metrics.p95_latency_s) }}</div>
        </div>

        <div class="metric-card">
          <div class="metric-label tooltip-wrapper" data-tooltip="首字节时间 - 等待第一个响应字节的平均时间">首字节时间</div>
          <div class="metric-value" style="color: var(--accent-primary)">{{ fmtTTFB }}</div>
          <div class="metric-sub">最小值 {{ fmtMinLatency }}</div>
        </div>

        <div class="metric-card">
          <div class="metric-label tooltip-wrapper" data-tooltip="第99百分位延迟 - 99% 的请求在此时间内完成（最坏情况）">P99延迟</div>
          <div class="metric-value" style="color: var(--accent-danger)">{{ fmtLatency3(metrics.p99_latency_s) }}</div>
          <div class="metric-sub">{{ fmtErrorRate }} 错误率</div>
        </div>
      </div>

      <!-- Charts Row: Request Distribution + Error Rate Trend -->
      <div class="charts-row">
        <div class="chart-card">
          <div class="chart-header">
            <h3>请求分布</h3>
            <div class="chart-tip">成功/失败分布</div>
          </div>
          <div class="chart-body" ref="overviewChartRef"></div>
        </div>

        <div class="chart-card">
          <div class="chart-header">
            <h3>错误率趋势</h3>
            <div class="chart-tip">每区间失败/总数</div>
          </div>
          <div class="chart-body" ref="errorRateChartRef"></div>
          <div class="chart-type-toggle center">
            <button :class="['type-btn', { active: chartTypes.errorRate === 'smooth' }]" @click="switchChartType('errorRate', 'smooth')">平滑</button>
            <button :class="['type-btn', { active: chartTypes.errorRate === 'step' }]" @click="switchChartType('errorRate', 'step')">阶梯</button>
          </div>
        </div>
      </div>

      <!-- Error Breakdown by Reason -->
      <div class="charts-row" v-if="errorBreakdown.length > 0">
        <div class="chart-card wide">
          <div class="chart-header">
            <h3>错误明细</h3>
            <div class="chart-tip">按HTTP状态码分组</div>
          </div>
          <div class="chart-body" ref="errorBreakdownChartRef"></div>
        </div>
      </div>

      <!-- Latency Percentiles Bar Chart -->
      <div class="charts-row">
        <div class="chart-card wide">
          <div class="chart-header">
            <h3>延迟百分位</h3>
            <div class="chart-tip">均值/P50/P90/P95/P99 (ms)</div>
          </div>
          <div class="chart-body" ref="latencyChartRef"></div>
        </div>
      </div>

      <!-- Run Info & Performance Summary -->
      <div class="info-section">
        <div class="info-card">
          <h3>运行时配置</h3>
          <table class="info-table">
            <tbody>
            <tr><td class="info-label">运行模式</td><td><span class="mode-tag">{{ getRunModeLabel() }}</span></td></tr>
            <tr v-if="metrics.run_mode === 'duration'"><td class="info-label">计划持续时间</td><td>{{ getPlannedDuration() }}s</td></tr>
            <tr v-if="metrics.run_mode === 'count'"><td class="info-label">计划请求数</td><td>{{ getPlannedCount() }} 次</td></tr>
            <tr><td class="info-label">实际{{ metrics.run_mode === 'duration' ? '持续时间' : '请求数' }}</td><td class="actual-val">{{ metrics.run_mode === 'duration' ? (Number(metrics.duration_s) || 0).toFixed(3) + 's' : (Number(metrics.total_reqs) || 0).toLocaleString() + ' 次' }}</td></tr>
            <tr><td class="info-label">并发数</td><td>{{ metrics.worker_count || '-' }}</td></tr>
            <tr><td class="info-label">状态</td><td>
              <span :class="['status-badge', report.status]" class="tooltip-wrapper" :data-tooltip="getStatusTooltip(report.status)">{{ getStatusLabel(report.status) }}</span>
            </td></tr>
            <tr><td class="info-label">时间范围</td><td class="mono-sm">{{ formatTime(report.started_at) }} ~ {{ formatTime(report.finished_at) }}</td></tr>
            </tbody>
          </table>
        </div>

        <div class="info-card">
          <h3>性能概览</h3>
          <div class="perf-list">
            <div class="perf-row"><span class="perf-k tooltip-wrapper" data-tooltip="所有请求的平均延迟（算术平均值）">均值</span><span class="perf-v">{{ fmtLatency3(metrics.avg_latency_s) }}</span></div>
            <div class="perf-row"><span class="perf-k tooltip-wrapper" data-tooltip="第50百分位 - 中位数延迟，50% 的请求快于此值">P50</span><span class="perf-v">{{ fmtLatency3(metrics.p50_latency_s) }}</span></div>
            <div class="perf-row"><span class="perf-k tooltip-wrapper" data-tooltip="第90百分位 - 90% 的请求在此时间内完成">P90</span><span class="perf-v">{{ fmtLatency3(metrics.p90_latency_s) }}</span></div>
            <div class="perf-row"><span class="perf-k tooltip-wrapper" data-tooltip="第95百分位 - 95% 的请求在此时间内完成">P95</span><span class="perf-v">{{ fmtLatency3(metrics.p95_latency_s) }}</span></div>
            <div class="perf-row"><span class="perf-k tooltip-wrapper" data-tooltip="第99百分位 - 99% 的请求完成（最坏情况尾部延迟）">P99</span><span class="perf-v">{{ fmtLatency3(metrics.p99_latency_s) }}</span></div>
            <div class="perf-row"><span class="perf-k tooltip-wrapper" data-tooltip="观察到的最小响应时间（最快请求）">最小值</span><span class="perf-v">{{ fmtMinLatency }}</span></div>
            <div class="perf-divider"></div>
            <div class="perf-row"><span class="perf-k tooltip-wrapper" data-tooltip="首字节时间 - 等待第一个响应字节的平均时间">TTFB</span><span class="perf-v">{{ fmtTTFB }}</span></div>
            <div class="perf-row"><span class="perf-k tooltip-wrapper" data-tooltip="测试期间达到的最大每秒查询数">峰值QPS</span><span class="perf-v highlight-blue">{{ fmtPeakQPS }}</span></div>
            <div class="perf-row"><span class="perf-k tooltip-wrapper" data-tooltip="所有 worker 平均每秒处理的请求数">吞吐量</span><span class="perf-v success">{{ fmtThroughput }}</span></div>
            <div class="perf-divider"></div>
            <div class="perf-row"><span class="perf-k tooltip-wrapper" data-tooltip="成功的 HTTP 请求总数（2xx/3xx 状态码）">成功数</span><span class="perf-v success">{{ Number(metrics.success_reqs || 0).toLocaleString() }}</span></div>
            <div class="perf-row"><span class="perf-k tooltip-wrapper" data-tooltip="失败的请求总数（4xx/5xx/错误/超时）">失败数</span><span class="perf-v danger">{{ Number(metrics.failed_reqs || 0).toLocaleString() }}</span></div>
          </div>
        </div>
      </div>

      <!-- Trend Charts Section -->
      <section class="charts-section">
        <div class="charts-toolbar">
          <h3>趋势分析</h3>
        </div>

        <div class="charts-row">
          <div class="chart-card wide">
            <div class="chart-header">
              <h3>QPS趋势</h3>
            </div>
            <div class="chart-body" ref="qpsChartRef"></div>
            <div class="chart-type-toggle center">
              <button :class="['type-btn', { active: chartTypes.qpsTrend === 'smooth' }]" @click="switchChartType('qpsTrend', 'smooth')">平滑</button>
              <button :class="['type-btn', { active: chartTypes.qpsTrend === 'step' }]" @click="switchChartType('qpsTrend', 'step')">阶梯</button>
            </div>
          </div>
        </div>

        <div class="charts-row">
          <div class="chart-card wide">
            <div class="chart-header">
              <h3>延迟趋势</h3>
              <div class="chart-tip">P50 / P90 / P95 / P99</div>
            </div>
            <div class="chart-body" ref="latencyTrendChartRef"></div>
            <div class="chart-type-toggle center">
              <button :class="['type-btn', { active: chartTypes.latTrend === 'smooth' }]" @click="switchChartType('latTrend', 'smooth')">平滑</button>
              <button :class="['type-btn', { active: chartTypes.latTrend === 'step' }]" @click="switchChartType('latTrend', 'step')">阶梯</button>
            </div>
          </div>
        </div>
      </section>

      <!-- Node Ranking Table -->
      <section v-if="nodeTimeSeries && nodeTimeSeries.length > 0" class="nodes-section">
        <h3>节点排名</h3>
        <div class="ranking-table-wrap">
          <table class="ranking-table">
            <thead>
              <tr>
                <th>排名</th>
                <th>节点</th>
                <th>总请求</th>
                <th>成功</th>
                <th>失败</th>
                <th>成功率</th>
                <th>P50</th>
                <th>P90</th>
                <th>P95</th>
                <th>平均QPS</th>
                <th>峰值QPS</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(node, idx) in rankedNodes" :key="node.node_id">
                <td class="rank-cell">{{ idx + 1 }}</td>
                <td class="node-name-cell">{{ node.name || node.node_id }}</td>
                <td>{{ Number(node.summary?.total_requests || 0).toLocaleString() }}</td>
                <td class="success-text">{{ Number(node.summary?.success_count || 0).toLocaleString() }}</td>
                <td class="danger-text">{{ Number(node.summary?.fail_count || 0).toLocaleString() }}</td>
                <td>{{ fmtPercent(node.summary?.success_rate) }}</td>
                <td>{{ fmtLatencyMs(node.summary?.p50_latency_ms) }}</td>
                <td>{{ fmtLatencyMs(node.summary?.p90_latency_ms) }}</td>
                <td>{{ fmtLatencyMs(node.summary?.p95_latency_ms) }}</td>
                <td>{{ (node.summary?.avg_qps || 0).toFixed(3) }}</td>
                <td>{{ (node.summary?.peak_qps || 0).toFixed(3) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <!-- Node Details with charts -->
      <section v-if="nodeTimeSeries && nodeTimeSeries.length > 0" class="nodes-section">
        <h3>节点详情</h3>
        <div v-for="(node, idx) in nodeTimeSeries" :key="node.node_id" class="chart-card">
          <div class="chart-header">
            <h3>{{ node.name || node.node_id }}</h3>
            <div class="node-badges" v-if="node.summary">
              <span class="node-badge">QPS {{ (node.summary.avg_qps || 0).toFixed(3) }}</span>
              <span class="node-badge">P50 {{ fmtLatencyMs(node.summary.p50_latency_ms) }}</span>
              <span class="node-badge">P90 {{ fmtLatencyMs(node.summary.p90_latency_ms) }}</span>
              <span class="node-badge">P95 {{ fmtLatencyMs(node.summary.p95_latency_ms) }}</span>
              <span class="node-badge">TTFB {{ fmtLatencyMs(node.summary.ttfb_ms) }}</span>
            </div>
          </div>
          <div :ref="el => setNodeChartRef(idx, el as HTMLElement)" class="chart-body node-chart-body"></div>
          <div class="chart-type-toggle center">
            <button :class="['type-btn', { active: chartTypes[`node-${idx}`] === 'smooth' }]" @click.stop="switchChartType(`node-${idx}`, 'smooth')">平滑</button>
            <button :class="['type-btn', { active: chartTypes[`node-${idx}`] === 'step' }]" @click.stop="switchChartType(`node-${idx}`, 'step')">阶梯</button>
          </div>
        </div>
      </section>

      <!-- System Performance Analysis Section -->
      <section v-if="metrics?.system_metrics" class="nodes-section">
        <h3>系统性能分析</h3>

        <!-- Legacy Report Info Banner -->
        <div v-if="!metrics.system_metrics.time_series?.length && !metrics.system_metrics.summary" class="info-banner">
          <span class="banner-icon">ℹ️</span>
          <div class="banner-text">
            <strong>无系统性能数据</strong>
            <p>此报告生成时未启用系统指标采集，或测试运行时间过短未能收集到有效数据。建议在后续测试中开启系统监控以获取完整的性能分析。</p>
          </div>
        </div>

        <!-- Summary Cards -->
        <div v-if="metrics.system_metrics.summary || metrics.system_metrics.time_series?.length" class="sys-summary-row">
          <div class="sys-summary-card" title="Goroutine 峰值：测试运行期间 Go 运行时中活跃协程的最大数量。包含 Worker、HTTP 连接池、内部管理协程等。100 Worker 通常对应 1000-1500 Goroutine 属于正常范围">
            <div class="sys-summary-label">Goroutine 峰值</div>
            <div class="sys-summary-value">{{ Number(metrics.system_metrics.summary?.goroutine_max || 0).toLocaleString() }}</div>
            <div class="sys-summary-sub">平均 {{ (metrics.system_metrics.summary?.goroutine_avg || 0).toFixed(0) }}</div>
          </div>
          <div class="sys-summary-card" title="Heap 峰值：测试运行期间堆内存分配的最大值（MB）。反映应用程序的内存使用峰值">
            <div class="sys-summary-label">Heap 峰值</div>
            <div class="sys-summary-value">{{ (metrics.system_metrics.summary?.heap_alloc_max_mb || 0).toFixed(3) }} MB</div>
            <div class="sys-summary-sub">平均 {{ (metrics.system_metrics.summary?.heap_alloc_avg_mb || 0).toFixed(3) }} MB</div>
          </div>
          <div class="sys-summary-card" title="CPU 峰值：测试运行期间进程 CPU 占用的最大百分比。基于两次采样间 CPU 时间差计算。多核环境可能超过 100%">
            <div class="sys-summary-label">CPU 峰值</div>
            <div class="sys-summary-value" :style="{ color: (metrics.system_metrics.summary?.cpu_max || 0) > 90 ? 'var(--accent-danger)' : (metrics.system_metrics.summary?.cpu_max || 0) > 70 ? 'var(--accent-warning)' : 'var(--text-primary)' }">{{ (metrics.system_metrics.summary?.cpu_max || 0).toFixed(3) }}%</div>
            <div class="sys-summary-sub">平均 {{ (metrics.system_metrics.summary?.cpu_avg || 0).toFixed(3) }}%</div>
          </div>
          <div class="sys-summary-card" title="GC 暂停：测试运行期间垃圾回收暂停的总时间和次数。频繁或长时间的 GC 暂停会影响测试精度">
            <div class="sys-summary-label">GC 暂停</div>
            <div class="sys-summary-value">{{ (metrics.system_metrics.summary?.gc_pause_total_ms || 0).toFixed(3) }} ms</div>
            <div class="sys-summary-sub">共 {{ metrics.system_metrics.summary?.gc_count || 0 }} 次</div>
          </div>
          <div class="sys-summary-card" title="任务等待 P99 峰值：99% 的任务从进入队列到被 Worker 取出的最大等待时间（ms）。&gt;100ms 说明 Worker 不够用，应增加 Worker 数量">
            <div class="sys-summary-label">任务等待 P99 峰值</div>
            <div class="sys-summary-value">{{ (metrics.system_metrics.summary?.task_wait_p99_max_ms || 0).toFixed(3) }} ms</div>
            <div class="sys-summary-sub">平均 {{ (metrics.system_metrics.summary?.task_wait_avg_ms || 0).toFixed(3) }} ms</div>
          </div>
        </div>

        <!-- System Trend Charts -->
        <div v-if="metrics.system_metrics.time_series?.length >= 2" class="sys-charts-row">
          <div class="chart-card">
            <div class="chart-header"><h3>Goroutine 趋势</h3></div>
            <div class="chart-body" ref="sysGoroutineChartRef"></div>
            <div class="chart-type-toggle center">
              <button :class="['type-btn', { active: chartTypes.sysGoroutine === 'smooth' }]" @click.stop="switchChartType('sysGoroutine', 'smooth')">平滑</button>
              <button :class="['type-btn', { active: chartTypes.sysGoroutine === 'step' }]" @click.stop="switchChartType('sysGoroutine', 'step')">阶梯</button>
            </div>
          </div>
          <div class="chart-card">
            <div class="chart-header"><h3>Heap 内存趋势</h3></div>
            <div class="chart-body" ref="sysHeapChartRef"></div>
            <div class="chart-type-toggle center">
              <button :class="['type-btn', { active: chartTypes.sysHeap === 'smooth' }]" @click.stop="switchChartType('sysHeap', 'smooth')">平滑</button>
              <button :class="['type-btn', { active: chartTypes.sysHeap === 'step' }]" @click.stop="switchChartType('sysHeap', 'step')">阶梯</button>
            </div>
          </div>
          <div class="chart-card">
            <div class="chart-header"><h3>CPU 使用率趋势</h3></div>
            <div class="chart-body" ref="sysCpuChartRef"></div>
            <div class="chart-type-toggle center">
              <button :class="['type-btn', { active: chartTypes.sysCpu === 'smooth' }]" @click.stop="switchChartType('sysCpu', 'smooth')">平滑</button>
              <button :class="['type-btn', { active: chartTypes.sysCpu === 'step' }]" @click.stop="switchChartType('sysCpu', 'step')">阶梯</button>
            </div>
          </div>
          <div class="chart-card">
            <div class="chart-header"><h3>任务等待时间趋势</h3></div>
            <div class="chart-body" ref="sysTaskWaitChartRef"></div>
            <div class="chart-type-toggle center">
              <button :class="['type-btn', { active: chartTypes.sysTaskWait === 'smooth' }]" @click.stop="switchChartType('sysTaskWait', 'smooth')">平滑</button>
              <button :class="['type-btn', { active: chartTypes.sysTaskWait === 'step' }]" @click.stop="switchChartType('sysTaskWait', 'step')">阶梯</button>
            </div>
          </div>
        </div>

        <!-- System Metrics Data Table -->
        <div v-if="metrics.system_metrics.time_series?.length > 0" class="sys-table-section">
          <div class="table-header-row">
            <h4>系统指标详细数据</h4>
            <div class="table-info">共 {{ metrics.system_metrics.time_series.length }} 条记录</div>
          </div>

          <div class="table-wrapper">
            <table class="data-table">
              <thead>
                <tr>
                  <th class="sortable" @click="toggleSort('timestamp')">
                    时间戳
                    <span class="sort-icon">{{ getSortIcon('timestamp') }}</span>
                  </th>
                  <th class="sortable" @click="toggleSort('goroutine_count')">
                    Goroutines
                    <span class="sort-icon">{{ getSortIcon('goroutine_count') }}</span>
                  </th>
                  <th class="sortable" @click="toggleSort('heap_alloc_mb')">
                    Heap (MB)
                    <span class="sort-icon">{{ getSortIcon('heap_alloc_mb') }}</span>
                  </th>
                  <th class="sortable" @click="toggleSort('cpu_percent')">
                    CPU (%)
                    <span class="sort-icon">{{ getSortIcon('cpu_percent') }}</span>
                  </th>
                  <th class="sortable" @click="toggleSort('gc_pause_last_ms')">
                    GC 暂停 (ms)
                    <span class="sort-icon">{{ getSortIcon('gc_pause_last_ms') }}</span>
                  </th>
                  <th class="sortable" @click="toggleSort('active_workers')">
                    Workers
                    <span class="sort-icon">{{ getSortIcon('active_workers') }}</span>
                  </th>
                  <th class="sortable" @click="toggleSort('pending_queue_len')">
                    队列长度
                    <span class="sort-icon">{{ getSortIcon('pending_queue_len') }}</span>
                  </th>
                  <th class="sortable" @click="toggleSort('task_wait_p99_ms')">
                    Wait P99 (ms)
                    <span class="sort-icon">{{ getSortIcon('task_wait_p99_ms') }}</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(row, idx) in paginatedTableData" :key="idx"
                    :class="{ 'danger-row': isDangerRow(row) }">
                  <td>{{ formatTimestamp(row.timestamp) }}</td>
                  <td :class="{ 'danger-cell': row.goroutine_count > 50000 }">
                    {{ Number(row.goroutine_count || 0).toLocaleString() }}
                  </td>
                  <td :class="{ 'danger-cell': row.heap_alloc_mb > 500 }">
                    {{ (row.heap_alloc_mb || 0).toFixed(3) }}
                  </td>
                  <td :class="{ 'danger-cell': row.cpu_percent > 90 }">
                    {{ (row.cpu_percent || 0).toFixed(3) }}
                  </td>
                  <td :class="{ 'danger-cell': row.gc_pause_last_ms > 10 }">
                    {{ (row.gc_pause_last_ms || 0).toFixed(3) }}
                  </td>
                  <td>{{ row.active_workers || 0 }}</td>
                  <td :class="{ 'danger-cell': row.pending_queue_len > 100 }">
                    {{ row.pending_queue_len || 0 }}
                  </td>
                  <td :class="{ 'danger-cell': row.task_wait_p99_ms > 100 }">
                    {{ (row.task_wait_p99_ms || 0).toFixed(3) }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <!-- Pagination Controls -->
          <div class="pagination-controls" v-if="totalPages > 1">
            <button class="page-btn" :disabled="currentPage === 1" @click="currentPage = 1">首页</button>
            <button class="page-btn" :disabled="currentPage === 1" @click="currentPage--">上一页</button>
            <span class="page-info">第 {{ currentPage }} / {{ totalPages }} 页</span>
            <button class="page-btn" :disabled="currentPage === totalPages" @click="currentPage++">下一页</button>
            <button class="page-btn" :disabled="currentPage === totalPages" @click="currentPage = totalPages">末页</button>
          </div>
        </div>
      </section>
    </div>

    <div v-else-if="!report" class="empty-state">
      <div class="spinner"></div>
      <p>加载中...</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { useRoute } from 'vue-router'
import { getReport } from '@/api/report'
import axios from 'axios'
import type { ReportDTO } from '@/types'
import * as echarts from 'echarts'

const route = useRoute()
const report = ref<ReportDTO | null>(null)
const metrics = ref<Record<string, any> | null>(null)
const qpsChartRef = ref<HTMLElement>()
const latencyTrendChartRef = ref<HTMLElement>()
const latencyChartRef = ref<HTMLElement>()
const overviewChartRef = ref<HTMLElement>()
const errorRateChartRef = ref<HTMLElement>()
const errorBreakdownChartRef = ref<HTMLElement>()
const sysGoroutineChartRef = ref<HTMLElement>()
const sysHeapChartRef = ref<HTMLElement>()
const sysCpuChartRef = ref<HTMLElement>()
const sysTaskWaitChartRef = ref<HTMLElement>()

let qpsChart: echarts.ECharts | null = null
let latTrendChart: echarts.ECharts | null = null
let latChart: echarts.ECharts | null = null
let ovChart: echarts.ECharts | null = null
let errRateChart: echarts.ECharts | null = null
let errBreakdownChart: echarts.ECharts | null = null
let sysGoroutineChart: echarts.ECharts | null = null
let sysHeapChart: echarts.ECharts | null = null
let sysCpuChart: echarts.ECharts | null = null
let sysTaskWaitChart: echarts.ECharts | null = null
let themeObserver: MutationObserver | null = null

interface NodeTimeSeries {
  node_id: string
  name?: string
  summary?: any
  timestamps?: string[]
  ts_qps?: number[]
  ts_p50?: number[]
  ts_p90?: number[]
  ts_p95?: number[]
  ts_p99?: number[]
}

interface ErrorItem {
  error_type?: string
  code?: string
  status?: string
  message?: string
  msg?: string
  count?: number
}

const nodeTimeSeries = ref<NodeTimeSeries[]>([])
const nodeChartRefs = new Map<number, HTMLElement>()
const nodeCharts = new Map<number, echarts.ECharts>()
const chartTypes = ref<Record<string, 'smooth' | 'step'>>({
  errorRate: 'smooth',
  qpsTrend: 'smooth',
  latTrend: 'smooth',
  sysGoroutine: 'smooth',
  sysHeap: 'smooth',
  sysCpu: 'smooth',
  sysTaskWait: 'smooth',
})

const tableSortKey = ref<string>('timestamp')
const tableSortOrder = ref<'asc' | 'desc'>('asc')
const currentPage = ref<number>(1)
const pageSize = 20

const sortedTableData = computed(() => {
  const ts = metrics.value?.system_metrics?.time_series || []
  if (!tableSortKey.value) return ts
  const key = tableSortKey.value
  const order = tableSortOrder.value === 'asc' ? 1 : -1
  return [...ts].sort((a: any, b: any) => {
    const valA = a[key] ?? 0
    const valB = b[key] ?? 0
    if (typeof valA === 'string' && typeof valB === 'string') {
      return order * valA.localeCompare(valB)
    }
    return order * ((valA as number) - (valB as number))
  })
})

const totalPages = computed(() => Math.ceil(sortedTableData.value.length / pageSize))

const paginatedTableData = computed(() => {
  const start = (currentPage.value - 1) * pageSize
  return sortedTableData.value.slice(start, start + pageSize)
})

function toggleSort(key: string) {
  if (tableSortKey.value === key) {
    tableSortOrder.value = tableSortOrder.value === 'asc' ? 'desc' : 'asc'
  } else {
    tableSortKey.value = key
    tableSortOrder.value = 'asc'
  }
  currentPage.value = 1
}

function getSortIcon(key: string): string {
  if (tableSortKey.value !== key) return '⇅'
  return tableSortOrder.value === 'asc' ? '↑' : '↓'
}

function formatTimestamp(ts: string): string {
  if (!ts) return '-'
  try {
    const d = new Date(ts)
    if (isNaN(d.getTime())) return ts
    return d.toLocaleString('zh-CN', {
      year: 'numeric', month: '2-digit', day: '2-digit',
      hour: '2-digit', minute: '2-digit', second: '2-digit'
    })
  } catch {
    return ts
  }
}

function isDangerRow(row: any): boolean {
  return (row.goroutine_count > 50000) ||
         (row.heap_alloc_mb > 500) ||
         (row.cpu_percent > 90) ||
         (row.gc_pause_last_ms > 10) ||
         (row.pending_queue_len > 100) ||
         (row.task_wait_p99_ms > 100)
}

function initNodeChartTypes() {
  nodeTimeSeries.value.forEach((_, idx) => {
    chartTypes.value[`node-${idx}`] = 'smooth'
  })
}

function switchChartType(chartId: string, type: 'smooth' | 'step') {
  chartTypes.value[chartId] = type
  const tc = themeColors()
  const m = metrics.value || {}
  if (chartId === 'errorRate') renderErrorRateChart(tc, m)
  else if (chartId === 'qpsTrend') renderQPSTrend(tc, m)
  else if (chartId === 'latTrend') renderLatencyTrend(tc, m)
  else if (chartId.startsWith('node-')) renderNodeCharts(tc)
  else if (chartId === 'sysGoroutine') renderSysGoroutineChart(tc)
  else if (chartId === 'sysHeap') renderSysHeapChart(tc)
  else if (chartId === 'sysCpu') renderSysCpuChart(tc)
  else if (chartId === 'sysTaskWait') renderSysTaskWaitChart(tc)
}

function setNodeChartRef(idx: number, el: HTMLElement | null) {
  if (el) nodeChartRefs.set(idx, el)
}

function parseMetrics(r: ReportDTO): Record<string, any> {
  try {
    let result: Record<string, any> = {}
    if (r.summary) {
      const summaryData = typeof r.summary === 'string' ? JSON.parse(r.summary) : r.summary
      result = { ...result, ...summaryData }
    }
    if (r.detail) {
      const detailData = typeof r.detail === 'string' ? JSON.parse(r.detail) : r.detail
      if (detailData.global_summary) result = { ...result, ...detailData.global_summary }
      if (detailData.global_time_series && Array.isArray(detailData.global_time_series)) {
        result.global_time_series = detailData.global_time_series
        const ts = detailData.global_time_series as any[]
        result.timestamps = ts.map(s => s.t || s.Timestamp)
        result.ts_qps = ts.map(s => s.qps || s.QPS || 0)
        result.ts_total = ts.map(s => s.total || s.TotalRequests || 0)
        result.ts_success = ts.map(s => s.success || s.SuccessCount || 0)
        result.ts_fail = ts.map(s => s.fail || s.FailCount || 0)
        result.ts_p50 = ts.map(s => s.p50_ms || s.P50LatencyMs || 0)
        result.ts_p90 = ts.map(s => s.p90_ms || s.P90LatencyMs || 0)
        result.ts_p95 = ts.map(s => s.p95_ms || s.P95LatencyMs || 0)
        result.ts_p99 = ts.map(s => s.p99_ms || s.P99LatencyMs || 0)
      }
      if (detailData.error_summary && Array.isArray(detailData.error_summary)) {
        result.error_breakdown = detailData.error_summary
      }
      if (detailData.node_metrics && Array.isArray(detailData.node_metrics)) {
        result.node_metrics = detailData.node_metrics.map((node: any) => ({
          node_id: node.node_id,
          name: node.node_name || node.node_id,
          summary: node.summary,
          time_series: node.time_series,
          timestamps: (node.time_series || []).map((s: any) => s.t || s.Timestamp || ''),
          ts_qps: (node.time_series || []).map((s: any) => s.qps || s.QPS || 0),
          ts_total: (node.time_series || []).map((s: any) => s.total || s.TotalRequests || 0),
          ts_success: (node.time_series || []).map((s: any) => s.success || s.SuccessCount || 0),
          ts_fail: (node.time_series || []).map((s: any) => s.fail || s.FailCount || 0),
          ts_p50: (node.time_series || []).map((s: any) => s.p50_ms || s.P50LatencyMs || 0),
          ts_p90: (node.time_series || []).map((s: any) => s.p90_ms || s.P90LatencyMs || 0),
          ts_p95: (node.time_series || []).map((s: any) => s.p95_ms || s.P95LatencyMs || 0),
          ts_p99: (node.time_series || []).map((s: any) => s.p99_ms || s.P99LatencyMs || 0),
        }))
      }
      if (detailData.metadata) result = { ...result, ...detailData.metadata }
      if (detailData.system_metrics) {
        result.system_metrics = detailData.system_metrics
      }
    }
    return result
  } catch (e) {
    console.error('Parse metrics error:', e)
    return {}
  }
}

const calcQPS = computed(() => {
  const m = metrics.value
  if (!m) return '-'
  const dur = Number(m.duration_s || 0)
  const total = Number(m.total_reqs || 0)
  if (dur <= 0 || total <= 0) return '-'
  return (total / dur).toFixed(3)
})

const fmtPeakQPS = computed(() => {
  const m = metrics.value
  if (!m) return '-'
  const peak = Number(m.peak_qps || 0)
  if (peak <= 0) return '-'
  return peak.toFixed(3)
})

const fmtThroughput = computed(() => {
  const m = metrics.value
  if (!m) return '-'
  const tp = Number(m.throughput || 0)
  if (tp <= 0) return '-'
  return tp.toFixed(3) + '/s'
})

const fmtTTFB = computed(() => {
  const m = metrics.value
  if (!m) return '-'
  const ttfb = Number(m.ttfb_ms || m.min_latency_ms || 0)
  if (ttfb <= 0) return '-'
  if (ttfb >= 1000) return (ttfb / 1000).toFixed(3) + 's'
  return ttfb.toFixed(3) + 'ms'
})

const fmtMinLatency = computed(() => {
  const m = metrics.value
  if (!m) return '-'
  const min = Number(m.min_latency_ms || 0)
  if (min <= 0) return '-'
  if (min >= 1000) return (min / 1000).toFixed(3) + 's'
  return min.toFixed(3) + 'ms'
})

const fmtErrorRate = computed(() => {
  const m = metrics.value
  if (!m) return '-'
  const total = Number(m.total_reqs || 0)
  const fail = Number(m.failed_reqs || 0)
  if (total <= 0) return '0%'
  return ((fail / total) * 100).toFixed(3) + '%'
})

const successRateColor = computed(() => {
  const m = metrics.value
  if (!m) return 'var(--accent-success)'
  const total = Number(m.total_reqs || 0)
  const succ = Number(m.success_reqs || 0)
  if (total <= 0) return 'var(--accent-success)'
  const rate = (succ / total) * 100
  if (rate >= 99) return 'var(--accent-success)'
  if (rate >= 95) return 'var(--accent-success)'
  if (rate >= 80) return 'var(--accent-warning)'
  return 'var(--accent-danger)'
})

const rankedNodes = computed(() => {
  const nodes = [...(nodeTimeSeries.value || [])]
  nodes.sort((a, b) => {
    const aRate = Number(a.summary?.success_rate || 0)
    const bRate = Number(b.summary?.success_rate || 0)
    if (aRate !== bRate) return bRate - aRate
    const aP95 = Number(a.summary?.p95_latency_ms || 0)
    const bP95 = Number(b.summary?.p95_latency_ms || 0)
    return aP95 - bP95
  })
  return nodes
})

const errorBreakdown = computed((): ErrorItem[] => {
  const m = metrics.value
  if (!m?.error_breakdown) return []
  return m.error_breakdown as ErrorItem[]
})

async function fetchReport() {
  const id = route.params.id as string
  if (!id) return
  try {
    const resp = await getReport(id)
    if (resp.code === 0) {
      report.value = resp.data
      metrics.value = parseMetrics(resp.data)
      nodeTimeSeries.value = (metrics.value?.node_metrics as NodeTimeSeries[]) || []
      initNodeChartTypes()
      await nextTick()
      renderAll()
    }
  } catch (e) { console.error('Fetch report error:', e) }
}

function isDark(): boolean {
  return document.documentElement.getAttribute('data-theme') !== 'light'
}

function themeColors() {
  const dark = isDark()
  return {
    textColor: dark ? '#c9d1d9' : '#656d76',
    lineColor: dark ? '#30363d' : '#dde2e8',
    gridColor: dark ? '#21262d' : '#f6f8fa',
    bg: 'transparent',
    gridLineDash: dark ? [4, 3] : [4, 3],
    colors: [
      dark ? '#00E5FF' : '#0891b2',
      dark ? '#4ade80' : '#16a34a',
      dark ? '#FFB74D' : '#d97706',
      dark ? '#B388FF' : '#8b5cf6',
      dark ? '#ef4444' : '#dc2626',
      dark ? '#eab308' : '#ca8a04',
      dark ? '#3b82f6' : '#2563eb',
      dark ? '#94a3b8' : '#64748b',
    ],
    latencyColors: [
      dark ? '#00E5FF' : '#0891b2',
      dark ? '#4ade80' : '#16a34a',
      dark ? '#FFB74D' : '#d97706',
      dark ? '#B388FF' : '#8b5cf6',
      dark ? '#ef4444' : '#dc2626',
      dark ? '#eab308' : '#ca8a04',
      dark ? '#3b82f6' : '#2563eb',
      dark ? '#94a3b8' : '#64748b',
    ],
    dangerColor: dark ? '#ef4444' : '#dc2626',
  }
}

function renderSysGoroutineChart(tc: any) {
  const sm = metrics.value?.system_metrics
  if (!sysGoroutineChartRef.value || !sm?.time_series?.length) return
  if (sysGoroutineChart) sysGoroutineChart.dispose()
  sysGoroutineChart = echarts.init(sysGoroutineChartRef.value)
  const isSmooth = chartTypes.value.sysGoroutine === 'smooth'
  const ts = sm.time_series
  const labels = ts.map((s: any) => {
    const t = new Date(s.timestamp || s.Timestamp)
    return t.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  })
  const data = ts.map((s: any) => s.goroutine_count || s.GoroutineCount || 0)
  sysGoroutineChart.setOption({
    backgroundColor: tc.bg,
    grid: { top: 30, right: 20, bottom: 50, left: 50 },
    xAxis: { type: 'category', data: labels, axisLine: { lineStyle: { color: tc.lineColor } }, axisLabel: { color: tc.textColor, fontSize: 10 } },
    yAxis: { type: 'value', axisLine: { show: false }, splitLine: { lineStyle: { color: tc.lineColor, type: 'dashed' } }, axisLabel: { color: tc.textColor, fontSize: 10 } },
    series: [{ name: 'Goroutines', data, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { color: tc.colors[0], width: 2 }, itemStyle: { color: tc.colors[0] }, markLine: { silent: true, lineStyle: { type: 'dashed' }, data: [{ yAxis: 10000, lineStyle: { color: tc.colors[5] }, label: { formatter: '10K', color: tc.colors[5], fontSize: 10 } }, { yAxis: 50000, lineStyle: { color: tc.dangerColor }, label: { formatter: '50K', color: tc.dangerColor, fontSize: 10 } }] } }],
    tooltip: { trigger: 'axis', confine: true, backgroundColor: 'rgba(255,255,255,0.96)', borderColor: 'rgba(148,163,184,0.2)', borderWidth: 1, borderRadius: 12, padding: [12, 16], textStyle: { fontSize: 11, color: '#475569' }, formatter: (params: any) => { let h = `<div style="font-size:11.5px;font-weight:600;margin-bottom:4px">${(params[0]?.axisValue || '')}</div>`; params.forEach((p: any) => { h += `${p.marker} ${p.seriesName}: <strong>${Number(p.value).toFixed(3)}</strong><br/>` }); return h } },
    legend: { data: ['Goroutines'], textStyle: { color: tc.textColor }, top: 0 },
  }, true)
}

function renderSysHeapChart(tc: any) {
  const sm = metrics.value?.system_metrics
  if (!sysHeapChartRef.value || !sm?.time_series?.length) return
  if (sysHeapChart) sysHeapChart.dispose()
  sysHeapChart = echarts.init(sysHeapChartRef.value)
  const isSmooth = chartTypes.value.sysHeap === 'smooth'
  const ts = sm.time_series
  const labels = ts.map((s: any) => {
    const t = new Date(s.timestamp || s.Timestamp)
    return t.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  })
  const allocData = ts.map((s: any) => s.heap_alloc_mb || s.HeapAllocMB || 0)
  const sysData = ts.map((s: any) => s.heap_sys_mb || s.HeapSysMB || 0)
  sysHeapChart.setOption({
    backgroundColor: tc.bg,
    grid: { top: 30, right: 20, bottom: 50, left: 50 },
    xAxis: { type: 'category', data: labels, axisLine: { lineStyle: { color: tc.lineColor } }, axisLabel: { color: tc.textColor, fontSize: 10 } },
    yAxis: { type: 'value', axisLine: { show: false }, splitLine: { lineStyle: { color: tc.lineColor, type: 'dashed' } }, axisLabel: { color: tc.textColor, fontSize: 10, formatter: '{value}MB' } },
    series: [
      { name: 'HeapAlloc', data: allocData, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { color: tc.colors[0], width: 2 }, itemStyle: { color: tc.colors[0] } },
      { name: 'HeapSys', data: sysData, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { color: tc.colors[6], width: 2, type: 'dashed' }, itemStyle: { color: tc.colors[6] } },
    ],
    tooltip: { trigger: 'axis', confine: true, backgroundColor: 'rgba(255,255,255,0.96)', borderColor: 'rgba(148,163,184,0.2)', borderWidth: 1, borderRadius: 12, padding: [12, 16], textStyle: { fontSize: 11, color: '#475569' }, formatter: (params: any) => { let h = `<div style="font-size:11.5px;font-weight:600;margin-bottom:4px">${(params[0]?.axisValue || '')}</div>`; params.forEach((p: any) => { h += `${p.marker} ${p.seriesName}: <strong>${Number(p.value).toFixed(3)} MB</strong><br/>` }); return h } },
    legend: { data: ['HeapAlloc', 'HeapSys'], textStyle: { color: tc.textColor }, top: 0 },
  }, true)
}

function renderSysCpuChart(tc: any) {
  const sm = metrics.value?.system_metrics
  if (!sysCpuChartRef.value || !sm?.time_series?.length) return
  if (sysCpuChart) sysCpuChart.dispose()
  sysCpuChart = echarts.init(sysCpuChartRef.value)
  const isSmooth = chartTypes.value.sysCpu === 'smooth'
  const ts = sm.time_series
  const labels = ts.map((s: any) => {
    const t = new Date(s.timestamp || s.Timestamp)
    return t.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  })
  const data = ts.map((s: any) => s.cpu_percent || s.CPUUsagePercent || 0)
  sysCpuChart.setOption({
    backgroundColor: tc.bg,
    grid: { top: 30, right: 20, bottom: 50, left: 50 },
    xAxis: { type: 'category', data: labels, axisLine: { lineStyle: { color: tc.lineColor } }, axisLabel: { color: tc.textColor, fontSize: 10 } },
    yAxis: { type: 'value', min: 0, max: 100, axisLine: { show: false }, splitLine: { lineStyle: { color: tc.lineColor, type: 'dashed' } }, axisLabel: { color: tc.textColor, fontSize: 10, formatter: '{value}%' } },
    series: [{ name: 'CPU', data, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { color: tc.colors[5], width: 2 }, itemStyle: { color: tc.colors[5] }, areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{ offset: 0, color: 'rgba(234,179,8,0.2)' }, { offset: 1, color: 'rgba(234,179,8,0.01)' }]) }, markLine: { silent: true, lineStyle: { type: 'dashed' }, data: [{ yAxis: 70, lineStyle: { color: tc.colors[5] }, label: { formatter: '70%', color: tc.colors[5], fontSize: 10 } }, { yAxis: 90, lineStyle: { color: tc.dangerColor }, label: { formatter: '90%', color: tc.dangerColor, fontSize: 10 } }] } }],
    tooltip: { trigger: 'axis', confine: true, backgroundColor: 'rgba(255,255,255,0.96)', borderColor: 'rgba(148,163,184,0.2)', borderWidth: 1, borderRadius: 12, padding: [12, 16], textStyle: { fontSize: 11, color: '#475569' }, formatter: (params: any) => { let h = `<div style="font-size:11.5px;font-weight:600;margin-bottom:4px">${(params[0]?.axisValue || '')}</div>`; params.forEach((p: any) => { h += `${p.marker} ${p.seriesName}: <strong>${Number(p.value).toFixed(3)}%</strong><br/>` }); return h } },
    legend: { data: ['CPU'], textStyle: { color: tc.textColor }, top: 0 },
  }, true)
}

function renderSysTaskWaitChart(tc: any) {
  const sm = metrics.value?.system_metrics
  if (!sysTaskWaitChartRef.value || !sm?.time_series?.length) return
  if (sysTaskWaitChart) sysTaskWaitChart.dispose()
  sysTaskWaitChart = echarts.init(sysTaskWaitChartRef.value)
  const isSmooth = chartTypes.value.sysTaskWait === 'smooth'
  const ts = sm.time_series
  const labels = ts.map((s: any) => {
    const t = new Date(s.timestamp || s.Timestamp)
    return t.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  })
  const p50 = ts.map((s: any) => s.task_wait_p50_ms || s.TaskWaitP50Ms || 0)
  const p95 = ts.map((s: any) => s.task_wait_p95_ms || s.TaskWaitP95Ms || 0)
  const p99 = ts.map((s: any) => s.task_wait_p99_ms || s.TaskWaitP99Ms || 0)
  sysTaskWaitChart.setOption({
    backgroundColor: tc.bg,
    grid: { top: 30, right: 20, bottom: 50, left: 50 },
    xAxis: { type: 'category', data: labels, axisLine: { lineStyle: { color: tc.lineColor } }, axisLabel: { color: tc.textColor, fontSize: 10 } },
    yAxis: { type: 'value', axisLine: { show: false }, splitLine: { lineStyle: { color: tc.lineColor, type: 'dashed' } }, axisLabel: { color: tc.textColor, fontSize: 10, formatter: '{value}ms' } },
    series: [
      { name: 'P50', data: p50, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { color: tc.colors[6], width: 2 }, itemStyle: { color: tc.colors[6] } },
      { name: 'P95', data: p95, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { color: tc.colors[5], width: 2 }, itemStyle: { color: tc.colors[5] } },
      { name: 'P99', data: p99, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { color: tc.dangerColor, width: 2 }, itemStyle: { color: tc.dangerColor } },
    ],
    tooltip: { trigger: 'axis', confine: true, backgroundColor: 'rgba(255,255,255,0.96)', borderColor: 'rgba(148,163,184,0.2)', borderWidth: 1, borderRadius: 12, padding: [12, 16], textStyle: { fontSize: 11, color: '#475569' }, formatter: (params: any) => { let h = `<div style="font-size:11.5px;font-weight:600;margin-bottom:4px">${(params[0]?.axisValue || '')}</div>`; params.forEach((p: any) => { h += `${p.marker} ${p.seriesName}: <strong>${Number(p.value).toFixed(3)} ms</strong><br/>` }); return h } },
    legend: { data: ['P50', 'P95', 'P99'], textStyle: { color: tc.textColor }, top: 0 },
  }, true)
}

function renderErrorRateChart(tc: any, m: any) {
  if (!errorRateChartRef.value) return
  if (errRateChart) errRateChart.dispose()
  errRateChart = echarts.init(errorRateChartRef.value)

  const timestamps = m.timestamps as string[] | undefined
  const totals = (m.ts_total as number[]|undefined)||[]
  const fails = (m.ts_fail as number[]|undefined)||[]

  const errRates: number[] = []
  for (let i = 0; i < totals.length; i++) {
    errRates.push(totals[i] > 0 ? (fails[i] / totals[i]) * 100 : 0)
  }

  if (timestamps?.length && errRates.length) {
    const timeLabels = timestamps.map(ts => formatTimeShort(ts))
    const maxErrRate = Math.max(...errRates, 0.01)
    const globalErrRate = ((Number(m.failed_reqs || 0) / Math.max(Number(m.total_reqs || 1), 1)) * 100)
    errRateChart.setOption({
      backgroundColor: tc.bg,
      tooltip: { trigger: 'axis', confine: true, backgroundColor: isDark() ? 'rgba(30,41,59,0.95)' : 'rgba(255,255,255,0.96)', borderColor: isDark() ? 'rgba(71,85,105,0.3)' : 'rgba(148,163,184,0.2)', borderWidth: 1, borderRadius: 8, padding: [10,14], textStyle: { fontSize: 11, color: isDark() ? '#cbd5e1' : '#475569' }, formatter: (params: any) => {
        const p = Array.isArray(params) ? params[0] : params
        const idx = p?.dataIndex ?? 0
        return `${timeLabels[idx]}<br/>Error Rate: <strong>${Number(p?.value || 0).toFixed(3)}%</strong><br/>Failed: ${fails[idx] || 0} / Total: ${totals[idx] || 0}`
      }},
      grid: { left: 50, right: 16, top: 20, bottom: 44 },
      dataZoom: [{ type: 'slider', height: 14, bottom: 2, borderColor: 'transparent', backgroundColor: tc.lineColor, fillerColor: isDark() ? 'rgba(239,68,68,0.10)' : 'rgba(220,38,38,0.10)', handleStyle: { color: tc.dangerColor }, textStyle: { color: tc.textColor, fontSize: 9 }, showDetail: false }],
      xAxis: { type: 'category', data: timeLabels, axisLine: { show: false }, axisLabel: { color: tc.textColor, fontSize: 9, interval: Math.floor(timeLabels.length / 8) }, splitLine: { show: false } },
      yAxis: { type: 'value', name: '%', min: 0, max: Math.max(maxErrRate * 1.5, globalErrRate * 1.5, 1), axisLine: { show: false }, axisLabel: { color: tc.textColor, fontSize: 10, formatter: (v: number) => v.toFixed(3) + '%' }, splitLine: { lineStyle: { color: tc.lineColor, type: 'dashed' as const } } },
      markLine: {
        silent: true,
        data: [{ yAxis: globalErrRate, label: { formatter: `Total: ${globalErrRate.toFixed(3)}%`, color: tc.dangerColor, fontSize: 9 }, lineStyle: { color: tc.dangerColor, type: 'dashed', width: 1 } }],
        symbol: 'none',
      },
      series: [{
        name: 'Error Rate',
        type: 'line',
        smooth: chartTypes.value.errorRate === 'smooth',
        step: chartTypes.value.errorRate === 'step' ? 'middle' as const : false,
        data: errRates,
        lineStyle: { width: 2, color: tc.dangerColor },
        areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: isDark() ? 'rgba(239,68,68,0.18)' : 'rgba(220,38,38,0.18)' },
          { offset: 1, color: isDark() ? 'rgba(239,68,68,0.01)' : 'rgba(220,38,38,0.01)' }
        ])},
        symbol: 'none',
      }]
    })
  }
}

function renderAll() {
  if (!latencyChartRef.value || !overviewChartRef.value) return
  const tc = themeColors()
  const m = metrics.value || {}

  // Request distribution donut chart
  if (ovChart) ovChart.dispose()
  ovChart = echarts.init(overviewChartRef.value)
  ovChart.setOption({
    backgroundColor: tc.bg,
    color: [tc.colors[1], tc.colors[3]],
    tooltip: {
      trigger: 'item',
      confine: true,
      backgroundColor: isDark() ? 'rgba(22,27,34,0.96)' : 'rgba(255,255,255,0.97)',
      borderColor: isDark() ? 'rgba(48,54,61,0.8)' : 'rgba(208,215,222,0.5)',
      borderWidth: 1,
      borderRadius: 10,
      padding: [12, 16],
      textStyle: { fontSize: 12, color: isDark() ? '#e6edf3' : '#24292f' },
      extraCssText: 'box-shadow: 0 4px 16px rgba(0,0,0,0.12);',
      formatter: (p: any) => {
        const total = Number(m.total_reqs || 1)
        const pct = ((p.value / total) * 100).toFixed(3)
        return `<div style="font-weight:600;margin-bottom:4px">${p.name}</div><div style="font-size:13px"><strong>${Number(p.value).toLocaleString()}</strong> requests <span style="color:${isDark()?'#8b949e':'#656d76'}">(${pct}%)</span></div>`
      }
    },
    graphic: [
      {
        type: 'text',
        left: '30%',
        top: '44%',
        style: {
          text: Number(m.total_reqs || 0).toLocaleString(),
          fontSize: 22,
          fontWeight: 700,
          fill: tc.textColor,
          textAlign: 'center',
        },
      },
      {
        type: 'text',
        left: '30%',
        top: '58%',
        style: {
          text: '总请求',
          fontSize: 11,
          fill: isDark() ? '#8b949e' : '#8c959f',
          textAlign: 'center',
        },
      },
    ],
    legend: {
      orient: 'vertical', right: 20, top: 'center',
      itemWidth: 12, itemHeight: 12, itemGap: 14,
      icon: 'roundRect',
      formatter: (name: string) => {
        const val = name === 'Success' ? Number(m.success_reqs || 0) : Number(m.failed_reqs || 0)
        const total = Number(m.total_reqs || 1)
        const pct = (val / total * 100).toFixed(3)
        return `{name|${name}}   {val|${pct}%}`
      },
      textStyle: { rich: { name: { color: isDark() ? '#c9d1d9' : '#24292f', fontWeight: 500, fontSize: 13 }, val: { color: isDark() ? '#8b949e' : '#8c959f', fontSize: 12, fontWeight: 400 } } }
    },
    series: [{
      type: 'pie',
      radius: ['50%', '72%'],
      center: ['36%', '50%'],
      avoidLabelOverlap: false,
      startAngle: 90,
      padAngle: 2,
      itemStyle: {
        borderRadius: 8,
        borderColor: tc.bg,
        borderWidth: 3,
      },
      label: { show: false },
      emphasis: {
        scale: true,
        scaleSize: 8,
        itemStyle: { shadowBlur: 20, shadowColor: 'rgba(0,0,0,0.15)' },
        label: { show: true, fontSize: 14, fontWeight: 'bold', color: tc.textColor }
      },
      data: [
        { value: Number(m.success_reqs || 0), name: 'Success', itemStyle: { color: new echarts.graphic.LinearGradient(0, 0, 1, 1, [
          { offset: 0, color: isDark() ? '#4ade80' : '#16a34a' },
          { offset: 1, color: isDark() ? '#22c55e' : '#15803d' }
        ])}},
        { value: Number(m.failed_reqs || 0), name: 'Failed', itemStyle: { color: new echarts.graphic.LinearGradient(0, 0, 1, 1, [
          { offset: 0, color: isDark() ? '#ef4444' : '#dc2626' },
          { offset: 1, color: isDark() ? '#dc2626' : '#b91c1c' }
        ])}},
      ],
    }],
  })

  // Error rate trend chart
  renderErrorRateChart(tc, m)

  // Latency percentile bar chart (with P90)
  if (latChart) latChart.dispose()
  latChart = echarts.init(latencyChartRef.value)
  latChart.setOption({
    backgroundColor: tc.bg,
    tooltip: {
      trigger: 'axis', axisPointer: { type: 'shadow', shadowStyle: { color: isDark() ? 'rgba(0,229,255,0.05)' : 'rgba(8,145,178,0.04)' } },
      confine: true,
      position: function (point: number[], params: any, dom: HTMLElement, rect: DOMRect, size: { contentSize: [number, number]; viewSize: [number, number] }) {
        const x = point[0]
        const y = point[1]
        if (x < size.viewSize[0] / 2) {
          return [x + 12, y - size.contentSize[1] - 12]
        }
        return [x - size.contentSize[0] - 12, y - size.contentSize[1] - 12]
      },
      backgroundColor: isDark() ? 'rgba(22,27,34,0.96)' : 'rgba(255,255,255,0.97)',
      borderColor: isDark() ? 'rgba(48,54,61,0.8)' : 'rgba(208,215,222,0.5)',
      borderWidth: 1,
      borderRadius: 10,
      padding: [12, 16],
      textStyle: { fontSize: 12, color: isDark() ? '#e6edf3' : '#24292f' },
      extraCssText: 'box-shadow: 0 4px 16px rgba(0,0,0,0.10);',
      formatter: (params: any) => {
        const p = Array.isArray(params) ? params[0] : params
        const labels: Record<string, string> = { Avg: '平均延迟', P50: '中位数 (50%)', P90: 'P90 延迟', P95: 'P95 延迟', P99: 'P99 尾部延迟' }
        return `<div style="font-weight:600;margin-bottom:4px">${labels[p.name] || p.name}</div><div style="font-size:13px"><strong>${Number(p.value).toFixed(3)}</strong> ms</div>`
      }
    },
    grid: { left: 44, right: 20, top: 24, bottom: 32 },
    xAxis: {
      type: 'category',
      data: ['Avg', 'P50', 'P90', 'P95', 'P99'],
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: {
        color: isDark() ? '#8b949e' : '#656d76',
        fontSize: 12,
        fontWeight: 600,
        margin: 12,
      },
    },
    yAxis: {
      type: 'value',
      name: '',
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: { color: isDark() ? '#6e7681' : '#8c959f', fontSize: 10, formatter: (v: number) => v.toFixed(0) },
      splitLine: { lineStyle: { color: isDark() ? 'rgba(48,54,61,0.5)' : 'rgba(208,215,222,0.6)', type: 'dashed' as const } }
    },
    series: [{
      type: 'bar',
      barWidth: '40%',
      data: [
        { value: parseFloat(m.avg_latency_s || '0') * 1000, itemStyle: { color: tc.colors[0], borderRadius: [6, 6, 2, 2] }},
        { value: parseFloat(m.p50_latency_s || '0') * 1000, itemStyle: { color: tc.colors[1], borderRadius: [6, 6, 2, 2] }},
        { value: parseFloat(m.p90_latency_s || '0') * 1000, itemStyle: { color: tc.colors[2], borderRadius: [6, 6, 2, 2] }},
        { value: parseFloat(m.p95_latency_s || '0') * 1000, itemStyle: { color: tc.colors[3], borderRadius: [6, 6, 2, 2] }},
        { value: parseFloat(m.p99_latency_s || '0') * 1000, itemStyle: { color: tc.colors[4 % tc.colors.length], borderRadius: [6, 6, 2, 2] }},
      ],
      label: {
        show: true, position: 'top',
        formatter: (p: any) => Number(p.value).toFixed(3),
        color: isDark() ? '#8b949e' : '#656d76',
        fontSize: 11,
        fontWeight: 600,
        offset: [0, -4]
      },
      emphasis: {
        itemStyle: {
          shadowBlur: 18,
        }
      }
    }],
  })

  renderQPSTrend(tc, m)
  renderLatencyTrend(tc, m)
  renderNodeCharts(tc)
  renderErrorBreakdownChart(tc)
  renderSysGoroutineChart(tc)
  renderSysHeapChart(tc)
  renderSysCpuChart(tc)
  renderSysTaskWaitChart(tc)
}

function renderQPSTrend(tc: any, m: any) {
  const isSmooth = chartTypes.value.qpsTrend === 'smooth'
  if (!qpsChartRef.value) return
  if (qpsChart) qpsChart.dispose()
  qpsChart = echarts.init(qpsChartRef.value)

  const timestamps = m.timestamps as string[] | undefined
  const qpsData = (m.ts_qps as number[] | undefined) || []

  if (!timestamps?.length || !qpsData.length) {
    qpsChart.setOption({ title: { text: 'No Data', left: 'center', top: 'center', textStyle: { color: tc.textColor, fontSize: 14 } } })
    return
  }

  const timeLabels = timestamps.map(ts => formatTimeShort(ts))

  qpsChart.setOption({
    backgroundColor: tc.bg,
    tooltip: {
      trigger: 'axis' as const,
      confine: true,
      backgroundColor: isDark() ? 'rgba(30,41,59,0.95)' : 'rgba(255,255,255,0.96)',
      borderColor: isDark() ? 'rgba(71,85,105,0.3)' : 'rgba(148,163,184,0.2)',
      borderWidth: 1,
      borderRadius: 12,
      padding: [12, 16],
      textStyle: { fontSize: 11, color: isDark() ? '#cbd5e1' : '#475569' },
      extraCssText: 'box-shadow: 0 1px 3px rgba(0,0,0,0.04), 0 8px 24px rgba(0,0,0,0.08); backdrop-filter: blur(8px);',
      formatter: (params: any) => {
        const p = Array.isArray(params) ? params[0] : params
        const idx = p?.dataIndex ?? 0
        return `<div style="font-size:11.5px;color:${isDark()?'#e2e8f0':'#1e293b'};margin-bottom:6px;font-weight:600">${timeLabels[idx]}</div>QPS: <strong>${Number(p?.value || 0).toFixed(3)}</strong>`
      }
    },
    grid: { left: 50, right: 20, top: 20, bottom: 50 },
    dataZoom: [{ type: 'slider', height: 18, bottom: 4, borderColor: 'transparent', backgroundColor: tc.lineColor, fillerColor: `rgba(${isDark() ? '0,229,255' : '8,145,178'}, 0.15)`, handleStyle: { color: tc.colors[0] }, textStyle: { color: tc.textColor, fontSize: 10 }, brushSelect: true }],
    xAxis: { type: 'category', data: timeLabels, axisLine: { lineStyle: { color: tc.lineColor } }, axisLabel: { color: tc.textColor, fontSize: 10 } },
    yAxis: { type: 'value', axisLine: { show: false }, axisLabel: { color: tc.textColor, fontSize: 10, formatter: (v: number) => v.toFixed(3) }, splitLine: { lineStyle: { color: tc.lineColor, type: 'dashed' } } },
    series: [{
      name: 'QPS',
      type: 'line',
      smooth: isSmooth,
      step: isSmooth ? false : 'middle',
      data: qpsData,
      lineStyle: { width: 2, color: tc.colors[0] },
      itemStyle: { color: tc.colors[0] },
      areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
        { offset: 0, color: `rgba(${isDark() ? '0,229,255' : '8,145,178'}, 0.15)` },
        { offset: 1, color: `rgba(${isDark() ? '0,229,255' : '8,145,178'}, 0.01)` }
      ])},
      symbol: 'none',
    }]
  }, true)
}

function renderLatencyTrend(tc: any, m: any) {
  const isSmooth = chartTypes.value.latTrend === 'smooth'
  if (!latencyTrendChartRef.value) return
  if (latTrendChart) latTrendChart.dispose()
  latTrendChart = echarts.init(latencyTrendChartRef.value)

  const timestamps = m.timestamps as string[] | undefined
  const p50 = (m.ts_p50 as number[]|undefined)||[]
  const p90 = (m.ts_p90 as number[]|undefined)||[]
  const p95 = (m.ts_p95 as number[]|undefined)||[]
  const p99 = (m.ts_p99 as number[]|undefined)||[]

  if (!timestamps?.length) {
    latTrendChart.setOption({ title: { text: 'No Data', left: 'center', top: 'center', textStyle: { color: tc.textColor, fontSize: 14 } } })
    return
  }

  const timeLabels = timestamps.map(ts => formatTimeShort(ts))
  const lc = tc.latencyColors

  latTrendChart.setOption({
    backgroundColor: tc.bg,
    tooltip: {
      trigger: 'axis' as const,
      confine: true,
      backgroundColor: isDark() ? 'rgba(30,41,59,0.95)' : 'rgba(255,255,255,0.96)',
      borderColor: isDark() ? 'rgba(71,85,105,0.3)' : 'rgba(148,163,184,0.2)',
      borderWidth: 1,
      borderRadius: 12,
      padding: [12, 16],
      textStyle: { fontSize: 11, color: isDark() ? '#cbd5e1' : '#475569' },
      extraCssText: 'box-shadow: 0 1px 3px rgba(0,0,0,0.04), 0 8px 24px rgba(0,0,0,0.08); backdrop-filter: blur(8px);',
      formatter: (params: any) => {
        if (!Array.isArray(params)) return ''
        const i = params[0]?.dataIndex ?? 0
        let h = `<div style="font-size:11.5px;color:${isDark()?'#e2e8f0':'#1e293b'};margin-bottom:6px;font-weight:600">${timeLabels[i]}</div>`
        params.forEach((item: any) => { h += `${item.marker} ${item.seriesName}: <strong>${Number(item.value).toFixed(3)}</strong>ms<br/>` })
        return h
      }
    },
    legend: { data: ['P50','P90','P95','P99'], textStyle: { color: tc.textColor }, top: 0 },
    grid: { top: 30, right: 20, bottom: 50, left: 50 },
    dataZoom: [{ type: 'slider', height: 18, bottom: 4, borderColor: 'transparent', backgroundColor: tc.lineColor, fillerColor: `rgba(${isDark() ? '0,229,255' : '8,145,178'}, 0.15)`, handleStyle: { color: tc.colors[0] }, textStyle: { color: tc.textColor, fontSize: 10 }, brushSelect: true }],
    xAxis: { type: 'category', data: timeLabels, axisLine: { lineStyle: { color: tc.lineColor } }, axisLabel: { color: tc.textColor, fontSize: 10 } },
    yAxis: { type: 'value', axisLine: { show: false }, splitLine: { lineStyle: { color: tc.lineColor, type: 'dashed' } }, axisLabel: { color: tc.textColor, fontSize: 10, formatter: '{value}ms' } },
    series: [
      { name:'P50', type:'line', smooth: isSmooth, step: isSmooth?false:'middle', data:p50, lineStyle:{width:2,color:lc[0]}, itemStyle:{color:lc[0]}, symbol:'none' },
      { name:'P90', type:'line', smooth: isSmooth, step: isSmooth?false:'middle', data:p90, lineStyle:{width:2,color:lc[1]}, itemStyle:{color:lc[1]}, symbol:'none' },
      { name:'P95', type:'line', smooth: isSmooth, step: isSmooth?false:'middle', data:p95, lineStyle:{width:2,color:lc[2]}, itemStyle:{color:lc[2]}, symbol:'none' },
      { name:'P99', type:'line', smooth: isSmooth, step: isSmooth?false:'middle', data:p99, lineStyle:{width:2,color:lc[3]}, itemStyle:{color:lc[3]}, symbol:'none' },
    ]
  }, true)
}

function renderNodeCharts(tc: any) {
  nodeTimeSeries.value.forEach((node, idx) => {
    const isSmooth = chartTypes.value[`node-${idx}`] === 'smooth'
    const el = nodeChartRefs.get(idx)
    if (!el) return
    const oldChart = nodeCharts.get(idx)
    if (oldChart) { oldChart.dispose(); nodeCharts.delete(idx) }
    const chart = echarts.init(el)
    nodeCharts.set(idx, chart)

    const timestamps = node.timestamps || []
    const timeLabels = timestamps.map((ts: string) => formatTimeShort(ts))

    if (!timeLabels.length) {
      chart.setOption({ title: { text: 'No Data', left: 'center', top: 'center', textStyle: { color: tc.textColor, fontSize: 14 } } })
      return
    }

    const lc = tc.latencyColors
    chart.setOption({
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'axis' as const,
        confine: true,
        backgroundColor: isDark() ? 'rgba(30,41,59,0.95)' : 'rgba(255,255,255,0.96)',
        borderColor: isDark() ? 'rgba(71,85,105,0.3)' : 'rgba(148,163,184,0.2)',
        borderWidth: 1,
        borderRadius: 12,
        padding: [12, 16],
        textStyle: { fontSize: 11, color: isDark() ? '#cbd5e1' : '#475569' },
        extraCssText: 'box-shadow: 0 1px 3px rgba(0,0,0,0.04), 0 8px 24px rgba(0,0,0,0.08); backdrop-filter: blur(8px);',
        formatter: (params: any) => {
          if (!Array.isArray(params)) return ''
          const i = params[0]?.dataIndex ?? 0
          let h = `<div style="font-size:11.5px;color:${isDark()?'#e2e8f0':'#1e293b'};margin-bottom:6px;font-weight:600">${timeLabels[i]}</div>`
          params.forEach((item: any) => { h += `${item.marker} ${item.seriesName}: <strong>${typeof item.value === 'number' && item.seriesName === 'QPS' ? item.value.toFixed(3) : Number(item.value).toFixed(3)}</strong>${item.seriesName === 'QPS' ? '' : 'ms'}<br/>` })
          return h
        }
      },
      legend: { data: ['QPS','P50','P90','P95','P99'], textStyle: { color: tc.textColor }, top: 0 },
      grid: { left: 50, right: 50, top: 30, bottom: 50 },
      dataZoom: [{ type: 'slider', height: 18, bottom: 4, borderColor: 'transparent', backgroundColor: tc.lineColor, fillerColor: `rgba(${isDark() ? '0,229,255' : '8,145,178'}, 0.15)`, handleStyle: { color: tc.colors[0] }, textStyle: { color: tc.textColor, fontSize: 10 }, brushSelect: true }],
      xAxis: { type: 'category', data: timeLabels, axisLine: { lineStyle: { color: tc.lineColor } }, axisLabel: { color: tc.textColor, fontSize: 10 } },
      yAxis: [
        { type: 'value', name: 'req/s', position: 'left', axisLine: { show: false }, splitLine: { lineStyle: { color: tc.lineColor, type: 'dashed' as const } }, axisLabel: { color: tc.textColor, fontSize: 10 } },
        { type: 'value', name: 'ms', position: 'right', axisLine: { show: false }, splitLine: { show: false }, axisLabel: { color: tc.textColor, fontSize: 10, formatter: '{value}ms' } },
      ],
      series: [
        { name:'QPS', type:'bar', data:node.ts_qps||[], yAxisIndex:0, itemStyle:{ color:`rgba(${isDark()?'0,229,255':'8,145,178'}, 0.25)`, borderRadius:[2,2,0,0] }, animationDelay: () => 0 },
        { name:'P50', type:'line', smooth: isSmooth, step: isSmooth?false:'middle', data:node.ts_p50||[], yAxisIndex:1, lineStyle:{width:2,color:lc[0]}, itemStyle:{color:lc[0]}, symbol:'none' },
        { name:'P90', type:'line', smooth: isSmooth, step: isSmooth?false:'middle', data:node.ts_p90||[], yAxisIndex:1, lineStyle:{width:2,color:lc[1]}, itemStyle:{color:lc[1]}, symbol:'none' },
        { name:'P95', type:'line', smooth: isSmooth, step: isSmooth?false:'middle', data:node.ts_p95||[], yAxisIndex:1, lineStyle:{width:2,color:lc[2]}, itemStyle:{color:lc[2]}, symbol:'none' },
        { name:'P99', type:'line', smooth: isSmooth, step: isSmooth?false:'middle', data:node.ts_p99||[], yAxisIndex:1, lineStyle:{width:2,color:lc[3]}, itemStyle:{color:lc[3]}, symbol:'none' },
      ]
    }, true)
  })
}

function renderErrorBreakdownChart(tc: any) {
  if (!errorBreakdownChartRef.value) return
  if (errBreakdownChart) errBreakdownChart.dispose()
  errBreakdownChart = echarts.init(errorBreakdownChartRef.value)

  const items = errorBreakdown.value
  if (!items.length) {
    errBreakdownChart.setOption({ title: { text: 'No Data', left: 'center', top: 'center', textStyle: { color: tc.textColor, fontSize: 14 } } })
    return
  }

  const sorted = [...items].sort((a, b) => (b.count || 0) - (a.count || 0))
  const labels = sorted.map(e => e.error_type || e.code || e.status || 'Unknown')
  const counts = sorted.map(e => Number(e.count || 0))
  const totalCount = Math.max(counts.reduce((s, c) => s + c, 0), 1)
  const maxCount = Math.max(...counts, 1)

  errBreakdownChart.setOption({
    backgroundColor: tc.bg,
    tooltip: {
      trigger: 'axis', axisPointer: { type: 'shadow', shadowStyle: { color: isDark() ? 'rgba(225,29,72,0.06)' : 'rgba(225,29,72,0.04)' } },
      confine: true,
      backgroundColor: isDark() ? 'rgba(22,27,34,0.96)' : 'rgba(255,255,255,0.97)',
      borderColor: isDark() ? 'rgba(48,54,61,0.8)' : 'rgba(208,215,222,0.5)',
      borderWidth: 1,
      borderRadius: 10,
      padding: [12, 16],
      textStyle: { fontSize: 12, color: isDark() ? '#e6edf3' : '#24292f' },
      extraCssText: 'box-shadow: 0 4px 16px rgba(0,0,0,0.10);',
      formatter: (params: any) => {
        const p = Array.isArray(params) ? params[0] : params
        return `<div style="font-weight:600;margin-bottom:4px">${p.name}</div><div style="font-size:13px"><strong>${Number(p.value).toLocaleString()}</strong> <span style="color:${isDark()?'#8b949e':'#656d76'}">(${(Number(p.value)/totalCount*100).toFixed(3)}%)</span></div>`
      }
    },
    grid: { left: 105, right: 36, top: 16, bottom: 12 },
    xAxis: { type: 'value', show: false },
    yAxis: {
      type: 'category',
      data: labels,
      inverse: true,
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: {
        color: isDark() ? '#c9d1d9' : '#374151',
        fontSize: 12,
        fontWeight: 500,
        width: 95,
        overflow: 'truncate',
        margin: 14,
      },
      splitLine: { show: false },
    },
    series: [{
      type: 'bar',
      data: counts.map((v, i) => ({
        value: v,
        itemStyle: {
          borderRadius: [0, 6, 6, 0],
          color: tc.colors[i % tc.colors.length],
        }
      })),
      barMaxWidth: 22,
      label: {
        show: true, position: 'right',
        formatter: (p: any) => Number(p.value).toLocaleString(),
        color: isDark() ? '#8b949e' : '#656d76',
        fontSize: 11,
        fontWeight: 600,
        offset: [8, 0]
      },
      emphasis: {
        itemStyle: {
          shadowBlur: 16,
          shadowColor: isDark() ? 'rgba(225,29,72,0.4)' : 'rgba(190,18,60,0.3)',
        }
      }
    }],
  })
}

// ===== Formatting helpers =====
function formatSuccessRate(v: any): string {
  if (v == null) return '-'
  const n = Number(v)
  if (isNaN(n)) return '-'
  return n.toFixed(3) + '%'
}

function fmtLatency3(v: any): string {
  if (v == null || v === '' || v === 0) return '-'
  const ms = Number(v) * 1000
  if (isNaN(ms)) return '-'
  if (ms >= 1000) return (ms / 1000).toFixed(3) + 's'
  return ms.toFixed(3) + 'ms'
}

function fmtLatencyMs(v: any): string {
  if (v == null || v === '' || v === 0) return '-'
  const ms = Number(v)
  if (isNaN(ms)) return '-'
  if (ms >= 1000) return (ms / 1000).toFixed(3) + 's'
  return ms.toFixed(3) + 'ms'
}

function fmtPercent(v: any): string {
  if (v == null) return '-'
  const n = Number(v)
  if (isNaN(n)) return '-'
  return n.toFixed(3) + '%'
}

function formatTime(t: any): string {
  if (!t) return '-'
  try {
    const d = new Date(t)
    if (isNaN(d.getTime())) return String(t)
    const pad = (n: number) => String(n).padStart(2, '0')
    return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
  } catch { return String(t) }
}

function formatTimeShort(t: string): string {
  if (!t) return ''
  try {
    const d = new Date(t)
    if (isNaN(d.getTime())) return t
    const pad = (n: number) => String(n).padStart(2, '0')
    return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
  } catch { return t }
}

function getRunModeLabel(): string {
  const m = metrics.value
  if (!m) return '-'
  switch (m.run_mode) {
    case 'duration': return 'Duration Mode'
    case 'count': return 'Count Mode'
    default: return m.run_mode || '-'
  }
}

function getPlannedDuration(): string {
  return String(metrics.value?.planned_duration || metrics.value?.duration || '-')
}

function getPlannedCount(): string {
  return String(metrics.value?.planned_count || metrics.value?.total_requests || '-')
}

function getStatusLabel(status: string): string {
  const map: Record<string, string> = {
    completed: 'Completed',
    running: 'Running',
    failed: 'Failed',
    partial: 'Partial',
    pending: 'Pending',
    cancelled: 'Cancelled',
    canceled: 'Cancelled',
  }
  return map[status] || status
}

function getStatusTooltip(status: string): string {
  const map: Record<string, string> = {
    success: '全部请求成功（100% 成功率）',
    partial: '部分失败：成功率 ≥95% 但 <100%（存在少量失败请求）',
    failed: '测试失败：成功率 <95% 或运行时发生错误',
    completed: '测试运行已成功完成',
    running: '测试正在运行中',
    pending: '测试等待开始',
    cancelled: '测试在完成前被取消',
    canceled: '测试在完成前被取消',
  }
  return map[status] || status
}

async function exportHTML() {
  const reportId = route.params.id as string
  if (!reportId) return
  try {
    const token = localStorage.getItem('salvo_token')
    const resp = await axios.get(`/api/v1/reports/${reportId}/export`, {
      responseType: 'blob',
      headers: token ? { Authorization: `Bearer ${token}` } : {}
    })
    const blob = resp.data as Blob
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    const contentDisposition = resp.headers['content-disposition'] || ''
    const match = contentDisposition.match(/filename=(.+)/)
    link.download = match ? decodeURIComponent(match[1].replace(/"/g, '')) : `report-${reportId}.html`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(url)
  } catch (err) {
    console.error('Export HTML failed:', err)
  }
}

// ===== Resize handling =====
let resizeTimer: ReturnType<typeof setTimeout> | null = null
window.addEventListener('resize', () => {
  if (resizeTimer) clearTimeout(resizeTimer)
  resizeTimer = setTimeout(() => {
    qpsChart?.resize(); latTrendChart?.resize(); latChart?.resize(); ovChart?.resize(); errRateChart?.resize()
    nodeCharts.forEach(c => c.resize())
  }, 150)
})

onMounted(() => {
  fetchReport()
  themeObserver = new MutationObserver(() => {
    qpsChart?.dispose(); qpsChart = null
    latTrendChart?.dispose(); latTrendChart = null
    latChart?.dispose(); latChart = null
    ovChart?.dispose(); ovChart = null
    errRateChart?.dispose(); errRateChart = null
    errBreakdownChart?.dispose(); errBreakdownChart = null
    nodeCharts.forEach((c) => { c.dispose() })
    nodeCharts.clear()
    requestAnimationFrame(() => renderAll())
  })
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] })
})
onUnmounted(() => {
  themeObserver?.disconnect()
  qpsChart?.dispose(); latTrendChart?.dispose(); latChart?.dispose(); ovChart?.dispose(); errRateChart?.dispose()
  sysGoroutineChart?.dispose(); sysHeapChart?.dispose(); sysCpuChart?.dispose(); sysTaskWaitChart?.dispose()
  nodeCharts.forEach(c => c.dispose())
})
</script>

<style scoped>
/* ===== Base ===== */
.report-detail { display: flex; flex-direction: column; gap: 20px; max-width: 1280px; margin: 0 auto; }

.page-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--border-secondary);
}
.page-header h2 { font-size: 18px; font-weight: 600; flex: 1; }

.btn-back {
  display: flex; align-items: center; gap: 6px;
  padding: 6px 12px;
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--text-secondary);
  font-size: 13px;
  cursor: pointer;
  transition: all 0.15s;
}
.btn-back:hover { background: var(--bg-hover); border-color: var(--accent-primary); color: var(--accent-primary); }

.btn-export {
  display: flex; align-items: center; gap: 6px;
  padding: 7px 16px;
  border: 1px solid var(--accent-primary);
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--accent-primary);
  font-size: 13px; font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;
}
.btn-export:hover:not(:disabled) { background: rgba(0,229,255,0.08); }
.btn-export:disabled { opacity: 0.4; cursor: not-allowed; }

/* ===== Metrics Row ===== */
.metrics-row { display: grid; grid-template-columns: repeat(4, 1fr); gap: clamp(8px, 1.5vw, 16px); margin-bottom: 16px; }

.metric-card {
  background: var(--bg-card);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-md);
  padding: 16px 18px;
  text-align: center;
}
.metric-label { font-size: 11px; text-transform: uppercase; letter-spacing: 0.06em; color: var(--text-tertiary); margin-bottom: 6px; }
.metric-value { font-size: 24px; font-weight: 700; line-height: 1.2; }
.metric-sub { font-size: 11px; color: var(--text-tertiary); margin-top: 4px; }

/* ===== Chart Cards ===== */
.charts-row { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; margin-bottom: 16px; }
.chart-card {
  background: var(--bg-card);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-md);
  overflow: hidden;
}
.chart-card.wide { grid-column: span 2; }
.chart-header { padding: 14px 16px 10px; display: flex; align-items: baseline; justify-content: space-between; flex-wrap: wrap; }
.chart-header h3 { font-size: 13px; font-weight: 600; color: var(--text-secondary); }
.chart-tip { font-size: 11px; color: var(--text-tertiary); }
.chart-body { height: 260px; }

/* ===== Info Section ===== */
.info-section { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
.info-card {
  background: var(--bg-card);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-md);
  padding: 16px;
}
.info-card h3 { font-size: 13px; font-weight: 600; color: var(--text-secondary); margin-bottom: 12px; }

.info-table { width: 100%; border-collapse: collapse; }
.info-table td { padding: 7px 0; font-size: 13px; border-bottom: 1px solid var(--border-secondary); }
.info-table tr:last-child td { border-bottom: none; }
.info-label { color: var(--text-secondary); min-width: 110px; }
.mono-sm { font-family: monospace; font-size: 12px; }

.mode-tag {
  display: inline-block;
  padding: 2px 10px;
  border-radius: var(--radius-sm);
  font-size: 12px;
  font-weight: 600;
  background: rgba(0,229,255,0.1);
  color: var(--accent-primary);
}
.actual-val { color: var(--accent-success); font-weight: 500; }

.status-badge {
  font-size: 11px;
  padding: 3px 10px;
  border-radius: var(--radius-sm);
  font-weight: 600;
  letter-spacing: 0.04em;
}
.status-badge.success { background: rgba(163,230,53,0.15); color: #65a30d; }
.status-badge.partial { background: rgba(251,146,60,0.15); color: #ea580c; }
.status-badge.failed { background: rgba(225,29,72,0.15); color: #be123c; }
.status-badge.running { background: rgba(0,229,255,0.15); color: #0891b2; }
.status-badge.pending { background: rgba(139,148,158,0.15); color: #656d76; }
.status-badge.canceled { background: rgba(139,148,158,0.12); color: #8b949e; }

/* Performance List */
.perf-list { display: flex; flex-direction: column; gap: 6px; }
.perf-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 7px 12px;
  background: var(--bg-tertiary);
  border-radius: var(--radius-sm);
}
.perf-k { font-size: 12px; font-weight: 600; color: var(--text-tertiary); min-width: 48px; }
.perf-v { font-size: 13px; font-weight: 600; color: var(--text-primary); font-variant-numeric: tabular-nums; }
.perf-v.success { color: var(--accent-success); }
.perf-v.danger { color: var(--accent-danger); }
.perf-v.highlight-blue { color: var(--accent-info); }
.perf-divider { height: 1px; background: var(--border-secondary); margin: 4px 0; }

/* ===== Charts Toolbar ===== */
.charts-section { background: var(--bg-card); border: 1px solid var(--border-secondary); border-radius: var(--radius-md); padding: 16px; margin-bottom: 16px; }
.charts-toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; }
.charts-toolbar h3 { font-size: 14px; font-weight: 600; color: var(--text-primary); }

/* ===== Node Ranking Table ===== */
.ranking-table-wrap { overflow-x: auto; -webkit-overflow-scrolling: touch; }
.ranking-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}
.ranking-table thead th {
  padding: 10px 12px;
  text-align: left;
  font-weight: 600;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text-tertiary);
  border-bottom: 2px solid var(--border-secondary);
  white-space: nowrap;
  background: var(--bg-tertiary);
}
.ranking-table tbody td {
  padding: 9px 12px;
  border-bottom: 1px solid var(--border-secondary);
  color: var(--text-secondary);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.ranking-table tbody tr:hover td { background: var(--bg-hover); }
.rank-cell { font-weight: 700; color: var(--text-tertiary); width: 32px; }
.node-name-cell { font-weight: 500; color: var(--text-primary); max-width: 180px; overflow: hidden; text-overflow: ellipsis; }
.success-text { color: var(--accent-success); font-weight: 600; }
.danger-text { color: var(--accent-danger); font-weight: 600; }

/* ===== Nodes Section ===== */
.nodes-section { background: var(--bg-card); border: 1px solid var(--border-secondary); border-radius: var(--radius-md); padding: 16px; margin-bottom: 16px; }
.nodes-section h3 { font-size: 14px; font-weight: 600; color: var(--text-primary); margin-bottom: 12px; }
.node-badges { display: flex; gap: 8px; flex-wrap: wrap; }
.node-badge {
  padding: 2px 10px;
  border-radius: var(--radius-sm);
  font-size: 11px;
  font-weight: 600;
  background: var(--bg-tertiary);
  color: var(--text-secondary);
}
.node-chart-body { height: 280px; }

/* ===== Empty State ===== */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 80px 20px;
  gap: 16px;
  color: var(--text-tertiary);
}
.spinner {
  width: 32px; height: 32px;
  border: 3px solid var(--border-secondary);
  border-top-color: var(--accent-primary);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }

/* ===== Chart Type Toggle ===== */
.chart-type-toggle {
  display: flex;
  gap: 6px;
  justify-content: center;
  margin-top: 8px;
  padding-bottom: 4px;
}
.type-btn {
  padding: 3px 12px;
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--text-secondary);
  font-size: 11px;
  cursor: pointer;
  transition: all 0.15s ease;
}
.type-btn.active {
  background: var(--accent-primary);
  color: #fff;
  border-color: var(--accent-primary);
}
[data-theme='dark'] .type-btn.active {
  background: rgba(0,229,255,0.15);
  color: #00E5FF;
  border-color: rgba(0,229,255,0.3);
}
.type-btn:hover:not(.active) {
  background: var(--bg-hover);
  border-color: var(--accent-primary);
}

/* ===== Tooltip ===== */
.tooltip-wrapper {
  position: relative;
  cursor: help;
  display: inline-block;
}

.tooltip-wrapper::before {
  content: attr(data-tooltip);
  position: absolute;
  bottom: calc(100% + 10px);
  left: 50%;
  transform: translateX(-50%) translateY(6px);
  background: rgba(255, 255, 255, 0.96);
  color: #1e293b;
  font-size: 11.5px;
  padding: 8px 16px;
  border-radius: 8px;
  border: 1px solid rgba(0, 0, 0, 0.08);
  box-shadow:
    0 4px 6px -1px rgba(0, 0, 0, 0.1),
    0 10px 24px -4px rgba(0, 0, 0, 0.12);
  opacity: 0;
  visibility: hidden;
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  pointer-events: none;
  z-index: 1000;
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  font-weight: 500;
  letter-spacing: 0.3px;
  white-space: nowrap;
  text-align: center;
  line-height: 1.4;
}

.tooltip-wrapper::after {
  content: '';
  position: absolute;
  bottom: calc(100% + 4px);
  left: 50%;
  transform: translateX(-50%);
  border: 6px solid transparent;
  border-top-color: rgba(255, 255, 255, 0.96);
  filter: drop-shadow(0 -2px 2px rgba(0, 0, 0, 0.04));
  opacity: 0;
  visibility: hidden;
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  pointer-events: none;
  z-index: 1001;
}

.tooltip-wrapper:hover::before {
  opacity: 1;
  visibility: visible;
  transform: translateX(-50%) translateY(0);
}

.tooltip-wrapper:hover::after {
  opacity: 1;
  visibility: visible;
}

/* ===== Responsive ===== */
@media (max-width: 1100px) {
  .metrics-row { grid-template-columns: repeat(4, 1fr); gap: 10px; }
}
@media (max-width: 900px) {
  .info-section { grid-template-columns: 1fr; }
  .charts-row { grid-template-columns: 1fr; }
  .chart-card.wide { grid-column: span 1; }
  .metrics-row { grid-template-columns: repeat(2, 1fr); gap: 10px; }
}
@media (max-width: 520px) {
  .metrics-row { grid-template-columns: 1fr 1fr; gap: 8px; }
}

/* System Performance Section */
.sys-summary-row {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}

.sys-summary-card {
  flex: 1;
  min-width: 140px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-md);
  padding: 12px 16px;
  text-align: center;
}

.sys-summary-label {
  font-size: 11px;
  color: var(--text-secondary);
  margin-bottom: 4px;
  font-weight: 500;
}

.sys-summary-value {
  font-size: 20px;
  font-weight: 700;
  color: var(--text-primary);
  font-family: -apple-system, 'SF Mono', 'Monaco', 'Menlo', monospace;
}

.sys-summary-sub {
  font-size: 10px;
  color: var(--text-tertiary);
  margin-top: 2px;
}

.sys-charts-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

/* System Metrics Data Table */
.sys-table-section {
  margin-top: 16px;
}

.info-banner {
  display: flex;
  gap: 12px;
  padding: 16px;
  margin-bottom: 16px;
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.05), rgba(147, 197, 253, 0.08));
  border: 1px solid rgba(59, 130, 246, 0.2);
  border-radius: 8px;
}

.banner-icon {
  font-size: 20px;
  flex-shrink: 0;
}

.banner-text {
  flex: 1;
}

.banner-text strong {
  display: block;
  font-size: 14px;
  color: var(--text-primary);
  margin-bottom: 4px;
}

.banner-text p {
  margin: 0;
  font-size: 13px;
  color: var(--text-secondary);
  line-height: 1.5;
}

.table-header-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.table-header-row h4 {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
}

.table-info {
  font-size: 12px;
  color: var(--text-secondary);
}

.table-wrapper {
  overflow-x: auto;
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: 8px;
  background: var(--bg-card, white);
}

.data-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.data-table thead {
  background: var(--bg-secondary, #f9fafb);
  position: sticky;
  top: 0;
  z-index: 10;
}

.data-table th {
  padding: 10px 12px;
  text-align: left;
  font-weight: 600;
  color: var(--text-primary);
  border-bottom: 2px solid var(--border-color, #e5e7eb);
  white-space: nowrap;
  user-select: none;
}

.data-table th.sortable {
  cursor: pointer;
  transition: background-color 0.2s;
}

.data-table th.sortable:hover {
  background: var(--bg-hover, #f3f4f6);
}

.sort-icon {
  margin-left: 4px;
  opacity: 0.5;
  font-size: 11px;
}

.data-table td {
  padding: 8px 12px;
  border-bottom: 1px solid var(--border-light, #f3f4f6);
  color: var(--text-primary);
}

.data-table tbody tr:hover {
  background: var(--bg-hover, #f9fafb);
}

.danger-row {
  background: rgba(239, 68, 68, 0.05) !important;
}

.danger-cell {
  color: #dc2626 !important;
  font-weight: 600;
}

.pagination-controls {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 8px;
  margin-top: 16px;
}

.page-btn {
  padding: 6px 12px;
  font-size: 12px;
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: 6px;
  background: var(--bg-card, white);
  color: var(--text-primary);
  cursor: pointer;
  transition: all 0.2s;
}

.page-btn:hover:not(:disabled) {
  background: var(--bg-hover, #f3f4f6);
  border-color: var(--accent-primary, #3b82f6);
}

.page-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.page-info {
  font-size: 12px;
  color: var(--text-secondary);
  min-width: 100px;
  text-align: center;
}

</style>
