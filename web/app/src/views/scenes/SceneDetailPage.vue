<template>
  <div class="scene-detail">
    <div class="page-header">
      <button class="btn-back" @click="$router.push('/scenes')">← 返回</button>
      <h2 v-if="scene">{{ scene.name }}</h2>
      <div class="header-actions">
        <button class="btn-outline" @click="showCopyModal = true">复制场景</button>
        <button class="btn-primary" @click="showRunConfig = true">▶ 启动测试</button>
      </div>
    </div>

    <div v-if="scene" class="scene-info">
      <div class="info-grid">
        <div class="info-item"><span class="label">状态</span><span :class="['status-badge', scene.status]">{{ scene.status }}</span></div>
        <div class="info-item"><span class="label">描述</span><span class="value">{{ scene.description || '-' }}</span></div>
        <div class="info-item"><span class="label">创建时间</span><span class="value">{{ formatTime(scene.created_at) }}</span></div>
      </div>
    </div>

    <div class="dag-section">
      <div class="section-header">
        <h3>DAG 请求流</h3>
        <div class="dag-actions">
          <button class="btn-sm" @click="addNode('setup')">+ Setup</button>
          <button class="btn-sm" @click="addNode('http')">+ HTTP</button>
          <button class="btn-sm" @click="addNode('delay')">+ 延迟</button>
          <button class="btn-sm" @click="addNode('condition')">+ 条件</button>
          <button class="btn-sm" @click="addNode('teardown')">+ Teardown</button>
        </div>
      </div>

      <div class="dag-canvas" ref="canvasRef">
        <div v-if="nodes.length === 0" class="dag-empty">
          <div class="empty-icon">⬡</div>
          <p>暂无请求节点，点击上方按钮添加</p>
        </div>

        <div class="dag-flow" v-else>
          <template v-for="(node, idx) in sortedNodes" :key="node.id">
            <div
              :class="['dag-node', node.type, { active: selectedNode?.id === node.id }]"
              @click="selectNode(node)"
            >
              <div :class="['node-icon', node.type]">{{ nodeIcon(node.type) }}</div>
              <div class="node-info">
                <span class="node-name">{{ node.name }}</span>
                <span class="node-type-badge">{{ nodeTypeLabel(node.type) }}</span>
              </div>
              <div class="node-actions">
                <button class="node-btn" @click.stop="editNode(node)" title="编辑">✎</button>
                <button class="node-btn danger" @click.stop="handleDeleteNode(node.id)" title="删除">✕</button>
              </div>
            </div>
            <div v-if="idx < sortedNodes.length - 1" class="dag-edge">
              <div class="edge-line"></div>
              <div v-if="getEdgeCondition(sortedNodes[idx].id, sortedNodes[idx + 1].id)" class="edge-condition">
                {{ getEdgeCondition(sortedNodes[idx].id, sortedNodes[idx + 1].id) }}
              </div>
              <div class="edge-arrow">▼</div>
              <button class="edge-delete" @click="deleteEdgeBetween(idx)" title="删除连线">✕</button>
            </div>
          </template>
        </div>
      </div>
    </div>

    <div v-if="selectedNode" class="node-config-panel">
      <div class="panel-header">
        <h4>{{ selectedNode.name }} - 配置</h4>
        <button class="btn-close" @click="selectedNode = null">✕</button>
      </div>

      <div v-if="selectedNode.type === 'http' || selectedNode.type === 'setup' || selectedNode.type === 'teardown'" class="config-form">
        <div class="form-row">
          <label>节点名称</label>
          <input v-model="editingConfig.name" @change="saveNodeConfig" />
        </div>
        <div class="form-row">
          <label>请求方法</label>
          <select v-model="httpConfig.method" @change="saveNodeConfig">
            <option value="GET">GET</option>
            <option value="POST">POST</option>
            <option value="PUT">PUT</option>
            <option value="DELETE">DELETE</option>
            <option value="PATCH">PATCH</option>
          </select>
        </div>
        <div class="form-row">
          <label>URL</label>
          <input v-model="httpConfig.url" placeholder="http://localhost:9090/mock/api/users" @change="saveNodeConfig" />
        </div>
        <div class="form-row">
          <label>Headers (JSON)</label>
          <textarea v-model="httpConfig.headers" rows="3" placeholder='{"Content-Type":"application/json","Authorization":"Bearer ${token}"}' @change="saveNodeConfig"></textarea>
        </div>
        <div class="form-row">
          <label>Body</label>
          <textarea v-model="httpConfig.body" rows="4" placeholder='{"email":"admin@example.com","password":"admin123"}' @change="saveNodeConfig"></textarea>
        </div>
        <div class="form-row inline">
          <label>超时(ms)</label>
          <input v-model.number="httpConfig.timeout" type="number" @change="saveNodeConfig" />
        </div>
        <div class="form-row inline">
          <label>期望状态码</label>
          <input v-model.number="httpConfig.expect_status" type="number" @change="saveNodeConfig" />
        </div>
        <div class="form-row">
          <label>变量提取 (JSON)</label>
          <textarea v-model="httpConfig.extract" rows="2" placeholder='{"token":"$.data.token","user_id":"$.data.user.id"}' @change="saveNodeConfig"></textarea>
        </div>
        <div class="form-row">
          <label>参数生成器 (JSON)</label>
          <textarea v-model="httpConfig.generator" rows="3" placeholder='{"email":{"type":"string","format":"email"},"age":{"type":"integer","minimum":18,"maximum":65}}' @change="saveNodeConfig"></textarea>
        </div>
      </div>

      <div v-else-if="selectedNode.type === 'delay'" class="config-form">
        <div class="form-row">
          <label>节点名称</label>
          <input v-model="editingConfig.name" @change="saveNodeConfig" />
        </div>
        <div class="form-row inline">
          <label>延迟时间(ms)</label>
          <input v-model.number="delayConfig.ms" type="number" @change="saveNodeConfig" />
        </div>
      </div>

      <div v-else-if="selectedNode.type === 'condition'" class="config-form">
        <div class="form-row">
          <label>节点名称</label>
          <input v-model="editingConfig.name" @change="saveNodeConfig" />
        </div>
        <div class="form-row">
          <label>条件表达式</label>
          <input v-model="conditionConfig.expr" placeholder='${status_code} == 200' @change="saveNodeConfig" />
        </div>
      </div>
    </div>

    <div v-if="showRunConfig" class="modal-overlay" @click.self="showRunConfig = false">
      <div class="modal">
        <h3>启动测试</h3>
        <div v-if="nodes.length === 0" class="run-warning">
          ⚠ 当前场景没有配置 DAG 请求流，请先添加节点
        </div>
        <div class="form-group">
          <label>并发数</label>
          <input v-model.number="runConfig.workers" type="number" min="1" max="1000" />
        </div>
        <div class="form-group">
          <label>运行模式</label>
          <select v-model="runConfig.run_mode">
            <option value="count">按次数</option>
            <option value="duration">按时间</option>
          </select>
        </div>
        <div v-if="runConfig.run_mode === 'count'" class="form-group">
          <label>总次数</label>
          <input v-model.number="runConfig.count" type="number" min="1" />
        </div>
        <div v-if="runConfig.run_mode === 'duration'" class="form-group">
          <label>持续时间(秒)</label>
          <input v-model.number="runConfig.duration" type="number" min="1" />
        </div>
        <div class="modal-actions">
          <button class="btn-secondary" @click="showRunConfig = false">取消</button>
          <button class="btn-primary" :disabled="nodes.length === 0" @click="handleStart">
            {{ nodes.length === 0 ? '无 DAG 节点' : '启动' }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="showCopyModal" class="modal-overlay" @click.self="showCopyModal = false">
      <div class="modal">
        <h3>复制场景</h3>
        <div class="form-group">
          <label>新场景名称</label>
          <input v-model="copyName" placeholder="输入新场景名称" />
        </div>
        <div class="modal-actions">
          <button class="btn-secondary" @click="showCopyModal = false">取消</button>
          <button class="btn-primary" @click="handleCopyScene">确认复制</button>
        </div>
      </div>
    </div>

    <div v-if="showNodeEditor" class="modal-overlay" @click.self="closeNodeEditor">
      <div class="modal">
        <h3>{{ editingNode ? '编辑节点' : '添加节点' }}</h3>
        <div class="form-group">
          <label>节点名称</label>
          <input v-model="nodeForm.name" placeholder="如: Login / GetUsers / CreateOrder" />
        </div>
        <div class="form-group">
          <label>节点类型</label>
          <select v-model="nodeForm.type" :disabled="!!editingNode">
            <option value="setup">Setup (初始化)</option>
            <option value="http">HTTP 请求</option>
            <option value="delay">延迟</option>
            <option value="condition">条件判断</option>
            <option value="teardown">Teardown (清理)</option>
          </select>
        </div>
        <template v-if="nodeForm.type === 'http' || nodeForm.type === 'setup' || nodeForm.type === 'teardown'">
          <div class="form-group">
            <label>请求方法</label>
            <select v-model="nodeForm.httpMethod">
              <option value="GET">GET</option>
              <option value="POST">POST</option>
              <option value="PUT">PUT</option>
              <option value="DELETE">DELETE</option>
            </select>
          </div>
          <div class="form-group">
            <label>URL</label>
            <input v-model="nodeForm.url" placeholder="http://localhost:9090/mock/api/users" />
          </div>
          <div class="form-group">
            <label>Headers (JSON)</label>
            <textarea v-model="nodeForm.headers" rows="2" placeholder='{"Content-Type":"application/json"}'></textarea>
          </div>
          <div class="form-group">
            <label>Body</label>
            <textarea v-model="nodeForm.body" rows="3" placeholder='{"key":"value"}'></textarea>
          </div>
          <div class="form-group">
            <label>变量提取 (JSON)</label>
            <textarea v-model="nodeForm.extract" rows="2" placeholder='{"token":"$.data.token"}'></textarea>
          </div>
        </template>
        <template v-if="nodeForm.type === 'delay'">
          <div class="form-group">
            <label>延迟时间(ms)</label>
            <input v-model.number="nodeForm.delayMs" type="number" min="0" />
          </div>
        </template>
        <template v-if="nodeForm.type === 'condition'">
          <div class="form-group">
            <label>条件表达式</label>
            <input v-model="nodeForm.conditionExpr" placeholder='${status_code} == 200' />
          </div>
        </template>
        <div class="modal-actions">
          <button class="btn-secondary" @click="closeNodeEditor">取消</button>
          <button class="btn-primary" @click="handleSaveNode">{{ editingNode ? '保存' : '添加' }}</button>
        </div>
      </div>
    </div>

    <div v-if="toastMsg" class="toast" :class="toastType">{{ toastMsg }}</div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getScene, createScene, startScene } from '@/api/scene'
