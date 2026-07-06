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
            <option value="">选择场景</option>
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
        <button class="btn-login-primary" @click="handleStart" :disabled="!canRunScene || !form.scene_id || starting || selectedSceneHasNoDAG" :title="canRunScene ? '' : '您当前的角色没有运行权限'">
          {{ starting ? '启动中...' : '启动' }}
        </button>
        <div v-if="form.scene_id && selectedSceneHasNoDAG" class="no-dag-warning">
          ⚠ 该场景没有配置 DAG 请求流，请先编辑场景添加节点
        </div>
      </div>

      <div class="card status-card">
        <h3>运行状态</h3>
        <div v-if="activeRuns.length === 0 && recentFinishedRuns.length === 0" class="empty">暂无运行中的场景</div>
        <div v-for="run in activeRuns" :key="run.id" class="run-item">
          <div class="run-header">
            <span class="run-name">Scene #{{ run.scene_id }}</span>
            <span class="status running">运行中</span>
          </div>
          <div class="run-footer">
            <div class="run-metrics">
              <div class="metric"><span class="metric-label">工作线程</span><span class="metric-val">{{ run.worker_count }}</span></div>
              <div class="metric"><span class="metric-label">总请求</span><span class="metric-val">{{ run.total_reqs }}</span></div>
              <div class="metric"><span class="metric-label">成功</span><span class="metric-val success">{{ run.success_reqs }}</span></div>
              <div class="metric"><span class="metric-label">失败</span><span class="metric-val danger">{{ run.failed_reqs }}</span></div>
              <div class="metric"><span class="metric-label">P99</span><span class="metric-val">{{ formatMs(run.p99_latency) }}</span></div>
            </div>
            <button class="btn-sm danger" 
                    :disabled="!canRunScene || stoppingSceneIds.has(run.scene_id) || run.status !== 'running'" 
                    :title="canRunScene ? '' : '您当前的角色没有运行权限'"
                    @click="showStopConfirm(run.scene_id, getSceneName(run.scene_id))">
              {{ stoppingSceneIds.has(run.scene_id) ? '停止中...' : '停止' }}
            </button>
          </div>
        </div>
        
        <div v-if="stopConfirm.visible" class="modal-overlay" @click.self="cancelStop">
          <div class="confirm-dialog">
            <div class="confirm-icon">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="12" cy="12" r="10"/>
                <line x1="12" y1="8" x2="12" y2="12"/>
                <line x1="12" y1="16" x2="12.01" y2="16"/>
              </svg>
            </div>
            <h3 class="confirm-title">确认停止</h3>
            <p class="confirm-msg">{{ stopConfirm.message }}</p>
            <div class="confirm-actions">
              <button class="btn-cancel" @click="cancelStop">取消</button>
              <button class="btn-danger-confirm" @click="confirmStop" :disabled="stopConfirm.loading">
                {{ stopConfirm.loading ? '停止中...' : '确认停止' }}
              </button>
            </div>
          </div>
        </div>
        <div v-for="run in recentFinishedRuns" :key="'fin-'+run.id" :class="['run-item', run.status === 'failed' ? 'failed-item' : 'completed-item']">
          <div class="run-header">
            <span class="run-name">Scene #{{ run.scene_id }}</span>
            <span :class="['status', run.status === 'failed' ? 'failed' : 'completed']">{{ run.status === 'failed' ? '已失败' : '已完成' }}</span>
          </div>
          <div v-if="run.error_msg" class="error-msg">{{ run.error_msg }}</div>
          <div class="run-metrics">
            <div class="metric"><span class="metric-label">总请求</span><span class="metric-val">{{ run.total_reqs }}</span></div>
            <div class="metric"><span class="metric-label">成功</span><span class="metric-val success">{{ run.success_reqs }}</span></div>
            <div class="metric"><span class="metric-label">失败</span><span class="metric-val danger">{{ run.failed_reqs }}</span></div>
            <div class="metric"><span class="metric-label">P99</span><span class="metric-val">{{ formatMs(run.p99_latency) }}</span></div>
          </div>
        </div>
      </div>
    </div>

    <div class="card">
      <h3>运行历史</h3>
      <div class="table-scroll">
      <table class="data-table">
        <thead>
          <tr>
            <th>数据KEY</th>
            <th>运行ID</th>
            <th>场景</th>
            <th>状态</th>
            <th>并发</th>
            <th>运行模式</th>
            <th>配置值</th>
            <th>总请求</th>
            <th>成功率</th>
            <th>P99</th>
            <th>开始时间</th>
            <th>结束时间</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="runs.length === 0"><td colspan="12" class="empty">暂无运行记录</td></tr>
          <tr v-for="r in runs" :key="r.id">
            <td class="mono">{{ r.id }}</td>
            <td class="mono">{{ r.run_id }}</td>
            <td>{{ r.scene_id }}</td>
            <td><span :class="['status-badge', r.status]">{{ r.status }}</span></td>
            <td>{{ Math.round(r.worker_count || 0) }}</td>
            <td><span class="mode-tag" :class="r.run_mode">{{ r.run_mode === 'duration' ? '持续时间' : '请求数' }}</span></td>
            <td class="mono">{{ r.run_mode === 'duration' ? Math.round(r.duration || 0) + 's' : (r.count || 0).toLocaleString() }}</td>
            <td>{{ r.total_reqs }}</td>
            <td>{{ ((r.success_reqs / Math.max(r.total_reqs, 1)) * 100).toFixed(2) }}%</td>
            <td>{{ formatMs(r.p99_latency) }}</td>
            <td>{{ formatTime(r.started_at) }}</td>
            <td>{{ r.status === 'running' ? '--' : formatTime(r.finished_at) }}</td>
          </tr>
        </tbody>
      </table>
      </div>
    </div>

    <div v-if="toastMsg" class="toast" :class="toastType">{{ toastMsg }}</div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, watch } from 'vue'
