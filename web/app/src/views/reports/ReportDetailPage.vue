<template>
  <div class="report-detail">
    <div class="page-header">
      <button class="btn-back" @click="$router.push('/reports')">← 返回</button>
      <h2>测试报告详情</h2>
      <button class="btn-export" @click="exportHTML" :disabled="!report">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
        导出HTML报告
      </button>
    </div>

    <div v-if="report && metrics" ref="reportRef">
      <div class="metrics-row">
        <div v-for="m in metricCards" :key="m.key" class="metric-card" :class="'card-' + m.level">
          <div class="metric-label">{{ m.label }}</div>
          <div class="metric-value">{{ m.value }}</div>
          <div v-if="m.sub" class="metric-sub">{{ m.sub }}</div>
        </div>
      </div>

      <div class="charts-grid">
        <div class="chart-card wide">
          <h3>QPS 趋势</h3>
          <div ref="qpsChartRef" class="chart-body"></div>
        </div>
        <div class="chart-card wide">
          <h3>延迟趋势 (P50/P95/P99)</h3>
          <div ref="latencyTrendChartRef" class="chart-body"></div>
        </div>
        <div class="chart-card">
          <h3>延迟分布</h3>
          <div ref="latencyChartRef" class="chart-body"></div>
        </div>
        <div class="chart-card">
          <h3>请求概览</h3>
          <div ref="overviewChartRef" class="chart-body"></div>
        </div>
      </div>

      <div v-if="nodeTimeSeries && nodeTimeSeries.length > 0" class="node-charts-section">
        <h3 class="section-title">各节点性能趋势</h3>
        <div v-for="(node, idx) in nodeTimeSeries" :key="node.node_id" class="chart-card wide">
          <h4>{{ node.name || node.node_id }}</h4>
          <div :ref="el => setNodeChartRef(idx, el as HTMLElement)" class="chart-body"></div>
        </div>
      </div>

      <div class="info-sections">
        <div class="info-card">
          <h3>运行信息</h3>
          <table class="info-table">
            <tbody>
            <tr><td class="info-label">场景ID</td><td>{{ report.scene_id }}</td></tr>
            <tr><td class="info-label">运行ID</td><td class="mono">{{ report.run_id }}</td></tr>
            <tr><td class="info-label">状态</td><td><span :class="['status-badge', 'st-' + report.status]">{{ report.status.toUpperCase() }}</span></td></tr>
            <tr><td class="info-label">运行模式</td><td>{{ metrics.run_mode || '-' }}</td></tr>
            <tr><td class="info-label">并发数</td><td>{{ metrics.worker_count || '-' }}</td></tr>
            <tr><td class="info-label">开始时间</td><td>{{ formatTime(report.started_at) }}</td></tr>
            <tr><td class="info-label">结束时间</td><td>{{ formatTime(report.finished_at) }}</td></tr>
            <tr><td class="info-label">总耗时</td><td>{{ metrics.duration_s ? Number(metrics.duration_s).toFixed(2) + 's' : '-' }}</td></tr>
            </tbody>
          </table>
        </div>

        <div class="info-card">
          <h3>性能指标</h3>
          <table class="info-table">
            <tbody>
            <tr><td class="info-label">总请求数</td><td class="bold">{{ metrics.total_reqs ?? '-' }}</td></tr>
            <tr><td class="info-label">成功请求</td><td class="text-success">{{ metrics.success_reqs ?? '-' }}</td></tr>
            <tr><td class="info-label">失败请求</td><td class="text-danger">{{ metrics.failed_reqs ?? '-' }}</td></tr>
            <tr><td class="info-label">成功率</td><td class="bold">{{ metrics.success_rate ?? '-' }}</td></tr>
            <tr><td class="info-label">平均延迟</td><td>{{ fmtLatency(metrics.avg_latency_s) }}</td></tr>
            <tr><td class="info-label">P50 延迟</td><td>{{ fmtLatency(metrics.p50_latency_s) }}</td></tr>
            <tr><td class="info-label">P95 延迟</td><td>{{ fmtLatency(metrics.p95_latency_s) }}</td></tr>
            <tr><td class="info-label">P99 延迟</td><td>{{ fmtLatency(metrics.p99_latency_s) }}</td></tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
    <div v-else-if="!report" class="empty">加载中...</div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import { getReport } from '@/api/report'