import { listNodes, addNode as apiAddNode, updateNode as apiUpdateNode, deleteNode as apiDeleteNode, listEdges, addEdge, deleteEdge } from '@/api/node'
import type { SceneDTO, NodeDTO, EdgeDTO } from '@/types'

const route = useRoute()
const router = useRouter()

const scene = ref<SceneDTO | null>(null)
const nodes = ref<NodeDTO[]>([])
const edges = ref<EdgeDTO[]>([])
const selectedNode = ref<NodeDTO | null>(null)

const showRunConfig = ref(false)
const showCopyModal = ref(false)
const showNodeEditor = ref(false)
const editingNode = ref<NodeDTO | null>(null)
const copyName = ref('')

const toastMsg = ref('')
const toastType = ref('info')

const runConfig = reactive({
  workers: 10,
  run_mode: 'count',
  count: 100,
  duration: 30,
})

const nodeForm = reactive({
  name: '',
  type: 'http',
  httpMethod: 'GET',
  url: '',
  headers: '',
  body: '',
  delayMs: 1000,
  conditionExpr: '',
  extract: '',
})

const editingConfig = reactive({ name: '' })
const httpConfig = reactive({
  method: 'GET',
  url: '',
  headers: '',
  body: '',
  timeout: 5000,
  expect_status: 200,
  extract: '',
  generator: '',
})
const delayConfig = reactive({ ms: 1000 })
const conditionConfig = reactive({ expr: '' })

