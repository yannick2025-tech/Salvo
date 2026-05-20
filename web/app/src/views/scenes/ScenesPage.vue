<template>
  <div class="scenes-page">
    <div class="page-header">
      <h2>场景管理</h2>
      <div class="header-actions">
        <button class="btn-secondary" @click="showImport = true">导入 YAML</button>
        <button class="btn-login-primary" @click="showCreate = true">+ 新建场景</button>
      </div>
    </div>

    <div class="table-wrapper">
      <table class="data-table">
        <thead>
          <tr>
            <th>ID</th>
            <th>名称</th>
            <th>描述</th>
            <th>状态</th>
            <th>创建时间</th>
            <th>测试开始时间</th>
            <th>结束时间</th>
            <th>持续时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="scenes.length === 0">
            <td colspan="9" class="empty">暂无场景</td>
          </tr>
          <tr v-for="s in scenes" :key="s.id">
            <td class="mono">{{ s.id }}</td>
            <td><router-link :to="`/scenes/${s.id}`" class="link">{{ s.name }}</router-link></td>
            <td><div class="desc-cell" :title="s.description">{{ s.description || '-' }}</div></td>
            <td><span :class="['status-badge', s.status]">{{ s.status }}</span></td>
            <td>{{ formatTime(s.created_at) }}</td>
            <td class="time-cell">{{ getSceneLatestRun(s)?.started_at ? formatDateTime(getSceneLatestRun(s)!.started_at) : '-' }}</td>
            <td class="time-cell">{{ getSceneLatestRun(s)?.finished_at ? formatDateTime(getSceneLatestRun(s)!.finished_at) : (isSceneRunning(s) ? '--' : '-') }}</td>
            <td class="time-cell">{{ calculateSceneDuration(s) }}</td>
            <td class="actions">
              <button class="btn-sm" :class="{ disabled: isSceneRunning(s) }" :disabled="isSceneRunning(s)" @click="editScene(s)">编辑</button>
              <button class="btn-sm danger" @click="handleDelete(s.id)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="showCreate" class="modal-overlay" @click.self="showCreate = false">
      <div class="modal">
        <h3>新建场景</h3>
        <div class="form-group">
          <label>名称</label>
          <input v-model="createForm.name" placeholder="场景名称" />
        </div>
        <div class="form-group">
          <label>描述</label>
          <input v-model="createForm.description" placeholder="场景描述" />
        </div>
        <div class="modal-actions">
          <button class="btn-secondary" @click="showCreate = false">取消</button>
          <button class="btn-login-primary" @click="handleCreate">创建</button>
        </div>
      </div>
    </div>

    <div v-if="showConfirm" class="modal-overlay" @click.self="showConfirm = false">
      <div class="confirm-dialog">
        <div class="confirm-icon">
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10"/>
            <line x1="12" y1="8" x2="12" y2="12"/>
            <line x1="12" y1="16" x2="12.01" y2="16"/>
          </svg>
        </div>
        <h3 class="confirm-title">确认删除</h3>
        <p class="confirm-msg">{{ confirmMessage }}</p>
        <div class="confirm-actions">
          <button class="btn-cancel" @click="showConfirm = false">取消</button>
          <button class="btn-danger-confirm" @click="confirmDelete">确认删除</button>
        </div>
      </div>
    </div>

    <div v-if="showImport" class="modal-overlay" @click.self="showImport = false">
      <div class="modal modal-lg">
        <h3>导入 YAML 配置</h3>
        <p class="import-hint">粘贴 YAML 配置内容，系统将自动创建场景和 DAG 请求流节点。支持 setup/teardown 生命周期、参数生成器 (generator)、参数关联 (extract)、条件判断 (condition) 和延迟 (delay) 等节点类型。可点击"加载示例"查看完整电商全链路配置模板。</p>
        <div class="form-group">
          <label>场景名称</label>
          <input v-model="importForm.name" placeholder="留空则使用 YAML 中的 name 字段" />
        </div>
        <div class="form-group">
          <label>YAML 内容</label>
          <textarea v-model="importForm.yaml" class="yaml-input" placeholder="在此粘贴 YAML 配置..." rows="24"></textarea>
        </div>
        <div v-if="importError" class="form-error">{{ importError }}</div>
        <div class="modal-actions">
          <button class="btn-secondary" @click="loadExample">加载示例</button>
          <button class="btn-secondary" @click="showImport = false">取消</button>
          <button class="btn-login-primary" @click="handleImport" :disabled="importing">
            {{ importing ? '导入中...' : '导入' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { listScenes, createScene, deleteScene, importYAML } from '@/api/scene'
import type { SceneDTO } from '@/types'

const router = useRouter()
const scenes = ref<SceneDTO[]>([])
const showCreate = ref(false)
const createForm = reactive({ name: '', description: '' })
const showImport = ref(false)
const importForm = reactive({ name: '', yaml: '' })
const importing = ref(false)
const importError = ref('')
const showConfirm = ref(false)
const confirmMessage = ref('')
const pendingDeleteId = ref('')

const exampleYAML = `name: Mock API 电商全链路压测
description: |
  基于 Mock Server 的完整电商链路压测场景，展示 Salvo 核心能力：
  - Setup/Teardown 生命周期管理
  - 参数生成器 (Generator) 动态生成请求参数
  - 参数关联 (Extract) 从响应提取变量传递给后续请求
  - IF-ELSE 分支 (IF-ELSE) 根据运行时条件走不同分支
  - 延迟控制 (Delay) 模拟用户思考时间

variables:
  - key: base_url
    value: http://localhost:9090/mock/api
  - key: token
    value: ""
  - key: user_id
    value: "0"
  - key: product_id
    value: "1"
  - key: order_id
    value: ""
  - key: payment_status
    value: ""

setup:
  - name: 注册测试用户
    type: setup
    config:
      method: POST
      url: "\${base_url}/users"
      headers:
        Content-Type: application/json
      body: '{"name":"perf_test_user","email":"perf@example.com"}'
      timeout: 5000
      expect_status: 201
      extract:
        user_id: "$.data.id"

  - name: 用户登录获取Token
    type: setup
    config:
      method: POST
      url: "\${base_url}/auth/login"
      headers:
        Content-Type: application/json
      body: '{"email":"admin@example.com","password":"admin123"}'
      timeout: 5000
      expect_status: 200
      extract:
        token: "$.data.token"
        user_id: "$.data.user.id"

nodes:
  - name: 浏览商品列表
    type: http
    config:
      method: GET
      url: "\${base_url}/products?page=1"
      headers:
        Authorization: "Bearer \${token}"
      timeout: 5000
      expect_status: 200
      extract:
        product_id: "$.data[0].id"

  - name: 查看商品详情
    type: http
    config:
      method: GET
      url: "\${base_url}/products/\${product_id}"
      headers:
        Authorization: "Bearer \${token}"
      timeout: 5000
      expect_status: 200

  - name: 用户思考时间
    type: delay
    config:
      ms: 500

  - name: 创建订单
    type: http
    config:
      method: POST
      url: "\${base_url}/orders"
      headers:
        Content-Type: application/json
        Authorization: "Bearer \${token}"
      body: '{"user_id":\${user_id},"total":79.98}'
      timeout: 5000
      expect_status: 201
      extract:
        order_id: "$.data.id"
      generator:
        body_fields:
          total:
            type: number
            minimum: 10
            maximum: 500
            multipleOf: 0.01

  - name: 判断是否支付
    type: if-else
    config:
      expr: "\${order_id} != ''"

  - name: 支付订单
    type: http
    config:
      method: POST
      url: "\${base_url}/payment"
      headers:
        Content-Type: application/json
        Authorization: "Bearer \${token}"
      body: '{"amount":79.98,"order_id":"\${order_id}"}'
      timeout: 5000
      expect_status: 200
      extract:
        payment_status: "$.data.status"
      generator:
        body_fields:
          amount:
            type: number
            minimum: 10
            maximum: 500

  - name: 跳过支付
    type: http
    config:
      method: GET
      url: "\${base_url}/orders/\${order_id}"
      headers:
        Authorization: "Bearer \${token}"
      timeout: 5000
      expect_status: 200

  - name: 发送支付通知
    type: http
    config:
      method: POST
      url: "\${base_url}/notify"
      headers:
        Content-Type: application/json
        Authorization: "Bearer \${token}"
      body: '{"event":"payment_success","order_id":"\${order_id}"}'
      timeout: 5000
      expect_status: 200

teardown:
  - name: 查询最终订单状态
    type: teardown
    config:
      method: GET
      url: "\${base_url}/orders/\${order_id}"
      headers:
        Authorization: "Bearer \${token}"
      timeout: 5000
      expect_status: 200

  - name: 清理测试数据
    type: teardown
    config:
      method: DELETE
      url: "\${base_url}/users/\${user_id}"
      headers:
        Authorization: "Bearer \${token}"
      timeout: 5000
      expect_status: 200

edges:
  - from: 注册测试用户
    to: 用户登录获取Token

  - from: 用户登录获取Token
    to: 浏览商品列表

  - from: 浏览商品列表
    to: 查看商品详情

  - from: 查看商品详情
    to: 用户思考时间

  - from: 用户思考时间
    to: 创建订单

  - from: 创建订单
    to: 判断是否支付

  - from: 判断是否支付
    to: 支付订单
    condition: "__if_true__"

  - from: 判断是否支付
    to: 跳过支付
    condition: "__if_false__"

  - from: 支付订单
    to: 发送支付通知

  - from: 发送支付通知
    to: 查询最终订单状态

  - from: 跳过支付
    to: 查询最终订单状态

  - from: 查询最终订单状态
    to: 清理测试数据
`

async function fetchScenes() {
  try {
    const resp = await listScenes({ limit: 50 })
    if (resp.code === 0) {
      scenes.value = resp.data.items || []
    }
  } catch { /* ignore */ }
}

async function handleCreate() {
  if (!createForm.name) return
  const resp = await createScene(createForm)
  if (resp.code === 0) {
    showCreate.value = false
    createForm.name = ''
    createForm.description = ''
    if (resp.data?.id) {
      router.push(`/scenes/${resp.data.id}`)
    } else {
      fetchScenes()
    }
  }
}

async function handleDelete(id: string) {
  pendingDeleteId.value = id
  confirmMessage.value = '确定要删除该场景吗？此操作不可撤销。'
  showConfirm.value = true
}

async function confirmDelete() {
  const id = pendingDeleteId.value
  showConfirm.value = false
  pendingDeleteId.value = ''
  await deleteScene(id)
  fetchScenes()
}

function editScene(s: SceneDTO) {
  router.push(`/scenes/${s.id}`)
}

function loadExample() {
  importForm.yaml = exampleYAML
  importForm.name = ''
}

async function handleImport() {
  importError.value = ''
  if (!importForm.yaml.trim()) {
    importError.value = '请输入 YAML 内容'
    return
  }
  importing.value = true
  try {
    const resp = await importYAML({
      name: importForm.name,
      yaml: importForm.yaml,
    })
    if (resp.code === 0) {
      showImport.value = false
      importForm.name = ''
      importForm.yaml = ''
      fetchScenes()
      if (resp.data?.id) {
        router.push(`/scenes/${resp.data.id}`)
      }
    } else {
      importError.value = resp.message || '导入失败'
    }
  } catch (e: any) {
    importError.value = e.message || '导入失败'
  } finally {
    importing.value = false
  }
}

function formatTime(t: string) {
  if (!t) return '-'
  const d = new Date(t)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

function formatDateTime(timeStr?: string): string {
  if (!timeStr) return '-'
  const d = new Date(timeStr)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

interface SceneRunInfo {
  started_at?: string
  finished_at?: string
  status?: string
  duration?: number
}

const sceneRunsMap = ref<Map<string, SceneRunInfo[]>>(new Map())

async function fetchSceneRuns() {
  try {
    const { dashboardOverview } = await import('@/api/dashboard')
    const resp = await dashboardOverview(86400 * 7)
    if (resp.code === 0 && resp.data?.recent_runs) {
      const runsMap = new Map<string, SceneRunInfo[]>()
      resp.data.recent_runs.forEach((run: any) => {
        const sceneId = String(run.scene_id)
        const info: SceneRunInfo = {
          started_at: run.started_at,
          finished_at: run.finished_at,
          status: run.status,
          duration: run.duration,
        }
        if (!runsMap.has(sceneId)) {
          runsMap.set(sceneId, [])
        }
        runsMap.get(sceneId)!.push(info)
      })
      sceneRunsMap.value = runsMap
    }
  } catch (e) {
    console.error('Failed to fetch scene runs:', e)
  }
}

function getSceneLatestRun(scene: any): SceneRunInfo | undefined {
  const runs = sceneRunsMap.value.get(scene.id)
  if (!runs || runs.length === 0) return undefined
  return runs[runs.length - 1]
}

function isSceneRunning(scene: any): boolean {
  const latest = getSceneLatestRun(scene)
  return latest?.status === 'running'
}

function calculateSceneDuration(scene: any): string {
  const latest = getSceneLatestRun(scene)
  if (!latest?.started_at) return '-'

  const start = new Date(latest.started_at).getTime()
  const end = latest.status === 'running' ? Date.now() : (latest.finished_at ? new Date(latest.finished_at).getTime() : Date.now())

  const durationMs = end - start
  if (durationMs <= 0) return '-'

  const totalSeconds = Math.floor(durationMs / 1000)
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60
  const pad = (n: number) => String(n).padStart(2, '0')

  if (hours > 0) {
    return `${hours}小时${pad(minutes)}分${pad(seconds)}秒`
  } else if (minutes > 0) {
    return `${minutes}分${pad(seconds)}秒`
  } else {
    return `${seconds}秒`
  }
}

onMounted(() => {
  fetchScenes()
  fetchSceneRuns()
})
</script>

<style scoped>
.scenes-page { display: flex; flex-direction: column; gap: 16px; }
.page-header { display: flex; justify-content: space-between; align-items: center; }
.page-header h2 { font-size: 18px; font-weight: 600; }
.header-actions { display: flex; gap: 8px; }

.btn-primary {
  padding: 8px 16px; border: none; border-radius: var(--radius-md);
  background: var(--accent-primary); color: #fff; font-size: 13px; cursor: pointer;
}
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-secondary {
  padding: 8px 16px; border: 1px solid var(--border-primary); border-radius: var(--radius-md);
  background: transparent; color: var(--text-secondary); font-size: 13px; cursor: pointer;
}
.btn-sm {
  padding: 4px 10px;
  border: 1px solid var(--border-primary);
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

.btn-sm.disabled {
  opacity: 0.4;
  cursor: not-allowed;
  pointer-events: none;
}

.table-wrapper {
  background: var(--bg-card);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-md);
  overflow-x: auto;
  overflow-y: visible;
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
}
.data-table th { color: var(--text-secondary); font-weight: 500; background: var(--bg-tertiary); }
.data-table td { color: var(--text-primary); }
.empty { text-align: center; color: var(--text-tertiary); padding: 32px 0; }
.mono { font-family: monospace; font-size: 12px; color: var(--text-secondary); }
.desc-cell { max-width: 280px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 12px; color: var(--text-secondary); }
.link { color: var(--accent-primary); text-decoration: none; }
.link:hover { text-decoration: underline; }
.time-cell { font-size: 13px; color: var(--text-primary); white-space: nowrap; }
.status-badge { font-size: 11px; padding: 2px 8px; border-radius: 10px; }
.status-badge.draft { background: rgba(139,148,158,0.15); color: var(--text-secondary); }
.status-badge.ready { background: rgba(63,185,80,0.15); color: var(--accent-success); }
.status-badge.running { background: rgba(88,166,255,0.15); color: var(--accent-primary); }
.actions { display: flex; gap: 6px; }

.modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 100; }
.modal { background: var(--bg-card); border: 1px solid var(--border-primary); border-radius: var(--radius-lg); padding: 24px; width: 420px; }
.modal-lg { width: 680px; }
.modal h3 { font-size: 16px; margin-bottom: 16px; }
.form-group { margin-bottom: 14px; }
.form-group label { display: block; font-size: 13px; color: var(--text-secondary); margin-bottom: 4px; }
.form-group input { width: 100%; height: 36px; padding: 0 10px; border: 1px solid var(--border-primary); border-radius: var(--radius-sm); background: var(--bg-input); color: var(--text-primary); font-size: 13px; outline: none; }
.form-group input:focus { border-color: var(--accent-primary); }
.yaml-input { width: 100%; padding: 10px; border: 1px solid var(--border-primary); border-radius: var(--radius-sm); background: var(--bg-input); color: var(--text-primary); font-size: 12px; font-family: 'Menlo', 'Monaco', 'Courier New', monospace; outline: none; resize: vertical; line-height: 1.5; }
.yaml-input:focus { border-color: var(--accent-primary); }
.import-hint { font-size: 12px; color: var(--text-tertiary); margin-bottom: 14px; line-height: 1.5; }
.form-error { font-size: 12px; color: var(--accent-danger, #e74c3c); background: rgba(248,81,73,0.1); padding: 6px 10px; border-radius: var(--radius-sm); margin-bottom: 8px; }
.modal-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 20px; }

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
  width: 48px; height: 48px;
  margin: 0 auto 16px;
  border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  background: rgba(248, 81, 73, 0.12);
  color: var(--accent-danger);
}
.confirm-title {
  font-size: 16px; font-weight: 600; color: var(--text-primary); margin: 0 0 8px;
}
.confirm-msg {
  font-size: 13px; color: var(--text-secondary); margin: 0 0 24px; line-height: 1.5;
}
.confirm-actions { display: flex; justify-content: center; gap: 10px; }
.btn-cancel {
  padding: 8px 20px; border: 1px solid var(--border-primary); border-radius: var(--radius-md);
  background: var(--bg-tertiary); color: var(--text-primary); font-size: 13px; cursor: pointer;
  transition: background 0.15s ease;
}
.btn-cancel:hover { background: var(--bg-hover); }
.btn-danger-confirm {
  padding: 8px 20px; border: none; border-radius: var(--radius-md);
  background: var(--accent-danger); color: #fff; font-size: 13px; cursor: pointer;
  transition: opacity 0.15s ease;
}
.btn-danger-confirm:hover { opacity: 0.88; }
</style>