// import { exportReportHTML } from '@/api/report'
import type { ReportDTO } from '@/types'
import * as echarts from 'echarts'

const route = useRoute()
const report = ref<ReportDTO | null>(null)
const metrics = ref<Record<string, any> | null>(null)
const reportRef = ref<HTMLElement>()
const latencyChartRef = ref<HTMLElement>()
const overviewChartRef = ref<HTMLElement>()
const qpsChartRef = ref<HTMLElement>()
const latencyTrendChartRef = ref<HTMLElement>()
let latChart: echarts.ECharts | null = null
let ovChart: echarts.ECharts | null = null
let qpsChart: echarts.ECharts | null = null
let latTrendChart: echarts.ECharts | null = null

interface NodeTimeSeries {
  node_id: string
  name?: string
  timestamps?: string[]
  ts_qps?: number[]
  ts_p50?: number[]
  ts_p95?: number[]
  ts_p99?: number[]
}

const nodeTimeSeries = ref<NodeTimeSeries[]>([])
const nodeChartRefs = new Map<number, HTMLElement>()
const nodeCharts = new Map<number, echarts.ECharts>()

function setNodeChartRef(idx: number, el: HTMLElement | null) {
  if (el) {
    nodeChartRefs.set(idx, el)
  }
}

function parseMetrics(r: ReportDTO): Record<string, any> {
  try {
    let result: Record<string, any> = {}
    
    if (r.summary) {
      const summaryData = typeof r.summary === 'string' ? JSON.parse(r.summary) : r.summary
      result = { ...result, ...summaryData }
      console.log('📊 Parsed summary:', Object.keys(summaryData))
    }
    
    if (r.detail) {
      const detailData = typeof r.detail === 'string' ? JSON.parse(r.detail) : r.detail
      console.log('📊 Parsed detail keys:', Object.keys(detailData))
      console.log('📊 Detail structure:', {
        hasMetadata: !!detailData.metadata,
        hasGlobalSummary: !!detailData.global_summary,
        hasGlobalTimeSeries: !!detailData.global_time_series,
        globalTimeSeriesType: typeof detailData.global_time_series,
        globalTimeSeriesIsArray: Array.isArray(detailData.global_time_series),
        globalTimeSeriesLength: Array.isArray(detailData.global_time_series) ? detailData.global_time_series.length : 'N/A',
        hasNodeMetrics: !!detailData.node_metrics,
        nodeMetricsType: typeof detailData.node_metrics,
        nodeMetricsIsArray: Array.isArray(detailData.node_metrics),
        nodeMetricsLength: Array.isArray(detailData.node_metrics) ? detailData.node_metrics.length : 'N/A',
      })
      
      if (detailData.global_summary) {
        result = { ...result, ...detailData.global_summary }
        console.log('✅ Merged global_summary:', Object.keys(detailData.global_summary))
      }
      
      if (detailData.global_time_series && Array.isArray(detailData.global_time_series)) {
        result.global_time_series = detailData.global_time_series
        console.log('✅ Found global_time_series with', detailData.global_time_series.length, 'samples')
        if (detailData.global_time_series.length > 0) {
          console.log('📈 First sample:', JSON.stringify(detailData.global_time_series[0]))
          console.log('📈 Last sample:', JSON.stringify(detailData.global_time_series[detailData.global_time_series.length - 1]))
        }
      } else {
        console.warn('⚠️ No global_time_series found or not array')
        console.warn('   detailData.global_time_series value:', detailData.global_time_series)
        console.warn('   All available keys in detailData:', Object.keys(detailData))
        
        // 尝试查找可能的时序数据字段
        const timeSeriesKeys = Object.keys(detailData).filter(k => 
          k.toLowerCase().includes('time') || 
          k.toLowerCase().includes('series') || 
          k.toLowerCase().includes('sample') ||
          k.toLowerCase().includes('qps') ||
          k.toLowerCase().includes('latency')
        )
        if (timeSeriesKeys.length > 0) {
          console.warn('   Possible time series keys found:', timeSeriesKeys)
          timeSeriesKeys.forEach(k => {
            console.warn(`   ${k}:`, typeof detailData[k], Array.isArray(detailData[k]) ? `array[${detailData[k].length}]` : detailData[k])
          })
        }
      }
      
      if (detailData.node_metrics && Array.isArray(detailData.node_metrics)) {
        result.node_metrics = detailData.node_metrics
        console.log('✅ Found node_metrics with', detailData.node_metrics.length, 'nodes')
      }
      
      if (detailData.metadata) {
        result = { ...result, ...detailData.metadata }
        console.log('✅ Merged metadata:', Object.keys(detailData.metadata))
      }
    }
    
    return result
  } catch (e) {
    console.error('❌ Parse metrics error:', e)
    return {}
  }
}

