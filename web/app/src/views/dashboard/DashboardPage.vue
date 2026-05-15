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
          <button :class="['type-btn', { active: chartTypes.latTrend === 'smooth' }]" @click="switchChartType('latTrend', 'smooth')">平滑</button>
          <button :class="['type-btn', { active: chartTypes.latTrend === 'step' }]" @click="switchChartType('latTrend', 'step')">阶梯</button>
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
            <div v-if="expandedNodeId === node.node_id" class="node-detail" @click.stop>
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
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import * as echarts from 'echarts'
import type { DashboardOverviewDTO, RunHistoryDTO } from '@/types'

const qpsChartRef = ref<HTMLElement>()
const latencyChartRef = ref<HTMLElement>()
const errorChartRef = ref<HTMLElement>()

let qpsChart: echarts.ECharts | null = null
let latencyChart: echarts.ECharts | null = null
let errorChart: echarts.ECharts | null = null
let pollTimer: ReturnType<typeof setInterval> | null = null
let timeRefreshTimer: ReturnType<typeof setInterval> | null = null
const expandedNodeId = ref('')
const nodeChartRefs = new Map<string, HTMLElement>()
const nodeCharts = new Map<string, echarts.ECharts>()
const chartTypes = ref<Record<string, 'smooth' | 'step'>>({
  errorRate: 'smooth',
  qpsTrend: 'smooth',
  latTrend: 'smooth'
})

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
}

const overview = ref<DashboardOverviewDTO | null>(null)

