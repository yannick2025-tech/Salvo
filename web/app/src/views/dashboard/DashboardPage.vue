<template>
  <div class="dashboard-page">
    <div class="metrics-row">
      <div class="metric-card" v-for="m in summaryMetrics" :key="m.label">
        <div class="metric-label">{{ m.label }}</div>
        <div class="metric-value" :style="{ color: m.color }">{{ m.value }}</div>
        <div class="metric-sub" v-if="m.sub">{{ m.sub }}</div>
      </div>
    </div>

    <div class="charts-row">
      <div class="chart-card">
        <div class="chart-header">
          <h3>QPS</h3>
          <div class="chart-controls">
            <button
              v-for="r in timeRanges"
              :key="r.value"
              :class="['time-btn', { active: qpsRange === r.value }]"
              @click="qpsRange = r.value"
            >{{ r.label }}</button>
          </div>
        </div>
        <div class="chart-body" ref="qpsChartRef"></div>
      </div>

      <div class="chart-card">
        <div class="chart-header">
          <h3>延迟分布</h3>
          <div class="chart-controls">
            <button
              v-for="r in timeRanges"
              :key="r.value"
              :class="['time-btn', { active: latencyRange === r.value }]"
              @click="latencyRange = r.value"
            >{{ r.label }}</button>
          </div>
        </div>
        <div class="chart-body" ref="latencyChartRef"></div>
      </div>
    </div>

    <div class="charts-row">
      <div class="chart-card wide">
        <div class="chart-header">
          <h3>错误率</h3>
          <div class="chart-controls">
            <button
              v-for="r in timeRanges"
              :key="r.value"
              :class="['time-btn', { active: errorRange === r.value }]"
              @click="errorRange = r.value"
            >{{ r.label }}</button>
          </div>
        </div>
        <div class="chart-body" ref="errorChartRef"></div>
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
              <span>成功率: {{ ((run.success_reqs / Math.max(run.total_reqs, 1)) * 100).toFixed(1) }}%</span>
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
                <div class="bar-track"><div class="bar-fill" :style="{ width: barWidth(node.p50_latency) }"></div></div>
                <span class="bar-value">{{ formatMs(node.p50_latency) }}</span>
              </div>
              <div class="bar-row">
                <span class="bar-label">P95</span>
                <div class="bar-track"><div class="bar-fill warn" :style="{ width: barWidth(node.p95_latency) }"></div></div>
                <span class="bar-value">{{ formatMs(node.p95_latency) }}</span>
              </div>
              <div class="bar-row">
                <span class="bar-label">P99</span>
                <div class="bar-track"><div class="bar-fill danger" :style="{ width: barWidth(node.p99_latency) }"></div></div>
                <span class="bar-value">{{ formatMs(node.p99_latency) }}</span>
              </div>
            </div>
            <div v-if="expandedNodeId === node.node_id" class="node-detail" @click.stop>
              <div class="detail-grid">
                <div class="detail-item"><span class="detail-label">总请求数</span><span class="detail-val">{{ formatNum(node.total_reqs) }}</span></div>
                <div class="detail-item"><span class="detail-label">成功数</span><span class="detail-val success">{{ formatNum(node.success_reqs) }}</span></div>
                <div class="detail-item"><span class="detail-label">失败数</span><span class="detail-val danger">{{ formatNum(node.total_reqs - node.success_reqs) }}</span></div>
                <div class="detail-item"><span class="detail-label">成功率</span><span class="detail-val">{{ node.total_reqs > 0 ? ((node.success_reqs / node.total_reqs) * 100).toFixed(1) : '0' }}%</span></div>
                <div class="detail-item"><span class="detail-label">平均延迟</span><span class="detail-val">{{ formatMs(node.avg_latency) }}</span></div>
              </div>
              <div class="detail-chart" :ref="el => setNodeChartRef(node.node_id, el as HTMLElement)"></div>
              <div class="chart-type-toggle">
                <button :class="['type-btn', { active: nodeChartType === 'smooth' }]" @click.stop="switchNodeChartType('smooth')">平滑</button>
                <button :class="['type-btn', { active: nodeChartType === 'step' }]" @click.stop="switchNodeChartType('step')">阶梯</button>
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
import { dashboardOverview } from '@/api/dashboard'
import type { DashboardOverviewDTO } from '@/types'

const timeRanges = [
  { label: '1m', value: 60 },
  { label: '5m', value: 300 },
  { label: '15m', value: 900 },
  { label: '1h', value: 3600 },
  { label: '6h', value: 21600 },
  { label: '24h', value: 86400 },
]

const qpsRange = ref(300)
const latencyRange = ref(300)
const errorRange = ref(300)

const qpsChartRef = ref<HTMLElement>()
const latencyChartRef = ref<HTMLElement>()
const errorChartRef = ref<HTMLElement>()