const metricCards = computed(() => {
  const m = metrics.value
  if (!m) return []
  const totalReqs = Number(m.total_reqs || 0)
  
  let successRate: number
  if (typeof m.success_rate === 'string') {
    successRate = parseFloat(m.success_rate.replace('%', '') || '0')
  } else if (typeof m.success_rate === 'number') {
    successRate = m.success_rate * 100  // 转换为百分比
  } else {
    successRate = 0
  }
  
  const p99ms = parseFloat(m.p99 || '0')
  
  const successRateDisplay = typeof m.success_rate === 'string' 
    ? m.success_rate 
    : `${(successRate).toFixed(1)}%`
    
  return [
    { key: 'total', label: '总请求数', value: totalReqs.toLocaleString(), sub: '', level: 'primary' },
    { key: 'success', label: '成功率', value: successRateDisplay, sub: `${m.success_reqs}/${totalReqs}`, level: successRate >= 99 ? 'success' : successRate >= 90 ? 'warn' : 'danger' },
    { key: 'p50', label: 'P50 延迟', value: m.p50 || '-', sub: `P95: ${m.p95 || '-'} / P99: ${m.p99 || '-'}`, level: p99ms < 500 ? 'ok' : p99ms < 1000 ? 'warn' : 'danger' },
    { key: 'duration', label: '总耗时', value: m.duration_s ? Number(m.duration_s).toFixed(1) + 's' : '-', sub: m.worker_count ? `${m.worker_count} 并发` : '', level: 'info' },
  ]
})

