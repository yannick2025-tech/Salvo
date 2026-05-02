<template>
  <div class="trace-detail">
    <div class="page-header">
      <button class="btn-back" @click="$router.push('/traces')">← 返回</button>
      <h2>追踪详情</h2>
    </div>
    <div v-if="trace" class="detail-card">
      <div class="detail-row"><span class="label">ID</span><span class="value mono">{{ trace.id }}</span></div>
      <div class="detail-row"><span class="label">场景</span><span class="value mono">{{ trace.scene_id }}</span></div>
      <div class="detail-row"><span class="label">状态</span><span :class="['status-badge', trace.status]">{{ trace.status }}</span></div>
      <div class="detail-row"><span class="label">耗时</span><span class="value">{{ formatDuration(trace.duration_ns) }}</span></div>
    </div>

    <div v-if="trace && trace.spans" class="spans-card">
      <h3>Spans</h3>
      <div class="span-list">
        <div v-for="span in trace.spans" :key="span.id" class="span-item">
          <div class="span-header">
            <span class="span-name">Node #{{ span.node_id }}</span>
            <span :class="['status-badge', span.status]">{{ span.status }}</span>
          </div>
          <div class="span-bar-container">
            <div class="span-bar" :style="spanStyle(span)"></div>
          </div>
          <div class="span-meta">
            <span>耗时: {{ formatDuration(span.duration_ns) }}</span>
            <span v-if="span.error" class="error-text">错误: {{ span.error }}</span>
          </div>
        </div>
      </div>
    </div>
    <div v-else-if="!trace" class="empty">加载中...</div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
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
    if (resp.code === 0) trace.value = resp.data
  } catch { /* ignore */ }
}

function formatDuration(ns: number): string {
  if (!ns) return '0ms'
  const ms = ns / 1e6
  if (ms < 1000) return ms.toFixed(1) + 'ms'
  return (ms / 1000).toFixed(2) + 's'
}

function spanStyle(span: SpanDTO) {
  if (!trace.value) return {}
  const total = trace.value.duration_ns || 1
  const start = new Date(span.started_at).getTime() - new Date(trace.value.started_at).getTime()
  const left = (start / (total / 1e6)) * 100
  const width = (span.duration_ns / total) * 100
  return {
    left: Math.max(0, Math.min(left, 100)) + '%',
    width: Math.max(0.5, Math.min(width, 100 - left)) + '%',
    background: span.status === 'error' ? 'var(--accent-danger)' : 'var(--accent-primary)',
  }
}

onMounted(fetchTrace)
</script>

<style scoped>
.trace-detail { display: flex; flex-direction: column; gap: 16px; }
.page-header { display: flex; align-items: center; gap: 12px; }
.page-header h2 { font-size: 18px; font-weight: 600; }
.btn-back { padding: 6px 12px; border: 1px solid var(--border-primary); border-radius: var(--radius-sm); background: transparent; color: var(--text-secondary); font-size: 13px; cursor: pointer; }
.detail-card { background: var(--bg-card); border: 1px solid var(--border-secondary); border-radius: var(--radius-md); padding: 20px; }
.detail-row { display: flex; align-items: center; padding: 10px 0; border-bottom: 1px solid var(--border-secondary); }
.detail-row:last-child { border-bottom: none; }
.label { width: 120px; font-size: 13px; color: var(--text-secondary); flex-shrink: 0; }
.value { font-size: 14px; color: var(--text-primary); }
.mono { font-family: monospace; }
.status-badge { font-size: 11px; padding: 2px 8px; border-radius: 10px; }
.status-badge.success { background: rgba(63,185,80,0.15); color: var(--accent-success); }
.status-badge.error { background: rgba(248,81,73,0.15); color: var(--accent-danger); }
.empty { text-align: center; color: var(--text-tertiary); padding: 48px 0; }

.spans-card { background: var(--bg-card); border: 1px solid var(--border-secondary); border-radius: var(--radius-md); padding: 20px; }
.spans-card h3 { font-size: 14px; font-weight: 600; margin-bottom: 16px; }
.span-list { display: flex; flex-direction: column; gap: 12px; }
.span-item { padding: 10px 12px; background: var(--bg-tertiary); border-radius: var(--radius-sm); }
.span-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
.span-name { font-size: 13px; font-weight: 500; }
.span-bar-container { height: 20px; background: var(--bg-hover); border-radius: 4px; position: relative; overflow: hidden; }
.span-bar { position: absolute; top: 0; height: 100%; border-radius: 4px; opacity: 0.7; min-width: 2px; }
.span-meta { display: flex; gap: 16px; font-size: 12px; color: var(--text-secondary); margin-top: 6px; }
.error-text { color: var(--accent-danger); }
</style>