const sortedNodes = computed(() => {
  if (nodes.value.length === 0) return []

  const nodeMap = new Map<string, NodeDTO>()
  for (const n of nodes.value) nodeMap.set(n.id, n)

  const outEdges = new Map<string, EdgeDTO[]>()
  const inEdges = new Map<string, EdgeDTO[]>()
  for (const e of edges.value) {
    outEdges.set(e.from_node, [...(outEdges.get(e.from_node) || []), e])
    inEdges.set(e.to_node, [...(inEdges.get(e.to_node) || []), e])
  }

  const rootNodes = nodes.value.filter(n => !edges.value.some(e => e.to_node === n.id))

  const result: NodeDTO[] = []
  const visited = new Set<string>()

  function dfs(nodeId: string) {
    if (visited.has(nodeId)) return
    visited.add(nodeId)
    const n = nodeMap.get(nodeId)
    if (n) result.push(n)
    const children = outEdges.get(nodeId) || []
    for (const e of children) {
      dfs(e.to_node)
    }
  }

  for (const root of rootNodes) {
    dfs(root.id)
  }

  for (const n of nodes.value) {
    if (!visited.has(n.id)) result.push(n)
  }

  return result
})

function getEdgeCondition(fromId: string, toId: string): string {
  const edge = edges.value.find(e => e.from_node === fromId && e.to_node === toId)
  return edge?.condition || ''
}