import { listScenes, listRuns, startScene, stopScene } from '@/api/scene'
import { listNodes } from '@/api/node'
import { useAuthStore } from '@/stores/auth'
import type { SceneDTO, RunRecordDTO } from '@/types'

const authStore = useAuthStore()
const canRunScene = computed(() => authStore.canAccess(['scene:run']))

const scenes = ref<SceneDTO[]>([])
const runs = ref<RunRecordDTO[]>([])
const activeRuns = ref<RunRecordDTO[]>([])
const starting = ref(false)
const selectedSceneHasNoDAG = ref(false)
const toastMsg = ref('')
const toastType = ref('info')

const stopConfirm = reactive({
  visible: false,
  sceneId: '',
  sceneName: '',
  message: '',
  loading: false,
})

const stoppingSceneIds = ref<Set<string>>(new Set())

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

      for (const sceneId of stoppingSceneIds.value) {
        const run = runs.value.find(r => r.scene_id === sceneId)
        if (!run || run.status !== 'running') {
          stoppingSceneIds.value.delete(sceneId)
        }
      }
    }
  } catch { /* ignore */ }
}

const recentFinishedRuns = computed(() => {
  return runs.value
    .filter((r) => r.status !== 'running')
    .sort((a, b) => {
      const aTime = a.started_at ? new Date(a.started_at).getTime() : 0
      const bTime = b.started_at ? new Date(b.started_at).getTime() : 0
      return bTime - aTime
    })
    .slice(0, 10)
})

function showToast(msg: string, type = 'info') {
  toastMsg.value = msg
  toastType.value = type
  setTimeout(() => { toastMsg.value = '' }, 5000)
}

async function handleStart() {
  if (!form.scene_id) return
  starting.value = true
  try {
    const resp = await startScene({
      scene_id: form.scene_id,
      workers: form.workers,
      run_mode: form.run_mode,
      duration: form.duration,
      count: form.count,
    })
    if (resp.code === 0) {
      showToast('测试已启动')
      fetchRuns()
    } else {
      showToast(resp.message || '启动失败', 'error')
      fetchRuns()
    }
  } catch (e: any) {
    showToast('启动失败: ' + (e.message || '未知错误'), 'error')
    fetchRuns()
  }
  starting.value = false
}

