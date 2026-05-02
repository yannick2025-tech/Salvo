<template>
  <div class="reports-page">
    <div class="page-header">
      <h2>测试报告</h2>
    </div>
    <div class="table-wrapper">
      <table class="data-table">
        <thead>
          <tr>
            <th>ID</th>
            <th>场景</th>
            <th>状态</th>
            <th>总请求</th>
            <th>成功率</th>
            <th>P50</th>
            <th>P95</th>
            <th>P99</th>
            <th>创建时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="reports.length === 0"><td colspan="10" class="empty">暂无报告</td></tr>
          <tr v-for="r in reports" :key="r.id">
            <td class="mono">{{ r.id }}</td>
            <td>{{ r.scene_id }}</td>
            <td><span :class="['status-badge', r.status]">{{ r.status }}</span></td>
            <td>{{ extractMetric(r, 'total_reqs') }}</td>
            <td>{{ extractMetric(r, 'success_rate') }}</td>
            <td>{{ extractMetric(r, 'p50') }}</td>
            <td>{{ extractMetric(r, 'p95') }}</td>
            <td>{{ extractMetric(r, 'p99') }}</td>
            <td>{{ formatTime(r.created_at) }}</td>
            <td><router-link :to="`/reports/${r.id}`" class="link">查看</router-link></td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { listReports } from '@/api/report'
import type { ReportDTO } from '@/types'

const reports = ref<ReportDTO[]>([])

async function fetchReports() {
  try {
    const resp = await listReports({ limit: 50 })
    if (resp.code === 0) reports.value = resp.data.items || []
  } catch { /* ignore */ }
}

function extractMetric(r: ReportDTO, key: string): string {
  try {
    const detail = typeof r.detail === 'string' ? JSON.parse(r.detail) : r.detail
    return detail?.[key] ?? '-'
  } catch { return '-' }
}

function formatTime(t: string) {
  if (!t) return '-'
  return new Date(t).toLocaleString()
}

onMounted(fetchReports)
</script>

<style scoped>
.reports-page { display: flex; flex-direction: column; gap: 16px; }
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
.status-badge.completed { background: rgba(63,185,80,0.15); color: var(--accent-success); }
.status-badge.failed { background: rgba(248,81,73,0.15); color: var(--accent-danger); }
</style>