function nodeIcon(type: string) {
  switch (type) {
    case 'setup': return '▶'
    case 'http': return '⇄'
    case 'delay': return '⏱'
    case 'condition': return '◇'
    case 'loop': return '↻'
    case 'teardown': return '■'
    default: return '○'
  }
}

function nodeTypeLabel(type: string) {
  switch (type) {
    case 'setup': return 'SETUP'
    case 'http': return 'HTTP'
    case 'delay': return 'DELAY'
    case 'condition': return 'CONDITION'
    case 'loop': return 'LOOP'
    case 'teardown': return 'TEARDOWN'
    default: return type.toUpperCase()
  }
}

function showToast(msg: string, type = 'info') {
  toastMsg.value = msg
  toastType.value = type
  setTimeout(() => { toastMsg.value = '' }, 3000)
}

async function fetchScene() {
  const id = route.params.id as string
  if (!id) return
  try {
    const resp = await getScene(id)
    if (resp.code === 0) scene.value = resp.data
  } catch { /* ignore */ }
}

async function fetchNodes() {
  const id = route.params.id as string
  if (!id) return
  try {
    const resp = await listNodes(id)
    if (resp.code === 0) {
      nodes.value = resp.data.items || []
    }
  } catch { /* ignore */ }
  try {
    const resp = await listEdges(id)
    if (resp.code === 0) {
      edges.value = resp.data.items || []
    }
  } catch { /* ignore */ }
}

async function fetchEdges() {
  const id = route.params.id as string
  if (!id) return
  try {
    const resp = await listEdges(id)
    if (resp.code === 0) {
      edges.value = resp.data.items || []
    }
  } catch { /* ignore */ }
}

function addNode(type: string) {
  editingNode.value = null
  nodeForm.name = ''
  nodeForm.type = type
  nodeForm.httpMethod = 'GET'
  nodeForm.url = ''
  nodeForm.headers = ''
  nodeForm.body = ''
  nodeForm.delayMs = 1000
  nodeForm.conditionExpr = ''
  nodeForm.extract = ''
  showNodeEditor.value = true
}

