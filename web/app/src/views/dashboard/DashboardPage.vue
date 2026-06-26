<template>
  <div class="dashboard-page">
    <div class="scene-selector">
      <div class="selector-header">
        <h3>场景选择</h3>
        <select v-model="selectedSceneId" @change="onSceneChange" class="scene-select">
          <option 
            v-for="scene in allScenes" 
            :key="scene.scene_id" 
            :value="String(scene.scene_id)"
          >
            {{ `场景-${String(scene.scene_id).slice(-8)}` }}
            {{ scene.status === 'running' ? '(运行中)' : `(已结束)` }}
          </option>
          <option v-if="allScenes.length === 0 && loading" value="" disabled>加载中...</option>
          <option v-else-if="allScenes.length === 0" value="" disabled>暂无场景</option>
        </select>
      </div>
      <div v-if="selectedSceneId" class="time-window-row">
        <div class="time-window-info">
          <span class="window-label">时间范围:</span>
          <span class="window-value">{{ timeWindowDisplay }}</span>
          <span v-if="durationDisplay" class="duration-value"> | 持续: {{ durationDisplay }}</span>
          <span v-if="isSceneRunning" class="live-indicator">● 实时</span>
        </div>
        <div v-if="showRefreshSelector" class="refresh-selector">
          <span class="refresh-label">刷新:</span>
          <select v-model="refreshInterval" @change="onRefreshIntervalChange" class="refresh-select">
            <option v-for="sec in refreshOptions" :key="sec" :value="sec">{{ sec }}秒</option>
          </select>
        </div>
      </div>
    </div>

    <div class="metrics-row">
      <div class="metric-card" v-for="m in summaryMetrics" :key="m.label" :class="{ 'time-info-card': m.isTimeCard }">
        <div class="metric-label">{{ m.label }}</div>
        <div v-if="!m.isTimeCard" class="metric-value" :style="{ color: m.color }">{{ m.value }}</div>
        <div v-if="m.isTimeCard" class="metric-value" :style="{ color: m.color }">{{ m.value }}</div>
        <div v-if="m.sub && !m.isTimeCard" class="metric-sub">{{ m.sub }}</div>
      </div>
    </div>

    <div class="charts-row">
      <div class="chart-card">
        <div class="chart-header">
          <h3>QPS</h3>
          <div class="chart-tip">拖动下方滑块或使用鼠标框选查看特定时间范围</div>
        </div>
        <div class="chart-body" ref="qpsChartRef"></div>
        <div class="chart-type-toggle center">
          <button :class="['type-btn', { active: chartTypes.qpsTrend === 'smooth' }]" @click="switchChartType('qpsTrend', 'smooth')">平滑</button>
          <button :class="['type-btn', { active: chartTypes.qpsTrend === 'step' }]" @click="switchChartType('qpsTrend', 'step')">阶梯</button>
        </div>
      </div>

      <div class="chart-card">
        <div class="chart-header">
          <h3>延迟分布</h3>
          <div class="chart-tip">点击图例可显示/隐藏对应线条</div>
        </div>
        <div class="chart-body" ref="latencyChartRef"></div>
        <div class="chart-type-toggle center">
          <div class="type-btn-group">
            <button :class="['type-btn', { active: latencyDataSource === 'full' }]" @click="latencyDataSource = 'full'; renderLatencyChart()">端到端</button>
            <button :class="['type-btn', { active: latencyDataSource === 'httpOnly' }]" @click="latencyDataSource = 'httpOnly'; renderLatencyChart()">纯HTTP</button>
          </div>
          <div class="type-btn-group">
            <button :class="['type-btn', { active: chartTypes.latTrend === 'smooth' }]" @click="switchChartType('latTrend', 'smooth')">平滑</button>
            <button :class="['type-btn', { active: chartTypes.latTrend === 'step' }]" @click="switchChartType('latTrend', 'step')">阶梯</button>
          </div>
        </div>
      </div>
    </div>

    <div class="charts-row">
      <div class="chart-card wide">
        <div class="chart-header">
          <h3>错误率</h3>
          <div class="chart-tip">拖动下方滑块或使用鼠标框选查看特定时间范围</div>
        </div>
        <div class="chart-body" ref="errorChartRef"></div>
        <div class="chart-type-toggle center">
          <button :class="['type-btn', { active: chartTypes.errorRate === 'smooth' }]" @click="switchChartType('errorRate', 'smooth')">平滑</button>
          <button :class="['type-btn', { active: chartTypes.errorRate === 'step' }]" @click="switchChartType('errorRate', 'step')">阶梯</button>
        </div>
      </div>
    </div>

    <div class="bottom-section">
      <div class="card card-full">
        <h3>最近运行</h3>
        <div class="run-list">
          <div v-if="overview?.recent_runs?.length === 0" class="empty">暂无运行记录</div>
          <div v-for="run in overview?.recent_runs || []" :key="run.id" class="run-item">
            <div class="run-info">
              <span class="run-name">Scene #{{ run.scene_id }}</span>
              <span :class="['run-status', run.status]">{{ run.status }}</span>
            </div>
            <div class="run-metrics">
              <span>QPS: {{ formatNum(run.total_reqs / Math.max(run.duration, 1)) }}</span>
              <span>P99: {{ formatMs(run.p99_latency) }}</span>
              <span>成功率: {{ ((run.success_reqs / Math.max(run.total_reqs, 1)) * 100).toFixed(2) }}%</span>
            </div>
          </div>
        </div>
      </div>

      <div class="card card-full">
        <div class="node-section-header">
          <h3>节点指标</h3>
          <span class="time-range-label" v-if="overview?.recent_runs?.length">时间区间: {{ runTimeRange }}</span>
        </div>
        <div v-if="!overview?.node_metrics?.length" class="empty">暂无节点数据</div>
        <div class="node-grid" v-else>
          <div v-for="node in overview?.node_metrics || []" :key="node.node_id" class="node-card" :class="{ expanded: expandedNodeId === node.node_id }" @click="toggleNodeExpand(node.node_id)">
            <div class="node-header">
              <span class="node-name">{{ node.name }}</span>
              <span class="node-id">ID: {{ node.node_id.slice(-8) }}</span>
              <span :class="['node-type', node.type]">{{ node.type }}</span>
              <span class="expand-icon">{{ expandedNodeId === node.node_id ? '▼' : '▶' }}</span>
            </div>
            <div class="node-qps">
              <span class="qps-label">QPS</span>
              <span class="qps-value">{{ nodeQPS(node) }}</span>
            </div>
            <div class="node-bars">
              <div class="bar-row">
                <span class="bar-label">P50</span>
                <div class="bar-track bar-tooltip" data-tooltip="全周期中位数：整个测试期间所有请求的P50延迟"><div class="bar-fill p50" :style="{ width: barWidth(node.p50_latency, getNodeMaxLatency(node)) }"></div></div>
                <span class="bar-value">{{ formatMs(node.p50_latency) }}</span>
              </div>
              <div class="bar-row">
                <span class="bar-label">P95</span>
                <div class="bar-track bar-tooltip" data-tooltip="全周期P95：整个测试期间所有请求的P95延迟"><div class="bar-fill p95" :style="{ width: barWidth(node.p95_latency, getNodeMaxLatency(node)) }"></div></div>
                <span class="bar-value">{{ formatMs(node.p95_latency) }}</span>
              </div>
              <div class="bar-row">
                <span class="bar-label">P99</span>
                <div class="bar-track bar-tooltip" data-tooltip="全周期P99：整个测试期间所有请求的P99延迟"><div class="bar-fill p99" :style="{ width: barWidth(node.p99_latency, getNodeMaxLatency(node)) }"></div></div>
                <span class="bar-value">{{ formatMs(node.p99_latency) }}</span>
              </div>
            </div>
            <div v-show="expandedNodeId === node.node_id" class="node-detail" @click.stop>
              <div class="node-time-range">
                <span class="time-range-label">时间区间:</span>
                <span class="time-range-value">{{ getNodeTimeRange(node) }}</span>
              </div>
              <div class="detail-grid">
                <div class="detail-item"><span class="detail-label">总请求数</span><span class="detail-val">{{ formatNum(node.total_reqs) }}</span></div>
                <div class="detail-item"><span class="detail-label">成功数</span><span class="detail-val success">{{ formatNum(node.success_reqs) }}</span></div>
                <div class="detail-item"><span class="detail-label">失败数</span><span class="detail-val danger">{{ formatNum(node.total_reqs - node.success_reqs) }}</span></div>
                <div class="detail-item"><span class="detail-label">成功率</span><span class="detail-val">{{ node.total_reqs > 0 ? ((node.success_reqs / node.total_reqs) * 100).toFixed(2) : '0' }}%</span></div>
                <div class="detail-item"><span class="detail-label">平均延迟</span><span class="detail-val">{{ formatMs(node.avg_latency) }}</span></div>
              </div>
              <div class="detail-chart" :ref="el => setNodeChartRef(node.node_id, el as HTMLElement)"></div>
              <div class="chart-type-toggle center">
                <button :class="['type-btn', { active: chartTypes[`node-${node.node_id}`] === 'smooth' }]" @click.stop="switchChartType(`node-${node.node_id}`, 'smooth')">平滑</button>
                <button :class="['type-btn', { active: chartTypes[`node-${node.node_id}`] === 'step' }]" @click.stop="switchChartType(`node-${node.node_id}`, 'step')">阶梯</button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- System Monitoring Section -->
    <div v-if="overview?.system_metrics" ref="sysMonitorSectionRef" class="system-monitor-section">
      <h3 class="section-title">系统监控</h3>
      <div class="sys-gauge-row">
        <div class="sys-gauge-card" :class="gaugeStatus('goroutine')" :style="{ borderColor: gaugeColor('goroutine'), '--gauge-color': gaugeColor('goroutine') }" data-tooltip="Goroutine 数量：当前 Go 运行时活跃协程总数" :data-alert="gaugeAlert('goroutine')">
          <div class="gauge-label">Goroutines</div>
          <div class="gauge-value" :style="{ color: gaugeValueColor('goroutine') }">{{ overview.system_metrics.goroutine_count }}</div>
          <div class="gauge-unit">个</div>
          <div v-if="gaugeAlert('goroutine')" class="gauge-alert">{{ gaugeAlert('goroutine') }}</div>
        </div>
        <div class="sys-gauge-card" :class="gaugeStatus('heap')" :style="{ borderColor: gaugeColor('heap'), '--gauge-color': gaugeColor('heap') }" data-tooltip="堆内存分配量 (Heap Alloc)：Go 运行时当前已分配的堆内存大小 (MB)" :data-alert="gaugeAlert('heap')">
          <div class="gauge-label">Heap Alloc</div>
          <div class="gauge-value" :style="{ color: gaugeValueColor('heap') }">{{ overview.system_metrics.heap_alloc_mb.toFixed(1) }}</div>
          <div class="gauge-unit">MB</div>
          <div v-if="gaugeAlert('heap')" class="gauge-alert">{{ gaugeAlert('heap') }}</div>
        </div>
        <div class="sys-gauge-card" :class="gaugeStatus('cpu')" :style="{ borderColor: gaugeColor('cpu'), '--gauge-color': gaugeColor('cpu') }" data-tooltip="CPU 使用率：当前进程瞬时 CPU 占用百分比" :data-alert="gaugeAlert('cpu')">
          <div class="gauge-label">CPU</div>
          <div class="gauge-value" :style="{ color: gaugeValueColor('cpu') }">{{ overview.system_metrics.cpu_percent.toFixed(1) }}</div>
          <div class="gauge-unit">%</div>
          <div v-if="gaugeAlert('cpu')" class="gauge-alert">{{ gaugeAlert('cpu') }}</div>
        </div>
        <div class="sys-gauge-card" :class="gaugeStatus('wait')" :style="{ borderColor: gaugeColor('wait'), '--gauge-color': gaugeColor('wait') }" data-tooltip="任务等待时间 P99：99% 的任务从入队到被 Worker 取出的等待时间" :data-alert="gaugeAlert('wait')">
          <div class="gauge-label">Task Wait P99</div>
          <div class="gauge-value" :style="{ color: gaugeValueColor('wait') }">{{ overview.system_metrics.task_wait_p99_ms.toFixed(1) }}</div>
          <div class="gauge-unit">ms</div>
          <div v-if="gaugeAlert('wait')" class="gauge-alert">{{ gaugeAlert('wait') }}</div>
        </div>
        <div class="sys-gauge-card" :class="gaugeStatus('queue')" :style="{ borderColor: gaugeColor('queue'), '--gauge-color': gaugeColor('queue') }" data-tooltip="待处理队列长度：当前在队列中等待执行的任务数" :data-alert="gaugeAlert('queue')">
          <div class="gauge-label">Pending Queue</div>
          <div class="gauge-value" :style="{ color: gaugeValueColor('queue') }">{{ overview.system_metrics.pending_queue_len }}</div>
          <div class="gauge-unit">任务</div>
          <div v-if="gaugeAlert('queue')" class="gauge-alert">{{ gaugeAlert('queue') }}</div>
        </div>
        <div class="sys-gauge-card status-normal" :style="{ borderColor: gaugeColor('workers'), '--gauge-color': gaugeColor('workers') }" data-tooltip="活跃 Worker 数：当前正在执行任务的 Worker 协程数">
          <div class="gauge-label">Active Workers</div>
          <div class="gauge-value" :style="{ color: gaugeValueColor('workers') }">{{ overview.system_metrics.active_workers }}</div>
          <div class="gauge-unit">个</div>
        </div>
      </div>

      <!-- System Trend Charts -->
      <div v-if="sysMetricsHistory.length >= 2 || sysMetricsTimeSeries.length >= 2" class="sys-charts-row">
        <div class="sys-chart-item" :class="{ expanded: expandedSysChartId === 'sysGoroutine' }" @click="toggleSysChartExpand('sysGoroutine')">
          <div class="sys-chart-header">
            <span class="expand-icon">{{ expandedSysChartId === 'sysGoroutine' ? '▼' : '▶' }}</span>
            <div class="chart-type-toggle center">
              <button :class="['type-btn', { active: chartTypes.sysGoroutine === 'smooth' }]" @click.stop="switchChartType('sysGoroutine', 'smooth')">平滑</button>
              <button :class="['type-btn', { active: chartTypes.sysGoroutine === 'step' }]" @click.stop="switchChartType('sysGoroutine', 'step')">阶梯</button>
            </div>
          </div>
          <div v-show="expandedSysChartId !== 'sysGoroutine'" ref="sysGoroutineChartRef" class="sys-chart-canvas"></div>
          <div class="sys-chart-desc">当前 Go 运行时中活跃的协程数量，反映并发任务规模</div>
          <div v-show="expandedSysChartId === 'sysGoroutine'" class="sys-chart-expanded" @click.stop>
            <div ref="sysGoroutineExpandedRef" class="sys-expanded-canvas"></div>
            <div class="chart-type-toggle center">
              <button :class="['type-btn', { active: chartTypes.sysGoroutine === 'smooth' }]" @click.stop="switchChartType('sysGoroutine', 'smooth')">平滑</button>
              <button :class="['type-btn', { active: chartTypes.sysGoroutine === 'step' }]" @click.stop="switchChartType('sysGoroutine', 'step')">阶梯</button>
            </div>
          </div>
        </div>
        <div class="sys-chart-item" :class="{ expanded: expandedSysChartId === 'sysHeap' }" @click="toggleSysChartExpand('sysHeap')">
          <div class="sys-chart-header">
            <span class="expand-icon">{{ expandedSysChartId === 'sysHeap' ? '▼' : '▶' }}</span>
            <div class="chart-type-toggle center">
              <button :class="['type-btn', { active: chartTypes.sysHeap === 'smooth' }]" @click.stop="switchChartType('sysHeap', 'smooth')">平滑</button>
              <button :class="['type-btn', { active: chartTypes.sysHeap === 'step' }]" @click.stop="switchChartType('sysHeap', 'step')">阶梯</button>
            </div>
          </div>
          <div v-show="expandedSysChartId !== 'sysHeap'" ref="sysHeapChartRef" class="sys-chart-canvas"></div>
          <div class="sys-chart-desc">堆内存分配量（Heap Alloc），反映运行时内存使用情况</div>
          <div v-show="expandedSysChartId === 'sysHeap'" class="sys-chart-expanded" @click.stop>
            <div ref="sysHeapExpandedRef" class="sys-expanded-canvas"></div>
            <div class="chart-type-toggle center">
              <button :class="['type-btn', { active: chartTypes.sysHeap === 'smooth' }]" @click.stop="switchChartType('sysHeap', 'smooth')">平滑</button>
              <button :class="['type-btn', { active: chartTypes.sysHeap === 'step' }]" @click.stop="switchChartType('sysHeap', 'step')">阶梯</button>
            </div>
          </div>
        </div>
        <div class="sys-chart-item" :class="{ expanded: expandedSysChartId === 'sysCpu' }" @click="toggleSysChartExpand('sysCpu')">
          <div class="sys-chart-header">
            <span class="expand-icon">{{ expandedSysChartId === 'sysCpu' ? '▼' : '▶' }}</span>
            <div class="chart-type-toggle center">
              <button :class="['type-btn', { active: chartTypes.sysCpu === 'smooth' }]" @click.stop="switchChartType('sysCpu', 'smooth')">平滑</button>
              <button :class="['type-btn', { active: chartTypes.sysCpu === 'step' }]" @click.stop="switchChartType('sysCpu', 'step')">阶梯</button>
            </div>
          </div>
          <div v-show="expandedSysChartId !== 'sysCpu'" ref="sysCpuChartRef" class="sys-chart-canvas"></div>
          <div class="sys-chart-desc">进程 CPU 使用率，反映计算资源消耗</div>
          <div v-show="expandedSysChartId === 'sysCpu'" class="sys-chart-expanded" @click.stop>
            <div ref="sysCpuExpandedRef" class="sys-expanded-canvas"></div>
            <div class="chart-type-toggle center">
              <button :class="['type-btn', { active: chartTypes.sysCpu === 'smooth' }]" @click.stop="switchChartType('sysCpu', 'smooth')">平滑</button>
              <button :class="['type-btn', { active: chartTypes.sysCpu === 'step' }]" @click.stop="switchChartType('sysCpu', 'step')">阶梯</button>
            </div>
          </div>
        </div>
        <div class="sys-chart-item" :class="{ expanded: expandedSysChartId === 'sysTaskWait' }" @click="toggleSysChartExpand('sysTaskWait')">
          <div class="sys-chart-header">
            <span class="expand-icon">{{ expandedSysChartId === 'sysTaskWait' ? '▼' : '▶' }}</span>
            <div class="chart-type-toggle center">
              <button :class="['type-btn', { active: chartTypes.sysTaskWait === 'smooth' }]" @click.stop="switchChartType('sysTaskWait', 'smooth')">平滑</button>
              <button :class="['type-btn', { active: chartTypes.sysTaskWait === 'step' }]" @click.stop="switchChartType('sysTaskWait', 'step')">阶梯</button>
            </div>
          </div>
          <div v-show="expandedSysChartId !== 'sysTaskWait'" ref="sysTaskWaitChartRef" class="sys-chart-canvas"></div>
          <div class="sys-chart-desc">任务排队等待时间：任务提交到 Worker 接手执行的等待耗时（非网络延迟），反映 Worker 池繁忙程度</div>
          <div v-show="expandedSysChartId === 'sysTaskWait'" class="sys-chart-expanded" @click.stop>
            <div ref="sysTaskWaitExpandedRef" class="sys-expanded-canvas"></div>
            <div class="chart-type-toggle center">
              <button :class="['type-btn', { active: chartTypes.sysTaskWait === 'smooth' }]" @click.stop="switchChartType('sysTaskWait', 'smooth')">平滑</button>
              <button :class="['type-btn', { active: chartTypes.sysTaskWait === 'step' }]" @click.stop="switchChartType('sysTaskWait', 'step')">阶梯</button>
            </div>
          </div>
        </div>
        <div class="sys-chart-item" :class="{ expanded: expandedSysChartId === 'sysQueue' }" @click="toggleSysChartExpand('sysQueue')">
          <div class="sys-chart-header">
            <span class="expand-icon">{{ expandedSysChartId === 'sysQueue' ? '▼' : '▶' }}</span>
            <span class="sys-chart-title">Pending Queue</span>
          </div>
          <div v-show="expandedSysChartId !== 'sysQueue'" ref="sysQueueChartRef" class="sys-chart-canvas"></div>
          <div class="sys-chart-desc">待处理队列中积压的任务数，0 表示所有任务被即时消费</div>
          <div v-show="expandedSysChartId === 'sysQueue'" class="sys-chart-expanded" @click.stop>
            <div ref="sysQueueExpandedRef" class="sys-expanded-canvas"></div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import * as echarts from 'echarts'
