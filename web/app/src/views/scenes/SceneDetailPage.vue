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

    <div class="workspace-split">
      <div class="dag-section">
        <div class="section-header">
          <h3>DAG 请求流</h3>
          <div class="dag-actions">
            <button class="btn-sm" @click="addNode('setup')">+ 初始化</button>
            <button class="btn-sm" @click="addNode('http')">+ HTTP</button>
            <button class="btn-sm" @click="addNode('delay')">+ 延迟</button>
            <button class="btn-sm" @click="addNode('condition')">+ 条件</button>
            <button class="btn-sm" @click="addNode('if-else')">+ IF-ELSE</button>
            <button class="btn-sm" @click="addNode('teardown')">+ 清理</button>
          </div>
        </div>

        <div class="dag-canvas" ref="canvasRef">
          <div v-if="nodes.length === 0" class="dag-empty">
            <div class="empty-icon">⬡</div>
            <p>暂无请求节点，点击上方按钮添加</p>
          </div>

          <DagFlow
            v-else
            :nodes="nodes"
            :edges="edges"
            @edit="editNode"
            @delete-node="handleDeleteNode"
            @add-edge="onDagAddEdge"
            @delete-edge="handleDeleteEdge"
            @update-edge="handleUpdateEdge"
            @node-select="selectNode"
            @node-position-update="saveNodePosition"
          />
      </div>
    </div>

    <div v-if="selectedNode" class="config-sidebar">
      <div class="node-config-panel">
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
          <label>参数生成器</label>
          <div class="generator-selector">
            <div class="generator-dropdown-row">
              <select v-model="selectedGeneratorCategory" class="gen-category-select" @change="selectedGeneratorName = ''">
                <option value="">选择分类</option>
                <option v-for="cat in generatorCategories" :key="cat.key" :value="cat.key">{{ cat.label }}</option>
              </select>
              <select v-model="selectedGeneratorName" class="gen-name-select" @change="applyGenerator">
                <option value="">选择函数</option>
                <option v-for="gen in filteredGenerators" :key="gen.name" :value="gen.name">{{ gen.label }} - {{ gen.description }}</option>
              </select>
              <button class="btn-sm" @click="insertGeneratorToBody" :disabled="!selectedGeneratorName">插入</button>
            </div>
            <textarea v-model="httpConfig.generator" rows="3" placeholder='{"email":{"type":"string","format":"email"},"age":{"type":"integer","minimum":18,"maximum":65}}' @change="saveNodeConfig"></textarea>
          </div>
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

      <div v-else-if="selectedNode.type === 'if-else'" class="config-form">
        <div class="form-row">
          <label>节点名称</label>
          <input v-model="editingConfig.name" @change="saveNodeConfig" />
        </div>
        <div class="form-row">
          <label>条件表达式</label>
          <input v-model="ifElseConfig.expr" placeholder='${order_id} != ""' @change="saveNodeConfig" />
        </div>
        <div class="form-row">
          <label>IF 分支目标 (true)</label>
          <select v-model="ifElseConfig.trueTarget" @change="saveIfElseBranches">
            <option value="">选择 IF 分支目标节点</option>
            <option v-for="n in availableIfElseTargets" :key="n.id" :value="n.id">{{ n.name }} ({{ nodeTypeLabel(n.type) }})</option>
          </select>
        </div>
        <div class="form-row">
          <label>ELSE 分支目标 (false)</label>
          <select v-model="ifElseConfig.falseTarget" @change="saveIfElseBranches">
            <option value="">选择 ELSE 分支目标节点</option>
            <option v-for="n in availableIfElseTargets" :key="n.id" :value="n.id">{{ n.name }} ({{ nodeTypeLabel(n.type) }})</option>
          </select>
        </div>
        <div class="form-row">
          <label class="hint-label">表达式求值为 true 时走 IF 分支，false 时走 ELSE 分支。</label>
        </div>
      </div>

      <div class="panel-footer">
        <button class="btn-primary btn-save-panel" @click="saveNodeConfig" :disabled="!selectedNode">保存配置</button>
      </div>
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
          <input v-model.number="runConfig.workers" type="number" min="1" max="1000" step="1" @input="normalizeNumber($event, 'workers')" />
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
          <input v-model.number="runConfig.count" type="number" min="1" max="1000000" step="1" @input="normalizeNumber($event, 'count')" />
        </div>
        <div v-if="runConfig.run_mode === 'duration'" class="form-group">
          <label>持续时间(秒)</label>
          <input v-model.number="runConfig.duration" type="number" min="1" max="3600" step="1" @input="normalizeNumber($event, 'duration')" />
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
            <option value="if-else">IF-ELSE 分支</option>
            <option value="teardown">Teardown (清理)</option>
          </select>
        </div>
        <div v-if="!editingNode && nodes.length > 0" class="form-group">
          <label>连接到（父节点）</label>
          <select v-model="nodeForm.parentId">
            <option value="">不连接（作为起始节点）</option>
            <option v-for="n in nodes" :key="n.id" :value="n.id">{{ n.name }} ({{ nodeTypeLabel(n.type) }})</option>
          </select>
          <span class="field-hint">选择新节点的上游父节点，留空则作为DAG起始节点</span>
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
        <template v-if="nodeForm.type === 'if-else'">
          <div class="form-group">
            <label>条件表达式</label>
            <input v-model="nodeForm.conditionExpr" placeholder='${order_id} != ""' />
          </div>
        </template>
        <div class="modal-actions">
          <button class="btn-secondary" @click="closeNodeEditor">取消</button>
          <button class="btn-primary" @click="handleSaveNode">{{ editingNode ? '保存' : '添加' }}</button>
        </div>
      </div>
    </div>

    <div v-if="toastMsg" class="toast" :class="toastType">{{ toastMsg }}</div>

    <div v-if="showConfirm" class="modal-overlay" @click.self="showConfirm = false">
      <div class="confirm-dialog">
        <div class="confirm-icon">
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>
        </div>
        <h3 class="confirm-title">确认删除</h3>
        <p class="confirm-msg">{{ confirmMessage }}</p>
        <div class="confirm-actions">
          <button class="btn-cancel" @click="showConfirm = false">取消</button>
          <button class="btn-danger-confirm" @click="confirmDeleteNode">确认删除</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getScene, createScene, startScene } from '@/api/scene'