async function handleStop(sceneId: string) {
  try {
    await stopScene(sceneId)
    fetchRuns()
  } catch { /* ignore */ }
}

function showStopConfirm(sceneId: string, sceneName: string) {
  stopConfirm.visible = true
  stopConfirm.sceneId = sceneId
  stopConfirm.sceneName = sceneName
  stopConfirm.message = `确定要停止场景「${sceneName}」的运行吗？此操作不可撤销。`
  stopConfirm.loading = false
}

function cancelStop() {
  stopConfirm.visible = false
  stopConfirm.sceneId = ''
  stopConfirm.sceneName = ''
  stopConfirm.message = ''
  stopConfirm.loading = false
}

async function confirmStop() {
  if (!stopConfirm.sceneId) return

  stopConfirm.loading = true
  stoppingSceneIds.value.add(stopConfirm.sceneId)
  try {
    await handleStop(stopConfirm.sceneId)
    cancelStop()
    showToast('场景已停止', 'success')
  } catch (e: any) {
    showToast('停止失败: ' + (e.message || '未知错误'), 'error')
    stoppingSceneIds.value.delete(stopConfirm.sceneId)
    stopConfirm.loading = false
  }
}

function formatMs(sec: number): string {
  if (!sec) return '0ms'
  const ms = sec * 1000
  if (ms < 1) return ms.toFixed(3) + 'ms'
  if (ms < 1000) return ms.toFixed(1) + 'ms'
  return (ms / 1000).toFixed(2) + 's'
}

