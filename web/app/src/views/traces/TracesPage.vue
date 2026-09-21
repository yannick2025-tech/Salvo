<template>
  <div class="traces-page">
    <div class="page-header">
      <h2>链路追踪</h2>
    </div>

    <div class="filters-bar">
      <input v-model.trim="filters.trace_id" class="filter-input mono" placeholder="TraceID" @keyup.enter="applyFilters" />
      <input v-model.trim="filters.scene_name" class="filter-input" placeholder="场景名称（模糊）" @keyup.enter="applyFilters" />
      <CustomSelect v-model="filters.status" :options="statusOptions" placeholder="全部状态" min-width="130px" />
      <div class="duration-range">
        <input v-model="filters.min_ms" class="filter-input num" type="number" step="0.1" min="0" placeholder="耗时≥(ms)" @keyup.enter="applyFilters" />
        <span class="range-sep">—</span>
        <input v-model="filters.max_ms" class="filter-input num" type="number" step="0.1" min="0" placeholder="耗时≤(ms)" @keyup.enter="applyFilters" />
      </div>
      <button class="filter-btn" @click="applyFilters">查询</button>
      <button class="filter-btn ghost" @click="resetFilters">重置</button>
      <span v-if="filterError" class="filter-error">{{ filterError }}</span>
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
import { ref, reactive, computed, onMounted } from 'vue'
import { listTraces } from '@/api/trace'
import type { TraceListQuery } from '@/api/trace'
import CustomSelect from '@/components/CustomSelect.vue'
import type { TraceDTO } from '@/types'

const traces = ref<TraceDTO[]>([])
const total = ref(0)
const offset = ref(0)
const pageSize = ref(30)
const pageSizes = [10, 20, 30, 50, 100]

const currentPage = computed(() => Math.floor(offset.value / pageSize.value) + 1)
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))

// --- Filters ---
const filters = reactive({
  trace_id: '',
  scene_name: '',
  status: '',
  // type="number" inputs make v-model produce numbers once filled.
  min_ms: '' as string | number,
  max_ms: '' as string | number,
})
const filterError = ref('')

const statusOptions = [
  { value: '', label: '全部状态' },
  { value: 'ok', label: 'ok' },
  { value: 'error', label: 'error' },
  { value: 'skip', label: 'skip' },
  { value: 'canceled', label: 'canceled' },
]

// Validates duration inputs; returns an error message or empty string.
// NOTE: with type="number" inputs, v-model produces numbers (not strings),
// so coerce before trimming.
function validateFilters(): string {
  const min = String(filters.min_ms ?? '').trim()
  const max = String(filters.max_ms ?? '').trim()
  if (min && (isNaN(Number(min)) || Number(min) < 0)) return '最小耗时必须为非负数字（ms）'
  if (max && (isNaN(Number(max)) || Number(max) < 0)) return '最大耗时必须为非负数字（ms）'
  if (min && max && Number(min) > Number(max)) return '最小耗时不能大于最大耗时'
  return ''
}

function buildQuery(): TraceListQuery {
  const q: TraceListQuery = { limit: pageSize.value, offset: offset.value }
  if (filters.trace_id) q.trace_id = filters.trace_id
  if (filters.scene_name) q.scene_name = filters.scene_name
  if (filters.status) q.status = filters.status
  if (filters.min_ms) q.min_duration_ms = Number(filters.min_ms)
  if (filters.max_ms) q.max_duration_ms = Number(filters.max_ms)
  return q
}

function applyFilters() {
  filterError.value = validateFilters()
  if (filterError.value) return
  offset.value = 0
  fetchTraces()
}

function resetFilters() {
  filters.trace_id = ''
  filters.scene_name = ''
  filters.status = ''
  filters.min_ms = ''
  filters.max_ms = ''
  filterError.value = ''
  offset.value = 0
  fetchTraces()
}

async function fetchTraces() {
  try {
    const resp = await listTraces(buildQuery())
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

.filters-bar { display: flex; align-items: center; flex-wrap: wrap; gap: 10px; padding: 12px 16px; background: var(--bg-card); border: 1px solid var(--border-secondary); border-radius: var(--radius-md); }
.filter-input { height: 32px; padding: 0 10px; border: 1px solid var(--border-primary); border-radius: var(--radius-sm); background: transparent; color: var(--text-primary); font-size: 13px; outline: none; transition: border-color 0.15s; }
.filter-input:focus { border-color: var(--accent-primary); }
.filter-input.mono { font-family: var(--font-mono); width: 160px; }
.filter-input:not(.mono):not(.num) { width: 180px; }
.filter-input.num { width: 110px; font-family: var(--font-mono); }
.duration-range { display: flex; align-items: center; gap: 6px; }
.range-sep { color: var(--text-tertiary); }
.filter-btn { padding: 6px 16px; border: 1px solid var(--accent-primary); border-radius: var(--radius-sm); background: var(--accent-primary); color: #fff; font-size: 13px; cursor: pointer; transition: all 0.15s; }
.filter-btn:hover { opacity: 0.9; }
.filter-btn.ghost { background: transparent; color: var(--text-secondary); border-color: var(--border-primary); }
.filter-btn.ghost:hover { border-color: var(--accent-primary); color: var(--accent-primary); }
.filter-error { font-size: 12px; color: var(--accent-danger, #f85149); }
.table-wrapper { background: var(--bg-card); border: 1px solid var(--border-secondary); border-radius: var(--radius-md); overflow: auto; }
.data-table { width: 100%; border-collapse: collapse; }
.data-table th, .data-table td { padding: 10px 14px; text-align: left; font-size: 13px; border-bottom: 1px solid var(--border-secondary); }
.data-table th { color: var(--text-secondary); font-weight: 500; background: var(--bg-tertiary); white-space: nowrap; }
.empty { text-align: center; color: var(--text-tertiary); padding: 32px 0; }
.mono { font-family: var(--font-mono); font-size: 12px; cursor: default; max-width: 160px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
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
.status-badge.canceled { background: rgba(255,193,7,0.15); color: var(--accent-warning, #f0ad4e); }
.status-badge.skip { background: rgba(108,117,125,0.15); color: var(--text-secondary, #6c757d); }
</style>