const historyData = ref<RunHistoryDTO[]>([])
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
        const firstRunning = sceneList.value.find(s => s.status === 'running')
        selectedSceneId.value = (firstRunning || sceneList.value[0]).scene_id
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
    return `${hours}小时${pad(minutes)}分${pad(secs)}秒`
  } else if (minutes > 0) {
    return `${minutes}分${pad(secs)}秒`
  } else {
    return `${secs}秒`
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
  expandedNodeId.value = expandedNodeId.value === nodeId ? '' : nodeId
  if (expandedNodeId.value) {
    setTimeout(() => renderNodeDetailChart(nodeId), 50)
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

function getChartTheme() {
  const isDark = document.documentElement.getAttribute('data-theme') !== 'light'
  if (isDark) {
    return {
      textColor: '#8b949e',
      lineColor: '#30363d',
      bgColor: 'transparent',
      colors: {
        primary: '#58a6ff',
        success: '#3fb950',
        warning: '#d29922',
        danger: '#f85149',
        info: '#89d0ff',
      }
    }
  }
  return {
    textColor: '#475569',
    lineColor: '#e2e8f0',
    bgColor: 'transparent',
    colors: {
      primary: '#0ea5e9',
      success: '#22c55e',
      warning: '#a855f7',
      danger: '#ef4444',
      info: '#06b6d4',
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
    confine: true,
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
        const val = isNaN(rawVal) ? '-' : (param.seriesName === 'QPS' ? rawVal.toFixed(1) : (param.seriesName === '错误率' || param.seriesName === 'error_rate' ? rawVal.toFixed(2) : rawVal.toFixed(1)))
        const unit = param.seriesName === 'QPS' ? '' : (param.seriesName === '错误率' || param.seriesName === 'error_rate' ? '%' : 'ms')
        result += `<div style="display:flex;justify-content:space-between;align-items:center;gap:24px;margin-top:6px;padding:2px 0"><span style="font-size:11px;color:${labelColor};font-weight:500">${param.marker}${param.seriesName}</span><span style="font-size:11px;color:${valueColor};font-weight:600;font-family:-apple-system,'SF Mono','Monaco','Menlo',monospace;letter-spacing:0.5px">${val}${unit}</span></div>`
      }
      return result
    }
  }
}

function getFilteredTimeSeries() {
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
    dataZoom: [{ type: 'slider', height: 18, bottom: 4, borderColor: 'transparent', backgroundColor: theme.lineColor, fillerColor: `rgba(${theme.colors.primary === '#0ea5e9' ? '14, 165, 233' : '88, 166, 255'}, 0.15)`, handleStyle: { color: theme.colors.primary }, textStyle: { color: theme.textColor, fontSize: 10 }, brushSelect: true }],
    xAxis: { type: 'category', data: ts.timestamps, axisLine: { lineStyle: { color: theme.lineColor } }, axisLabel: { color: theme.textColor, fontSize: 10 } },
    yAxis: { type: 'value', axisLine: { show: false }, splitLine: { lineStyle: { color: theme.lineColor, type: 'dashed' } }, axisLabel: { color: theme.textColor, fontSize: 10 } },
    series: [{ data: ts.qps, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { color: theme.colors.primary, width: 2 }, areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{ offset: 0, color: `rgba(${theme.colors.primary === '#0ea5e9' ? '14, 165, 233' : '88, 166, 255'}, 0.3)` }, { offset: 1, color: 'rgba(88,166,255,0)' }]) } }],
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
    dataZoom: [{ type: 'slider', height: 18, bottom: 4, borderColor: 'transparent', backgroundColor: theme.lineColor, fillerColor: `rgba(${theme.colors.primary === '#0ea5e9' ? '14, 165, 233' : '88, 166, 255'}, 0.15)`, handleStyle: { color: theme.colors.primary }, textStyle: { color: theme.textColor, fontSize: 10 }, brushSelect: true }],
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
    dataZoom: [{ type: 'slider', height: 14, bottom: 2, borderColor: 'transparent', backgroundColor: theme.lineColor, fillerColor: `rgba(${theme.colors.danger === '#ef4444' ? '239, 68, 68' : '248, 81, 73'}, 0.10)`, handleStyle: { color: theme.colors.danger }, textStyle: { color: theme.textColor, fontSize: 9 }, showDetail: false }],
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
      areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{ offset: 0, color: `rgba(${theme.colors.danger === '#ef4444' ? '239, 68, 68' : '248, 81, 73'}, 0.18)` }, { offset: 1, color: `rgba(${theme.colors.danger === '#ef4444' ? '239, 68, 68' : '248, 81, 73'}, 0.01)` }]) }
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
      series.push({ name: 'QPS', data: padArray(tsQPS, timestampsLength), type: 'bar', lineStyle: undefined, itemStyle: { color: `rgba(${theme.colors.primary === '#0ea5e9' ? '14, 165, 233' : '88, 166, 255'}, 0.25)`, borderRadius: [2, 2, 0, 0] }, areaStyle: undefined })
    }

    const hasQPS = series.length > 4
    chart.setOption({
      backgroundColor: 'transparent',
      legend: { top: 0, textStyle: { color: theme.textColor, fontSize: 10 }, itemWidth: 16, itemHeight: 3 },
      grid: { top: 28, right: hasQPS ? 48 : 12, bottom: 36, left: 48 },
      dataZoom: [{ type: 'slider', height: 14, bottom: 2, borderColor: 'transparent', backgroundColor: theme.lineColor, fillerColor: `rgba(${theme.colors.primary === '#0ea5e9' ? '14, 165, 233' : '88, 166, 255'}, 0.15)`, handleStyle: { color: theme.colors.primary }, textStyle: { color: theme.textColor, fontSize: 9 }, showDetail: false }],
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
            { offset: 1, color: `rgba(${theme.colors.primary === '#0ea5e9' ? '14, 165, 233' : '88, 166, 255'}, 0.3)` },
          ]),
        },
      }],
      tooltip: getTooltipConfig(),
    }, true)
  }
}