async function fetchReport() {
  const id = route.params.id as string
  if (!id) return
  try {
    console.log('📊 Fetching report:', id)
    const resp = await getReport(id)
    if (resp.code === 0) {
      report.value = resp.data
      console.log('📋 Report data received:', {
        scene_id: resp.data.scene_id,
        run_id: resp.data.run_id,
        hasSummary: !!resp.data.summary,
        hasDetail: !!resp.data.detail,
        summaryLength: resp.data.summary?.length || 0,
        detailLength: resp.data.detail?.length || 0
      })
      
      const parsed = parseMetrics(resp.data)
      
      console.log('🔍 Parsed metrics keys:', Object.keys(parsed))
      console.log('🔍 Global time series:', parsed.global_time_series, 'type:', typeof parsed.global_time_series, 'isArray:', Array.isArray(parsed.global_time_series))
      
      if (parsed.global_time_series && Array.isArray(parsed.global_time_series)) {
        const ts = parsed.global_time_series as any[]
        console.log('✅ Time series found! Length:', ts.length)
        if (ts.length > 0) {
          console.log('📈 First sample:', JSON.stringify(ts[0]))
          console.log('📈 Last sample:', JSON.stringify(ts[ts.length - 1]))
        }
        
        parsed.timestamps = ts.map(s => s.t || s.Timestamp)
        parsed.ts_qps = ts.map(s => s.qps || s.QPS || 0)
        parsed.ts_p50 = ts.map(s => s.p50_ms || s.P50LatencyMs || 0)
        parsed.ts_p95 = ts.map(s => s.p95_ms || s.P95LatencyMs || 0)
        parsed.ts_p99 = ts.map(s => s.p99_ms || s.P99LatencyMs || 0)
        
        console.log('🎯 Converted time series:')
        console.log('   - timestamps count:', parsed.timestamps?.length)
        console.log('   - ts_qps count:', parsed.ts_qps?.length)
        console.log('   - ts_p50 count:', parsed.ts_p50?.length)
      } else {
        console.warn('⚠️ No global_time_series found or not array')
        console.warn('   Available keys:', Object.keys(parsed).filter(k => k.includes('time') || k.includes('series')))
      }
      
      metrics.value = parsed
      
      const m = metrics.value || {}
      nodeTimeSeries.value = (m.node_metrics as NodeTimeSeries[]) || []
      console.log('📦 Node time series count:', nodeTimeSeries.value.length)
      
      await nextTick()
      renderCharts()
    }
  } catch (e) { 
    console.error('❌ Fetch report error:', e)
  }
}

function getTheme() {
  const isDark = document.documentElement.getAttribute('data-theme') !== 'light'
  return {
    textColor: '#8b949e',
    lineColor: '#30363d',
    bgColor: 'transparent',
    gridColor: isDark ? '#21262d' : '#f6f8fa',
    colors: ['#58a6ff', '#3fb950', '#d29922', '#f85149'],
  }
}

function renderCharts() {
  if (!latencyChartRef.value || !overviewChartRef.value) return
  const t = getTheme()
  const m = metrics.value || {}

  if (latChart) latChart.dispose()
  latChart = echarts.init(latencyChartRef.value)
  latChart.setOption({
    backgroundColor: t.bgColor,
    color: t.colors,
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    grid: { left: 40, right: 16, top: 16, bottom: 30 },
    xAxis: { type: 'category', data: ['Avg', 'P50', 'P95', 'P99'], axisLabel: { color: t.textColor, fontSize: 11 }, axisLine: { lineStyle: { color: t.lineColor } } },
    yAxis: { type: 'value', name: 'ms', axisLabel: { color: t.textColor, fontSize: 11 }, splitLine: { lineStyle: { color: t.gridColor } } },
    series: [{
      type: 'bar',
      barWidth: '45%',
      data: [
        { value: parseFloat(m.avg_latency_s || '0') * 1000, itemStyle: { color: t.colors[0] } },
        { value: parseFloat(m.p50_latency_s || '0') * 1000, itemStyle: { color: t.colors[1] } },
        { value: parseFloat(m.p95_latency_s || '0') * 1000, itemStyle: { color: t.colors[2] } },
        { value: parseFloat(m.p99_latency_s || '0') * 1000, itemStyle: { color: t.colors[3] } },
      ],
      label: { show: true, position: 'top', formatter: '{c}ms', color: t.textColor, fontSize: 10 },
    }],
  })

  if (ovChart) ovChart.dispose()
  ovChart = echarts.init(overviewChartRef.value)
  const total = Number(m.total_reqs || 0)
  const succ = Number(m.success_reqs || 0)
  const fail = Number(m.failed_reqs || 0)
  ovChart.setOption({
    backgroundColor: t.bgColor,
    color: [t.colors[1], t.colors[3]],
    tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
    legend: { orient: 'vertical', right: 16, top: 'center', textStyle: { color: t.textColor, fontSize: 12 } },
    series: [{
      type: 'pie',
      radius: ['42%', '68%'],
      center: ['38%', '50%'],
      avoidLabelOverlap: false,
      label: { show: false },
      emphasis: { label: { show: true, fontSize: 14, fontWeight: 'bold' } },
      data: [
        { value: succ, name: '成功' },
        { value: fail, name: '失败' },
      ],
    }],
  })

  renderQPSTrend(t, m)
  renderLatencyTrend(t, m)
  renderNodeCharts(t)
  
  console.log('✅ All charts rendered successfully')
}