let qpsChart: echarts.ECharts | null = null
let latencyChart: echarts.ECharts | null = null
let errorChart: echarts.ECharts | null = null
let pollTimer: ReturnType<typeof setInterval> | null = null
const expandedNodeId = ref('')
const nodeChartRefs = new Map<string, HTMLElement>()
const nodeCharts = new Map<string, echarts.ECharts>()
const nodeChartType = ref<'smooth' | 'step'>('smooth')

const overview = ref<DashboardOverviewDTO | null>(null)

const runTimeRange = computed(() => {
  const runs = overview.value?.recent_runs
  if (!runs?.length) return '-'
  const first = runs[runs.length - 1]
  const last = runs[0]
  const fmt = (t?: string) => t ? new Date(t).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) : '-'
  return `${fmt(first.started_at)} ~ ${fmt(last.started_at)}`
})

function toggleNodeExpand(nodeId: string) {
  expandedNodeId.value = expandedNodeId.value === nodeId ? '' : nodeId
  if (expandedNodeId.value) {
    setTimeout(() => renderNodeDetailChart(nodeId), 50)
  }
}

function switchNodeChartType(type: 'smooth' | 'step') {
  nodeChartType.value = type
  if (expandedNodeId.value) renderNodeDetailChart(expandedNodeId.value)
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
      { label: '运行中', value: '-', color: 'var(--accent-primary)', sub: '' },
    ]
  }
  const rate = d.total_reqs > 0 ? ((d.success_reqs / d.total_reqs) * 100).toFixed(1) + '%' : '0%'
  return [
    { label: '总请求数', value: formatNum(d.total_reqs), color: 'var(--accent-primary)', sub: '' },
    { label: '成功率', value: rate, color: 'var(--accent-success)', sub: '' },
    { label: 'P50 延迟', value: formatMs(d.p50_latency), color: 'var(--accent-info)', sub: '' },
    { label: 'P95 延迟', value: formatMs(d.p95_latency), color: 'var(--accent-warning)', sub: '' },
    { label: 'P99 延迟', value: formatMs(d.p99_latency), color: 'var(--accent-danger)', sub: '' },
    { label: '运行中', value: String(d.running), color: 'var(--accent-primary)', sub: '' },
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
  if (ms < 1000) return ms.toFixed(1) + 'ms'
  return (ms / 1000).toFixed(2) + 's'
}

function barWidth(val: number): string {
  if (!val) return '0%'
  const ms = val * 1000
  const pct = Math.min((ms / 500) * 100, 100)
  return pct + '%'
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
  return {
    textColor: isDark ? '#8b949e' : '#656d76',
    lineColor: isDark ? '#30363d' : '#d0d7de',
    bgColor: 'transparent',
  }
}

function renderQpsChart() {
  if (!qpsChartRef.value) return
  if (!qpsChart) {
    qpsChart = echarts.init(qpsChartRef.value)
  }
  const theme = getChartTheme()
  const ts = overview.value?.time_series
  if (!ts || !ts.timestamps.length) {
    qpsChart.setOption({
      backgroundColor: theme.bgColor,
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
    dataZoom: [{ type: 'slider', height: 18, bottom: 4, borderColor: 'transparent', backgroundColor: theme.lineColor, fillerColor: 'rgba(88,166,255,0.15)', handleStyle: { color: '#58a6ff' }, textStyle: { color: theme.textColor, fontSize: 10 }, brushSelect: true }],
    xAxis: { type: 'category', data: ts.timestamps, axisLine: { lineStyle: { color: theme.lineColor } }, axisLabel: { color: theme.textColor, fontSize: 10 } },
    yAxis: { type: 'value', axisLine: { show: false }, splitLine: { lineStyle: { color: theme.lineColor, type: 'dashed' } }, axisLabel: { color: theme.textColor, fontSize: 10 } },
    series: [{ data: ts.qps, type: 'line', smooth: true, symbol: 'none', lineStyle: { color: '#58a6ff', width: 2 }, areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{ offset: 0, color: 'rgba(88,166,255,0.3)' }, { offset: 1, color: 'rgba(88,166,255,0)' }]) } }],
    tooltip: { trigger: 'axis' },
  }, true)
}