function editNode(node: NodeDTO) {
  editingNode.value = node
  nodeForm.name = node.name
  nodeForm.type = node.type
  try {
    const cfg = JSON.parse(node.config || '{}')
    nodeForm.httpMethod = cfg.method || 'GET'
    nodeForm.url = cfg.url || ''
    nodeForm.headers = cfg.headers ? JSON.stringify(cfg.headers, null, 2) : ''
    nodeForm.body = cfg.body || ''
    nodeForm.delayMs = cfg.ms || 1000
    nodeForm.conditionExpr = cfg.expr || ''
    nodeForm.extract = cfg.extract ? JSON.stringify(cfg.extract, null, 2) : ''
  } catch {
    nodeForm.httpMethod = 'GET'
    nodeForm.url = ''
    nodeForm.headers = ''
    nodeForm.body = ''
    nodeForm.delayMs = 1000
    nodeForm.conditionExpr = ''
    nodeForm.extract = ''
  }
  showNodeEditor.value = true
}

function closeNodeEditor() {
  showNodeEditor.value = false
  editingNode.value = null
}

async function handleSaveNode() {
  if (!nodeForm.name) {
    showToast('请输入节点名称', 'error')
    return
  }

  const sceneId = route.params.id as string
  let config = '{}'

  if (nodeForm.type === 'http' || nodeForm.type === 'setup' || nodeForm.type === 'teardown') {
    let headers: Record<string, string> = {}
    try { headers = JSON.parse(nodeForm.headers || '{}') } catch { /* ignore */ }
    let extract: Record<string, string> = {}
    try { extract = JSON.parse(nodeForm.extract || '{}') } catch { /* ignore */ }
    config = JSON.stringify({
      method: nodeForm.httpMethod,
      url: nodeForm.url,
      headers,
      body: nodeForm.body,
      timeout: httpConfig.timeout || 5000,
      expect_status: httpConfig.expect_status || 200,
      extract,
    })
  } else if (nodeForm.type === 'delay') {
    config = JSON.stringify({ ms: nodeForm.delayMs })
  } else if (nodeForm.type === 'condition') {
    config = JSON.stringify({ expr: nodeForm.conditionExpr })
  }

  if (editingNode.value) {
    const resp = await apiUpdateNode({
      id: editingNode.value.id,
      name: nodeForm.name,
      type: nodeForm.type,
      config,
    })
    if (resp.code === 0) {
      showToast('节点已更新')
      fetchNodes()
    } else {
      showToast(resp.message || '更新失败', 'error')
    }
  } else {
    const resp = await apiAddNode({
      scene_id: sceneId,
      name: nodeForm.name,
      type: nodeForm.type,
      config,
    })
    if (resp.code === 0) {
      const newNode = resp.data
      if (nodes.value.length > 0) {
        const lastNode = sortedNodes.value[sortedNodes.value.length - 1]
        if (lastNode) {
          await addEdge({
            scene_id: sceneId,
            from_node: lastNode.id,
            to_node: newNode.id,
          })
        }
      }
      showToast('节点已添加')
      fetchNodes()
    } else {
      showToast(resp.message || '添加失败', 'error')
    }
  }

  closeNodeEditor()
}

async function handleDeleteNode(id: string) {
  if (!confirm('确定要删除该节点吗？')) return
  const resp = await apiDeleteNode(id)
  if (resp.code === 0) {
    showToast('节点已删除')
    if (selectedNode.value?.id === id) selectedNode.value = null
    fetchNodes()
  } else {
    showToast(resp.message || '删除失败', 'error')
  }
}

async function deleteEdgeBetween(idx: number) {
  const sorted = sortedNodes.value
  if (idx >= sorted.length - 1) return
  const fromNode = sorted[idx]
  const toNode = sorted[idx + 1]
  const edge = edges.value.find(e => String(e.from_node) === String(fromNode.id) && String(e.to_node) === String(toNode.id))
  if (edge) {
    await deleteEdge(edge.id)
    fetchNodes()
  }
}

function selectNode(node: NodeDTO) {
  selectedNode.value = node
  editingConfig.name = node.name
  try {
    const cfg = JSON.parse(node.config || '{}')
    httpConfig.method = cfg.method || 'GET'
    httpConfig.url = cfg.url || ''
    httpConfig.headers = cfg.headers ? JSON.stringify(cfg.headers, null, 2) : ''
    httpConfig.body = cfg.body || ''
    httpConfig.timeout = cfg.timeout || 5000
    httpConfig.expect_status = cfg.expect_status || 200
    httpConfig.extract = cfg.extract ? JSON.stringify(cfg.extract, null, 2) : ''
    httpConfig.generator = cfg.generator ? JSON.stringify(cfg.generator, null, 2) : ''
    delayConfig.ms = cfg.ms || 1000
    conditionConfig.expr = cfg.expr || ''
  } catch { /* ignore */ }
}