function renderQPSTrend(t: any, m: any) {
  console.log('🎨 Rendering QPS trend chart...')
  if (!qpsChartRef.value) {
    console.warn('⚠️ qpsChartRef not found')
    return
  }
  if (qpsChart) qpsChart.dispose()
  qpsChart = echarts.init(qpsChartRef.value)

  const timestamps = m.timestamps as string[] | undefined
  const qpsData = (m.ts_qps as number[] | undefined) || []

  console.log('📊 QPS data check:', {
    timestampsLength: timestamps?.length || 0,
    qpsDataLength: qpsData.length,
    hasTimestamps: !!timestamps,
    timestampsSample: timestamps?.slice(0, 3),
    qpsSample: qpsData.slice(0, 3)
  })

  if (!timestamps || timestamps.length === 0 || qpsData.length === 0) {
    console.warn('⚠️ No QPS time series data - showing empty message')
    qpsChart.setOption({
      title: { text: '暂无QPS时间序列数据', left: 'center', top: 'center', textStyle: { color: t.textColor, fontSize: 14 } }
    })
    return
  }

  const timeLabels = timestamps.map(ts => formatTimeShort(ts))
  console.log('📈 QPS chart rendering with', timeLabels.length, 'data points')

  qpsChart.setOption({
    backgroundColor: t.bgColor,
    tooltip: { trigger: 'axis' },
    legend: { data: ['QPS'], top: 8, textStyle: { color: t.textColor } },
    grid: { left: 50, right: 20, top: 40, bottom: 30 },
    xAxis: { 
      type: 'category', 
      data: timeLabels, 
      axisLabel: { color: t.textColor, fontSize: 10, rotate: 30 },
      axisLine: { lineStyle: { color: t.lineColor } }
    },
    yAxis: { 
      type: 'value', 
      name: 'req/s',
      axisLabel: { color: t.textColor, fontSize: 11 }, 
      splitLine: { lineStyle: { color: t.gridColor } } 
    },
    series: [{
      name: 'QPS',
      type: 'line',
      smooth: true,
      data: qpsData,
      lineStyle: { width: 2, color: '#58a6ff' },
      areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
        { offset: 0, color: 'rgba(88,166,255,0.3)' },
        { offset: 1, color: 'rgba(88,166,255,0.02)' }
      ])},
      symbol: 'none',
    }]
  })
}

function renderLatencyTrend(t: any, m: any) {
  if (!latencyTrendChartRef.value) return
  if (latTrendChart) latTrendChart.dispose()
  latTrendChart = echarts.init(latencyTrendChartRef.value)

  const timestamps = m.timestamps as string[] | undefined
  const p50Data = (m.ts_p50 as number[] | undefined) || []
  const p95Data = (m.ts_p95 as number[] | undefined) || []
  const p99Data = (m.ts_p99 as number[] | undefined) || []

  if (!timestamps || timestamps.length === 0) {
    latTrendChart.setOption({
      title: { text: '暂无延迟时间序列数据', left: 'center', top: 'center', textStyle: { color: t.textColor, fontSize: 14 } }
    })
    return
  }

  const timeLabels = timestamps.map(ts => formatTimeShort(ts))

  latTrendChart.setOption({
    backgroundColor: t.bgColor,
    tooltip: { trigger: 'axis' },
    legend: { data: ['P50', 'P95', 'P99'], top: 8, textStyle: { color: t.textColor } },
    grid: { left: 50, right: 20, top: 40, bottom: 30 },
    xAxis: { 
      type: 'category', 
      data: timeLabels, 
      axisLabel: { color: t.textColor, fontSize: 10, rotate: 30 },
      axisLine: { lineStyle: { color: t.lineColor } }
    },
    yAxis: { 
      type: 'value', 
      name: 'ms',
      axisLabel: { color: t.textColor, fontSize: 11 }, 
      splitLine: { lineStyle: { color: t.gridColor } } 
    },
    series: [
      {
        name: 'P50',
        type: 'line',
        smooth: true,
        data: p50Data.map(v => v * 1000),
        lineStyle: { width: 2, color: '#3fb950' },
        symbol: 'none',
      },
      {
        name: 'P95',
        type: 'line',
        smooth: true,
        data: p95Data.map(v => v * 1000),
        lineStyle: { width: 2, color: '#d29922' },
        symbol: 'none',
      },
      {
        name: 'P99',
        type: 'line',
        smooth: true,
        data: p99Data.map(v => v * 1000),
        lineStyle: { width: 2, color: '#f85149' },
        symbol: 'none',
      },
    ]
  })
}

