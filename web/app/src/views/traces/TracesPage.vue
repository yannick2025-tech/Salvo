<template>
  <div class="traces-page">
    <div class="page-header">
      <h2>链路追踪</h2>
    </div>
    <div class="table-wrapper">
      <table class="data-table">
        <thead>
          <tr>
            <th>TraceID</th>
            <th>RunID</th>
            <th>场景</th>
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
            <td class="mono" :title="'数据库中的Trace主键ID'">{{ t.id }}</td>
            <td class="mono" :title="'运行记录ID，可在日志中搜索'">{{ t.run_id }}</td>
            <td>{{ t.scene_name || t.scene_id }}</td>
            <td><span :class="['status-badge', t.status]">{{ t.status }}</span></td>
            <td>{{ t.spans?.length || 0 }}</td>
            <td>{{ formatDuration(t.duration_ns) }}</td>
            <td>{{ formatTime(t.started_at) }}</td>
            <td><router-link :to="`/traces/${t.id}`" class="link">查看</router-link></td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="pagination-bar">
      <div class="page-size-selector">
        <span class="label">每页：</span>
        <button v-for="s in pageSizes" :key="s"
          :class="['size-btn', { active: pageSize === s }]"
          @click="changePageSize(s)">{{ s }}</button>
      </div>
      <div class="page-nav">
        <button class="nav-btn" :disabled="offset <= 0" @click="prevPage">上一页</button>
        <span class="page-info">第 {{ currentPage }} / {{ totalPages }} 页</span>
        <button class="nav-btn" :disabled="traces.length < pageSize" @click="nextPage">下一页</button>
        <span class="total-info">共 {{ total }} 条</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { listTraces } from '@/api/trace'
import type { TraceDTO } from '@/types'

const traces = ref<TraceDTO[]>([])
const total = ref(0)
const offset = ref(0)
const pageSize = ref(30)
const pageSizes = [10, 20, 30, 50, 100]

const currentPage = computed(() => Math.floor(offset.value / pageSize.value) + 1)
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))

async function fetchTraces() {
  try {
    const resp = await listTraces({ limit: pageSize.value, offset: offset.value })
    if (resp.code === 0) {
      traces.value = resp.data.items || []
      total.value = resp.data.pagination?.total ?? traces.value.length
    }
  } catch { /* ignore */ }
}

function changePageSize(s: number) {
  pageSize.value = s
  offset.value = 0
  fetchTraces()
}

function prevPage() {
  if (offset.value > 0) {
    offset.value = Math.max(0, offset.value - pageSize.value)
    fetchTraces()
  }
}

function nextPage() {
  if (traces.value.length >= pageSize.value) {
    offset.value += pageSize.value
    fetchTraces()
  }
}

function formatDuration(ns: number): string {
  if (!ns) return '0ms'
  const ms = ns / 1e6
  if (ms < 1000) return ms.toFixed(1) + 'ms'
  return (ms / 1000).toFixed(2) + 's'
}

function formatTime(t: string) {
  if (!t) return '-'
  const d = new Date(t)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

onMounted(fetchTraces)
</script>

<style scoped>
.traces-page { display: flex; flex-direction: column; gap: 16px; }
.page-header { display: flex; align-items: center; justify-content: space-between; }
.page-header h2 { font-size: 18px; font-weight: 600; }
.total-count { font-size: 13px; color: var(--text-tertiary); }
.table-wrapper { background: var(--bg-card); border: 1px solid var(--border-secondary); border-radius: var(--radius-md); overflow: auto; }
.data-table { width: 100%; border-collapse: collapse; }
.data-table th, .data-table td { padding: 10px 14px; text-align: left; font-size: 13px; border-bottom: 1px solid var(--border-secondary); }
.data-table th { color: var(--text-secondary); font-weight: 500; background: var(--bg-tertiary); white-space: nowrap; }
.empty { text-align: center; color: var(--text-tertiary); padding: 32px 0; }
.mono { font-family: monospace; font-size: 12px; cursor: default; max-width: 160px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.link { color: var(--accent-primary); text-decoration: none; }
.link:hover { text-decoration: underline; }

.pagination-bar { display: flex; align-items: center; justify-content: space-between; padding: 12px 16px; background: var(--bg-card); border: 1px solid var(--border-secondary); border-radius: var(--radius-md); }
.page-size-selector { display: flex; align-items: center; gap: 4px; }
.page-size-selector .label { font-size: 13px; color: var(--text-secondary); margin-right: 4px; }
.size-btn { padding: 4px 10px; border: 1px solid var(--border-primary); border-radius: var(--radius-sm); background: transparent; color: var(--text-secondary); font-size: 12px; cursor: pointer; transition: all 0.15s; }
.size-btn:hover { border-color: var(--accent-primary); color: var(--accent-primary); }
.size-btn.active { background: var(--accent-primary); color: #fff; border-color: var(--accent-primary); }
.page-nav { display: flex; align-items: center; gap: 12px; }
.nav-btn { padding: 5px 14px; border: 1px solid var(--border-primary); border-radius: var(--radius-sm); background: transparent; color: var(--text-secondary); font-size: 13px; cursor: pointer; transition: all 0.15s; }
.nav-btn:not(:disabled):hover { border-color: var(--accent-primary); color: var(--accent-primary); }
.nav-btn:disabled { opacity: 0.4; cursor: not-allowed; }
.page-info { font-size: 13px; color: var(--text-secondary); }
.total-info { font-size: 13px; color: var(--text-tertiary); margin-left: 8px; }

.status-badge { font-size: 11px; padding: 2px 8px; border-radius: 10px; }
.status-badge.ok { background: rgba(63,185,80,0.15); color: var(--accent-success); }
.status-badge.error { background: rgba(248,81,73,0.15); color: var(--accent-danger); }
</style>