async function fetchOverview() {
  try {
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
      const hasTimeSeries = resp.data?.time_series?.timestamps?.length > 0
      if (!hasRunning && !hasTimeSeries) {
        loadHistoryData()
      }
      renderQpsChart()
      renderLatencyChart()
      renderErrorChart()
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
}

onMounted(() => {
  fetchSceneList()
  fetchOverview()
  loadHistoryData()

  setTimeout(() => {
    renderQpsChart()
    renderLatencyChart()
    renderErrorChart()
  }, 100)
  window.addEventListener('resize', handleResize)
  
  watch(allScenes, (scenes) => {
    if (scenes.length > 0 && !selectedSceneId.value) {
      const firstRunning = scenes.find(s => s.status === 'running')
      selectedSceneId.value = (firstRunning || scenes[0]).scene_id
      console.log('🔄 Auto-selected scene:', selectedSceneId.value)
    }
  }, { immediate: true })
  
  restartPolling()

  timeRefreshTimer = setInterval(() => {
    if (overview.value?.recent_runs?.some(r => r.status === 'running')) {
      overview.value = { ...overview.value }
    }
  }, 1000)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
  if (timeRefreshTimer) { clearInterval(timeRefreshTimer); timeRefreshTimer = null }
  qpsChart?.dispose()
  latencyChart?.dispose()
  errorChart?.dispose()
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
  box-shadow: 0 0 0 3px rgba(88,166,255,0.1);
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
  border-color: #3fb950;
  background: rgba(63,185,80,0.08);
}

.scene-tab.running.active {
  background: #3fb950;
  border-color: #3fb950;
}

.scene-status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  display: inline-block;
  flex-shrink: 0;
}

.scene-status-dot.running {
  background: #3fb950;
  box-shadow: 0 0 4px rgba(63,185,80,0.5);
  animation: pulse 2s infinite;
}

.scene-status-dot.done {
  background: var(--text-tertiary);
}

.scene-status-dot.failed {
  background: #f85149;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.running-badge {
  background: rgba(63,185,80,0.15);
  color: #3fb950;
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
  color: #3fb950;
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

.run-status.running { background: rgba(88,166,255,0.15); color: var(--accent-primary); }
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
.node-type.http { background: rgba(88,166,255,0.15); color: var(--accent-primary); }
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
  box-shadow: 0 8px 24px rgba(88, 166, 255, 0.12);
  transform: translateY(-2px);
}
.node-card.expanded {
  border-color: var(--accent-primary);
  box-shadow: 0 12px 40px rgba(88, 166, 255, 0.15);
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
  background: linear-gradient(90deg, transparent, #0ea5e9, transparent);
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
  background: linear-gradient(135deg, rgba(6, 182, 212, 0.18) 0%, rgba(14, 165, 233, 0.08) 100%); 
  color: #0ea5e9;
}

[data-theme='light'] .node-type.setup, 
[data-theme='light'] .node-type.teardown { 
  background: linear-gradient(135deg, rgba(34, 197, 94, 0.18) 0%, rgba(22, 163, 74, 0.08) 100%); 
  color: #22c55e;
}

[data-theme='light'] .node-type.delay { 
  background: linear-gradient(135deg, rgba(245, 158, 11, 0.18) 0%, rgba(202, 138, 4, 0.08) 100%); 
  color: #f59e0b;
}

[data-theme='light'] .node-type.condition { 
  background: linear-gradient(135deg, rgba(168, 85, 247, 0.18) 0%, rgba(124, 58, 237, 0.08) 100%); 
  color: #a855f7;
}

[data-theme='light'] .qps-value {
  background: linear-gradient(135deg, #0ea5e9 0%, #06b6d4 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

[data-theme='light'] .bar-track {
  background: rgba(0,0,0,0.05);
  box-shadow: inset 0 1px 3px rgba(0,0,0,0.06);
}

[data-theme='light'] .bar-fill.p50 { 
  background: linear-gradient(90deg, #06b6d4 0%, #0ea5e9 100%); 
  box-shadow: 0 1px 4px rgba(6, 182, 212, 0.4);
}

[data-theme='light'] .bar-fill.p95 { 
  background: linear-gradient(90deg, #a855f7 0%, #8b5cf6 100%); 
  box-shadow: 0 1px 4px rgba(168, 85, 247, 0.4);
}

[data-theme='light'] .bar-fill.p99 { 
  background: linear-gradient(90deg, #ef4444 0%, #f87171 100%); 
  box-shadow: 0 1px 4px rgba(239, 68, 68, 0.4);
}

[data-theme='light'] .node-qps {
  border-bottom-color: rgba(0,0,0,0.05);
}

[data-theme='light'] .node-card.expanded {
  background: linear-gradient(180deg, #f8fafc 0%, rgba(255,255,255,0.98) 100%);
}

[data-theme='light'] .node-card:hover {
  border-color: #0ea5e9;
  box-shadow: 0 8px 24px rgba(14, 165, 233, 0.15);
}

[data-theme='light'] .node-card.expanded {
  border-color: #0ea5e9;
  box-shadow: 0 12px 40px rgba(14, 165, 233, 0.18);
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
</style>