function renderNodeCharts(t: any) {
  nodeTimeSeries.value.forEach((node, idx) => {
    const el = nodeChartRefs.get(idx)
    if (!el) return

    const chart = echarts.init(el)
    nodeCharts.set(idx, chart)

    const timestamps = node.timestamps || []
    const timeLabels = timestamps.map((ts: string) => formatTimeShort(ts))
    
    const qpsData = node.ts_qps || []
    const p95Data = (node.ts_p95 || []).map((v: number) => v * 1000)

    chart.setOption({
      backgroundColor: t.bgColor,
      tooltip: { trigger: 'axis' },
      legend: { data: ['QPS', 'P95'], top: 4, textStyle: { color: t.textColor, fontSize: 11 } },
      grid: { left: 55, right: 20, top: 35, bottom: 25 },
      xAxis: {
        type: 'category',
        data: timeLabels,
        axisLabel: { color: t.textColor, fontSize: 9, rotate: 20 },
        axisLine: { lineStyle: { color: t.lineColor } }
      },
      yAxis: [
        {
          type: 'value',
          name: 'QPS',
          position: 'left',
          axisLabel: { color: t.textColor, fontSize: 10 },
          splitLine: { lineStyle: { color: t.gridColor } }
        },
        {
          type: 'value',
          name: 'P95(ms)',
          position: 'right',
          axisLabel: { color: t.textColor, fontSize: 10 },
        }
      ],
      series: [
        {
          name: 'QPS',
          type: 'bar',
          yAxisIndex: 0,
          data: qpsData,
          itemStyle: { color: '#58a6ff', opacity: 0.7 },
          barWidth: '40%',
        },
        {
          name: 'P95',
          type: 'line',
          yAxisIndex: 1,
          smooth: true,
          data: p95Data,
          lineStyle: { width: 2, color: '#f85149' },
          symbol: 'circle',
          symbolSize: 4,
        }
      ]
    })
  })
}

function formatTimeShort(timeStr?: string): string {
  if (!timeStr) return ''
  const d = new Date(timeStr)
  return `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}`
}

function fmtLatency(v: any): string {
  if (!v) return '-'
  const s = Number(v)
  if (s <= 0) return '-'
  if (s < 1) return (s * 1000).toFixed(1) + 'ms'
  return s.toFixed(3) + 's'
}

function formatTime(t?: string) {
  if (!t) return '-'
  return new Date(t).toLocaleString()
}

async function exportHTML() {
  if (!report.value || !metrics.value) return
  alert('HTML 报告导出功能开发中，敬请期待！')
}

onMounted(fetchReport)

let resizeTimer: ReturnType<typeof setTimeout>
window.addEventListener('resize', () => {
  clearTimeout(resizeTimer)
  resizeTimer = setTimeout(() => {
    latChart?.resize()
    ovChart?.resize()
    qpsChart?.resize()
    latTrendChart?.resize()
    nodeCharts.forEach(chart => chart.resize())
  }, 150)
})

onUnmounted(() => {
  latChart?.dispose()
  ovChart?.dispose()
  qpsChart?.dispose()
  latTrendChart?.dispose()
  nodeCharts.forEach(chart => chart.dispose())
})
</script>