import { listNodes, addNode as apiAddNode, updateNode as apiUpdateNode, deleteNode as apiDeleteNode, listEdges, addEdge, deleteEdge } from '@/api/node'
import { listGenerators } from '@/api/generator'
import type { SceneDTO, NodeDTO, EdgeDTO, GeneratorCategoryInfo, GeneratorInfo } from '@/types'
import DagFlow from './DagFlow.vue'

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
const showConfirm = ref(false)
const confirmMessage = ref('')
const pendingDeleteNodeId = ref('')

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
  parentId: '',
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
const ifElseConfig = reactive({ expr: '', trueTarget: '', falseTarget: '' })

const generatorCategories = ref<GeneratorCategoryInfo[]>([])
const selectedGeneratorCategory = ref('')
const selectedGeneratorName = ref('')

const filteredGenerators = computed<GeneratorInfo[]>(() => {
  if (!selectedGeneratorCategory.value) return []
  const cat = generatorCategories.value.find(c => c.key === selectedGeneratorCategory.value)
  return cat?.generators || []
})

const availableIfElseTargets = computed(() => {
  if (!selectedNode.value) return []
  return nodes.value.filter(n => n.id !== selectedNode.value?.id)
})

async function saveIfElseBranches() {
  if (!selectedNode.value) return
  const sceneId = route.params.id as string
  const nodeId = selectedNode.value.id
  
  const existingEdges = edges.value.filter(e => e.from_node === nodeId)
  for (const e of existingEdges) {
    await deleteEdge(e.id)
  }
  
  if (ifElseConfig.trueTarget) {
    await addEdge({ scene_id: sceneId, from_node: nodeId, to_node: ifElseConfig.trueTarget, condition: '__if_true__' })
  }
  if (ifElseConfig.falseTarget) {
    await addEdge({ scene_id: sceneId, from_node: nodeId, to_node: ifElseConfig.falseTarget, condition: '__if_false__' })
  }
  
  fetchEdges()
  showToast('分支配置已保存')
}