async function saveNodeConfig() {
  if (!selectedNode.value) return
  let config = '{}'
  const nodeType = selectedNode.value.type
  if (nodeType === 'http' || nodeType === 'setup' || nodeType === 'teardown') {
    let headers: Record<string, string> = {}
    try { headers = JSON.parse(httpConfig.headers || '{}') } catch { /* ignore */ }
    let extract: Record<string, string> = {}
    try { extract = JSON.parse(httpConfig.extract || '{}') } catch { /* ignore */ }
    let generator: Record<string, any> = {}
    try { generator = JSON.parse(httpConfig.generator || '{}') } catch { /* ignore */ }
    const cfg: Record<string, any> = {
      method: httpConfig.method,
      url: httpConfig.url,
      headers,
      body: httpConfig.body,
      timeout: httpConfig.timeout,
      expect_status: httpConfig.expect_status,
    }
    if (Object.keys(extract).length > 0) cfg.extract = extract
    if (Object.keys(generator).length > 0) cfg.generator = generator
    config = JSON.stringify(cfg)
  } else if (nodeType === 'delay') {
    config = JSON.stringify({ ms: delayConfig.ms })
  } else if (nodeType === 'condition') {
    config = JSON.stringify({ expr: conditionConfig.expr })
  }

  await apiUpdateNode({
    id: selectedNode.value.id,
    name: editingConfig.name || selectedNode.value.name,
    config,
  })
  fetchNodes()
}

async function handleStart() {
  if (nodes.value.length === 0) {
    showToast('请先添加 DAG 节点', 'error')
    return
  }
  const sceneId = route.params.id as string
  try {
    const resp = await startScene({
      scene_id: sceneId,
      workers: runConfig.workers,
      run_mode: runConfig.run_mode,
      count: runConfig.count,
      duration: runConfig.duration,
    })
    if (resp.code === 0) {
      showToast('测试已启动')
      showRunConfig.value = false
      router.push('/runner')
    } else {
      showToast(resp.message || '启动失败', 'error')
    }
  } catch (e: any) {
    showToast('启动失败: ' + (e.message || ''), 'error')
  }
}

async function handleCopyScene() {
  if (!copyName.value) {
    showToast('请输入新场景名称', 'error')
    return
  }
  try {
    const resp = await createScene({
      name: copyName.value,
      description: scene.value?.description ? `复制自: ${scene.value.name}` : '',
    })
    if (resp.code === 0) {
      showToast('场景已复制')
      showCopyModal.value = false
      router.push(`/scenes/${resp.data.id}`)
    } else {
      showToast(resp.message || '复制失败', 'error')
    }
  } catch (e: any) {
    showToast('复制失败', 'error')
  }
}

function formatTime(t: string) {
  if (!t) return '-'
  return new Date(t).toLocaleString()
}

onMounted(() => {
  fetchScene()
  fetchNodes()
  fetchEdges()
})
</script>

<style scoped>
.scene-detail { display: flex; flex-direction: column; gap: 16px; }

.page-header { display: flex; align-items: center; gap: 12px; }
.page-header h2 { font-size: 18px; font-weight: 600; flex: 1; }
.header-actions { display: flex; gap: 8px; }