<style scoped>
.report-detail { display: flex; flex-direction: column; gap: 16px; }
.page-header { display: flex; align-items: center; gap: 12px; }
.page-header h2 { font-size: 18px; font-weight: 600; flex: 1; }
.btn-back { padding: 6px 12px; border: 1px solid var(--border-primary); border-radius: var(--radius-sm); background: transparent; color: var(--text-secondary); font-size: 13px; cursor: pointer; }
.btn-export { display: flex; align-items: center; gap: 6px; padding: 7px 16px; border: 1px solid var(--accent-primary); border-radius: var(--radius-sm); background: rgba(88,166,255,0.08); color: var(--accent-primary); font-size: 13px; cursor: pointer; transition: all 0.15s; }
.btn-export:hover:not(:disabled) { background: rgba(88,166,255,0.16); }
.btn-export:disabled { opacity: 0.4; cursor: not-allowed; }

.metrics-row { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 12px; }
.metric-card { background: var(--bg-card); border: 1px solid var(--border-secondary); border-radius: var(--radius-md); padding: 18px 20px; text-align: center; border-left: 3px solid transparent; }
.card-primary { border-left-color: var(--accent-primary); }
.card-success { border-left-color: var(--accent-success); }
.card-warn { border-left-color: var(--accent-warning); }
.card-danger { border-left-color: var(--accent-danger); }
.card-info { border-left-color: var(--accent-info); }
.metric-label { font-size: 11px; text-transform: uppercase; letter-spacing: 0.8px; color: var(--text-tertiary); margin-bottom: 6px; }
.metric-value { font-size: 28px; font-weight: 700; color: var(--text-primary); line-height: 1.2; }
.metric-sub { font-size: 11px; color: var(--text-tertiary); margin-top: 4px; }

.charts-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.chart-card { background: var(--bg-card); border: 1px solid var(--border-secondary); border-radius: var(--radius-md); padding: 16px; }
.chart-card.wide { grid-column: span 2; }
.chart-card h3 { font-size: 13px; font-weight: 600; margin-bottom: 8px; color: var(--text-secondary); }
.chart-card h4 { font-size: 12px; font-weight: 600; margin-bottom: 6px; color: var(--text-primary); }
.chart-body { height: 260px; }

.node-charts-section {
  margin-top: 8px;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 10px;
  padding-left: 4px;
}

.info-sections { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.info-card { background: var(--bg-card); border: 1px solid var(--border-secondary); border-radius: var(--radius-md); padding: 16px; }
.info-card h3 { font-size: 13px; font-weight: 600; margin-bottom: 10px; color: var(--text-secondary); }
.info-table { width: 100%; border-collapse: collapse; }
.info-table td { padding: 7px 0; font-size: 13px; border-bottom: 1px solid var(--border-secondary); }
.info-table tr:last-child td { border-bottom: none; }
.info-label { color: var(--text-secondary); min-width: 80px; }
.mono { font-family: monospace; font-size: 12px; }
.bold { font-weight: 600; }
.text-success { color: var(--accent-success); }
.text-danger { color: var(--accent-danger); }

.status-badge { font-size: 11px; padding: 3px 10px; border-radius: 12px; font-weight: 600; letter-spacing: 0.5px; }
.st-success { background: rgba(63,185,80,0.15); color: #3fb950; border: 1px solid rgba(63,185,80,0.25); }
.st-failed { background: rgba(248,81,73,0.15); color: #f85149; border: 1px solid rgba(248,81,73,0.25); }
.st-partial { background: rgba(210,153,34,0.15); color: #d29922; border: 1px solid rgba(210,153,34,0.25); }

.empty { text-align: center; color: var(--text-tertiary); padding: 48px 0; }

@media (max-width: 768px) {
  .charts-grid, .info-sections { grid-template-columns: 1fr; }
  .metrics-row { grid-template-columns: repeat(2, 1fr); }
}
</style>