import type { DashboardOverviewDTO, RunHistoryDTO, RuntimeMetricsDTO } from '@/types'

const qpsChartRef = ref<HTMLElement>()
const latencyChartRef = ref<HTMLElement>()
const errorChartRef = ref<HTMLElement>()
const sysGoroutineChartRef = ref<HTMLElement>()
const sysHeapChartRef = ref<HTMLElement>()
const sysCpuChartRef = ref<HTMLElement>()
const sysTaskWaitChartRef = ref<HTMLElement>()
const sysQueueChartRef = ref<HTMLElement>()
const sysGoroutineExpandedRef = ref<HTMLElement>()
const sysHeapExpandedRef = ref<HTMLElement>()
const sysCpuExpandedRef = ref<HTMLElement>()
const sysTaskWaitExpandedRef = ref<HTMLElement>()
const sysQueueExpandedRef = ref<HTMLElement>()

let qpsChart: echarts.ECharts | null = null
let latencyChart: echarts.ECharts | null = null
let errorChart: echarts.ECharts | null = null
let sysGoroutineChart: echarts.ECharts | null = null
let sysHeapChart: echarts.ECharts | null = null
let sysCpuChart: echarts.ECharts | null = null
let sysTaskWaitChart: echarts.ECharts | null = null
let sysQueueChart: echarts.ECharts | null = null
let sysGoroutineExpandedChart: echarts.ECharts | null = null
let sysHeapExpandedChart: echarts.ECharts | null = null
let sysCpuExpandedChart: echarts.ECharts | null = null
let sysTaskWaitExpandedChart: echarts.ECharts | null = null
let sysQueueExpandedChart: echarts.ECharts | null = null
let pollTimer: ReturnType<typeof setInterval> | null = null
let timeRefreshTimer: ReturnType<typeof setInterval> | null = null
let themeObserver: MutationObserver | null = null
let sysObserver: IntersectionObserver | null = null
const expandedNodeId = ref('')
const nodeChartRefs = new Map<string, HTMLElement>()
const nodeCharts = new Map<string, echarts.ECharts>()
const chartTypes = ref<Record<string, 'smooth' | 'step'>>({
  errorRate: 'smooth',
  qpsTrend: 'smooth',
  latTrend: 'smooth',
  sysGoroutine: 'smooth',
  sysHeap: 'smooth',
  sysCpu: 'smooth',
  sysTaskWait: 'smooth',
  sysQueue: 'smooth',
})

const latencyDataSource = ref<'full' | 'httpOnly'>('full')

// Accumulated system metrics time series for trend charts.
const sysMetricsHistory = ref<RuntimeMetricsDTO[]>([])
const MAX_SYS_HISTORY = 300
const sysMetricsTimeSeries = ref<any[]>([])
const sysChartsVisible = ref(false)
let sysChartsRendered = false
let prewarmRetryTimer: ReturnType<typeof setTimeout> | null = null
function prewarmSysCharts() {
  if (sysMetricsHistory.value.length === 0 && sysMetricsTimeSeries.value.length === 0) return
  renderSysGoroutineChart()
  if (!sysGoroutineChart) {
    if (prewarmRetryTimer) clearTimeout(prewarmRetryTimer)
    prewarmRetryTimer = setTimeout(() => prewarmSysCharts(), 100)
    return
  }
  if (prewarmRetryTimer) { clearTimeout(prewarmRetryTimer); prewarmRetryTimer = null }
  sysChartsRendered = true
  renderSysHeapChart()
  renderSysCpuChart()
  renderSysTaskWaitChart()
  renderSysQueueChart()
}

function refreshSysCharts() {
  if (!sysChartsRendered) return
  if (sysMetricsTimeSeries.value.length > 0 || sysMetricsHistory.value.length >= 2) {
    renderSysGoroutineChart()
    renderSysHeapChart()
    renderSysCpuChart()
    renderSysTaskWaitChart()
    renderSysQueueChart()
  }
}
const sysMonitorSectionRef = ref<HTMLElement | null>(null)
const expandedSysChartId = ref<string | null>(null)

function initNodeChartTypes() {
  overview.value?.node_metrics?.forEach(node => {
    chartTypes.value[`node-${node.node_id}`] = 'smooth'
  })
}

function switchChartType(chartId: string, type: 'smooth' | 'step') {
  chartTypes.value[chartId] = type
  if (chartId === 'errorRate') renderErrorChart()
  else if (chartId === 'qpsTrend') renderQpsChart()
  else if (chartId === 'latTrend') renderLatencyChart()
  else if (chartId.startsWith('node-')) renderNodeDetailChart(chartId.slice(5))
  else if (chartId === 'sysGoroutine') { renderSysGoroutineChart(); if (expandedSysChartId.value === 'sysGoroutine') renderSysExpandedChart('sysGoroutine') }
  else if (chartId === 'sysHeap') { renderSysHeapChart(); if (expandedSysChartId.value === 'sysHeap') renderSysExpandedChart('sysHeap') }
  else if (chartId === 'sysCpu') { renderSysCpuChart(); if (expandedSysChartId.value === 'sysCpu') renderSysExpandedChart('sysCpu') }
  else if (chartId === 'sysTaskWait') { renderSysTaskWaitChart(); if (expandedSysChartId.value === 'sysTaskWait') renderSysExpandedChart('sysTaskWait') }
  else if (chartId === 'sysQueue') renderSysQueueChart()
}

const overview = ref<DashboardOverviewDTO | null>(null)

const historyData = ref<RunHistoryDTO[]>([])
const pollCheckCounter = ref(0)
const selectedSceneId = ref<string>('')
const sceneList = ref<SceneInfo[]>([])
const loading = ref(true)

const refreshInterval = ref<number>(Number(localStorage.getItem('dashboard_refresh_interval')) || 5)
const refreshOptions = [1, 5, 10, 15, 30]
const userAdjustedZoom = ref(false)