function formatTime(t?: string) {
  if (!t) return '-'
  const d = new Date(t)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

function getSceneName(sceneId: string): string {
  const scene = scenes.value.find(s => s.id === sceneId)
  return scene?.name || `Scene #${sceneId}`
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
.card h3 { font-size: 14px; font-weight: 600; margin-bottom: 12px; color: var(--text-primary); }
.form-group { display: flex; flex-direction: column; gap: 4px; margin-bottom: 10px; }
.form-group label { font-size: 12px; color: var(--text-secondary); }
.form-group input, .form-group select { height: 34px; padding: 0 8px; border: 1px solid var(--border-secondary); border-radius: var(--radius-sm); background: var(--bg-tertiary); color: var(--text-primary); font-size: 13px; outline: none; }
.form-row { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }

.btn-primary { padding: 8px 20px; border: none; border-radius: var(--radius-md); background: var(--accent-primary); color: #fff; font-size: 13px; cursor: pointer; margin-top: 8px; }
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-login-primary:disabled { opacity: 0.5; cursor: not-allowed; }
.no-dag-warning { margin-top: 8px; font-size: 12px; color: #f0ad4e; background: rgba(240,173,78,0.1); padding: 6px 10px; border-radius: var(--radius-sm); border: 1px solid rgba(240,173,78,0.3); }

.btn-sm {
  padding: 5px 12px;
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.btn-sm:hover {
  background: var(--accent-danger);
  color: #fff;
  border-color: var(--accent-danger);
}

.btn-sm.danger {
  color: var(--accent-danger);
  border-color: var(--accent-danger);
}

.btn-sm.danger:hover {
  background: var(--accent-danger);
  color: #fff;
}

.btn-sm:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  pointer-events: none;
}

.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}

.confirm-dialog {
  background: var(--bg-card);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  padding: 28px;
  width: 380px;
  text-align: center;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3), 0 0 1px rgba(0, 0, 0, 0.15);
}

.confirm-icon {
  width: 48px;
  height: 48px;
  margin: 0 auto 16px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(248, 81, 73, 0.12);
  color: var(--accent-danger);
}

.confirm-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 8px;
}

.confirm-msg {
  font-size: 13px;
  color: var(--text-secondary);
  margin: 0 0 24px;
  line-height: 1.5;
}

.confirm-actions {
  display: flex;
  justify-content: center;
  gap: 10px;
}

.btn-cancel {
  padding: 8px 20px;
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  background: var(--bg-tertiary);
  color: var(--text-primary);
  font-size: 13px;
  cursor: pointer;
  transition: background 0.15s ease;
}

.btn-cancel:hover {
  background: var(--bg-hover);
}

.btn-danger-confirm {
  padding: 8px 20px;
  border: none;
  border-radius: var(--radius-md);
  background: var(--accent-danger);
  color: #fff;
  font-size: 13px;
  cursor: pointer;
  transition: opacity 0.15s ease;
}

.btn-danger-confirm:hover:not(:disabled) {
  opacity: 0.88;
}

.btn-danger-confirm:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.empty { text-align: center; color: var(--text-tertiary); font-size: 13px; padding: 24px 0; }
.run-item { padding: 12px; background: var(--bg-tertiary); border-radius: var(--radius-sm); margin-bottom: 8px; }
.run-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
.run-name { font-size: 13px; font-weight: 500; }
.status { font-size: 11px; padding: 2px 8px; border-radius: 10px; }
.status.running { background: rgba(88,166,255,0.15); color: var(--accent-primary); }
.run-footer { display: flex; justify-content: space-between; align-items: flex-end; gap: 12px; }
.run-metrics { display: flex; gap: 16px; flex-wrap: wrap; }
.metric { display: flex; flex-direction: column; gap: 2px; }
.metric-label { font-size: 11px; color: var(--text-tertiary); }
.metric-val { font-size: 13px; font-weight: 600; font-variant-numeric: tabular-nums; }
.metric-val.success { color: var(--accent-success); }
.metric-val.danger { color: var(--accent-danger); }

.data-table { width: 100%; border-collapse: collapse; }
.table-scroll { overflow-x: auto; -webkit-overflow-scrolling: touch; }
.data-table th, .data-table td { padding: 10px 14px; text-align: left; font-size: 13px; border-bottom: 1px solid var(--border-secondary); white-space: nowrap; }
.data-table th { color: var(--text-secondary); font-weight: 500; background: var(--bg-tertiary); }
.mono { font-family: var(--font-mono); font-size: 12px; }
.status-badge { font-size: 11px; padding: 2px 8px; border-radius: 10px; }
.status-badge.running { background: rgba(88,166,255,0.15); color: var(--accent-primary); }
.status-badge.completed { background: rgba(63,185,80,0.15); color: var(--accent-success); }
.status-badge.failed { background: rgba(248,81,73,0.15); color: var(--accent-danger); }
.mode-tag { font-size: 11px; padding: 2px 8px; border-radius: 10px; font-weight: 500; }
.mode-tag.duration { background: rgba(210,153,34,0.15); color: #9a6700; }
.mode-tag.count { background: rgba(130,80,223,0.15); color: #8250df; }
.status.failed { background: rgba(248,81,73,0.15); color: var(--accent-danger); }
.status.completed { background: rgba(63,185,80,0.15); color: var(--accent-success); }
.failed-item { border-left: 3px solid var(--accent-danger); }
.completed-item { border-left: 3px solid var(--accent-success); }
.error-msg { font-size: 12px; color: var(--accent-danger); margin-bottom: 8px; padding: 6px 10px; background: rgba(248,81,73,0.08); border-radius: var(--radius-sm); word-break: break-all; }

.toast {
  position: fixed; bottom: 24px; right: 24px; padding: 10px 20px;
  border-radius: var(--radius-md); font-size: 13px; z-index: 200;
  animation: slideIn 0.3s ease;
}
.toast.info { background: var(--accent-primary); color: #fff; }
.toast.error { background: var(--accent-danger, #e74c3c); color: #fff; }
@keyframes slideIn { from { transform: translateY(20px); opacity: 0; } to { transform: translateY(0); opacity: 1; } }
</style>
