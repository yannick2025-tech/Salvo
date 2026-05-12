<template>
  <div class="trace-detail">
    <div class="page-header">
      <button class="btn-back" @click="$router.push('/traces')">← 返回</button>
      <h2>追踪详情</h2>
    </div>

    <div v-if="trace" class="detail-card">
      <div class="detail-row"><span class="label">TraceID</span><span class="value mono" :title="'数据库Trace主键'">{{ trace.id }}</span></div>
      <div class="detail-row"><span class="label">RunID</span><span class="value mono" :title="'可在日志中搜索此ID'">{{ trace.run_id }}</span></div>
      <div class="detail-row"><span class="label">场景</span><span class="value">{{ trace.scene_name || trace.scene_id }}</span></div>
      <div class="detail-row"><span class="label">状态</span><span :class="['status-badge', 'st-' + trace.status]">{{ trace.status.toUpperCase() }}</span></div>
      <div class="detail-row"><span class="label">耗时</span><span class="value">{{ formatDuration(trace.duration_ns) }}</span></div>
      <div class="detail-row"><span class="label">开始时间</span><span class="value">{{ formatTime(trace.started_at) }}</span></div>
      <div class="detail-row"><span class="label">结束时间</span><span class="value">{{ formatTime(trace.finished_at) }}</span></div>
    </div>

    <div v-if="trace && trace.spans" class="spans-card">
      <h3>Span列表 <span class="count">(共 {{ trace.spans.length }} 个)</span></h3>
      <div class="legend">
        <span class="legend-item"><i class="dot lat-ok"></i> &lt;200毫秒</span>
        <span class="legend-item"><i class="dot lat-warn"></i> 200~600毫秒</span>
        <span class="legend-item"><i class="dot lat-alert"></i> 600~1500毫秒</span>
        <span class="legend-item"><i class="dot lat-critical"></i> ≥1500毫秒</span>
      </div>
      <div class="span-list" ref="wrapperRef">
        <div v-for="(span, idx) in trace.spans" :key="span.id" class="span-item">
          <div class="span-header">
            <div class="span-name-area">
              <span class="span-index">#{{ idx + 1 }}</span>
              <span class="span-name">{{ span.node_name || span.node_id }}</span>
              <span class="span-node-id mono">{{ span.node_id }}</span>
            </div>
            <span :class="['status-badge', 'st-' + span.status]">{{ span.status.toUpperCase() }}</span>
          </div>
          <div class="span-bar-wrapper" :style="{ width: containerWidth + 'px' }">
            <div class="span-wait-bar">等待 {{ spanWaitMs(span).toFixed(3) }}ms</div>
            <div class="span-bar" :style="getSpanBarStyle(span)">{{ formatDurationMs(span.duration_ns) }}</div>
          </div>
          <div class="span-meta">
            <span :class="['latency-text', 'lat-' + latencyLevel(span)]">耗时: {{ formatDuration(span.duration_ns) }}</span>
            <span v-if="span.chain_id" class="chain-id mono">链路ID: {{ span.chain_id }}</span>
            <span v-if="span.error" class="error-text">错误: {{ span.error }}</span>
          </div>
        </div>
      </div>
    </div>
    <div v-else-if="!trace" class="empty">加载中...</div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, nextTick, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { getTrace } from '@/api/trace'
import type { TraceDTO, SpanDTO } from '@/types'

const route = useRoute()
const trace = ref<TraceDTO | null>(null)

async function fetchTrace() {
  const id = route.params.id as string
  if (!id) return
  try {
    const resp = await getTrace(id)
    if (resp.code === 0) {
      trace.value = resp.data
      calculateContainerWidth()
    }
  } catch { /* ignore */ }
}

function formatDuration(ns: number): string {
  if (!ns) return '0.000ms'
  const ms = ns / 1e6
  if (ms < 1000) return ms.toFixed(3) + 'ms'
  return (ms / 1000).toFixed(3) + 's'
}