interface DagTreeNode {
  node: NodeDTO
  level: number
  children: { edge: EdgeDTO; child: DagTreeNode }[]
}

const dagTree = computed((): DagTreeNode[] => {
  if (nodes.value.length === 0) return []

  const nodeMap = new Map<string, NodeDTO>()
  for (const n of nodes.value) nodeMap.set(n.id, n)

  const outEdgesMap = new Map<string, EdgeDTO[]>()
  for (const e of edges.value) {
    outEdgesMap.set(e.from_node, [...(outEdgesMap.get(e.from_node) || []), e])
  }

  const rootNodes = nodes.value.filter(n => !edges.value.some(e => e.to_node === n.id))
  const visited = new Set<string>()

  function buildTree(nodeId: string, level: number): DagTreeNode | null {
    if (visited.has(nodeId)) return null
    visited.add(nodeId)
    
    const node = nodeMap.get(nodeId)
    if (!node) return null
    
    const childrenOut = outEdgesMap.get(nodeId) || []
    const children: { edge: EdgeDTO; child: DagTreeNode }[] = []
    
    for (const e of childrenOut) {
      const childTree = buildTree(e.to_node, level + 1)
      if (childTree) {
        children.push({ edge: e, child: childTree })
      }
    }

    return { node, level, children }
  }

  const result: DagTreeNode[] = []
  for (const root of rootNodes) {
    const tree = buildTree(root.id, 0)
    if (tree) result.push(tree)
  }

  for (const n of nodes.value) {
    if (!visited.has(n.id)) {
      result.push({ node: n, level: 0, children: [] })
    }
  }

  return result
})

function flattenDagLevels(): { nodes: NodeDTO[]; isBranch: boolean; branchFrom?: string; branchLabel?: string }[][] {
  const levels: { nodes: NodeDTO[]; isBranch: boolean; branchFrom?: string; branchLabel?: string }[][] = [[]]
  const added = new Set<string>()
  
  function addNodeToLevel(node: NodeDTO, levelIdx: number, branchInfo?: { from: string; label: string }) {
    while (levels.length <= levelIdx) levels.push([])
    if (!added.has(node.id)) {
      added.add(node.id)
      levels[levelIdx].push({
        nodes: [node],
        isBranch: !!branchInfo,
        branchFrom: branchInfo?.from,
        branchLabel: branchInfo?.label,
      })
    }
  }

  function traverse(treeNodes: DagTreeNode[], startLevel: number) {
    for (const tree of treeNodes) {
      addNodeToLevel(tree.node, startLevel)
      
      if (tree.children.length > 1) {
        for (const c of tree.children) {
          const label = c.edge.condition === '__if_true__' ? 'TRUE' : 
                       c.edge.condition === '__if_false__' ? 'FALSE' : 
                       c.edge.condition || ''
          addNodeToLevel(c.child.node, startLevel + 1, { from: tree.node.id, label })
          traverse([c.child], startLevel + 2)
        }
      } else if (tree.children.length === 1) {
        traverse([tree.children[0].child], startLevel + 1)
      }
    }
  }

  traverse(dagTree.value, 0)
  return levels
}

const dagLevels = computed(() => flattenDagLevels())

function getOutEdgesOfNode(nodeId: string): EdgeDTO[] {
  return edges.value.filter(e => e.from_node === nodeId)
}