function renderLatencyChart() {
  if (!latencyChartRef.value) return
  if (!latencyChart) {
    latencyChart = echarts.init(latencyChartRef.value)
  }
  const theme = getChartTheme()
  const ts = overview.value?.time_series
  if (!ts || !ts.timestamps.length) {
    latencyChart.setOption({
      backgroundColor: theme.bgColor,
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
    dataZoom: [{ type: 'slider', height: 18, bottom: 4, borderColor: 'transparent', backgroundColor: theme.lineColor, fillerColor: 'rgba(88,166,255,0.15)', handleStyle: { color: '#58a6ff' }, textStyle: { color: theme.textColor, fontSize: 10 }, brushSelect: true }],
    xAxis: { type: 'category', data: ts.timestamps, axisLine: { lineStyle: { color: theme.lineColor } }, axisLabel: { color: theme.textColor, fontSize: 10 } },
    yAxis: { type: 'value', axisLine: { show: false }, splitLine: { lineStyle: { color: theme.lineColor, type: 'dashed' } }, axisLabel: { color: theme.textColor, fontSize: 10, formatter: '{value}ms' } },
    series: [
      { name: 'P50', data: ts.p50, type: 'line', smooth: true, symbol: 'none', lineStyle: { color: '#3fb950', width: 2 } },
      { name: 'P95', data: ts.p95, type: 'line', smooth: true, symbol: 'none', lineStyle: { color: '#d29922', width: 2 } },
      { name: 'P99', data: ts.p99, type: 'line', smooth: true, symbol: 'none', lineStyle: { color: '#f85149', width: 2 } },
    ],
    tooltip: { trigger: 'axis' },
    legend: { data: ['P50', 'P95', 'P99'], textStyle: { color: theme.textColor }, top: 0 },
  }, true)
}

function renderErrorChart() {
  if (!errorChartRef.value) return
  if (!errorChart) {
    errorChart = echarts.init(errorChartRef.value)
  }
  const theme = getChartTheme()
  const ts = overview.value?.time_series
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
  errorChart.setOption({
    backgroundColor: theme.bgColor,
    grid: { top: 20, right: 20, bottom: 50, left: 50 },
    dataZoom: [{ type: 'slider', height: 18, bottom: 4, borderColor: 'transparent', backgroundColor: theme.lineColor, fillerColor: 'rgba(88,166,255,0.15)', handleStyle: { color: '#58a6ff' }, textStyle: { color: theme.textColor, fontSize: 10 }, brushSelect: true }],
    xAxis: { type: 'category', data: ts.timestamps, axisLine: { lineStyle: { color: theme.lineColor } }, axisLabel: { color: theme.textColor, fontSize: 10 } },
    yAxis: { type: 'value', min: 0, max: ts.error_rate.some(v => v > 0) ? undefined : 1, axisLine: { show: false }, splitLine: { lineStyle: { color: theme.lineColor, type: 'dashed' } }, axisLabel: { color: theme.textColor, fontSize: 10, formatter: '{value}%' } },
    series: [{ data: ts.error_rate, type: 'bar', barMinHeight: 3, itemStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{ offset: 0, color: '#f85149' }, { offset: 1, color: 'rgba(248,81,73,0.3)' }]) } }],
    tooltip: { trigger: 'axis' },
  }, true)
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
  const isSmooth = nodeChartType.value === 'smooth'

  const hasTimeSeries = node.timestamps && node.timestamps.length > 0 && (node.ts_p50?.length || 0) > 0

  if (hasTimeSeries) {
    const series: any[] = [
      { name: 'P50', data: node.ts_p50!, lineStyle: { color: '#58a6ff', width: 2 }, itemStyle: { color: '#58a6ff' }, areaStyle: undefined },
      { name: 'Avg', data: node.ts_avg!, lineStyle: { color: '#3fb950', width: 2 }, itemStyle: { color: '#3fb950' }, areaStyle: undefined },
      { name: 'P95', data: node.ts_p95!, lineStyle: { color: '#d29922', width: 2 }, itemStyle: { color: '#d29922' }, areaStyle: undefined },
      { name: 'P99', data: node.ts_p99!, lineStyle: { color: '#f85149', width: 2 }, itemStyle: { color: '#f85149' }, areaStyle: undefined },
    ]
    if (node.ts_qps && node.ts_qps.some((v: number) => v > 0)) {
      series.push({ name: 'QPS', data: node.ts_qps, type: 'bar', lineStyle: undefined, itemStyle: { color: 'rgba(88,166,255,0.25)', borderRadius: [2, 2, 0, 0] }, areaStyle: undefined })
    }

    const hasQPS = series.length > 4
    chart.setOption({
      backgroundColor: 'transparent',
      legend: { top: 0, textStyle: { color: theme.textColor, fontSize: 10 }, itemWidth: 16, itemHeight: 3 },
      grid: { top: 28, right: hasQPS ? 48 : 12, bottom: 36, left: 48 },
      dataZoom: [{ type: 'slider', height: 14, bottom: 2, borderColor: 'transparent', backgroundColor: theme.lineColor, fillerColor: 'rgba(88,166,255,0.15)', handleStyle: { color: '#58a6ff' }, textStyle: { color: theme.textColor, fontSize: 9 }, showDetail: false }],
      xAxis: { type: 'category', data: node.timestamps, axisLine: { lineStyle: { color: theme.lineColor } }, axisLabel: { color: theme.textColor, fontSize: 9, interval: Math.floor((node.timestamps!.length || 1) / 6) - 1 } },
      yAxis: hasQPS ? [
        { type: 'value', axisLine: { show: false }, splitLine: { lineStyle: { color: theme.lineColor, type: 'dashed' } }, axisLabel: { color: theme.textColor, fontSize: 9, formatter: '{value}ms' } },
        { type: 'value', axisLine: { show: false }, splitLine: { show: false }, axisLabel: { color: theme.textColor, fontSize: 9, formatter: '{value}' } },
      ] : { type: 'value', axisLine: { show: false }, splitLine: { lineStyle: { color: theme.lineColor, type: 'dashed' } }, axisLabel: { color: theme.textColor, fontSize: 9, formatter: '{value}ms' } },
      tooltip: { trigger: 'axis', confine: true, formatter: (p: any[]) => {
        let s = `<div style="font-size:11px">${p[0].axisValue}</div>`
        for (const pt of p) {
          const val = pt.seriesName === 'QPS' ? pt.value.toFixed(1) : pt.value.toFixed(1) + 'ms'
          s += `<div style="display:flex;justify-content:space-between;gap:12px"><span>${pt.marker}${pt.seriesName}</span><b>${val}</b></div>`
        }
        return s
      }},
      series: series.map((s, idx) => ({
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
            { offset: 0, color: '#58a6ff' },
            { offset: 1, color: 'rgba(88,166,255,0.3)' },
          ]),
        },
      }],
      tooltip: { trigger: 'axis', formatter: (p: any[]) => `${p[0].name}: ${formatMs(p[0].value / 1000)}` },
    }, true)
  }
}