.btn-back { padding: 6px 12px; border: 1px solid var(--border-primary); border-radius: var(--radius-sm); background: transparent; color: var(--text-secondary); font-size: 13px; cursor: pointer; }
.btn-primary { padding: 8px 16px; border: none; border-radius: var(--radius-md); background: var(--accent-primary); color: #fff; font-size: 13px; cursor: pointer; }
.btn-primary:disabled { opacity: 0.4; cursor: not-allowed; }
.btn-secondary { padding: 8px 16px; border: 1px solid var(--border-primary); border-radius: var(--radius-md); background: transparent; color: var(--text-secondary); font-size: 13px; cursor: pointer; }
.btn-outline { padding: 8px 16px; border: 1px solid var(--accent-primary); border-radius: var(--radius-md); background: transparent; color: var(--accent-primary); font-size: 13px; cursor: pointer; }
.btn-sm { padding: 4px 10px; border: 1px solid var(--border-primary); border-radius: var(--radius-sm); background: transparent; color: var(--text-secondary); font-size: 12px; cursor: pointer; }
.btn-sm:hover { border-color: var(--accent-primary); color: var(--accent-primary); }
.btn-close { border: none; background: transparent; color: var(--text-tertiary); font-size: 16px; cursor: pointer; padding: 4px; }

.scene-info { background: var(--bg-card); border: 1px solid var(--border-secondary); border-radius: var(--radius-md); padding: 16px; }
.info-grid { display: flex; gap: 24px; flex-wrap: wrap; }
.info-item { display: flex; align-items: center; gap: 8px; }
.info-item .label { font-size: 12px; color: var(--text-tertiary); }
.info-item .value { font-size: 13px; color: var(--text-primary); }

.status-badge { font-size: 11px; padding: 2px 8px; border-radius: 10px; }
.status-badge.draft { background: rgba(139,148,158,0.15); color: var(--text-secondary); }
.status-badge.ready { background: rgba(63,185,80,0.15); color: var(--accent-success); }
.status-badge.running { background: rgba(88,166,255,0.15); color: var(--accent-primary); }
.status-badge.completed { background: rgba(63,185,80,0.15); color: var(--accent-success); }

.dag-section { background: var(--bg-card); border: 1px solid var(--border-secondary); border-radius: var(--radius-md); overflow: hidden; }
.section-header { display: flex; justify-content: space-between; align-items: center; padding: 12px 16px; border-bottom: 1px solid var(--border-secondary); }
.section-header h3 { font-size: 14px; font-weight: 600; }
.dag-actions { display: flex; gap: 6px; }

.dag-canvas { padding: 20px; min-height: 200px; }
.dag-empty { text-align: center; padding: 40px 0; color: var(--text-tertiary); }
.empty-icon { font-size: 36px; margin-bottom: 8px; opacity: 0.3; }

.dag-flow { display: flex; flex-direction: column; align-items: center; gap: 0; }

.dag-node {
  display: flex; align-items: center; gap: 10px;
  padding: 10px 16px; border: 1px solid var(--border-primary); border-radius: var(--radius-md);
  background: var(--bg-tertiary); cursor: pointer; min-width: 300px;
  transition: all 0.15s ease;
}
.dag-node:hover { border-color: var(--accent-primary); }
.dag-node.active { border-color: var(--accent-primary); box-shadow: 0 0 0 2px rgba(88,166,255,0.2); }
.dag-node.setup { border-left: 3px solid #2ecc71; }
.dag-node.http { border-left: 3px solid var(--accent-primary); }
.dag-node.delay { border-left: 3px solid #f0ad4e; }
.dag-node.condition { border-left: 3px solid #9b59b6; }
.dag-node.teardown { border-left: 3px solid #e74c3c; }

.node-icon { font-size: 14px; width: 28px; height: 28px; display: flex; align-items: center; justify-content: center; border-radius: 6px; }
.node-icon.setup { background: rgba(46,204,113,0.15); color: #2ecc71; }
.node-icon.http { background: rgba(88,166,255,0.1); color: var(--accent-primary); }
.node-icon.delay { background: rgba(240,173,78,0.15); color: #f0ad4e; }
.node-icon.condition { background: rgba(155,89,182,0.15); color: #9b59b6; }
.node-icon.teardown { background: rgba(231,76,60,0.15); color: #e74c3c; }

.node-info { flex: 1; display: flex; align-items: center; gap: 8px; }
.node-name { font-size: 13px; font-weight: 500; color: var(--text-primary); }
.node-type-badge { font-size: 10px; padding: 1px 6px; border-radius: 8px; background: rgba(139,148,158,0.15); color: var(--text-tertiary); letter-spacing: 0.5px; }
.node-actions { display: flex; gap: 4px; }
.node-btn { border: none; background: transparent; color: var(--text-tertiary); font-size: 12px; cursor: pointer; padding: 2px 6px; border-radius: 4px; }
.node-btn:hover { background: rgba(255,255,255,0.1); color: var(--text-primary); }
.node-btn.danger:hover { color: var(--accent-danger); }

.dag-edge { display: flex; flex-direction: column; align-items: center; position: relative; padding: 4px 0; }
.edge-line { width: 2px; height: 12px; background: var(--border-primary); }
.edge-condition { font-size: 10px; padding: 2px 8px; border-radius: 8px; background: rgba(155,89,182,0.15); color: #9b59b6; margin: 2px 0; }
.edge-arrow { color: var(--text-tertiary); font-size: 10px; line-height: 1; }
.edge-delete { position: absolute; right: -20px; top: 50%; transform: translateY(-50%); border: none; background: transparent; color: var(--text-tertiary); font-size: 10px; cursor: pointer; opacity: 0; transition: opacity 0.15s; }
.dag-edge:hover .edge-delete { opacity: 1; }
.edge-delete:hover { color: var(--accent-danger); }

.node-config-panel {
  background: var(--bg-card); border: 1px solid var(--border-secondary); border-radius: var(--radius-md);
  padding: 16px;
}
.panel-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.panel-header h4 { font-size: 14px; font-weight: 600; }

.config-form { display: flex; flex-direction: column; gap: 12px; }
.form-row { display: flex; flex-direction: column; gap: 4px; }
.form-row.inline { flex-direction: row; align-items: center; gap: 8px; }
.form-row.inline label { width: 100px; flex-shrink: 0; }
.form-row label { font-size: 12px; color: var(--text-secondary); }
.form-row input, .form-row select, .form-row textarea {
  padding: 6px 10px; border: 1px solid var(--border-primary); border-radius: var(--radius-sm);
  background: var(--bg-input); color: var(--text-primary); font-size: 13px; outline: none;
  font-family: inherit;
}
.form-row input:focus, .form-row select:focus, .form-row textarea:focus { border-color: var(--accent-primary); }
.form-row textarea { resize: vertical; min-height: 60px; font-family: 'SF Mono', Monaco, monospace; font-size: 12px; }

.run-warning { background: rgba(240,173,78,0.1); border: 1px solid rgba(240,173,78,0.3); border-radius: var(--radius-sm); padding: 10px 14px; font-size: 13px; color: #f0ad4e; margin-bottom: 12px; }

.modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 100; }
.modal { background: var(--bg-card); border: 1px solid var(--border-primary); border-radius: var(--radius-lg); padding: 24px; width: 480px; max-height: 80vh; overflow-y: auto; }
.modal h3 { font-size: 16px; margin-bottom: 16px; }
.form-group { margin-bottom: 14px; }
.form-group label { display: block; font-size: 13px; color: var(--text-secondary); margin-bottom: 4px; }
.form-group input, .form-group select, .form-group textarea {
  width: 100%; padding: 0 10px; height: 36px; border: 1px solid var(--border-primary); border-radius: var(--radius-sm);
  background: var(--bg-input); color: var(--text-primary); font-size: 13px; outline: none; font-family: inherit;
}
.form-group textarea { height: auto; min-height: 60px; padding: 8px 10px; resize: vertical; font-family: 'SF Mono', Monaco, monospace; font-size: 12px; }
.form-group input:focus, .form-group select:focus, .form-group textarea:focus { border-color: var(--accent-primary); }
.modal-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 20px; }

.toast {
  position: fixed; bottom: 24px; right: 24px; padding: 10px 20px;
  border-radius: var(--radius-md); font-size: 13px; z-index: 200;
  animation: slideIn 0.3s ease;
}
.toast.info { background: var(--accent-primary); color: #fff; }
.toast.error { background: var(--accent-danger, #e74c3c); color: #fff; }
@keyframes slideIn { from { transform: translateY(20px); opacity: 0; } to { transform: translateY(0); opacity: 1; } }
</style>
