<template>
  <div class="scene-detail">
    <div class="page-header">
      <button class="btn-back" @click="$router.push('/scenes')">← 返回</button>
      <h2>场景详情</h2>
    </div>
    <div v-if="scene" class="detail-card">
      <div class="detail-row"><span class="label">ID</span><span class="value mono">{{ scene.id }}</span></div>
      <div class="detail-row"><span class="label">名称</span><span class="value">{{ scene.name }}</span></div>
      <div class="detail-row"><span class="label">描述</span><span class="value">{{ scene.description || '-' }}</span></div>
      <div class="detail-row"><span class="label">状态</span><span :class="['status-badge', scene.status]">{{ scene.status }}</span></div>
      <div class="detail-row"><span class="label">创建时间</span><span class="value">{{ formatTime(scene.created_at) }}</span></div>
    </div>
    <div v-else class="empty">加载中...</div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { getScene } from '@/api/scene'
import type { SceneDTO } from '@/types'

const route = useRoute()
const scene = ref<SceneDTO | null>(null)

async function fetchScene() {
  const id = route.params.id as string
  if (!id) return
  try {
    const resp = await getScene(id)
    if (resp.code === 0) scene.value = resp.data
  } catch { /* ignore */ }
}

function formatTime(t: string) {
  if (!t) return '-'
  return new Date(t).toLocaleString()
}

onMounted(fetchScene)
</script>

<style scoped>
.scene-detail { display: flex; flex-direction: column; gap: 16px; }
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
.status-badge.draft { background: rgba(139,148,158,0.15); color: var(--text-secondary); }
.status-badge.ready { background: rgba(63,185,80,0.15); color: var(--accent-success); }
.status-badge.running { background: rgba(88,166,255,0.15); color: var(--accent-primary); }
.empty { text-align: center; color: var(--text-tertiary); padding: 48px 0; }
</style>
