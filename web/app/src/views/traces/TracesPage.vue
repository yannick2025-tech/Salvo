<template>
  <div class="traces-page">
    <div class="page-header">
      <h2>链路追踪</h2>
    </div>
    <div class="table-wrapper">
      <table class="data-table">
        <thead>
          <tr>
            <th>ID</th>
            <th>场景</th>
            <th>运行</th>
            <th>状态</th>
            <th>Span数</th>
            <th>耗时</th>
            <th>开始时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="traces.length === 0"><td colspan="8" class="empty">暂无追踪数据</td></tr>
          <tr v-for="t in traces" :key="t.id">
            <td class="mono">{{ t.id }}</td>
            <td class="mono">{{ t.scene_id }}</td>
            <td class="mono">{{ t.run_id }}</td>
            <td><span :class="['status-badge', t.status]">{{ t.status }}</span></td>
            <td>{{ t.spans?.length || 0 }}</td>
            <td>{{ formatDuration(t.duration_ns) }}</td>
            <td>{{ formatTime(t.started_at) }}</td>
            <td><router-link :to="`/traces/${t.id}`" class="link">查看</router-link></td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { listTraces } from '@/api/trace'
import type { TraceDTO } from '@/types'

const traces = ref<TraceDTO[]>([])

async function fetchTraces() {
  try {
    const resp = await listTraces({ limit: 50 })
    if (resp.code === 0) traces.value = resp.data.items || []
  } catch { /* ignore */ }
}

function formatDuration(ns: number): string {
  if (!ns) return '0ms'
  const ms = ns / 1e6
  if (ms < 1000) return ms.toFixed(1) + 'ms'
  return (ms / 1000).toFixed(2) + 's'
}

function formatTime(t: string) {
  if (!t) return '-'
  return new Date(t).toLocaleString()
}

onMounted(fetchTraces)
</script>

<style scoped>
.traces-page { display: flex; flex-direction: column; gap: 16px; }
.page-header h2 { font-size: 18px; font-weight: 600; }
.table-wrapper { background: var(--bg-card); border: 1px solid var(--border-secondary); border-radius: var(--radius-md); overflow: auto; }
.data-table { width: 100%; border-collapse: collapse; }
.data-table th, .data-table td { padding: 10px 14px; text-align: left; font-size: 13px; border-bottom: 1px solid var(--border-secondary); }
.data-table th { color: var(--text-secondary); font-weight: 500; background: var(--bg-tertiary); }
.empty { text-align: center; color: var(--text-tertiary); padding: 32px 0; }
.mono { font-family: monospace; font-size: 12px; }
.link { color: var(--accent-primary); text-decoration: none; }
.link:hover { text-decoration: underline; }
.status-badge { font-size: 11px; padding: 2px 8px; border-radius: 10px; }
.status-badge.success { background: rgba(63,185,80,0.15); color: var(--accent-success); }
.status-badge.error { background: rgba(248,81,73,0.15); color: var(--accent-danger); }
</style>
