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
          <div class="chart-tip">拖动下方滑块或使用鼠标框选查看特定时间范围</div>
        </div>
        <div class="chart-body" ref="qpsChartRef"></div>
      </div>

      <div class="chart-card">
        <div class="chart-header">
          <h3>延迟分布</h3>
          <div class="chart-tip">点击图例可显示/隐藏对应线条</div>
        </div>
        <div class="chart-body" ref="latencyChartRef"></div>
      </div>
    </div>

    <div class="charts-row">
      <div class="chart-card wide">
        <div class="chart-header">
          <h3>错误率</h3>
          <div class="chart-tip">拖动下方滑块或使用鼠标框选查看特定时间范围</div>
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
                <div class="bar-track"><div class="bar-fill p50" :style="{ width: barWidth(node.p50_latency) }"></div></div>
                <span class="bar-value">{{ formatMs(node.p50_latency) }}</span>
              </div>
              <div class="bar-row">
                <span class="bar-label">P95</span>
                <div class="bar-track"><div class="bar-fill p95" :style="{ width: barWidth(node.p95_latency) }"></div></div>
                <span class="bar-value">{{ formatMs(node.p95_latency) }}</span>
              </div>
              <div class="bar-row">
                <span class="bar-label">P99</span>
                <div class="bar-track"><div class="bar-fill p99" :style="{ width: barWidth(node.p99_latency) }"></div></div>
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
  if (ms < 1000) return ms.toFixed(3) + 'ms'
  return (ms / 1000).toFixed(3) + 's'
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
    dataZoom: [{ type: 'slider', height: 18, bottom: 4, borderColor: 'transparent', backgroundColor: theme.lineColor, fillerColor: `rgba(${theme.colors.primary === '#0ea5e9' ? '14, 165, 233' : '88, 166, 255'}, 0.15)`, handleStyle: { color: theme.colors.primary }, textStyle: { color: theme.textColor, fontSize: 10 }, brushSelect: true }],
    xAxis: { type: 'category', data: ts.timestamps, axisLine: { lineStyle: { color: theme.lineColor } }, axisLabel: { color: theme.textColor, fontSize: 10 } },
    yAxis: { type: 'value', axisLine: { show: false }, splitLine: { lineStyle: { color: theme.lineColor, type: 'dashed' } }, axisLabel: { color: theme.textColor, fontSize: 10 } },
    series: [{ data: ts.qps, type: 'line', smooth: true, symbol: 'none', lineStyle: { color: theme.colors.primary, width: 2 }, areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{ offset: 0, color: `rgba(${theme.colors.primary === '#0ea5e9' ? '14, 165, 233' : '88, 166, 255'}, 0.3)` }, { offset: 1, color: 'rgba(88,166,255,0)' }]) } }],
    tooltip: { 
      trigger: 'axis',
      formatter: (params: any[]) => {
        const param = params[0]
        const value = Number(param.value).toFixed(3)
        return `<div style="font-size:12px">${param.axisValue}</div><div style="display:flex;justify-content:space-between;gap:16px"><span>${param.marker}QPS</span><b>${value}</b></div>`
      }
    },
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
    dataZoom: [{ type: 'slider', height: 18, bottom: 4, borderColor: 'transparent', backgroundColor: theme.lineColor, fillerColor: `rgba(${theme.colors.primary === '#0ea5e9' ? '14, 165, 233' : '88, 166, 255'}, 0.15)`, handleStyle: { color: theme.colors.primary }, textStyle: { color: theme.textColor, fontSize: 10 }, brushSelect: true }],
    xAxis: { type: 'category', data: ts.timestamps, axisLine: { lineStyle: { color: theme.lineColor } }, axisLabel: { color: theme.textColor, fontSize: 10 } },
    yAxis: { type: 'value', axisLine: { show: false }, splitLine: { lineStyle: { color: theme.lineColor, type: 'dashed' } }, axisLabel: { color: theme.textColor, fontSize: 10, formatter: '{value}ms' } },
    series: [
      { name: 'P50', data: ts.p50, type: 'line', smooth: true, symbol: 'none', lineStyle: { color: theme.colors.info, width: 2 } },
      { name: 'P95', data: ts.p95, type: 'line', smooth: true, symbol: 'none', lineStyle: { color: theme.colors.warning, width: 2 } },
      { name: 'P99', data: ts.p99, type: 'line', smooth: true, symbol: 'none', lineStyle: { color: theme.colors.danger, width: 2 } },
    ],
    tooltip: { 
      trigger: 'axis',
      formatter: (params: any[]) => {
        let result = `<div style="font-size:12px;margin-bottom:6px">${params[0].axisValue}</div>`
        for (const param of params) {
          const value = Number(param.value).toFixed(3)
          result += `<div style="display:flex;justify-content:space-between;gap:16px"><span>${param.marker}${param.seriesName}</span><b>${value}ms</b></div>`
        }
        return result
      }
    },
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
    dataZoom: [{ type: 'slider', height: 18, bottom: 4, borderColor: 'transparent', backgroundColor: theme.lineColor, fillerColor: `rgba(${theme.colors.primary === '#0ea5e9' ? '14, 165, 233' : '88, 166, 255'}, 0.15)`, handleStyle: { color: theme.colors.primary }, textStyle: { color: theme.textColor, fontSize: 10 }, brushSelect: true }],
    xAxis: { type: 'category', data: ts.timestamps, axisLine: { lineStyle: { color: theme.lineColor } }, axisLabel: { color: theme.textColor, fontSize: 10 } },
    yAxis: { type: 'value', min: 0, max: ts.error_rate.some(v => v > 0) ? undefined : 1, axisLine: { show: false }, splitLine: { lineStyle: { color: theme.lineColor, type: 'dashed' } }, axisLabel: { color: theme.textColor, fontSize: 10, formatter: '{value}%' } },
    series: [{ data: ts.error_rate, type: 'bar', barMinHeight: 3, itemStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{ offset: 0, color: theme.colors.danger }, { offset: 1, color: `rgba(${theme.colors.danger === '#ef4444' ? '239, 68, 68' : '248, 81, 73'}, 0.3)` }]) } }],
    tooltip: { 
      trigger: 'axis',
      formatter: (params: any[]) => {
        const param = params[0]
        const value = Number(param.value).toFixed(3)
        return `<div style="font-size:12px">${param.axisValue}</div><div style="display:flex;justify-content:space-between;gap:16px"><span>${param.marker}错误率</span><b>${value}%</b></div>`
      }
    },
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
      tooltip: { trigger: 'axis', confine: true, formatter: (p: any[]) => {
        let s = `<div style="font-size:11px">${p[0].axisValue}</div>`
        for (const pt of p) {
          const val = pt.seriesName === 'QPS' ? pt.value.toFixed(3) : pt.value.toFixed(3) + 'ms'
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
            { offset: 0, color: theme.colors.primary },
            { offset: 1, color: `rgba(${theme.colors.primary === '#0ea5e9' ? '14, 165, 233' : '88, 166, 255'}, 0.3)` },
          ]),
        },
      }],
      tooltip: { trigger: 'axis', formatter: (p: any[]) => {
        const val = (p[0].value / 1000).toFixed(3)
        return `${p[0].name}: ${val}ms`
      } },
    }, true)
  }
}

async function fetchOverview() {
  try {
    // Fetch all available data without time range limit
    const resp = await dashboardOverview(0)
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

onMounted(() => {
  fetchOverview()
  setTimeout(() => {
    renderQpsChart()
    renderLatencyChart()
    renderErrorChart()
  }, 100)
  window.addEventListener('resize', handleResize)
  pollTimer = setInterval(() => {
    fetchOverview()
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
  overflow: hidden;
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