interface SceneInfo {
  scene_id: string
  name?: string
  status: 'running' | 'done' | 'failed'
  started_at?: string
  finished_at?: string
}

const runningScenes = computed<SceneInfo[]>(() => {
  return sceneList.value.filter(s => s.status === 'running')
})

const historyScenes = computed<SceneInfo[]>(() => {
  return sceneList.value.filter(s => s.status !== 'running').slice(0, 10)
})

const allScenes = computed<SceneInfo[]>(() => {
  return [...runningScenes.value, ...historyScenes.value]
})

const isSceneRunning = computed(() => {
  if (!selectedSceneId.value) return runningScenes.value.length > 0
  return runningScenes.value.some(s => s.scene_id === selectedSceneId.value)
})

const showRefreshSelector = computed(() => {
  return overview.value?.time_series?.has_running === true
})

const timeWindowDisplay = computed(() => {
  if (!selectedSceneId.value) return '全部场景'

  const runs = overview.value?.recent_runs
  if (!runs?.length) return '-'

  const sceneRuns = runs.filter((r: any) => String(r.scene_id) === selectedSceneId.value)
  if (!sceneRuns.length) return '-'

  const run = sceneRuns[0]
  if (!run.started_at) return '-'

  const start = formatDateTime(run.started_at)
  if (run.status === 'running') {
    return `${start} ~ now`
  }
  if (run.finished_at) {
    return `${start} ~ ${formatDateTime(run.finished_at)}`
  }
  return `${start} ~ now`
})

const durationDisplay = computed(() => {
  if (!selectedSceneId.value) return ''

  const runs = overview.value?.recent_runs
  if (!runs?.length) return ''

  const sceneRuns = runs.filter((r: any) => String(r.scene_id) === selectedSceneId.value)
  if (!sceneRuns.length) return ''

  const runningRun = sceneRuns.find((r: any) => r.status === 'running')
  if (runningRun && runningRun.started_at) {
    return formatDuration((Date.now() - new Date(runningRun.started_at).getTime()) / 1000)
  }

  const doneRun = sceneRuns[0]
  if (doneRun && doneRun.finished_at && doneRun.started_at) {
    return formatDuration((new Date(doneRun.finished_at).getTime() - new Date(doneRun.started_at).getTime()) / 1000)
  }

  return ''
})

function onSceneChange() {
  sysMetricsHistory.value = []
  sysMetricsTimeSeries.value = []
  fetchOverview()
  userAdjustedZoom.value = false
}

function onRefreshIntervalChange() {
  localStorage.setItem('dashboard_refresh_interval', String(refreshInterval.value))
  restartPolling()
}

function restartPolling() {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
  const interval = refreshInterval.value * 1000
  pollTimer = setInterval(() => {
    if (!userAdjustedZoom.value) {
      fetchOverview()
    }
  }, interval)
}

async function loadHistoryData() {
  if (!selectedSceneId.value) return
  try {
    const token = localStorage.getItem('salvo_token')
    const resp = await fetch('/api/v1/dashboard/history', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
      body: JSON.stringify({ scene_id: Number(selectedSceneId.value), limit: 5 })
    })
    const json = await resp.json()
    if (json.code === 0 && json.data?.history?.length) {
      historyData.value = json.data.history
      renderQpsChart()
      renderLatencyChart()
      renderErrorChart()
    }
  } catch (e) {
    console.error('Failed to load history data:', e)
  }
}

