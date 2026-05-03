<template>
  <div class="report-detail">
    <div class="page-header">
      <button class="btn-back" @click="$router.push('/reports')">← 返回</button>
      <h2>报告详情</h2>
    </div>
    <div v-if="report" class="detail-card">
      <div class="detail-row"><span class="label">ID</span><span class="value mono">{{ report.id }}</span></div>
      <div class="detail-row"><span class="label">场景</span><span class="value mono">{{ report.scene_id }}</span></div>
      <div class="detail-row"><span class="label">状态</span><span :class="['status-badge', report.status]">{{ report.status }}</span></div>
      <div class="detail-row"><span class="label">摘要</span><span class="value">{{ report.summary || '-' }}</span></div>
      <div class="detail-row"><span class="label">创建时间</span><span class="value">{{ formatTime(report.created_at) }}</span></div>
    </div>
    <div v-else class="empty">加载中...</div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { getReport } from '@/api/report'
import type { ReportDTO } from '@/types'

const route = useRoute()
const report = ref<ReportDTO | null>(null)

async function fetchReport() {
  const id = route.params.id as string
  if (!id) return
  try {
    const resp = await getReport(id)
    if (resp.code === 0) report.value = resp.data
  } catch { /* ignore */ }
}

function formatTime(t: string) {
  if (!t) return '-'
  return new Date(t).toLocaleString()
}

onMounted(fetchReport)
</script>

<style scoped>
.report-detail { display: flex; flex-direction: column; gap: 16px; }
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
.status-badge.completed { background: rgba(63,185,80,0.15); color: var(--accent-success); }
.status-badge.failed { background: rgba(248,81,73,0.15); color: var(--accent-danger); }
.empty { text-align: center; color: var(--text-tertiary); padding: 48px 0; }
</style>
