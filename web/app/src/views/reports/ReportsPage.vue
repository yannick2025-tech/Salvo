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
            <th>开始时间</th>
            <th>结束时间</th>
            <th>持续时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="reports.length === 0"><td colspan="12" class="empty">暂无报告</td></tr>
          <tr v-for="r in reports" :key="r.id">
            <td class="mono">{{ r.id }}</td>
            <td>{{ r.scene_id }}</td>
            <td><span :class="['status-badge', r.status]" class="tooltip-wrapper" :data-tooltip="getReportStatusTooltip(r.status)">{{ r.status }}</span></td>
            <td>{{ extractMetric(r, 'total_reqs') }}</td>
            <td>{{ extractMetric(r, 'success_rate') }}</td>
            <td>{{ extractMetric(r, 'p50') }}</td>
            <td>{{ extractMetric(r, 'p95') }}</td>
            <td>{{ extractMetric(r, 'p99') }}</td>
            <td>{{ r.started_at ? formatTime(r.started_at) : '-' }}</td>
            <td>{{ r.finished_at ? formatTime(r.finished_at) : '-' }}</td>
            <td class="duration-cell">{{ calculateDuration(r.started_at, r.finished_at) }}</td>
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
    const source = r.summary || r.detail
    const detail = typeof source === 'string' ? JSON.parse(source) : source
    return detail?.[key] ?? '-'
  } catch { return '-' }
}

function formatTime(t: string) {
  if (!t) return '-'
  const d = new Date(t)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

function calculateDuration(startedAt?: string, finishedAt?: string): string {
  if (!startedAt) return '-'
  const start = new Date(startedAt).getTime()
  const end = finishedAt ? new Date(finishedAt).getTime() : Date.now()
  const durationMs = end - start
  if (durationMs <= 0) return '-'

  const totalSeconds = Math.floor(durationMs / 1000)
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60
  const pad = (n: number) => String(n).padStart(2, '0')

  if (hours > 0) {
    return `${pad(hours)}小时${pad(minutes)}分${pad(seconds)}秒`
  } else if (minutes > 0) {
    return `${pad(minutes)}分${pad(seconds)}秒`
  } else {
    return `${pad(seconds)}秒`
  }
}

onMounted(fetchReports)

function getReportStatusTooltip(status: string): string {
  const map: Record<string, string> = {
    success: '全部请求成功（100% 成功率）',
    partial: '部分失败：成功率 ≥95% 但 <100%（存在少量失败请求）',
    failed: '测试失败：成功率 <95% 或运行时发生错误',
    completed: '测试运行已成功完成',
    running: '测试正在运行中',
    pending: '测试等待开始',
    cancelled: '测试在完成前被取消',
    canceled: '测试在完成前被取消',
  }
  return map[status] || status
}
</script>

<style scoped>
.reports-page { display: flex; flex-direction: column; gap: 16px; }
.page-header h2 { font-size: 18px; font-weight: 600; }
.table-wrapper {
  background: var(--bg-card);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-md);
  overflow: visible;
}

.data-table {
  width: max-content;
  min-width: 100%;
  border-collapse: collapse;
}

.data-table th, .data-table td {
  padding: 10px 14px;
  text-align: left;
  font-size: 13px;
  border-bottom: 1px solid var(--border-secondary);
  white-space: nowrap;
  position: relative;
}
.data-table th { color: var(--text-secondary); font-weight: 500; background: var(--bg-tertiary); white-space: nowrap; }
.empty { text-align: center; color: var(--text-tertiary); padding: 32px 0; }
.mono { font-family: monospace; font-size: 12px; }
.link { color: var(--accent-primary); text-decoration: none; }
.link:hover { text-decoration: underline; }
.duration-cell { font-family: 'Monaco', 'Menlo', monospace; font-size: 11px; color: var(--text-secondary); white-space: nowrap; }

.status-badge { font-size: 11px; padding: 2px 8px; border-radius: 10px; font-weight: 500; }
.status-badge.success { background: rgba(63,185,80,0.15); color: #3fb950; }
.status-badge.failed { background: rgba(248,81,73,0.15); color: #f85149; }
.status-badge.partial { background: rgba(210,153,34,0.15); color: #d29922; }

/* Tooltip */
.tooltip-wrapper {
  position: relative;
  cursor: help;
}

.tooltip-wrapper::before {
  content: attr(data-tooltip);
  position: absolute;
  top: calc(100% + 10px);
  left: 50%;
  transform: translateX(-50%) translateY(-6px);
  background: rgba(255, 255, 255, 0.96);
  color: #1e293b;
  font-size: 11.5px;
  padding: 8px 14px;
  border-radius: 8px;
  border: 1px solid rgba(0, 0, 0, 0.08);
  box-shadow:
    0 4px 6px -1px rgba(0, 0, 0, 0.1),
    0 10px 24px -4px rgba(0, 0, 0, 0.12);
  opacity: 0;
  visibility: hidden;
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  pointer-events: none;
  z-index: 1000;
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  font-weight: 500;
  letter-spacing: 0.2px;
  white-space: nowrap;
  text-align: center;
  line-height: 1.4;
}

.tooltip-wrapper::after {
  content: '';
  position: absolute;
  top: calc(100% + 3px);
  left: 50%;
  transform: translateX(-50%);
  border: 6px solid transparent;
  border-bottom-color: rgba(255, 255, 255, 0.96);
  filter: drop-shadow(0 2px 2px rgba(0, 0, 0, 0.04));
  opacity: 0;
  visibility: hidden;
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  pointer-events: none;
  z-index: 1001;
}

.tooltip-wrapper:hover::before {
  opacity: 1;
  visibility: visible;
  transform: translateX(-50%) translateY(0);
}

.tooltip-wrapper:hover::after {
  opacity: 1;
  visibility: visible;
}

/* ===== Tooltip Dark Mode ===== */
/* Keep light theme unchanged; override only in dark mode to match ECharts dark tooltip. */
[data-theme='dark'] .tooltip-wrapper::before {
  background: rgba(22, 27, 34, 0.96);
  color: #e6edf3;
  border-color: rgba(48, 54, 61, 0.8);
  box-shadow:
    0 4px 6px -1px rgba(0, 0, 0, 0.5),
    0 10px 24px -4px rgba(0, 0, 0, 0.6);
  -webkit-backdrop-filter: blur(8px);
  backdrop-filter: blur(8px);
}

[data-theme='dark'] .tooltip-wrapper::after {
  border-bottom-color: rgba(22, 27, 34, 0.96);
  filter: drop-shadow(0 2px 2px rgba(0, 0, 0, 0.3));
}
</style>