async function fetchSceneList() {
  try {
    loading.value = true
    const token = localStorage.getItem('salvo_token')
    const fetchResp = await fetch('/api/v1/scenes/list', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`
      },
      body: JSON.stringify({ limit: 100 })
    })

    const resp = await fetchResp.json()

    if (resp.code === 0 && resp.data?.items) {
      const scenes = resp.data.items
      sceneList.value = scenes.map((s: any) => ({
        scene_id: String(s.id),
        name: s.name || '',
        status: (s.status === 'running' ? 'running' : s.status === 'failed' ? 'failed' : 'done') as 'running' | 'done' | 'failed',
        started_at: s.started_at,
        finished_at: s.finished_at,
      }))

      if (!selectedSceneId.value && sceneList.value.length > 0) {
        // Try to find the scene with the most recent run by querying history (no scene filter)
        try {
          const historyResp = await fetch('/api/v1/dashboard/history', {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
              'Authorization': `Bearer ${token}`
            },
            body: JSON.stringify({ limit: 1 })
          })
          const historyJson = await historyResp.json()
          if (historyJson.code === 0 && historyJson.data?.history?.length > 0) {
            const latestRun = historyJson.data.history[0]
            const latestSceneId = String(latestRun.scene_id)
            // Only switch if the scene exists in the scene list
            if (sceneList.value.some(s => s.scene_id === latestSceneId)) {
              selectedSceneId.value = latestSceneId
            } else {
              const firstRunning = sceneList.value.find(s => s.status === 'running')
              selectedSceneId.value = firstRunning ? firstRunning.scene_id : sceneList.value[0].scene_id
            }
          } else {
            const firstRunning = sceneList.value.find(s => s.status === 'running')
            selectedSceneId.value = firstRunning ? firstRunning.scene_id : sceneList.value[0].scene_id
          }
        } catch {
          // Fallback: select first running scene or first scene in list
          const firstRunning = sceneList.value.find(s => s.status === 'running')
          selectedSceneId.value = firstRunning ? firstRunning.scene_id : sceneList.value[0].scene_id
        }
      }

      syncRunningStatus()
    } else {
      console.error('❌ Invalid scenes/list response:', resp)
    }
  } catch (e) {
    console.error('❌ Failed to fetch scene list:', e)
  } finally {
    loading.value = false
  }
}

function syncRunningStatus() {
  if (!overview.value?.recent_runs) return
  const runningSceneIds = new Set(
    overview.value.recent_runs
      .filter((r: any) => r.status === 'running')
      .map((r: any) => String(r.scene_id))
  )
  for (const s of sceneList.value) {
    if (runningSceneIds.has(s.scene_id)) {
      s.status = 'running'
    }
  }
}

function formatDateTime(timeStr?: string): string {
  if (!timeStr) return '-'
  const d = new Date(timeStr)
  const pad = (n: number, len: number) => String(n).padStart(len, '0')
  return `${d.getFullYear()}-${pad(d.getMonth()+1,2)}-${pad(d.getDate(),2)} ${pad(d.getHours(),2)}:${pad(d.getMinutes(),2)}:${pad(d.getSeconds(),2)}.${pad(d.getMilliseconds(),3)}`
}

function formatDuration(seconds: number): string {
  if (!seconds || seconds <= 0) return '0秒'
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const secs = Math.floor(seconds % 60)
  const pad = (n: number) => String(n).padStart(2, '0')

  if (hours > 0) {
    return `${pad(hours)}小时${pad(minutes)}分${pad(secs)}秒`
  } else if (minutes > 0) {
    return `${pad(minutes)}分${pad(secs)}秒`
  } else {
    return `${pad(secs)}秒`
  }
}

const runTimeRange = computed(() => {
  const runs = overview.value?.recent_runs
  if (!runs?.length) {
    const firstHistory = historyData.value?.[0]
    if (firstHistory?.started_at) {
      const s = formatDateTime(firstHistory.started_at)
      const e = firstHistory.finished_at ? formatDateTime(firstHistory.finished_at) : 'now'
      return `${s} ~ ${e}`
    }
    return '-'
  }
  const first = runs[runs.length - 1]
  const last = runs[0]
  const s = first.started_at ? formatDateTime(first.started_at) : '-'
  let e: string
  if (last.status === 'running') {
    e = 'now'
  } else {
    e = last.finished_at ? formatDateTime(last.finished_at) : '-'
  }
  return `${s} ~ ${e}`
})

function getNodeTimeRange(_node: any): string {
  const runningRun = overview.value?.recent_runs?.find(r => r.status === 'running')
  
  if (runningRun && overview.value?.recent_runs?.[0]?.started_at) {
    const start = formatDateTime(overview.value.recent_runs[0].started_at)
    return `${start} ~ now`
  }
  
  if (overview.value && overview.value.recent_runs && overview.value.recent_runs.length > 0) {
    const lastRun = overview.value.recent_runs[0]
    if (lastRun.started_at) {
      const start = formatDateTime(lastRun.started_at)
      if (lastRun.status === 'running') {
        return `${start} ~ now`
      } else if (lastRun.finished_at) {
        return `${start} ~ ${formatDateTime(lastRun.finished_at)}`
      } else {
        return `${start} ~ now`
      }
    }
  }
  
  return '-'
}

function toggleNodeExpand(nodeId: string) {
  const wasExpanded = expandedNodeId.value === nodeId
  expandedNodeId.value = wasExpanded ? '' : nodeId
  if (expandedNodeId.value) {
    setTimeout(() => renderNodeDetailChart(nodeId), 50)
  } else {
    requestAnimationFrame(() => {
      if (sysGoroutineChart) sysGoroutineChart.resize()
      if (sysHeapChart) sysHeapChart.resize()
      if (sysCpuChart) sysCpuChart.resize()
      if (sysTaskWaitChart) sysTaskWaitChart.resize()
      if (sysQueueChart) sysQueueChart.resize()
    })
  }
}

function setNodeChartRef(nodeId: string, el: HTMLElement | null) {
  if (el) nodeChartRefs.set(nodeId, el)
  else nodeChartRefs.delete(nodeId)
}

const summaryMetrics = computed(() => {
  const d = overview.value
  if (!d) {
    return [
      { label: '总请求数', value: '-', color: 'var(--accent-primary)', sub: '' },
      { label: '成功率', value: '-', color: 'var(--accent-success)', sub: '' },
      { label: 'P50 延迟', value: '-', color: 'var(--accent-info)', sub: '' },
      { label: 'P95 延迟', value: '-', color: 'var(--accent-warning)', sub: '' },
      { label: 'P99 延迟', value: '-', color: 'var(--accent-danger)', sub: '' },
      { label: '运行中', value: '-', color: 'var(--accent-primary)', sub: '', isTimeCard: true, timeInfo: null },
    ]
  }
  const rate = d.total_reqs > 0 ? ((d.success_reqs / d.total_reqs) * 100).toFixed(2) + '%' : '0%'

  const runningRun = d.recent_runs?.find(r => r.status === 'running')
  const timeInfo = runningRun ? {
    startedAt: runningRun.started_at ? formatDateTime(runningRun.started_at) : '-',
    finishedAt: runningRun.status === 'running' ? 'now' : (runningRun.finished_at ? formatDateTime(runningRun.finished_at) : '-'),
    duration: formatDuration(runningRun.status === 'running' ? (Date.now() - new Date(runningRun.started_at || '').getTime()) / 1000 : runningRun.duration)
  } : null

  return [
    { label: '总请求数', value: formatNum(d.total_reqs), color: 'var(--accent-primary)', sub: '' },
    { label: '成功率', value: rate, color: 'var(--accent-success)', sub: '' },
    { label: 'P50 延迟', value: formatMs(d.p50_latency), color: 'var(--accent-info)', sub: '' },
    { label: 'P95 延迟', value: formatMs(d.p95_latency), color: 'var(--accent-warning)', sub: '' },
    { label: 'P99 延迟', value: formatMs(d.p99_latency), color: 'var(--accent-danger)', sub: '' },
    { label: '运行中', value: String(d.running), color: 'var(--accent-primary)', sub: '', isTimeCard: true, timeInfo: timeInfo },
  ]
})

function formatNum(n: number): string {
  if (n >= 1e6) return (n / 1e6).toFixed(1) + 'M'
  if (n >= 1e3) return (n / 1e3).toFixed(1) + 'K'
  return Math.round(n).toString()
}

function formatMs(sec: number): string {
  if (!sec) return '0ms'
  const ms = sec * 1000
  if (ms < 1) return ms.toFixed(3) + 'ms'
  if (ms < 1000) return ms.toFixed(3) + 'ms'
  return (ms / 1000).toFixed(3) + 's'
}

function barWidth(val: number, maxVal?: number): string {
  if (!val) return '0%'
  const ms = val * 1000
  const maxMs = (maxVal || val) * 1000
  const safeMax = Math.max(maxMs, 1)
  const pct = Math.min((ms / safeMax) * 100, 100)
  return pct + '%'
}

function getNodeMaxLatency(node: any): number {
  return Math.max(
    node.p50_latency || 0,
    node.p95_latency || 0,
    node.p99_latency || 0,
    0.001
  )
}

function nodeQPS(node: any): string {
  if (node.ts_qps?.length) {
    const valid = node.ts_qps.filter((v: number) => v > 0)
    if (valid.length) return formatNum(valid.reduce((a: number, b: number) => a + b, 0) / valid.length)
  }
  if (node.total_reqs > 0 && node.avg_latency > 0) return formatNum(1 / node.avg_latency)
  return '0'
}

function gaugeStatus(metric: string): string {
  const m = overview.value?.system_metrics
  if (!m) return 'status-normal'
  switch (metric) {
    case 'goroutine':
      if (m.goroutine_count > 50000) return 'status-danger'
      if (m.goroutine_count > 10000) return 'status-warning'
      return 'status-normal'
    case 'heap':
      if (m.heap_alloc_mb > 1024) return 'status-danger'
      if (m.heap_alloc_mb > 512) return 'status-warning'
      return 'status-normal'
    case 'cpu':
      if (m.cpu_percent > 90) return 'status-danger'
      if (m.cpu_percent > 70) return 'status-warning'
      return 'status-normal'
    case 'wait':
      if (m.task_wait_p99_ms > 100) return 'status-danger'
      if (m.task_wait_p99_ms > 10) return 'status-warning'
      return 'status-normal'
    case 'queue':
      if (m.pending_queue_len > 1000) return 'status-danger'
      if (m.pending_queue_len > 100) return 'status-warning'
      return 'status-normal'
    default:
      return 'status-normal'
  }
}

function gaugeAlert(metric: string): string {
  const m = overview.value?.system_metrics
  if (!m) return ''
  switch (metric) {
    case 'goroutine':
      if (m.goroutine_count > 50000) return '🚨 超过 50K'
      if (m.goroutine_count > 10000) return '⚠️ 超过 10K'
      return ''
    case 'heap':
      if (m.heap_alloc_mb > 1024) return '🚨 超过 1GB'
      if (m.heap_alloc_mb > 512) return '⚠️ 超过 512MB'
      return ''
    case 'cpu':
      if (m.cpu_percent > 90) return '🚨 超过 90%'
      if (m.cpu_percent > 70) return '⚠️ 超过 70%'
      return ''
    case 'wait':
      if (m.task_wait_p99_ms > 60000) return '🚨 超过 60000ms'
      if (m.task_wait_p99_ms > 30000) return '⚠️ 超过 30000ms'
      return ''
    case 'queue':
      if (m.pending_queue_len > 1000) return '🚨 超过 1K'
      if (m.pending_queue_len > 500) return '⚠️ 超过 500'
      return ''
    default:
      return ''
  }
}

const gaugeColors: Record<string, string> = {
  goroutine: '#0891b2',
  heap: '#8b5cf6',
  cpu: '#d97706',
  wait: '#dc2626',
  queue: '#2563eb',
  workers: '#16a34a',
}

function gaugeColor(metric: string): string {
  return gaugeColors[metric] || '#94a3b8'
}

function gaugeValueColor(metric: string): string {
  const status = gaugeStatus(metric)
  if (status === 'status-danger') return 'var(--accent-danger, #dc2626)'
  if (status === 'status-warning') return 'var(--accent-warning, #ca8a04)'
  return gaugeColor(metric)
}

function getSysTimeLabels(): string[] {
  if (sysMetricsTimeSeries.value.length > 0) {
    return sysMetricsTimeSeries.value.map((m: any, i: number) => {
      const t = m.timestamp ? new Date(m.timestamp) : null
      if (t && !isNaN(t.getTime())) {
        return t.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
      }
      return `${i}s`
    })
  }
  return sysMetricsHistory.value.map((_, i) => {
    const t = new Date(Date.now() - (sysMetricsHistory.value.length - 1 - i) * refreshInterval.value * 1000)
    return t.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  })
}

function getSysData(key: string): any[] {
  if (sysMetricsTimeSeries.value.length > 0) {
    return sysMetricsTimeSeries.value.map((m: any) => m[key])
  }
  return sysMetricsHistory.value.map((m: any) => m[key])
}

function toggleSysChartExpand(chartId: string) {
  const wasExpanded = expandedSysChartId.value === chartId
  expandedSysChartId.value = wasExpanded ? null : chartId
  if (expandedSysChartId.value) {
    setTimeout(() => renderSysExpandedChart(chartId), 50)
  } else {
    requestAnimationFrame(() => {
      sysGoroutineChart?.resize()
      sysHeapChart?.resize()
      sysCpuChart?.resize()
      sysTaskWaitChart?.resize()
      sysQueueChart?.resize()
    })
  }
}

function renderSysExpandedChart(chartId: string) {
  const refMap: Record<string, HTMLElement | undefined> = {
    sysGoroutine: sysGoroutineExpandedRef.value,
    sysHeap: sysHeapExpandedRef.value,
    sysCpu: sysCpuExpandedRef.value,
    sysTaskWait: sysTaskWaitExpandedRef.value,
    sysQueue: sysQueueExpandedRef.value,
  }
  const el = refMap[chartId]
  if (!el) return
  const chartMap: Record<string, echarts.ECharts | null> = {
    sysGoroutine: sysGoroutineExpandedChart,
    sysHeap: sysHeapExpandedChart,
    sysCpu: sysCpuExpandedChart,
    sysTaskWait: sysTaskWaitExpandedChart,
    sysQueue: sysQueueExpandedChart,
  }
  let chart = chartMap[chartId]
  if (chart) { chart.dispose(); chart = null }
  chart = echarts.init(el)
  if (chartId === 'sysGoroutine') sysGoroutineExpandedChart = chart
  else if (chartId === 'sysHeap') sysHeapExpandedChart = chart
  else if (chartId === 'sysCpu') sysCpuExpandedChart = chart
  else if (chartId === 'sysTaskWait') sysTaskWaitExpandedChart = chart
  else if (chartId === 'sysQueue') sysQueueExpandedChart = chart

  const theme = getChartTheme()
  const isSmooth = chartTypes.value[chartId] === 'smooth'
  const labels = getSysTimeLabels()

  const baseGrid = { top: 50, right: 40, bottom: 50, left: 55 }
  const baseXAxis = { type: 'category' as const, data: labels, axisLine: { lineStyle: { color: theme.lineColor } }, axisLabel: { color: theme.textColor, fontSize: 11 } }
  const baseYAxis = { type: 'value' as const, axisLine: { show: false }, splitLine: { lineStyle: { color: theme.lineColor, type: 'dashed' } }, axisLabel: { color: theme.textColor, fontSize: 11 } }

  if (chartId === 'sysGoroutine') {
    const data = getSysData('goroutine_count')
    if (!data.length) return
    const ml = { silent: true, lineStyle: { type: 'dashed' }, data: [{ yAxis: 10000, lineStyle: { color: theme.colors.warning }, label: { formatter: '10K', color: theme.colors.warning, fontSize: 11 } }, { yAxis: 50000, lineStyle: { color: theme.colors.danger }, label: { formatter: '50K', color: theme.colors.danger, fontSize: 11 } }] }
    chart.setOption({ backgroundColor: theme.bgColor, grid: baseGrid, xAxis: baseXAxis, yAxis: baseYAxis, series: [{ name: 'Goroutines', data, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { width: 2, color: theme.colors.primary }, itemStyle: { color: theme.colors.primary }, markLine: ml }], tooltip: getTooltipConfig(), legend: { data: ['Goroutines'], textStyle: { color: theme.textColor }, top: 0 }, dataZoom: [{ type: 'slider', height: 18, bottom: 4, borderColor: 'transparent', backgroundColor: theme.lineColor, fillerColor: `rgba(14,165,233,0.15)`, handleStyle: { color: '#0ea5e9' }, textStyle: { color: theme.textColor, fontSize: 10 }, brushSelect: true }] }, true)
  } else if (chartId === 'sysHeap') {
    const allocData = getSysData('heap_alloc_mb')
    const sysData = getSysData('heap_sys_mb')
    if (!allocData.length) return
    chart.setOption({ backgroundColor: theme.bgColor, grid: baseGrid, xAxis: baseXAxis, yAxis: { ...baseYAxis, axisLabel: { ...baseYAxis.axisLabel, formatter: '{value}MB' } }, series: [{ name: 'HeapAlloc', data: allocData, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { width: 2, color: theme.colors.primary }, itemStyle: { color: theme.colors.primary } }, { name: 'HeapSys', data: sysData, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { width: 2, color: theme.colors.info, type: 'dashed' }, itemStyle: { color: theme.colors.info } }], tooltip: getTooltipConfig(), legend: { data: ['HeapAlloc', 'HeapSys'], textStyle: { color: theme.textColor }, top: 0 }, dataZoom: [{ type: 'slider', height: 18, bottom: 4, borderColor: 'transparent', backgroundColor: theme.lineColor, fillerColor: `rgba(14,165,233,0.15)`, handleStyle: { color: '#0ea5e9' }, textStyle: { color: theme.textColor, fontSize: 10 }, brushSelect: true }] }, true)
  } else if (chartId === 'sysCpu') {
    const data = getSysData('cpu_percent')
    if (!data.length) return
    const areaGrad = new echarts.graphic.LinearGradient(0, 0, 0, 1, [{ offset: 0, color: `rgba(${theme.colors.warning === '#ca8a04' ? '202,138,4' : '234,179,8'}, 0.2)` }, { offset: 1, color: `rgba(${theme.colors.warning === '#ca8a04' ? '202,138,4' : '234,179,8'}, 0.01)` }])
    const ml = { silent: true, lineStyle: { type: 'dashed' }, data: [{ yAxis: 70, lineStyle: { color: theme.colors.warning }, label: { formatter: '70%', color: theme.colors.warning, fontSize: 11, position: 'end' } }, { yAxis: 90, lineStyle: { color: theme.colors.danger }, label: { formatter: '90%', color: theme.colors.danger, fontSize: 11, position: 'end' } }] }
    chart.setOption({ backgroundColor: theme.bgColor, grid: { ...baseGrid, right: 60 }, xAxis: baseXAxis, yAxis: { ...baseYAxis, min: 0, max: 100, axisLabel: { ...baseYAxis.axisLabel, formatter: '{value}%' } }, series: [{ name: 'CPU', data, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { width: 2, color: theme.colors.warning }, itemStyle: { color: theme.colors.warning }, areaStyle: { color: areaGrad }, markLine: ml }], tooltip: getTooltipConfig(), legend: { data: ['CPU'], textStyle: { color: theme.textColor }, top: 0 }, dataZoom: [{ type: 'slider', height: 18, bottom: 4, borderColor: 'transparent', backgroundColor: theme.lineColor, fillerColor: `rgba(${theme.colors.warning === '#ca8a04' ? '202,138,6' : '234,179,8'}, 0.15)`, handleStyle: { color: theme.colors.warning }, textStyle: { color: theme.textColor, fontSize: 10 }, brushSelect: true }] }, true)
  } else if (chartId === 'sysTaskWait') {
    const p50 = getSysData('task_wait_p50_ms')
    const p95 = getSysData('task_wait_p95_ms')
    const p99 = getSysData('task_wait_p99_ms')
    if (!p50.length) return
    const ml = { silent: true, lineStyle: { type: 'dashed' }, data: [{ yAxis: 10, lineStyle: { color: theme.colors.warning }, label: { formatter: '10ms', color: theme.colors.warning, fontSize: 11, position: 'end' } }, { yAxis: 100, lineStyle: { color: theme.colors.danger }, label: { formatter: '100ms', color: theme.colors.danger, fontSize: 11, position: 'end' } }] }
    chart.setOption({ backgroundColor: theme.bgColor, grid: { ...baseGrid, right: 60 }, xAxis: baseXAxis, yAxis: { ...baseYAxis, axisLabel: { ...baseYAxis.axisLabel, formatter: '{value}ms' } }, series: [{ name: 'P50', data: p50, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { width: 2, color: theme.colors.info }, itemStyle: { color: theme.colors.info } }, { name: 'P95', data: p95, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { width: 2, color: theme.colors.warning }, itemStyle: { color: theme.colors.warning } }, { name: 'P99', data: p99, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { width: 2, color: theme.colors.danger }, itemStyle: { color: theme.colors.danger }, markLine: ml }], tooltip: getTooltipConfig(), legend: { data: ['P50', 'P95', 'P99'], textStyle: { color: theme.textColor }, top: 0 }, dataZoom: [{ type: 'slider', height: 18, bottom: 4, borderColor: 'transparent', backgroundColor: theme.lineColor, fillerColor: `rgba(220,38,38,0.15)`, handleStyle: { color: theme.colors.danger }, textStyle: { color: theme.textColor, fontSize: 10 }, brushSelect: true }] }, true)
  } else if (chartId === 'sysQueue') {
    const data = getSysData('pending_queue_len')
    if (!data.length) return
    chart.setOption({ backgroundColor: theme.bgColor, grid: baseGrid, xAxis: baseXAxis, yAxis: { ...baseYAxis, min: 0 }, series: [{ name: 'Queue', data, type: 'bar', itemStyle: { color: theme.colors.info, borderRadius: [2, 2, 0, 0] } }], tooltip: getTooltipConfig(), legend: { data: ['Queue'], textStyle: { color: theme.textColor }, top: 0 }, dataZoom: [{ type: 'slider', height: 18, bottom: 4, borderColor: 'transparent', backgroundColor: theme.lineColor, fillerColor: `rgba(37,99,235,0.15)`, handleStyle: { color: theme.colors.info }, textStyle: { color: theme.textColor, fontSize: 10 }, brushSelect: true }] }, true)
  }
}

function renderSysGoroutineChart() {
  if (!sysGoroutineChartRef.value) return
  if (!sysGoroutineChart) sysGoroutineChart = echarts.init(sysGoroutineChartRef.value)
  sysGoroutineChart.clear()
  const theme = getChartTheme()
  const isSmooth = chartTypes.value.sysGoroutine === 'smooth'
  const data = getSysData('goroutine_count')
  const labels = getSysTimeLabels()
  if (!data.length) return
  sysGoroutineChart.setOption({
    backgroundColor: theme.bgColor,
    grid: { top: 30, right: 20, bottom: 50, left: 50 },
    xAxis: { type: 'category', data: labels, axisLine: { lineStyle: { color: theme.lineColor } }, axisLabel: { color: theme.textColor, fontSize: 10 } },
    yAxis: { type: 'value', axisLine: { show: false }, splitLine: { lineStyle: { color: theme.lineColor, type: 'dashed' } }, axisLabel: { color: theme.textColor, fontSize: 10 } },
    series: [{ name: 'Goroutines', data, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { color: theme.colors.primary, width: 2 }, itemStyle: { color: theme.colors.primary }, markLine: { silent: true, lineStyle: { type: 'dashed' }, data: [{ yAxis: 10000, lineStyle: { color: theme.colors.warning }, label: { formatter: '10K', color: theme.colors.warning, fontSize: 10 } }, { yAxis: 50000, lineStyle: { color: theme.colors.danger }, label: { formatter: '50K', color: theme.colors.danger, fontSize: 10 } }] } }],
    tooltip: getTooltipConfig(),
    legend: { data: ['Goroutines'], textStyle: { color: theme.textColor }, top: 0 },
  }, true)
}

function renderSysHeapChart() {
  if (!sysHeapChartRef.value) return
  if (!sysHeapChart) sysHeapChart = echarts.init(sysHeapChartRef.value)
  sysHeapChart.clear()
  const theme = getChartTheme()
  const isSmooth = chartTypes.value.sysHeap === 'smooth'
  const allocData = getSysData('heap_alloc_mb')
  const sysData = getSysData('heap_sys_mb')
  const labels = getSysTimeLabels()
  if (!allocData.length) return
  sysHeapChart.setOption({
    backgroundColor: theme.bgColor,
    grid: { top: 30, right: 20, bottom: 50, left: 50 },
    xAxis: { type: 'category', data: labels, axisLine: { lineStyle: { color: theme.lineColor } }, axisLabel: { color: theme.textColor, fontSize: 10 } },
    yAxis: { type: 'value', axisLine: { show: false }, splitLine: { lineStyle: { color: theme.lineColor, type: 'dashed' } }, axisLabel: { color: theme.textColor, fontSize: 10, formatter: '{value}MB' } },
    series: [
      { name: 'HeapAlloc', data: allocData, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { color: theme.colors.primary, width: 2 }, itemStyle: { color: theme.colors.primary } },
      { name: 'HeapSys', data: sysData, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { color: theme.colors.info, width: 2, type: 'dashed' }, itemStyle: { color: theme.colors.info } },
    ],
    tooltip: getTooltipConfig(),
    legend: { data: ['HeapAlloc', 'HeapSys'], textStyle: { color: theme.textColor }, top: 0 },
  }, true)
}

function renderSysCpuChart() {
  if (!sysCpuChartRef.value) return
  if (!sysCpuChart) sysCpuChart = echarts.init(sysCpuChartRef.value)
  sysCpuChart.clear()
  const theme = getChartTheme()
  const isSmooth = chartTypes.value.sysCpu === 'smooth'
  const data = getSysData('cpu_percent')
  const labels = getSysTimeLabels()
  if (!data.length) return
  sysCpuChart.setOption({
    backgroundColor: theme.bgColor,
    grid: { top: 30, right: 48, bottom: 50, left: 50 },
    xAxis: { type: 'category', data: labels, axisLine: { lineStyle: { color: theme.lineColor } }, axisLabel: { color: theme.textColor, fontSize: 10 } },
    yAxis: { type: 'value', min: 0, max: 100, axisLine: { show: false }, splitLine: { lineStyle: { color: theme.lineColor, type: 'dashed' } }, axisLabel: { color: theme.textColor, fontSize: 10, formatter: '{value}%' } },
    series: [{ name: 'CPU', data, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { color: theme.colors.warning, width: 2 }, itemStyle: { color: theme.colors.warning }, areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{ offset: 0, color: `rgba(${theme.colors.warning === '#ca8a04' ? '202,138,4' : '234,179,8'}, 0.2)` }, { offset: 1, color: `rgba(${theme.colors.warning === '#ca8a04' ? '202,138,4' : '234,179,8'}, 0.01)` }]) }, markLine: { silent: true, lineStyle: { type: 'dashed' }, data: [{ yAxis: 70, lineStyle: { color: theme.colors.warning }, label: { formatter: '70%', color: theme.colors.warning, fontSize: 10, position: 'end' } }, { yAxis: 90, lineStyle: { color: theme.colors.danger }, label: { formatter: '90%', color: theme.colors.danger, fontSize: 10, position: 'end' } }] } }],
    tooltip: getTooltipConfig(),
    legend: { data: ['CPU'], textStyle: { color: theme.textColor }, top: 0 },
  }, true)
}

function renderSysTaskWaitChart() {
  if (!sysTaskWaitChartRef.value) return
  if (!sysTaskWaitChart) sysTaskWaitChart = echarts.init(sysTaskWaitChartRef.value)
  sysTaskWaitChart.clear()
  const theme = getChartTheme()
  const isSmooth = chartTypes.value.sysTaskWait === 'smooth'
  const p50 = getSysData('task_wait_p50_ms')
  const p95 = getSysData('task_wait_p95_ms')
  const p99 = getSysData('task_wait_p99_ms')
  const labels = getSysTimeLabels()
  if (!p50.length) return
  sysTaskWaitChart.setOption({
    backgroundColor: theme.bgColor,
    grid: { top: 30, right: 48, bottom: 50, left: 50 },
    xAxis: { type: 'category', data: labels, axisLine: { lineStyle: { color: theme.lineColor } }, axisLabel: { color: theme.textColor, fontSize: 10 } },
    yAxis: { type: 'value', axisLine: { show: false }, splitLine: { lineStyle: { color: theme.lineColor, type: 'dashed' } }, axisLabel: { color: theme.textColor, fontSize: 10, formatter: '{value}ms' } },
    series: [
      { name: 'P50', data: p50, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { color: theme.colors.info, width: 2 }, itemStyle: { color: theme.colors.info } },
      { name: 'P95', data: p95, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { color: theme.colors.warning, width: 2 }, itemStyle: { color: theme.colors.warning } },
      { name: 'P99', data: p99, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { color: theme.colors.danger, width: 2 }, itemStyle: { color: theme.colors.danger }, markLine: { silent: true, lineStyle: { type: 'dashed' }, data: [{ yAxis: 10, lineStyle: { color: theme.colors.warning }, label: { formatter: '10ms', color: theme.colors.warning, fontSize: 10, position: 'end' } }, { yAxis: 100, lineStyle: { color: theme.colors.danger }, label: { formatter: '100ms', color: theme.colors.danger, fontSize: 10, position: 'end' } }] } },
    ],
    tooltip: getTooltipConfig(),
    legend: { data: ['P50', 'P95', 'P99'], textStyle: { color: theme.textColor }, top: 0 },
  }, true)
}

function renderSysQueueChart() {
  if (!sysQueueChartRef.value) return
  if (!sysQueueChart) sysQueueChart = echarts.init(sysQueueChartRef.value)
  sysQueueChart.clear()
  const theme = getChartTheme()
  const data = getSysData('pending_queue_len')
  const labels = getSysTimeLabels()
  if (!data.length) return
  sysQueueChart.setOption({
    backgroundColor: theme.bgColor,
    grid: { top: 16, right: 20, bottom: 50, left: 50 },
    xAxis: { type: 'category', data: labels, axisLine: { lineStyle: { color: theme.lineColor } }, axisLabel: { color: theme.textColor, fontSize: 10 } },
    yAxis: { type: 'value', min: 0, axisLine: { show: false }, splitLine: { lineStyle: { color: theme.lineColor, type: 'dashed' } }, axisLabel: { color: theme.textColor, fontSize: 10 } },
    series: [{ name: 'Queue', data, type: 'bar', itemStyle: { color: theme.colors.info, borderRadius: [2, 2, 0, 0] } }],
    tooltip: getTooltipConfig(),
  }, true)
}

function getChartTheme() {
  const isDark = document.documentElement.getAttribute('data-theme') !== 'light'
  if (isDark) {
    return {
      textColor: '#8b949e',
      lineColor: '#30363d',
      bgColor: 'transparent',
      colors: {
        primary: '#00E5FF',
        success: '#4ade80',
        warning: '#eab308',
        danger: '#ef4444',
        info: '#3b82f6',
      }
    }
  }
  return {
    textColor: '#475569',
    lineColor: '#e2e8f0',
    bgColor: 'transparent',
    colors: {
      primary: '#0891b2',
      success: '#16a34a',
      warning: '#ca8a04',
      danger: '#dc2626',
      info: '#2563eb',
    }
  }
}

function getTooltipConfig() {
  const isDark = document.documentElement.getAttribute('data-theme') !== 'light'

  const bgColor = isDark ? 'rgba(30, 41, 59, 0.95)' : 'rgba(255, 255, 255, 0.96)'
  const borderColor = isDark ? 'rgba(71, 85, 105, 0.3)' : 'rgba(148, 163, 184, 0.2)'
  const titleColor = isDark ? '#e2e8f0' : '#1e293b'
  const labelColor = isDark ? '#cbd5e1' : '#475569'
  const valueColor = isDark ? '#94a3b8' : '#64748b'

  return {
    trigger: 'axis' as const,
    backgroundColor: bgColor,
    borderColor: borderColor,
    borderWidth: 1,
    padding: [12, 16],
    textStyle: { fontSize: 11, color: labelColor },
    extraCssText: `
      border-radius: 12px;
      box-shadow:
        0 1px 3px rgba(0, 0, 0, 0.04),
        0 8px 24px rgba(0, 0, 0, 0.08);
      backdrop-filter: blur(8px);
      -webkit-backdrop-filter: blur(8px);
    `,
    formatter: (params: any[]) => {
      if (!Array.isArray(params)) params = [params]
      let result = `<div style="font-size:11.5px;color:${titleColor};margin-bottom:10px;font-weight:600;letter-spacing:0.3px">${params[0].axisValue}</div>`
      for (const param of params) {
        const rawVal = Number(param.value)
        const name = param.seriesName
        let val: string
        let unit: string
        if (name === 'QPS') { val = rawVal.toFixed(1); unit = '' }
        else if (name === '错误率' || name === 'error_rate') { val = rawVal.toFixed(2); unit = '%' }
        else if (name === 'HeapAlloc' || name === 'HeapSys') { val = rawVal.toFixed(1); unit = ' MB' }
        else if (name === 'CPU') { val = rawVal.toFixed(1); unit = '%' }
        else if (name.startsWith('P5') || name.startsWith('P9')) { val = rawVal.toFixed(1); unit = 'ms' }
        else if (name === 'Goroutines') { val = Math.round(rawVal).toLocaleString(); unit = '' }
        else { val = rawVal.toFixed(1); unit = 'ms' }
        result += `<div style="display:flex;justify-content:space-between;align-items:center;gap:24px;margin-top:6px;padding:2px 0"><span style="font-size:11px;color:${labelColor};font-weight:500">${param.marker}${name}</span><span style="font-size:11px;color:${valueColor};font-weight:600;font-family:-apple-system,'SF Mono','Monaco','Menlo',monospace;letter-spacing:0.5px">${val}${unit}</span></div>`
      }
      return result
    }
  }
}

function getFilteredTimeSeries() {
  if (latencyDataSource.value === 'httpOnly') {
    const httpTs = overview.value?.http_only_time_series
    if (httpTs && httpTs.timestamps?.length) {
      return httpTs
    }
  }

  const ts = overview.value?.time_series
  if (ts && ts.timestamps?.length) {
    return ts
  }

  const firstRun = historyData.value?.[0]
  const samples = firstRun?.global_samples
  if (!samples?.length) return null

  const timestamps = samples.map((s: any) => new Date(s.timestamp * 1000).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }))
  return {
    timestamps,
    qps: samples.map((s: any) => s.qps),
    p50: samples.map((s: any) => s.p50_latency_ms),
    p95: samples.map((s: any) => s.p95_latency_ms),
    p99: samples.map((s: any) => s.p99_latency_ms),
    error_rate: samples.map((s: any) => s.total_requests > 0 ? (s.fail_count / s.total_requests) * 100 : 0),
  }
}

function renderQpsChart() {
  if (!qpsChartRef.value) return
  if (!qpsChart) {
    qpsChart = echarts.init(qpsChartRef.value)
  }
  qpsChart.clear()
  const theme = getChartTheme()
  const isSmooth = chartTypes.value.qpsTrend === 'smooth'
  const ts = getFilteredTimeSeries()
  if (!ts || !ts.timestamps.length) {
    qpsChart.setOption({
      title: { text: '暂无数据', left: 'center', top: 'center', textStyle: { color: theme.textColor, fontSize: 14 } },
      xAxis: { show: false },
      yAxis: { show: false },
      series: [],
    })
    return
  }
  qpsChart.setOption({
    backgroundColor: theme.bgColor,
    grid: { top: 20, right: 20, bottom: 50, left: 50 },
    dataZoom: [{ type: 'slider', height: 18, bottom: 4, borderColor: 'transparent', backgroundColor: theme.lineColor, fillerColor: `rgba(${theme.colors.primary === '#0891b2' ? '8, 145, 178' : '0, 229, 255'}, 0.15)`, handleStyle: { color: theme.colors.primary }, textStyle: { color: theme.textColor, fontSize: 10 }, brushSelect: true }],
    xAxis: { type: 'category', data: ts.timestamps, axisLine: { lineStyle: { color: theme.lineColor } }, axisLabel: { color: theme.textColor, fontSize: 10 } },
    yAxis: { type: 'value', axisLine: { show: false }, splitLine: { lineStyle: { color: theme.lineColor, type: 'dashed' } }, axisLabel: { color: theme.textColor, fontSize: 10 } },
    series: [{ data: ts.qps, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { color: theme.colors.primary, width: 2 }, areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{ offset: 0, color: `rgba(${theme.colors.primary === '#0891b2' ? '8, 145, 178' : '0, 229, 255'}, 0.3)` }, { offset: 1, color: 'rgba(0,229,255,0)' }]) } }],
    tooltip: getTooltipConfig(),
  }, true)
  qpsChart.off('datazoom')
  qpsChart.on('datazoom', () => {
    userAdjustedZoom.value = true
  })
}

function renderLatencyChart() {
  if (!latencyChartRef.value) return
  if (!latencyChart) {
    latencyChart = echarts.init(latencyChartRef.value)
  }
  latencyChart.clear()
  const theme = getChartTheme()
  const isSmooth = chartTypes.value.latTrend === 'smooth'
  const ts = getFilteredTimeSeries()
  if (!ts || !ts.timestamps.length) {
    latencyChart.setOption({
      title: { text: '暂无数据', left: 'center', top: 'center', textStyle: { color: theme.textColor, fontSize: 14 } },
      xAxis: { show: false },
      yAxis: { show: false },
      series: [],
    })
    return
  }
  latencyChart.setOption({
    backgroundColor: theme.bgColor,
    grid: { top: 30, right: 20, bottom: 50, left: 50 },
    dataZoom: [{ type: 'slider', height: 18, bottom: 4, borderColor: 'transparent', backgroundColor: theme.lineColor, fillerColor: `rgba(${theme.colors.primary === '#0891b2' ? '8, 145, 178' : '0, 229, 255'}, 0.15)`, handleStyle: { color: theme.colors.primary }, textStyle: { color: theme.textColor, fontSize: 10 }, brushSelect: true }],
    xAxis: { type: 'category', data: ts.timestamps, axisLine: { lineStyle: { color: theme.lineColor } }, axisLabel: { color: theme.textColor, fontSize: 10 } },
    yAxis: { type: 'value', axisLine: { show: false }, splitLine: { lineStyle: { color: theme.lineColor, type: 'dashed' } }, axisLabel: { color: theme.textColor, fontSize: 10, formatter: '{value}ms' } },
    series: [
      { name: 'P50', data: ts.p50, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { color: theme.colors.info, width: 2 } },
      { name: 'P95', data: ts.p95, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { color: theme.colors.warning, width: 2 } },
      { name: 'P99', data: ts.p99, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { color: theme.colors.danger, width: 2 } },
    ],
    tooltip: getTooltipConfig(),
    legend: { data: ['P50', 'P95', 'P99'], textStyle: { color: theme.textColor }, top: 0 },
  }, true)
  latencyChart.off('datazoom')
  latencyChart.on('datazoom', () => {
    userAdjustedZoom.value = true
  })
}

function renderErrorChart() {
  if (!errorChartRef.value) return
  if (!errorChart) {
    errorChart = echarts.init(errorChartRef.value)
  }
  errorChart.clear()
  const theme = getChartTheme()
  const ts = getFilteredTimeSeries()
  if (!ts || !ts.timestamps.length) {
    errorChart.setOption({
      backgroundColor: theme.bgColor,
      title: { text: '暂无数据', left: 'center', top: 'center', textStyle: { color: theme.textColor, fontSize: 14 } },
      xAxis: { show: false },
      yAxis: { show: false },
      series: [],
    })
    return
  }
  const isSmooth = chartTypes.value.errorRate === 'smooth'
  const maxErrRate = Math.max(...ts.error_rate, 0.01)
  errorChart.setOption({
    backgroundColor: theme.bgColor,
    grid: { top: 16, right: 16, bottom: 44, left: 50 },
    dataZoom: [{ type: 'slider', height: 14, bottom: 2, borderColor: 'transparent', backgroundColor: theme.lineColor, fillerColor: `rgba(${theme.colors.danger === '#be123c' ? '190, 18, 60' : '225, 29, 72'}, 0.10)`, handleStyle: { color: theme.colors.danger }, textStyle: { color: theme.textColor, fontSize: 9 }, showDetail: false }],
    xAxis: { type: 'category', data: ts.timestamps, axisLine: { show: false }, axisLabel: { color: theme.textColor, fontSize: 9, interval: Math.floor(ts.timestamps.length / 8) }, splitLine: { show: false } },
    yAxis: { type: 'value', min: 0, max: maxErrRate * 1.5 > 0 ? Math.max(maxErrRate * 1.5, 1) : 1, axisLine: { show: false }, splitLine: { lineStyle: { color: theme.lineColor, type: 'dashed' } }, axisLabel: { color: theme.textColor, fontSize: 10, formatter: (v: number) => v.toFixed(2) + '%' } },
    series: [{
      name: '错误率',
      data: ts.error_rate,
      type: 'line',
      smooth: isSmooth,
      step: isSmooth ? false : 'middle',
      symbol: 'none',
      lineStyle: { width: 2, color: theme.colors.danger },
      areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{ offset: 0, color: `rgba(${theme.colors.danger === '#be123c' ? '190, 18, 60' : '225, 29, 72'}, 0.18)` }, { offset: 1, color: `rgba(${theme.colors.danger === '#be123c' ? '190, 18, 60' : '225, 29, 72'}, 0.01)` }]) }
    }],
    tooltip: getTooltipConfig(),
  }, true)
  errorChart.off('datazoom')
  errorChart.on('datazoom', () => {
    userAdjustedZoom.value = true
  })
}

function renderNodeDetailChart(nodeId: string) {
  const el = nodeChartRefs.get(nodeId)
  if (!el) return
  let chart = nodeCharts.get(nodeId)
  if (!chart) {
    chart = echarts.init(el)
    nodeCharts.set(nodeId, chart)
  }
  chart.clear()
  const node = overview.value?.node_metrics?.find(n => n.node_id === nodeId)
  if (!node) return
  const theme = getChartTheme()
  const isSmooth = chartTypes.value[`node-${nodeId}`] === 'smooth'

  const hasTimeSeries = node.timestamps && node.timestamps.length > 0

  if (hasTimeSeries) {
    const tsP50 = (node.ts_p50 || []).map(v => v * 1000)
    const tsAvg = (node.ts_avg || []).map(v => v * 1000)
    const tsP95 = (node.ts_p95 || []).map(v => v * 1000)
    const tsP99 = (node.ts_p99 || []).map(v => v * 1000)
    const tsQPS = node.ts_qps || []
    
    // Ensure all arrays have the same length as timestamps
    const padArray = (arr: number[], length: number): number[] => {
      const result = [...arr]
      while (result.length < length) {
        result.push(0)
      }
      return result.slice(0, length)
    }
    
    const timestampsLength = node.timestamps?.length || 0
    const series: any[] = [
      { name: 'P50', data: padArray(tsP50, timestampsLength), lineStyle: { color: theme.colors.info, width: 2 }, itemStyle: { color: theme.colors.info }, areaStyle: undefined },
      { name: 'Avg', data: padArray(tsAvg, timestampsLength), lineStyle: { color: theme.colors.success, width: 2 }, itemStyle: { color: theme.colors.success }, areaStyle: undefined },
      { name: 'P95', data: padArray(tsP95, timestampsLength), lineStyle: { color: theme.colors.warning, width: 2 }, itemStyle: { color: theme.colors.warning }, areaStyle: undefined },
      { name: 'P99', data: padArray(tsP99, timestampsLength), lineStyle: { color: theme.colors.danger, width: 2 }, itemStyle: { color: theme.colors.danger }, areaStyle: undefined },
    ]
    if (tsQPS.length > 0) {
      series.push({ name: 'QPS', data: padArray(tsQPS, timestampsLength), type: 'bar', lineStyle: undefined, itemStyle: { color: `rgba(${theme.colors.primary === '#0891b2' ? '8, 145, 178' : '0, 229, 255'}, 0.25)`, borderRadius: [2, 2, 0, 0] }, areaStyle: undefined, animationDelay: () => 0 })
    }

    const hasQPS = series.length > 4
    chart.setOption({
      backgroundColor: 'transparent',
      legend: { top: 0, textStyle: { color: theme.textColor, fontSize: 10 }, itemWidth: 16, itemHeight: 3 },
      grid: { top: 28, right: hasQPS ? 48 : 12, bottom: 36, left: 48 },
      dataZoom: [{ type: 'slider', height: 14, bottom: 2, borderColor: 'transparent', backgroundColor: theme.lineColor, fillerColor: `rgba(${theme.colors.primary === '#0891b2' ? '8, 145, 178' : '0, 229, 255'}, 0.15)`, handleStyle: { color: theme.colors.primary }, textStyle: { color: theme.textColor, fontSize: 9 }, showDetail: false }],
      xAxis: { type: 'category', data: node.timestamps, axisLine: { lineStyle: { color: theme.lineColor } }, axisLabel: { color: theme.textColor, fontSize: 9, interval: Math.floor((node.timestamps!.length || 1) / 6) - 1 } },
      yAxis: hasQPS ? [
        { type: 'value', axisLine: { show: false }, splitLine: { lineStyle: { color: theme.lineColor, type: 'dashed' } }, axisLabel: { color: theme.textColor, fontSize: 9, formatter: '{value}ms' } },
        { type: 'value', axisLine: { show: false }, splitLine: { show: false }, axisLabel: { color: theme.textColor, fontSize: 9, formatter: '{value}' } },
      ] : { type: 'value', axisLine: { show: false }, splitLine: { lineStyle: { color: theme.lineColor, type: 'dashed' } }, axisLabel: { color: theme.textColor, fontSize: 9, formatter: '{value}ms' } },
      tooltip: getTooltipConfig(),
      series: series.map((s) => ({
        name: s.name,
        type: s.type || 'line',
        smooth: isSmooth,
        step: isSmooth ? false : 'middle',
        symbol: 'none',
        data: s.data,
        lineStyle: s.lineStyle,
        itemStyle: s.itemStyle,
        areaStyle: s.areaStyle,
        yAxisIndex: hasQPS && s.name === 'QPS' ? 1 : 0,
      })),
    }, true)
  } else {
    chart.setOption({
      backgroundColor: 'transparent',
      grid: { top: 16, right: 16, bottom: 36, left: 48 },
      xAxis: { type: 'category', data: ['P50', 'P95', 'P99', 'Avg'], axisLine: { lineStyle: { color: theme.lineColor } }, axisLabel: { color: theme.textColor, fontSize: 10 } },
      yAxis: { type: 'value', axisLine: { show: false }, splitLine: { lineStyle: { color: theme.lineColor, type: 'dashed' } }, axisLabel: { color: theme.textColor, fontSize: 9, formatter: '{value}ms' } },
      series: [{
        data: [node.p50_latency * 1000, node.p95_latency * 1000, node.p99_latency * 1000, node.avg_latency * 1000],
        type: 'bar',
        barWidth: '40%',
        itemStyle: {
          borderRadius: [3, 3, 0, 0],
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: theme.colors.primary },
            { offset: 1, color: `rgba(${theme.colors.primary === '#0891b2' ? '8, 145, 178' : '0, 229, 255'}, 0.3)` },
          ]),
        },
      }],
      tooltip: getTooltipConfig(),
    }, true)
  }
}

async function checkLatestRunScene() {
  try {
    const token = localStorage.getItem('salvo_token')
    const historyResp = await fetch('/api/v1/dashboard/history', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`
      },
      body: JSON.stringify({ limit: 1 })
    })
    const historyJson = await historyResp.json()
    if (historyJson.code === 0 && historyJson.data?.history?.length > 0) {
      const latestRunSceneId = String(historyJson.data.history[0].scene_id)
      // Switch to the latest run's scene if different from current selection
      // The next poll will automatically fetch overview for the new scene
      if (latestRunSceneId !== selectedSceneId.value) {
        selectedSceneId.value = latestRunSceneId
      }
    }
  } catch {
    // Silently fail - keep current scene
  }
}

