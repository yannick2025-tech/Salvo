<template>
  <div class="scenes-page">
    <div class="page-header">
      <h2>场景管理</h2>
      <div class="header-actions">
        <button class="btn-secondary" @click="showImport = true">导入 YAML</button>
        <button class="btn-primary" @click="showCreate = true">+ 新建场景</button>
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
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="scenes.length === 0">
            <td colspan="6" class="empty">暂无场景</td>
          </tr>
          <tr v-for="s in scenes" :key="s.id">
            <td class="mono">{{ s.id }}</td>
            <td><router-link :to="`/scenes/${s.id}`" class="link">{{ s.name }}</router-link></td>
            <td>{{ s.description || '-' }}</td>
            <td><span :class="['status-badge', s.status]">{{ s.status }}</span></td>
            <td>{{ formatTime(s.created_at) }}</td>
            <td class="actions">
              <button class="btn-sm" @click="editScene(s)">编辑</button>
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
          <button class="btn-primary" @click="handleCreate">创建</button>
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
          <button class="btn-primary" @click="handleImport" :disabled="importing">
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

const exampleYAML = `name: Mock API 电商全链路压测
description: |
  基于 Mock Server 的完整电商链路压测场景，展示 Salvo 核心能力：
  - Setup/Teardown 生命周期管理
  - 参数生成器 (Generator) 动态生成请求参数
  - 参数关联 (Extract) 从响应提取变量传递给后续请求
  - 条件判断 (Condition) 根据运行时条件走不同分支
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
    type: condition
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
    condition: "true"

  - from: 判断是否支付
    to: 跳过支付
    condition: "false"

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
  if (!confirm('确定要删除该场景吗？此操作不可撤销。')) return
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
  return new Date(t).toLocaleString()
}

onMounted(fetchScenes)
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
  padding: 4px 10px; border: 1px solid var(--border-primary); border-radius: var(--radius-sm);
  background: transparent; color: var(--text-secondary); font-size: 12px; cursor: pointer;
}
.btn-sm.danger { color: var(--accent-danger); border-color: var(--accent-danger); }

.table-wrapper { background: var(--bg-card); border: 1px solid var(--border-secondary); border-radius: var(--radius-md); overflow: auto; }
.data-table { width: 100%; border-collapse: collapse; }
.data-table th, .data-table td { padding: 10px 14px; text-align: left; font-size: 13px; border-bottom: 1px solid var(--border-secondary); }
.data-table th { color: var(--text-secondary); font-weight: 500; background: var(--bg-tertiary); }
.data-table td { color: var(--text-primary); }
.empty { text-align: center; color: var(--text-tertiary); padding: 32px 0; }
.mono { font-family: monospace; font-size: 12px; color: var(--text-secondary); }
.link { color: var(--accent-primary); text-decoration: none; }
.link:hover { text-decoration: underline; }
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
</style>