function formatTime(t: string) {
  if (!t) return '-'
  const d = new Date(t)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

function latencyMs(span: SpanDTO): number {
  return span.duration_ns / 1e6
}

function latencyLevel(span: SpanDTO): string {
  const ms = latencyMs(span)
  if (ms < 200) return 'ok'
  if (ms < 600) return 'warn'
  if (ms < 1500) return 'alert'
  return 'critical'
}

function latencyColor(span: SpanDTO): string {
  switch (latencyLevel(span)) {
    case 'ok': return '#2ea04f'      // 绿色 <200ms
    case 'warn': return '#17a2b8'    // 青色 200~600ms
    case 'alert': return '#d29922'    // 暗黄色 600~1500ms
    case 'critical': return '#f85149' // 红色 ≥1500ms
    default: return 'var(--accent-primary)'
  }
}

const FIXED_WIDTH = 100
const containerWidth = ref(800)
const availableWidth = ref(700)
const maxTotalTime = ref(0)
const scaleRatio = ref(1)
const wrapperRef = ref<HTMLElement | null>(null)

function calculateContainerWidth() {
  if (!trace.value || !trace.value.spans.length) {
    containerWidth.value = 800
    return
  }
  
  let maxTime = 0
  for (const span of trace.value.spans) {
    const waitMs = spanWaitMs(span)
    const durationMs = Math.max(span.duration_ns / 1e6, 1)
    const totalTime = waitMs + durationMs
    
    if (totalTime > maxTime) {
      maxTime = totalTime
    }
  }
  
  maxTotalTime.value = maxTime
  
  if (wrapperRef.value) {
    const rect = wrapperRef.value.getBoundingClientRect()
    containerWidth.value = rect.width || 800
    availableWidth.value = Math.max(containerWidth.value - FIXED_WIDTH, 400)
    scaleRatio.value = availableWidth.value / Math.max(maxTime, 1)
  } else {
    containerWidth.value = 800
    availableWidth.value = 700
    scaleRatio.value = 700 / Math.max(maxTime, 1)
  }
}

function updateContainerSize() {
  if (wrapperRef.value && trace.value) {
    calculateContainerWidth()
  }
}

let resizeObserver: ResizeObserver | null = null

onMounted(async () => {
  await fetchTrace()
  
  nextTick(() => {
    calculateContainerWidth()
    
    if (wrapperRef.value) {
      resizeObserver = new ResizeObserver(() => {
        updateContainerSize()
      })
      resizeObserver.observe(wrapperRef.value)
    }
  })
})

onUnmounted(() => {
  if (resizeObserver) {
    resizeObserver.disconnect()
  }
})

function spanWaitMs(span: SpanDTO): number {
  if (!trace.value) return 0
  const start = new Date(span.started_at).getTime() - new Date(trace.value.started_at).getTime()
  return Math.max(0, start)
}

function getSpanBarStyle(span: SpanDTO): any {
  if (!trace.value) return {}
  
  const color = latencyColor(span)
  const waitMs = spanWaitMs(span)
  const durationMs = Math.max(span.duration_ns / 1e6, 1)
  
  const safeMargin = 30
  const minReadableWidth = 85
  const leftGapWhenZeroWait = 10
  
  let scaledLeft = FIXED_WIDTH + (waitMs * scaleRatio.value)
  if (waitMs === 0) {
    scaledLeft += leftGapWhenZeroWait
  }
  
  let scaledWidth = Math.max(durationMs * scaleRatio.value, minReadableWidth)
  
  if (scaledLeft + scaledWidth > containerWidth.value - safeMargin) {
    const maxLeft = containerWidth.value - safeMargin - scaledWidth
    if (maxLeft > FIXED_WIDTH) {
      scaledLeft = maxLeft
    } else {
      scaledWidth = containerWidth.value - safeMargin - scaledLeft
      if (scaledWidth < minReadableWidth) {
        scaledWidth = minReadableWidth
        scaledLeft = Math.max(FIXED_WIDTH + leftGapWhenZeroWait, containerWidth.value - safeMargin - scaledWidth)
      }
    }
  }
  
  return {
    left: scaledLeft.toFixed(2) + 'px',
    width: scaledWidth.toFixed(2) + 'px',
    background: color,
  }
}

function formatDurationMs(ns: number): string {
  if (!ns) return '0.000ms'
  const ms = ns / 1e6
  return ms.toFixed(3) + 'ms'
}
</script>

<style scoped>
.trace-detail { display: flex; flex-direction: column; gap: 16px; }
.page-header { display: flex; align-items: center; gap: 12px; }
.page-header h2 { font-size: 18px; font-weight: 600; }
.btn-back { padding: 6px 12px; border: 1px solid var(--border-primary); border-radius: var(--radius-sm); background: transparent; color: var(--text-secondary); font-size: 13px; cursor: pointer; }

.detail-card { background: var(--bg-card); border: 1px solid var(--border-secondary); border-radius: var(--radius-md); padding: 20px; display: grid; grid-template-columns: auto 1fr; gap: 8px 24px; align-items: center; }
.detail-row { display: contents; }
.label { font-size: 13px; color: var(--text-secondary); }
.value { font-size: 14px; color: var(--text-primary); }
.mono { font-family: monospace; font-size: 12px; cursor: default; }

.status-badge { font-size: 10px; padding: 2px 8px; border-radius: 10px; font-weight: 600; letter-spacing: 0.3px; display: inline-flex; align-items: center; min-width: auto; max-width: fit-content; }
.st-ok { background: rgba(63,185,80,0.12); color: #238636; border: 1px solid rgba(63,185,80,0.2); }
.st-error { background: rgba(248,81,73,0.12); color: #da3633; border: 1px solid rgba(248,81,73,0.2); }
.st-skip { background: rgba(210,153,34,0.12); color: #9a6700; border: 1px solid rgba(210,153,34,0.2); }

.empty { text-align: center; color: var(--text-tertiary); padding: 48px 0; }

.spans-card { background: var(--bg-card); border: 1px solid var(--border-secondary); border-radius: var(--radius-md); padding: 20px; }
.spans-card h3 { font-size: 14px; font-weight: 600; margin-bottom: 4px; display: flex; align-items: center; gap: 8px; }
.count { font-size: 12px; color: var(--text-tertiary); font-weight: 400; }
.legend { display: flex; gap: 16px; margin-bottom: 16px; padding-bottom: 12px; border-bottom: 1px solid var(--border-secondary); }
.legend-item { font-size: 11px; color: var(--text-secondary); display: flex; align-items: center; gap: 4px; }
.dot { display: inline-block; width: 10px; height: 10px; border-radius: 50%; }
.dot.lat-ok { background: #2ea04f; }
.dot.lat-warn { background: #17a2b8; }
.dot.lat-alert { background: #d29922; }
.dot.lat-critical { background: #f85149; }
.lat-ok { color: #2ea04f; }
.lat-warn { color: #17a2b8; }
.lat-alert { color: #d29922; font-weight: 700; }
.lat-critical { color: #f85149; font-weight: 700; }

.span-list { display: flex; flex-direction: column; gap: 10px; }
.span-item { padding: 12px 14px; background: var(--bg-tertiary); border-radius: var(--radius-sm); border-left: 3px solid transparent; }
.span-item:hover { border-left-color: var(--accent-primary); }
.span-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
.span-name-area { display: flex; align-items: center; gap: 8px; }
.span-index { font-size: 11px; color: var(--text-tertiary); font-weight: 600; min-width: 24px; }
.span-name { font-size: 13px; font-weight: 600; color: var(--text-primary); }
.span-node-id { font-size: 11px; color: var(--text-tertiary); }

.span-bar-wrapper { height: 24px; background: transparent; border-radius: 4px; position: relative; overflow: hidden; }
.span-wait-bar { position: absolute; left: 0; top: 0; width: 100px; min-width: 100px; height: 100%; background: #8b949e; border-radius: 4px; display: flex; align-items: center; justify-content: center; padding: 0 6px; color: #fff; font-size: 9px; font-weight: 600; font-family: 'Monaco', 'Menlo', monospace; white-space: nowrap; z-index: 2; box-sizing: border-box; }
.span-bar { position: absolute; top: 0; height: 100%; border-radius: 4px; display: flex; align-items: center; justify-content: center; padding: 0 8px; color: #fff; font-size: 9px; font-weight: 600; font-family: 'Monaco', 'Menlo', monospace; white-space: nowrap; text-shadow: 0 1px 2px rgba(0,0,0,0.5); box-sizing: border-box; }

.span-meta { display: flex; gap: 16px; font-size: 12px; color: var(--text-secondary); margin-top: 6px; align-items: center; flex-wrap: wrap; }
.latency-text { font-weight: 600; }
.chain-id { color: var(--text-tertiary); }
.error-text { color: var(--accent-danger); max-width: 300px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