async function fetchOverview() {
  try {
    // Periodically check if a different scene has the most recent run
    // (every 6th poll = every 30 seconds with default 5s interval)
    pollCheckCounter.value++
    if (pollCheckCounter.value >= 6) {
      pollCheckCounter.value = 0
      await checkLatestRunScene()
    }

    const sceneId = selectedSceneId.value || undefined
    const requestData = { range_seconds: 0, scene_id: sceneId }

    const token = localStorage.getItem('salvo_token')
    const fetchResp = await fetch('/api/v1/dashboard/overview', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`
      },
      body: JSON.stringify(requestData)
    })

    const resp = await fetchResp.json()

    if (resp.code === 0) {
      overview.value = resp.data
      syncRunningStatus()
      initNodeChartTypes()

      const hasRunning = resp.data?.recent_runs?.some((r: any) => r.status === 'running')

      if (hasRunning) {
        sysMetricsTimeSeries.value = []
        if (resp.data?.system_metrics) {
          sysMetricsHistory.value.push(resp.data.system_metrics)
          if (sysMetricsHistory.value.length > MAX_SYS_HISTORY) {
            sysMetricsHistory.value = sysMetricsHistory.value.slice(-MAX_SYS_HISTORY)
          }
        }
      } else if (resp.data?.system_metrics_time_series?.length > 0) {
        sysMetricsTimeSeries.value = resp.data.system_metrics_time_series
        sysMetricsHistory.value = []
      }

      const hasTimeSeries = resp.data?.time_series?.timestamps?.length > 0
      if (!hasRunning && !hasTimeSeries) {
        loadHistoryData()
      }
      renderQpsChart()
      renderLatencyChart()
      renderErrorChart()
      prewarmSysCharts()
      refreshSysCharts()
      if (expandedNodeId.value) {
        renderNodeDetailChart(expandedNodeId.value)
      }
    }
  } catch (e) {
    console.error('❌ Dashboard fetch error:', e)
  }
}

function handleResize() {
  qpsChart?.resize()
  latencyChart?.resize()
  errorChart?.resize()
  sysGoroutineChart?.resize()
  sysHeapChart?.resize()
  sysCpuChart?.resize()
  sysTaskWaitChart?.resize()
  sysQueueChart?.resize()
  if (expandedSysChartId.value) {
    const id = expandedSysChartId.value
    const map: Record<string, echarts.ECharts | null> = { sysGoroutine: sysGoroutineExpandedChart, sysHeap: sysHeapExpandedChart, sysCpu: sysCpuExpandedChart, sysTaskWait: sysTaskWaitExpandedChart, sysQueue: sysQueueExpandedChart }
    map[id]?.resize()
  }
}

onMounted(async () => {
  await fetchSceneList()
  fetchOverview()
  loadHistoryData()

  setTimeout(() => {
    renderQpsChart()
    renderLatencyChart()
    renderErrorChart()
    prewarmSysCharts()
  }, 100)
  window.addEventListener('resize', handleResize)

  themeObserver = new MutationObserver(() => {
    qpsChart?.dispose(); qpsChart = null
    latencyChart?.dispose(); latencyChart = null
    errorChart?.dispose(); errorChart = null
    sysGoroutineChart?.dispose(); sysGoroutineChart = null
    sysHeapChart?.dispose(); sysHeapChart = null
    sysCpuChart?.dispose(); sysCpuChart = null
    sysTaskWaitChart?.dispose(); sysTaskWaitChart = null
    sysQueueChart?.dispose(); sysQueueChart = null
    sysGoroutineExpandedChart?.dispose(); sysGoroutineExpandedChart = null
    sysHeapExpandedChart?.dispose(); sysHeapExpandedChart = null
    sysCpuExpandedChart?.dispose(); sysCpuExpandedChart = null
    sysTaskWaitExpandedChart?.dispose(); sysTaskWaitExpandedChart = null
    sysQueueExpandedChart?.dispose(); sysQueueExpandedChart = null
    nodeCharts.forEach((c) => { c.dispose() })
    nodeCharts.clear()
    if (prewarmRetryTimer) { clearTimeout(prewarmRetryTimer); prewarmRetryTimer = null }
    sysChartsRendered = false
    requestAnimationFrame(() => {
      renderQpsChart()
      renderLatencyChart()
      renderErrorChart()
      prewarmSysCharts()
      if (expandedNodeId.value) renderNodeDetailChart(expandedNodeId.value)
      if (expandedSysChartId.value) renderSysExpandedChart(expandedSysChartId.value)
    })
  })
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] })
  
  watch(allScenes, (scenes) => {
    if (scenes.length > 0 && !selectedSceneId.value) {
      const firstRunning = scenes.find(s => s.status === 'running')
      if (firstRunning) {
        selectedSceneId.value = firstRunning.scene_id
        console.log('🔄 Auto-selected running scene:', selectedSceneId.value)
      } else {
        selectedSceneId.value = scenes[0].scene_id
      }
    }
  }, { immediate: true })
  
  restartPolling()

  sysObserver = new IntersectionObserver((entries) => {
    for (const entry of entries) {
      if (entry.isIntersecting) {
        sysChartsVisible.value = true
        sysObserver?.disconnect()
      }
    }
  }, { rootMargin: '800px 0px' })

  watch(sysChartsVisible, (visible) => {
    if (!visible) return
    prewarmSysCharts()
  }, { once: true })

  nextTick(() => {
    if (sysMonitorSectionRef.value) {
      sysObserver?.observe(sysMonitorSectionRef.value)
    } else {
      sysChartsVisible.value = true
    }
  })

  timeRefreshTimer = setInterval(() => {
    if (overview.value?.recent_runs?.some(r => r.status === 'running')) {
      overview.value = { ...overview.value }
    }
  }, 1000)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  themeObserver?.disconnect()
  sysObserver?.disconnect()
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
  if (timeRefreshTimer) { clearInterval(timeRefreshTimer); timeRefreshTimer = null }
  if (prewarmRetryTimer) { clearTimeout(prewarmRetryTimer); prewarmRetryTimer = null }
  qpsChart?.dispose()
  latencyChart?.dispose()
  errorChart?.dispose()
  sysGoroutineChart?.dispose()
  sysHeapChart?.dispose()
  sysCpuChart?.dispose()
  sysTaskWaitChart?.dispose()
  sysQueueChart?.dispose()
  sysGoroutineExpandedChart?.dispose()
  sysHeapExpandedChart?.dispose()
  sysCpuExpandedChart?.dispose()
  sysTaskWaitExpandedChart?.dispose()
  sysQueueExpandedChart?.dispose()
})
</script>

<style scoped>
.dashboard-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.scene-selector {
  background: var(--bg-card);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-md);
  padding: 16px;
}

.selector-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.selector-header h3 {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
}

.scene-select {
  padding: 8px 12px;
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-sm);
  background: var(--bg-tertiary);
  color: var(--text-primary);
  font-size: 13px;
  cursor: pointer;
  min-width: 250px;
  transition: all 0.2s;
}

.scene-select:hover {
  border-color: var(--accent-primary);
}

.scene-select:focus {
  outline: none;
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 3px rgba(0,229,255,0.1);
}

.scene-tabs {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}

.scene-tab {
  padding: 6px 14px;
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-sm);
  background: var(--bg-tertiary);
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
}

.scene-tab:hover {
  border-color: var(--accent-primary);
  color: var(--text-primary);
}

.scene-tab.active {
  background: var(--accent-primary);
  color: white;
  border-color: var(--accent-primary);
}

.scene-tab.running {
  border-color: #a3e635;
  background: rgba(163,230,53,0.08);
}

.scene-tab.running.active {
  background: #a3e635;
  border-color: #a3e635;
}

.scene-status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  display: inline-block;
  flex-shrink: 0;
}

.scene-status-dot.running {
  background: #a3e635;
  box-shadow: 0 0 4px rgba(163,230,53,0.5);
  animation: pulse 2s infinite;
}

.scene-status-dot.done {
  background: var(--text-tertiary);
}

.scene-status-dot.failed {
  background: #e11d48;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.running-badge {
  background: rgba(163,230,53,0.15);
  color: #a3e635;
  padding: 1px 6px;
  border-radius: 8px;
  font-size: 10px;
  font-weight: 600;
}

.time-window-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.time-window-info {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 12px;
  background: var(--bg-tertiary);
  border-radius: var(--radius-sm);
  font-size: 12px;
  flex: 1;
}

.window-label {
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 500;
}

.window-value {
  color: var(--text-primary);
  font-size: 12px;
}

.duration-value {
  color: var(--text-secondary);
  font-size: 12px;
}

.live-indicator {
  color: #a3e635;
  font-weight: 700;
  font-size: 11px;
  animation: blink 1.5s infinite;
}

.refresh-selector {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  background: var(--bg-tertiary);
  border-radius: var(--radius-sm);
  font-size: 12px;
}

.refresh-label {
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 500;
}

.refresh-select {
  background: var(--bg-secondary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-sm);
  color: var(--text-primary);
  font-size: 12px;
  padding: 4px 8px;
  cursor: pointer;
}

@keyframes blink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}

.metrics-row {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: 16px;
}

.metric-card {
  background: var(--bg-card);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-md);
  padding: 16px;
}

.metric-label {
  font-size: 12px;
  color: var(--text-secondary);
  margin-bottom: 6px;
}

.metric-value {
  font-size: 24px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.metric-sub {
  font-size: 11px;
  color: var(--text-tertiary);
  margin-top: 4px;
}

.time-info-card {
  grid-column: span 1;
  min-height: 120px;
}

.time-details {
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--border-secondary);
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 6px 12px;
  align-items: center;
}

.time-item {
  display: contents;
  font-size: 11px;
}

.time-label {
  color: var(--text-tertiary);
  white-space: nowrap;
}

.time-value {
  color: var(--text-secondary);
  font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
  font-size: 10.5px;
  word-break: break-all;
  line-height: 1.4;
}

.time-value.duration {
  color: var(--accent-primary);
  font-weight: 700;
  font-size: 11px;
}

.charts-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.chart-card {
  background: var(--bg-card);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-md);
  padding: 16px;
}

.chart-card.wide {
  grid-column: 1 / -1;
}

.chart-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.chart-header h3 {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
}

.chart-tip {
  font-size: 11px;
  color: var(--text-tertiary);
  font-weight: 400;
}

.chart-controls {
  display: flex;
  gap: 4px;
}

.time-btn {
  font-size: 11px;
  padding: 3px 8px;
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.15s ease;
}

.time-btn.active {
  background: var(--accent-primary);
  color: #fff;
  border-color: var(--accent-primary);
}

.chart-body {
  height: 220px;
}

.bottom-section {
  display: flex;
  flex-direction: column !important;
  gap: 16px;
  width: 100%;
}

.card-full {
  width: 100%;
}

.card {
  background: var(--bg-card);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-md);
  padding: 16px;
}

.card h3 {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 12px;
}

.empty {
  color: var(--text-tertiary);
  font-size: 13px;
  text-align: center;
  padding: 24px 0;
}

.run-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.run-item {
  padding: 10px 12px;
  background: var(--bg-tertiary);
  border-radius: var(--radius-sm);
}

.run-info {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}

.run-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-primary);
}

.run-status {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 10px;
}

.run-status.running { background: rgba(0,229,255,0.15); color: var(--accent-primary); }
.run-status.completed { background: rgba(63,185,80,0.15); color: var(--accent-success); }
.run-status.failed { background: rgba(248,81,73,0.15); color: var(--accent-danger); }

.run-metrics {
  display: flex;
  gap: 16px;
  font-size: 12px;
  color: var(--text-secondary);
}

.node-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 16px;
}

.node-card {
  padding: 16px 18px;
  background: var(--bg-tertiary);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-secondary);
  position: relative;
  overflow: hidden;
}

.node-header {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}

.node-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  letter-spacing: 0.2px;
}

.node-id {
  font-family: 'SF Mono', 'Monaco', 'Inconsolata', monospace;
  font-size: 11px;
  color: var(--text-tertiary);
  padding: 2px 6px;
  background: rgba(0,0,0,0.1);
  border-radius: 4px;
}

.node-type {
  font-size: 10px;
  padding: 3px 8px;
  border-radius: 12px;
  margin-left: auto;
  text-transform: uppercase;
  letter-spacing: 1px;
  font-weight: 600;
}
.node-type.http { background: rgba(0,229,255,0.15); color: var(--accent-primary); }
.node-type.setup, .node-type.teardown { background: rgba(63,185,80,0.15); color: var(--accent-success); }
.node-type.delay { background: rgba(210,153,34,0.15); color: var(--accent-warning); }
.node-type.condition { background: rgba(163,113,247,0.15); color: #a371f7; }

.node-qps {
  display: flex;
  align-items: baseline;
  gap: 10px;
  margin-bottom: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border-secondary);
}

.qps-label {
  font-size: 12px;
  color: var(--text-tertiary);
  font-weight: 500;
}

.qps-value {
  font-size: 24px;
  font-weight: 700;
  color: var(--accent-primary);
  letter-spacing: -1px;
}

.node-bars {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.bar-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.bar-label {
  font-size: 11px;
  color: var(--text-tertiary);
  width: 32px;
  flex-shrink: 0;
  font-weight: 500;
  letter-spacing: 0.5px;
}

.bar-track {
  flex: 1;
  height: 8px;
  background: rgba(0,0,0,0.1);
  border-radius: 4px;
  overflow: visible;
  position: relative;
}

.bar-tooltip {
  cursor: pointer;
}

.bar-tooltip::before {
  content: attr(data-tooltip);
  position: absolute;
  bottom: calc(100% + 12px);
  left: 50%;
  transform: translateX(-50%) translateY(6px);
  background: rgba(255, 255, 255, 0.96);
  color: #1e293b;
  font-size: 11.5px;
  padding: 10px 14px;
  border-radius: 12px;
  border: 1px solid rgba(148, 163, 184, 0.2);
  white-space: nowrap;
  opacity: 0;
  visibility: hidden;
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  pointer-events: none;
  z-index: 1000;
  box-shadow:
    0 1px 3px rgba(0, 0, 0, 0.04),
    0 8px 24px rgba(0, 0, 0, 0.08);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  font-weight: 500;
  letter-spacing: 0.2px;
  max-width: 380px;
  white-space: normal;
  text-align: center;
  line-height: 1.5;
}

.bar-tooltip::after {
  content: '';
  position: absolute;
  bottom: calc(100% + 5px);
  left: 50%;
  transform: translateX(-50%);
  border: 7px solid transparent;
  border-top-color: rgba(255, 255, 255, 0.96);
  filter: drop-shadow(0 -2px 2px rgba(0, 0, 0, 0.04));
  opacity: 0;
  visibility: hidden;
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  pointer-events: none;
  z-index: 1001;
}

.bar-tooltip:hover::before {
  opacity: 1;
  visibility: visible;
  transform: translateX(-50%) translateY(0);
}

.bar-tooltip:hover::after {
  opacity: 1;
  visibility: visible;
}

/* Dark theme tooltip */
[data-theme='dark'] .bar-tooltip::before {
  background: rgba(30, 41, 59, 0.95);
  color: #e2e8f0;
  border-color: rgba(71, 85, 105, 0.3);
  box-shadow:
    0 1px 3px rgba(0, 0, 0, 0.2),
    0 8px 24px rgba(0, 0, 0, 0.3);
}

[data-theme='dark'] .bar-tooltip::after {
  border-top-color: rgba(30, 41, 59, 0.95);
  filter: drop-shadow(0 -2px 2px rgba(0, 0, 0, 0.2));
}

.bar-fill {
  height: 100%;
  border-radius: 4px;
  background: var(--accent-success);
  transition: width 0.4s cubic-bezier(0.4, 0, 0.2, 1);
}
.bar-fill.p50 { background: var(--accent-primary); }
.bar-fill.p95 { background: var(--accent-warning); }
.bar-fill.p99 { background: var(--accent-danger); }

.bar-value {
  font-size: 12px;
  color: var(--text-secondary);
  width: 65px;
  text-align: right;
  flex-shrink: 0;
  font-family: 'SF Mono', 'Monaco', monospace;
  font-weight: 500;
}

.node-section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 14px;
}
.node-section-header h3 { margin-bottom: 0; }
.time-range-label {
  font-size: 12px;
  color: var(--text-tertiary);
  font-family: monospace;
}

.node-card { 
  cursor: pointer; 
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1); 
}
.node-card:hover { 
  border-color: var(--accent-primary); 
  box-shadow: 0 8px 24px rgba(0,229,255, 0.12);
  transform: translateY(-2px);
}
.node-card.expanded {
  border-color: var(--accent-primary);
  box-shadow: 0 12px 40px rgba(0,229,255, 0.15);
  grid-column: 1 / -1;
}

.node-stats-row {
  display: flex;
  gap: 16px;
  padding-top: 12px;
  margin-top: 12px;
  border-top: 1px solid var(--border-secondary);
}
.node-stat-item {
  flex: 1;
  text-align: center;
}
.node-stat-label {
  font-size: 11px;
  color: var(--text-tertiary);
  margin-bottom: 4px;
  display: block;
}
.node-stat-value {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
}
.node-stat-value.success { color: var(--accent-success); }
.node-stat-value.danger { color: var(--accent-danger); }
.node-stat-value.warning { color: var(--accent-warning); }

/* Light theme specific styles */
[data-theme='light'] .dashboard-page {
  background: linear-gradient(180deg, #f1f5f9 0%, #f8fafc 100%);
}

[data-theme='light'] .metric-card {
  background: linear-gradient(135deg, #ffffff 0%, #fafafa 100%);
  border-color: rgba(0,0,0,0.06);
  box-shadow: 0 1px 3px rgba(0,0,0,0.04);
}

[data-theme='light'] .chart-card {
  background: linear-gradient(135deg, #ffffff 0%, #fafafa 100%);
  border-color: rgba(0,0,0,0.06);
  box-shadow: 0 1px 3px rgba(0,0,0,0.04);
}

[data-theme='light'] .card {
  background: linear-gradient(135deg, #ffffff 0%, #fafafa 100%);
  border-color: rgba(0,0,0,0.06);
  box-shadow: 0 1px 3px rgba(0,0,0,0.04);
}

[data-theme='light'] .node-card {
  background: linear-gradient(135deg, #ffffff 0%, #fafafa 100%);
  border-color: rgba(0,0,0,0.06);
}

[data-theme='light'] .node-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 2px;
  background: linear-gradient(90deg, transparent, #0891b2, transparent);
  opacity: 0;
  transition: opacity 0.3s ease;
}

[data-theme='light'] .node-card:hover::before,
[data-theme='light'] .node-card.expanded::before {
  opacity: 1;
}

[data-theme='light'] .node-id {
  background: rgba(0,0,0,0.03);
}

[data-theme='light'] .node-type {
  box-shadow: 0 2px 8px rgba(0,0,0,0.06);
}

[data-theme='light'] .node-type.http { 
  background: linear-gradient(135deg, rgba(217, 70, 239, 0.18) 0%, rgba(8, 145, 178, 0.08) 100%); 
  color: #0891b2;
}

[data-theme='light'] .node-type.setup, 
[data-theme='light'] .node-type.teardown { 
  background: linear-gradient(135deg, rgba(101, 163, 13, 0.18) 0%, rgba(77, 124, 15, 0.08) 100%); 
  color: #65a30d;
}

[data-theme='light'] .node-type.delay { 
  background: linear-gradient(135deg, rgba(234, 88, 12, 0.18) 0%, rgba(194, 65, 12, 0.08) 100%); 
  color: #ea580c;
}

[data-theme='light'] .node-type.condition { 
  background: linear-gradient(135deg, rgba(168, 85, 247, 0.18) 0%, rgba(124, 58, 237, 0.08) 100%); 
  color: #a855f7;
}

[data-theme='light'] .qps-value {
  background: linear-gradient(135deg, #0891b2 0%, #d946ef 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

[data-theme='light'] .bar-track {
  background: rgba(0,0,0,0.05);
  box-shadow: inset 0 1px 3px rgba(0,0,0,0.06);
}

[data-theme='light'] .bar-fill.p50 { 
  background: linear-gradient(90deg, #d946ef 0%, #0891b2 100%); 
  box-shadow: 0 1px 4px rgba(217, 70, 239, 0.4);
}

[data-theme='light'] .bar-fill.p95 { 
  background: linear-gradient(90deg, #ea580c 0%, #c2410c 100%); 
  box-shadow: 0 1px 4px rgba(234, 88, 12, 0.4);
}

[data-theme='light'] .bar-fill.p99 { 
  background: linear-gradient(90deg, #be123c 0%, #e11d48 100%); 
  box-shadow: 0 1px 4px rgba(225, 29, 72, 0.4);
}

[data-theme='light'] .node-qps {
  border-bottom-color: rgba(0,0,0,0.05);
}

[data-theme='light'] .node-card.expanded {
  background: linear-gradient(180deg, #f8fafc 0%, rgba(255,255,255,0.98) 100%);
}

[data-theme='light'] .node-card:hover {
  border-color: #0891b2;
  box-shadow: 0 8px 24px rgba(8, 145, 178, 0.15);
}

[data-theme='light'] .node-card.expanded {
  border-color: #0891b2;
  box-shadow: 0 12px 40px rgba(8, 145, 178, 0.18);
}

[data-theme='light'] .detail-item {
  background: rgba(0,0,0,0.03);
}

[data-theme='light'] .run-item {
  background: rgba(0,0,0,0.03);
}

.expand-icon {
  font-size: 10px;
  color: var(--text-tertiary);
  transition: transform 0.15s ease;
}

.node-detail {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--border-secondary);
}

.node-time-range {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  background: var(--bg-tertiary);
  border-radius: var(--radius-sm);
  margin-bottom: 10px;
  font-size: 11.5px;
}

.time-range-label {
  color: var(--text-tertiary);
  font-weight: 500;
  white-space: nowrap;
}

.time-range-value {
  color: var(--accent-primary);
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 10.5px;
  font-weight: 600;
}

.detail-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
  gap: 8px;
  margin-bottom: 12px;
}

.detail-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 10px;
  background: var(--bg-hover);
  border-radius: var(--radius-sm);
}

.detail-label { font-size: 11px; color: var(--text-tertiary); }
.detail-val { font-size: 13px; font-weight: 600; color: var(--text-primary); }
.detail-val.success { color: var(--accent-success); }
.detail-val.danger { color: var(--accent-danger); }

.detail-chart { height: 140px; }

.chart-type-toggle {
  display: flex;
  gap: 6px;
  justify-content: center;
  margin-top: 8px;
}
.type-btn-group {
  display: flex;
  gap: 4px;
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

/* System Monitoring Section */
.system-monitor-section {
  background: var(--bg-card);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-md);
  padding: 16px;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 12px 0;
}

.sys-gauge-row {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.sys-gauge-card {
  flex: 1;
  min-width: 120px;
  background: var(--bg-secondary);
  border: 2px solid var(--border-secondary);
  border-radius: var(--radius-md);
  padding: 12px 16px;
  text-align: center;
  transition: border-color 0.3s ease, background-color 0.3s ease, transform 0.2s ease, box-shadow 0.2s ease;
  position: relative;
  cursor: default;
}

.sys-gauge-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
}

.sys-gauge-card[data-tooltip]::after {
  content: attr(data-tooltip);
  position: absolute;
  bottom: calc(100% + 8px);
  left: 50%;
  transform: translateX(-50%) scale(0.95);
  background: rgba(255, 255, 255, 0.96);
  color: #1e293b;
  font-size: 11.5px;
  line-height: 1.5;
  padding: 8px 12px;
  border-radius: 10px;
  white-space: normal;
  max-width: 220px;
  width: max-content;
  opacity: 0;
  visibility: hidden;
  transition: opacity 0.2s ease, transform 0.2s ease, visibility 0s 0.2s;
  pointer-events: none;
  z-index: 100;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.08), 0 1px 3px rgba(0, 0, 0, 0.04);
  font-weight: 400;
  letter-spacing: 0.2px;
  border: 1px solid rgba(148, 163, 184, 0.2);
}

.sys-gauge-card[data-tooltip]::before {
  content: '';
  position: absolute;
  bottom: calc(100% + 2px);
  left: 50%;
  transform: translateX(-50%);
  border: 6px solid transparent;
  border-top-color: rgba(255, 255, 255, 0.96);
  opacity: 0;
  visibility: hidden;
  transition: opacity 0.2s ease, visibility 0s 0.2s;
  z-index: 101;
  pointer-events: none;
}

.sys-gauge-card[data-tooltip]:hover::after,
.sys-gauge-card[data-tooltip]:hover::before {
  opacity: 1;
  visibility: visible;
  transition: opacity 0.2s ease, transform 0.2s ease, visibility 0s 0s;
  transform: translateX(-50%) scale(1);
}

.sys-gauge-card.status-warning {
  border-color: var(--accent-warning, #ca8a04) !important;
  background: rgba(202, 138, 4, 0.05);
}

.sys-gauge-card.status-danger {
  border-color: var(--accent-danger, #dc2626) !important;
  background: rgba(220, 38, 38, 0.05);
}

.sys-gauge-card .gauge-label {
  font-size: 11px;
  color: var(--text-secondary);
  margin-bottom: 4px;
  font-weight: 500;
}

.sys-gauge-card .gauge-value {
  font-size: 22px;
  font-weight: 700;
  color: var(--text-primary);
  font-family: -apple-system, 'SF Mono', 'Monaco', 'Menlo', monospace;
  transition: color 0.3s ease;
}

.sys-gauge-card.status-warning .gauge-value {
  color: var(--accent-warning, #ca8a04);
}

.sys-gauge-card.status-danger .gauge-value {
  color: var(--accent-danger, #dc2626);
}

.sys-gauge-card .gauge-unit {
  font-size: 10px;
  color: var(--text-tertiary);
  margin-top: 2px;
}

.gauge-alert {
  font-size: 10px;
  font-weight: 600;
  margin-top: 4px;
  padding: 1px 6px;
  border-radius: 4px;
  display: inline-block;
}

.status-warning .gauge-alert {
  color: var(--accent-warning, #ca8a04);
  background: rgba(202, 138, 4, 0.10);
}

.status-danger .gauge-alert {
  color: var(--accent-danger, #dc2626);
  background: rgba(220, 38, 38, 0.10);
}

.sys-gauge-row {
  display: flex;
  gap: 10px;
  overflow: visible;
}

.sys-charts-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  margin-top: 16px;
}

.sys-chart-item {
  background: var(--bg-secondary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-md);
  padding: 12px;
  overflow: visible;
}

.sys-chart-canvas {
  width: 100%;
  height: 220px;
  overflow: visible;
}

.sys-chart-desc {
  font-size: 11px;
  color: var(--text-tertiary);
  line-height: 1.5;
  margin-top: 4px;
  padding: 0 2px;
}

.sys-chart-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
}

.sys-chart-title {
  font-size: 11px;
  font-weight: 500;
  color: var(--text-secondary);
}

.expand-icon {
  font-size: 10px;
  color: var(--text-tertiary);
  flex-shrink: 0;
  width: 14px;
  text-align: center;
}

.sys-chart-item.expanded {
  grid-column: 1 / -1;
}

.sys-chart-expanded {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--border-secondary);
}

.sys-expanded-canvas {
  height: 320px;
}
</style>