async function fetchOverview(rangeSeconds: number) {
  try {
    const resp = await dashboardOverview(rangeSeconds)
    if (resp.code === 0) {
      overview.value = resp.data
    }
  } catch (e) { /* ignore */ }
}

function handleResize() {
  qpsChart?.resize()
  latencyChart?.resize()
  errorChart?.resize()
}

watch([qpsRange, latencyRange, errorRange], () => {
  fetchOverview(qpsRange.value)
  renderQpsChart()
  renderLatencyChart()
  renderErrorChart()
})

onMounted(() => {
  fetchOverview(qpsRange.value)
  setTimeout(() => {
    renderQpsChart()
    renderLatencyChart()
    renderErrorChart()
  }, 100)
  window.addEventListener('resize', handleResize)
  pollTimer = setInterval(() => {
    fetchOverview(qpsRange.value)
    setTimeout(() => {
      renderQpsChart()
      renderLatencyChart()
      renderErrorChart()
    }, 100)
  }, 5000)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
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
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 12px;
}

.node-card {
  padding: 14px 16px;
  background: var(--bg-tertiary);
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-secondary);
}

.node-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.node-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
}

.node-id {
  font-family: monospace;
  font-size: 11px;
  color: var(--text-tertiary);
}

.node-type {
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 4px;
  margin-left: auto;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
.node-type.http { background: rgba(88,166,255,0.15); color: var(--accent-primary); }
.node-type.setup, .node-type.teardown { background: rgba(63,185,80,0.15); color: var(--accent-success); }
.node-type.delay { background: rgba(210,153,34,0.15); color: var(--accent-warning); }
.node-type.condition { background: rgba(163,113,247,0.15); color: #a371f7; }

.node-qps {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin-bottom: 10px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border-secondary);
}

.qps-label {
  font-size: 11px;
  color: var(--text-tertiary);
}

.qps-value {
  font-size: 18px;
  font-weight: 700;
  color: var(--accent-primary);
}

.node-bars {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.bar-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.bar-label {
  font-size: 11px;
  color: var(--text-tertiary);
  width: 28px;
  flex-shrink: 0;
}

.bar-track {
  flex: 1;
  height: 6px;
  background: var(--bg-hover);
  border-radius: 3px;
  overflow: hidden;
}

.bar-fill {
  height: 100%;
  border-radius: 3px;
  background: var(--accent-success);
  transition: width 0.3s ease;
}

.bar-fill.warn { background: var(--accent-warning); }
.bar-fill.danger { background: var(--accent-danger); }

.bar-value {
  font-size: 11px;
  color: var(--text-secondary);
  width: 60px;
  text-align: right;
  flex-shrink: 0;
}

.node-section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}
.node-section-header h3 { margin-bottom: 0; }
.time-range-label {
  font-size: 12px;
  color: var(--text-tertiary);
  font-family: monospace;
}

.node-card { cursor: pointer; transition: border-color 0.15s ease, box-shadow 0.15s ease; }
.node-card:hover { border-color: var(--accent-primary); }
.node-card.expanded {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 1px rgba(88,166,255,0.2);
  grid-column: 1 / -1;
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