function getInEdgeOfNode(nodeId: string): EdgeDTO | undefined {
  return edges.value.find(e => e.to_node === nodeId)
}

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
    case 'if-else': return '⑂'
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
    case 'if-else': return 'IF-ELSE'
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

function normalizeNumber(event: Event, field: 'workers' | 'count' | 'duration') {
  const input = event.target as HTMLInputElement
  const value = input.value
  
  // Allow empty input - user may be typing
  if (!value) {
    return
  }
  
  // Allow single zero - user may be typing
  if (value === '0') {
    return
  }
  
  const numValue = parseInt(value, 10)
  if (isNaN(numValue)) {
    // Remove non-numeric characters
    input.value = value.replace(/[^0-9]/g, '')
    return
  }
  
  const limits: Record<string, { min: number; max: number }> = {
    workers: { min: 1, max: 1000 },
    count: { min: 1, max: 1000000 },
    duration: { min: 1, max: 3600 },
  }
  
  const limit = limits[field]
  if (numValue < limit.min) {
    input.value = String(limit.min)
    if (field === 'workers') runConfig.workers = limit.min
    else if (field === 'count') runConfig.count = limit.min
    else if (field === 'duration') runConfig.duration = limit.min
  } else if (numValue > limit.max) {
    input.value = String(limit.max)
    if (field === 'workers') runConfig.workers = limit.max
    else if (field === 'count') runConfig.count = limit.max
    else if (field === 'duration') runConfig.duration = limit.max
  } else {
    // Update the reactive value when valid input
    if (field === 'workers') runConfig.workers = numValue
    else if (field === 'count') runConfig.count = numValue
    else if (field === 'duration') runConfig.duration = numValue
  }
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
  nodeForm.parentId = ''
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

const SQL_INJECTION_PATTERN = /['";]|--|\/\*|\*\//

function validateNodeName(name: string, excludeId?: string): string | null {
  if (!name || !name.trim()) return '节点名称不能为空'
  const trimmed = name.trim()
  if (trimmed.length > 50) return '节点名称不能超过50个字符'
  if (SQL_INJECTION_PATTERN.test(trimmed)) return '节点名称包含非法字符（不允许引号、分号、注释符）'
  const duplicate = nodes.value.find(n => n.name === trimmed && n.id !== excludeId)
  if (duplicate) return `节点名称 "${trimmed}" 已存在，请使用唯一名称`
  return null
}

async function handleSaveNode() {
  const nameError = validateNodeName(nodeForm.name, editingNode.value?.id)
  if (nameError) {
    showToast(nameError, 'error')
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
  } else if (nodeForm.type === 'if-else') {
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
      if (nodeForm.parentId) {
        await addEdge({
          scene_id: sceneId,
          from_node: nodeForm.parentId,
          to_node: newNode.id,
        })
      } else if (nodes.value.length > 0) {
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
  pendingDeleteNodeId.value = id
  confirmMessage.value = '确定要删除该节点吗？'
  showConfirm.value = true
}

async function confirmDeleteNode() {
  const id = pendingDeleteNodeId.value
  showConfirm.value = false
  pendingDeleteNodeId.value = ''
  
  const relatedEdges = edges.value.filter(e => e.from_node === id || e.to_node === id)
  for (const edge of relatedEdges) {
    await deleteEdge(edge.id)
  }
  
  const resp = await apiDeleteNode(id)
  if (resp.code === 0) {
    showToast('节点已删除（关联连线已自动清理）')
    if (selectedNode.value?.id === id) selectedNode.value = null
    fetchNodes()
    fetchEdges()
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

function selectNode(node: NodeDTO | null) {
  if (!node) {
    selectedNode.value = null
    return
  }
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
    ifElseConfig.expr = cfg.expr || ''
    
    if (node.type === 'if-else') {
      const nodeEdges = edges.value.filter(e => e.from_node === node.id)
      const trueEdge = nodeEdges.find(e => e.condition === '__if_true__')
      const falseEdge = nodeEdges.find(e => e.condition === '__if_false__')
      ifElseConfig.trueTarget = trueEdge?.to_node || ''
      ifElseConfig.falseTarget = falseEdge?.to_node || ''
    } else {
      ifElseConfig.trueTarget = ''
      ifElseConfig.falseTarget = ''
    }
  } catch { /* ignore */ }
}

async function onDagAddEdge(from: string, to: string) {
  const sceneId = route.params.id as string
  await addEdge({ scene_id: sceneId, from_node: from, to_node: to })
  fetchEdges()
  showToast('连线已创建')
}

async function handleDeleteEdge(id: string) {
  await deleteEdge(id)
  fetchEdges()
  showToast('连线已删除')
}

async function handleUpdateEdge(edgeId: string, newSource: string, newTarget: string, _newSourceHandle?: string, _newTargetHandle?: string) {
  const edge = edges.value.find(e => e.id === edgeId)
  if (!edge) return
  const sceneId = route.params.id as string
  await deleteEdge(edgeId)
  await addEdge({ scene_id: sceneId, from_node: newSource, to_node: newTarget, condition: edge.condition || undefined })
  const idx = edges.value.findIndex(e => e.id === edgeId)
  if (idx >= 0) {
    edges.value[idx] = { ...edges.value[idx], from_node: newSource, to_node: newTarget }
  }
  showToast('连线已重新连接')
}

async function saveNodePosition(id: string, x: number, y: number) {
  const sceneId = route.params.id as string
  await apiUpdateNode({ id, position: JSON.stringify({ x: Math.round(x), y: Math.round(y) }) })
}

async function saveNodeConfig() {
  if (!selectedNode.value) return

  const newName = editingConfig.name || selectedNode.value.name
  const nameError = validateNodeName(newName, selectedNode.value.id)
  if (nameError) {
    showToast(nameError, 'error')
    return
  }

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
  } else if (nodeType === 'if-else') {
    config = JSON.stringify({ expr: ifElseConfig.expr })
  }

  try {
    const resp = await apiUpdateNode({
      id: selectedNode.value.id,
      name: newName,
      config,
    })
    if (resp.code === 0) {
      showToast('配置已保存')
      fetchNodes()
    } else {
      showToast(resp.message || '保存失败', 'error')
    }
  } catch (e: any) {
    showToast('保存失败: ' + (e.message || '网络错误'), 'error')
  }
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
  const d = new Date(t)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

async function fetchGenerators() {
  try {
    const resp = await listGenerators()
    if (resp.code === 0) {
      generatorCategories.value = resp.data.categories || []
    }
  } catch { /* ignore */ }
}

function applyGenerator() {
  if (!selectedGeneratorCategory.value || !selectedGeneratorName.value) return
  const cat = generatorCategories.value.find(c => c.key === selectedGeneratorCategory.value)
  if (!cat) return
  const gen = cat.generators.find(g => g.name === selectedGeneratorName.value)
  if (!gen) return

  let current: Record<string, any> = {}
  try { current = JSON.parse(httpConfig.generator || '{}') } catch { /* ignore */ }
  current[gen.name] = gen.schema_template
  httpConfig.generator = JSON.stringify(current, null, 2)
  saveNodeConfig()
}

function insertGeneratorToBody() {
  if (!selectedGeneratorCategory.value || !selectedGeneratorName.value) return
  const cat = generatorCategories.value.find(c => c.key === selectedGeneratorCategory.value)
  if (!cat) return
  const gen = cat.generators.find(g => g.name === selectedGeneratorName.value)
  if (!gen) return

  let body: Record<string, any> = {}
  try {
    const parsed = JSON.parse(httpConfig.body || '{}')
    if (typeof parsed === 'object' && parsed !== null && !Array.isArray(parsed)) {
      body = parsed
    }
  } catch { /* ignore */ }

  body[gen.name] = `\${generator.${gen.name}}`
  httpConfig.body = JSON.stringify(body, null, 2)
  saveNodeConfig()
}

onMounted(() => {
  fetchScene()
  fetchNodes()
  fetchEdges()
  fetchGenerators()
})
</script>

<style scoped>
.scene-detail { display: flex; flex-direction: column; gap: 16px; height: calc(100vh - 100px); max-height: calc(100vh - 100px); overflow: hidden; }

.page-header { display: flex; align-items: center; gap: 12px; flex-shrink: 0; }
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

.scene-info { background: var(--bg-card); border: 1px solid var(--border-secondary); border-radius: var(--radius-md); padding: 16px; flex-shrink: 0; }
.info-grid { display: flex; gap: 24px; flex-wrap: wrap; }
.info-item { display: flex; align-items: center; gap: 8px; }
.info-item .label { font-size: 12px; color: var(--text-tertiary); }
.info-item .value { font-size: 13px; color: var(--text-primary); }

.status-badge { font-size: 11px; padding: 2px 8px; border-radius: 10px; }
.status-badge.draft { background: rgba(139,148,158,0.15); color: var(--text-secondary); }
.status-badge.ready { background: rgba(63,185,80,0.15); color: var(--accent-success); }
.status-badge.running { background: rgba(88,166,255,0.15); color: var(--accent-primary); }
.status-badge.completed { background: rgba(63,185,80,0.15); color: var(--accent-success); }

.dag-section { background: var(--bg-card); border: 1px solid var(--border-secondary); border-radius: var(--radius-md); overflow: visible; flex: 1; min-width: 0; display: flex; flex-direction: column; }
.section-header { display: flex; justify-content: space-between; align-items: center; padding: 12px 16px; border-bottom: 1px solid var(--border-secondary); flex-shrink: 0; }
.section-header h3 { font-size: 14px; font-weight: 600; }
.dag-actions { display: flex; gap: 6px; flex-wrap: wrap; }

.dag-canvas { padding: 0; min-height: 480px; flex: 1; position: relative; padding-bottom: 8px; }
.dag-empty { text-align: center; padding: 40px 0; color: var(--text-tertiary); }
.empty-icon { font-size: 36px; margin-bottom: 8px; opacity: 0.3; }

.dag-flow { display: flex; flex-direction: column; align-items: center; gap: 0; }

.dag-branch-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 100%;
  padding: 16px 0;
}

.dag-level {
  display: flex;
  justify-content: center;
  gap: 24px;
  width: 100%;
  position: relative;
  margin-bottom: 8px;
}

.dag-level-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  position: relative;
}

.branch-item .dag-nodes-in-level::before {
  content: '';
  position: absolute;
  top: -20px;
  left: 50%;
  width: 2px;
  height: 16px;
  background: var(--border-primary);
}

.branch-label { margin-bottom: 4px; }
.label-badge {
  font-size: 10px; font-weight: 600; letter-spacing: 0.5px;
  padding: 2px 10px; border-radius: 10px;
}
.true-label { background: rgba(46,204,113,0.15); color: #2ecc71; border: 1px solid rgba(46,204,113,0.3); }
.false-label { background: rgba(231,76,60,0.12); color: #e74c3c; border: 1px solid rgba(231,76,60,0.25); }

.dag-nodes-in-level {
  display: flex;
  gap: 12px;
}

.branch-split-lines {
  display: flex;
  gap: 40px;
  margin-top: 4px;
  position: relative;
}

.split-line-wrapper {
  display: flex;
  flex-direction: column;
  align-items: center;
  min-width: 80px;
}

.split-line {
  display: flex;
  flex-direction: column;
  align-items: center;
  position: relative;
}

.split-condition {
  font-size: 9px; font-weight: 600; letter-spacing: 0.5px;
  padding: 1px 6px; border-radius: 8px; margin-bottom: 2px;
}
.true-line .split-condition { background: rgba(46,204,113,0.15); color: #2ecc71; }
.false-line .split-condition { background: rgba(231,76,60,0.12); color: #e74c3c; }

.split-svg {
  width: 40px; height: 50px; overflow: visible;
}
.true-line .split-svg path { stroke: #2ecc71; stroke-dasharray: none; opacity: 0.7; }
.false-line .split-svg path { stroke: #e74c3c; stroke-dasharray: none; opacity: 0.7; }

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
.dag-node.if-else { border-left: 3px solid #e67e22; }
.dag-node.teardown { border-left: 3px solid #e74c3c; }

.node-icon { font-size: 14px; width: 28px; height: 28px; display: flex; align-items: center; justify-content: center; border-radius: 6px; }
.node-icon.setup { background: rgba(46,204,113,0.15); color: #2ecc71; }
.node-icon.http { background: rgba(88,166,255,0.1); color: var(--accent-primary); }
.node-icon.delay { background: rgba(240,173,78,0.15); color: #f0ad4e; }
.node-icon.condition { background: rgba(155,89,182,0.15); color: #9b59b6; }
.node-icon.if-else { background: rgba(230,126,34,0.15); color: #e67e22; }
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

.workspace-split { display: flex; gap: 16px; flex: 1; min-height: 0; overflow: hidden; }

.config-sidebar {
  width: 420px; max-width: 45%; min-width: 340px;
  flex-shrink: 0;
  overflow-y: auto;
  overflow-x: hidden;
}

.node-config-panel {
  background: var(--bg-card); border: 1px solid var(--border-secondary); border-radius: var(--radius-md);
  padding: 16px; min-width: 0;
}
.panel-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.panel-header h4 { font-size: 14px; font-weight: 600; }

.config-form { display: flex; flex-direction: column; gap: 12px; }
.form-row { display: flex; flex-direction: column; gap: 4px; }
.form-row.inline { flex-direction: row; align-items: center; gap: 8px; }
.form-row.inline label { width: 100px; flex-shrink: 0; }
.form-row label { font-size: 12px; color: var(--text-secondary); }
.form-row input, .form-row select, .form-row textarea {
  width: 100%; box-sizing: border-box;
  padding: 6px 10px; border: 1px solid var(--border-primary); border-radius: var(--radius-sm);
  background: var(--bg-input); color: var(--text-primary); font-size: 13px; outline: none;
  font-family: inherit; word-break: break-all;
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
.confirm-title { font-size: 16px; font-weight: 600; color: var(--text-primary); margin: 0 0 8px; }
.confirm-msg { font-size: 13px; color: var(--text-secondary); margin: 0 0 24px; line-height: 1.5; }
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

.generator-selector { display: flex; flex-direction: column; gap: 8px; }
.generator-dropdown-row { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.gen-category-select, .gen-name-select { flex: 1 1 120px; min-width: 0; padding: 6px 10px; border: 1px solid var(--border-primary); border-radius: var(--radius-sm); background: var(--bg-input); color: var(--text-primary); font-size: 13px; outline: none; box-sizing: border-box; }
.gen-category-select:focus, .gen-name-select:focus { border-color: var(--accent-primary); }
.hint-label { font-size: 11px; color: var(--text-tertiary); line-height: 1.4; }
.field-hint { font-size: 11px; color: var(--text-tertiary); margin-top: 4px; display: block; }
.panel-footer { margin-top: 16px; padding-top: 12px; border-top: 1px solid var(--border-secondary); }
.btn-save-panel { width: 100%; padding: 10px; font-size: 14px; }

@media (max-width: 900px) {
  .workspace-split { flex-direction: column; overflow-y: auto; }
  .config-sidebar { width: 100%; max-width: none; min-width: 0; }
  .dag-section { min-width: 0; }
  .scene-detail { height: auto; max-height: none; overflow: visible; }
}
</style>
