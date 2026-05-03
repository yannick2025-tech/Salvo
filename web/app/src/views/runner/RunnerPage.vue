<template>
  <div class="runner-page">
    <div class="page-header">
      <h2>运行控制</h2>
    </div>

    <div class="runner-grid">
      <div class="card start-card">
        <h3>启动场景</h3>
        <div class="form-group">
          <label>场景</label>
          <select v-model="form.scene_id">
            <option :value="0">选择场景</option>
            <option v-for="s in scenes" :key="s.id" :value="s.id">{{ s.name }}</option>
          </select>
        </div>
        <div class="form-row">
          <div class="form-group">
            <label>并发数</label>
            <input v-model.number="form.workers" type="number" min="1" max="10000" />
          </div>
          <div class="form-group">
            <label>持续时间(s)</label>
            <input v-model.number="form.duration" type="number" min="1" />
          </div>
        </div>
        <div class="form-group">
          <label>运行模式</label>
          <select v-model="form.run_mode">
            <option value="duration">持续时间</option>
            <option value="count">请求数</option>
          </select>
        </div>
        <div class="form-group" v-if="form.run_mode === 'count'">
          <label>总请求数</label>
          <input v-model.number="form.count" type="number" min="1" />
        </div>
        <button class="btn-primary" @click="handleStart" :disabled="!form.scene_id || starting || selectedSceneHasNoDAG">
          {{ starting ? '启动中...' : '启动' }}
        </button>
        <div v-if="form.scene_id && selectedSceneHasNoDAG" class="no-dag-warning">
          ⚠ 该场景没有配置 DAG 请求流，请先编辑场景添加节点
        </div>
      </div>

      <div class="card status-card">
        <h3>运行状态</h3>
        <div v-if="activeRuns.length === 0" class="empty">暂无运行中的场景</div>
        <div v-for="run in activeRuns" :key="run.id" class="run-item">
          <div class="run-header">
            <span class="run-name">Scene #{{ run.scene_id }}</span>
            <span class="status running">RUNNING</span>
          </div>
          <div class="run-metrics">
            <div class="metric"><span class="metric-label">Workers</span><span class="metric-val">{{ run.worker_count }}</span></div>
            <div class="metric"><span class="metric-label">总请求</span><span class="metric-val">{{ run.total_reqs }}</span></div>
            <div class="metric"><span class="metric-label">成功</span><span class="metric-val success">{{ run.success_reqs }}</span></div>
            <div class="metric"><span class="metric-label">失败</span><span class="metric-val danger">{{ run.failed_reqs }}</span></div>
            <div class="metric"><span class="metric-label">P99</span><span class="metric-val">{{ formatMs(run.p99_latency) }}</span></div>
          </div>
          <button class="btn-stop" @click="handleStop(run.scene_id)">停止</button>
        </div>
      </div>
    </div>

    <div class="card">
      <h3>运行历史</h3>
      <table class="data-table">
        <thead>
          <tr>
            <th>ID</th>
            <th>场景</th>
            <th>状态</th>
            <th>并发</th>
            <th>总请求</th>
            <th>成功率</th>
            <th>P99</th>
            <th>开始时间</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="runs.length === 0"><td colspan="8" class="empty">暂无运行记录</td></tr>
          <tr v-for="r in runs" :key="r.id">
            <td class="mono">{{ r.id }}</td>
            <td>{{ r.scene_id }}</td>
            <td><span :class="['status-badge', r.status]">{{ r.status }}</span></td>
            <td>{{ r.worker_count }}</td>
            <td>{{ r.total_reqs }}</td>
            <td>{{ ((r.success_reqs / Math.max(r.total_reqs, 1)) * 100).toFixed(1) }}%</td>
            <td>{{ formatMs(r.p99_latency) }}</td>
            <td>{{ formatTime(r.started_at) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted, watch } from 'vue'
import { listScenes, listRuns, startScene, stopScene } from '@/api/scene'
import { listNodes } from '@/api/node'
import type { SceneDTO, RunRecordDTO } from '@/types'

const scenes = ref<SceneDTO[]>([])
const runs = ref<RunRecordDTO[]>([])
const activeRuns = ref<RunRecordDTO[]>([])
const starting = ref(false)
const selectedSceneHasNoDAG = ref(false)

const form = reactive({
  scene_id: '',
  workers: 10,
  duration: 60,
  run_mode: 'duration',
  count: 1000,
})

let pollTimer: ReturnType<typeof setInterval> | null = null

watch(() => form.scene_id, async (newId) => {
  if (!newId) {
    selectedSceneHasNoDAG.value = false
    return
  }
  try {
    const resp = await listNodes(newId)
    if (resp.code === 0) {
      selectedSceneHasNoDAG.value = !resp.data.items || resp.data.items.length === 0
    }
  } catch { selectedSceneHasNoDAG.value = false }
})

async function fetchScenes() {
  try {
    const resp = await listScenes({ limit: 100 })
    if (resp.code === 0) scenes.value = resp.data.items || []
  } catch { /* ignore */ }
}

async function fetchRuns() {
  try {
    const resp = await listRuns({ limit: 20 })
    if (resp.code === 0) {
      runs.value = resp.data.items || []
      activeRuns.value = runs.value.filter((r) => r.status === 'running')
    }
  } catch { /* ignore */ }
}

async function handleStart() {
  if (!form.scene_id) return
  starting.value = true
  try {
    await startScene({
      scene_id: form.scene_id,
      workers: form.workers,
      run_mode: form.run_mode,
      duration: form.duration * 1e9,
      count: form.count,
    })
    fetchRuns()
  } catch { /* ignore */ }
  starting.value = false
}

async function handleStop(sceneId: string) {
  try {
    await stopScene(sceneId)
    fetchRuns()
  } catch { /* ignore */ }
}

function formatMs(ns: number): string {
  if (!ns) return '0ms'
  const ms = ns / 1e6
  if (ms < 1) return ms.toFixed(3) + 'ms'
  if (ms < 1000) return ms.toFixed(1) + 'ms'
  return (ms / 1000).toFixed(2) + 's'
}

function formatTime(t?: string) {
  if (!t) return '-'
  return new Date(t).toLocaleString()
}

onMounted(() => {
  fetchScenes()
  fetchRuns()
  pollTimer = setInterval(fetchRuns, 5000)
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>

<style scoped>
.runner-page { display: flex; flex-direction: column; gap: 20px; }
.page-header h2 { font-size: 18px; font-weight: 600; }
.runner-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }

.card { background: var(--bg-card); border: 1px solid var(--border-secondary); border-radius: var(--radius-md); padding: 20px; }
.card h3 { font-size: 14px; font-weight: 600; margin-bottom: 16px; }

.form-group { margin-bottom: 12px; }
.form-group label { display: block; font-size: 12px; color: var(--text-secondary); margin-bottom: 4px; }
.form-group input, .form-group select {
  width: 100%; height: 36px; padding: 0 10px; border: 1px solid var(--border-primary);
  border-radius: var(--radius-sm); background: var(--bg-input); color: var(--text-primary); font-size: 13px; outline: none;
}
.form-group input:focus, .form-group select:focus { border-color: var(--accent-primary); }
.form-row { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }

.btn-primary { padding: 8px 20px; border: none; border-radius: var(--radius-md); background: var(--accent-primary); color: #fff; font-size: 13px; cursor: pointer; margin-top: 8px; }
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
.no-dag-warning { margin-top: 8px; font-size: 12px; color: #f0ad4e; background: rgba(240,173,78,0.1); padding: 6px 10px; border-radius: var(--radius-sm); border: 1px solid rgba(240,173,78,0.3); }
.btn-stop { padding: 4px 12px; border: 1px solid var(--accent-danger); border-radius: var(--radius-sm); background: transparent; color: var(--accent-danger); font-size: 12px; cursor: pointer; margin-top: 8px; }

.empty { text-align: center; color: var(--text-tertiary); font-size: 13px; padding: 24px 0; }
.run-item { padding: 12px; background: var(--bg-tertiary); border-radius: var(--radius-sm); margin-bottom: 8px; }
.run-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
.run-name { font-size: 13px; font-weight: 500; }
.status { font-size: 11px; padding: 2px 8px; border-radius: 10px; }
.status.running { background: rgba(88,166,255,0.15); color: var(--accent-primary); }
.run-metrics { display: flex; gap: 16px; flex-wrap: wrap; }
.metric { display: flex; flex-direction: column; gap: 2px; }
.metric-label { font-size: 11px; color: var(--text-tertiary); }
.metric-val { font-size: 13px; font-weight: 600; font-variant-numeric: tabular-nums; }
.metric-val.success { color: var(--accent-success); }
.metric-val.danger { color: var(--accent-danger); }

.data-table { width: 100%; border-collapse: collapse; }
.data-table th, .data-table td { padding: 10px 14px; text-align: left; font-size: 13px; border-bottom: 1px solid var(--border-secondary); }
.data-table th { color: var(--text-secondary); font-weight: 500; background: var(--bg-tertiary); }
.mono { font-family: monospace; font-size: 12px; }
.status-badge { font-size: 11px; padding: 2px 8px; border-radius: 10px; }
.status-badge.running { background: rgba(88,166,255,0.15); color: var(--accent-primary); }
.status-badge.completed { background: rgba(63,185,80,0.15); color: var(--accent-success); }
.status-badge.failed { background: rgba(248,81,73,0.15); color: var(--accent-danger); }
</style>
