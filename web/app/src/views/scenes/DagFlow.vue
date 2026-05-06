<template>
  <div class="dag-flow-wrapper">
    <VueFlow
      v-model:nodes="vfNodes"
      v-model:edges="vfEdges"
      :node-types="nodeTypes"
      :default-edge-options="defaultEdgeOptions"
      :connection-line-style="{ stroke: 'var(--accent-primary)', strokeWidth: 2 }"
      :snap-to-grid="true"
      :snap-grid="[16, 16]"
      fit-view-on-init
      :fit-view-options="{ padding: 0.25 }"
      :min-zoom="0.1"
      :max-zoom="2"
      :nodes-draggable="true"
      :nodes-connectable="true"
      :elements-selectable="true"
      class="vue-flow-canvas"
      ref="vueFlowRef"
      @node-click="onNodeClick"
      @connect="onConnect"
      @edge-click="onEdgeClick"
      @nodes-change="onNodesChange"
      @pane-click="onPaneClick"
    >
      <Background :gap="20" :size="1" pattern-color="var(--border-primary)" />
      <Controls position="bottom-left" :show-interactive="false" />
      <MiniMap :node-stroke-color="minimapNodeStroke as any" :node-color="minimapNodeColor as any" pannable zoomable />

      <template #node-scene-node="props">
        <SceneNode v-bind="props" @edit="$emit('edit', $event)" @delete="$emit('deleteNode', $event)" />
      </template>
    </VueFlow>

    <div class="dag-toolbar">
      <button class="toolbar-btn" title="自动美化布局" @click="autoLayout">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/></svg>
        美化布局
      </button>
      <div class="toolbar-divider"></div>
      <button class="toolbar-btn" title="复制YAML配置" @click="copyYaml">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/></svg>
        复制YAML
      </button>
      <button class="toolbar-btn" title="导出YAML文件" @click="exportYaml">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4"/><polyline points="7,10 12,15 17,10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
        导出YAML
      </button>
    </div>

    <div v-if="toastVisible" class="dag-toast" :class="toastType">{{ toastMsg }}</div>

    <div v-if="showEdgeMenu && selectedEdgeId" class="edge-context-menu" :style="{ top: edgeMenuPos.y + 'px', left: edgeMenuPos.x + 'px' }">
      <button @click="deleteSelectedEdge">删除连线</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, markRaw, watch } from 'vue'
import { VueFlow, MarkerType, useVueFlow } from '@vue-flow/core'
import { Background } from '@vue-flow/background'
import { Controls } from '@vue-flow/controls'
import { MiniMap } from '@vue-flow/minimap'
import type { Node, Edge, Connection, NodeChange } from '@vue-flow/core'
import SceneNode from './DagSceneNode.vue'
import type { NodeDTO, EdgeDTO } from '@/types'
import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'
import '@vue-flow/controls/dist/style.css'
import '@vue-flow/minimap/dist/style.css'

const props = defineProps<{
  nodes: NodeDTO[]
  edges: EdgeDTO[]
}>()

const emit = defineEmits<{
  edit: [node: NodeDTO]
  deleteNode: [id: string]
  addEdge: [from: string, to: string, condition?: string]
  deleteEdge: [id: string]
  nodeSelect: [node: NodeDTO | null]
  nodePositionUpdate: [id: string, x: number, y: number]
}>()

const nodeTypes: Record<string, any> = {
  'scene-node': markRaw(SceneNode),
}

const { fitView } = useVueFlow()

const defaultEdgeOptions = computed(() => ({
  type: 'smoothstep',
  style: { stroke: 'var(--text-tertiary)', strokeWidth: 2 },
  markerEnd: MarkerType.ArrowClosed,
}))

function getNodeTypeLabel(type: string) {
  const map: Record<string, string> = { setup: 'SETUP', http: 'HTTP', delay: 'DELAY', condition: 'COND', 'if-else': 'IF-ELSE', teardown: 'TEARDOWN' }
  return map[type] || type.toUpperCase()
}

function getNodeIcon(type: string) {
  const map: Record<string, string> = { setup: '▶', http: '⇄', delay: '⏱', condition: '◇', 'if-else': 'Y', teardown: '■' }
  return map[type] || '?'
}

function buildLayout(newNodes: NodeDTO[], newEdges: EdgeDTO[]): Map<string, { x: number; y: number }> {
  const positions = new Map<string, { x: number; y: number }>()
  if (newNodes.length === 0) return positions

  const nodeIds = new Set(newNodes.map(n => n.id))
  const fwdChildren = new Map<string, Set<string>>()
  const fwdParents = new Map<string, Set<string>>()
  for (const n of newNodes) {
    fwdChildren.set(n.id, new Set())
    fwdParents.set(n.id, new Set())
  }

  for (const e of newEdges) {
    if (nodeIds.has(e.from_node) && nodeIds.has(e.to_node)) {
      fwdChildren.get(e.from_node)!.add(e.to_node)
      fwdParents.get(e.to_node)!.add(e.from_node)
    }
  }

  const backEdges = new Set<string>()
  const allVisited = new Set<string>()

  for (const startNode of newNodes) {
    if (allVisited.has(startNode.id)) continue
    const stack: Array<{ id: string; childrenIter: IterableIterator<string> }> = []
    const inStack = new Set<string>()
    let current: { id: string; childrenIter: IterableIterator<string> } | undefined = { id: startNode.id, childrenIter: fwdChildren.get(startNode.id)! [Symbol.iterator]() }

    while (current) {
      const u = current.id
      allVisited.add(u)
      inStack.add(u)

      let foundUnvisited = false
      for (const v of current.childrenIter) {
        if (!allVisited.has(v)) {
          foundUnvisited = true
          stack.push(current!)
          current = { id: v, childrenIter: fwdChildren.get(v)! [Symbol.iterator]() }
          break
        } else if (inStack.has(v)) {
          backEdges.add(`${u}->${v}`)
        }
      }

      if (!foundUnvisited) {
        inStack.delete(u)
        current = stack.pop()
      }
    }
  }

  const children = new Map<string, Set<string>>()
  const parents = new Map<string, Set<string>>()
  for (const n of newNodes) {
    children.set(n.id, new Set(fwdChildren.get(n.id)!))
    parents.set(n.id, new Set(fwdParents.get(n.id)!))
  }

  for (const be of backEdges) {
    const [from, to] = be.split('->')
    if (from && to) {
      children.get(from)!.delete(to)
      parents.get(to)!.delete(from)
    }
  }

  const levels = new Map<string, number>()
  const inDegree = new Map<string, number>()
  for (const n of newNodes) {
    inDegree.set(n.id, parents.get(n.id)!.size)
  }

  let currentLevel = 0
  const queue: string[] = []
  for (const n of newNodes) {
    if ((inDegree.get(n.id) || 0) === 0) {
      queue.push(n.id)
      levels.set(n.id, 0)
    }
  }

  while (queue.length > 0) {
    const nextQueue: string[] = []
    for (const nid of queue) {
      for (const childId of children.get(nid) || []) {
        const deg = (inDegree.get(childId) || 0) - 1
        inDegree.set(childId, deg)
        if (!levels.has(childId)) {
          levels.set(childId, currentLevel + 1)
        }
        if (deg === 0 && !nextQueue.includes(childId)) {
          nextQueue.push(childId)
        }
      }
    }
    currentLevel++
    queue.length = 0
    queue.push(...nextQueue)
  }

  for (const n of newNodes) {
    if (!levels.has(n.id)) {
      levels.set(n.id, currentLevel)
    }
  }

  const levelGroups = new Map<number, string[]>()
  for (const n of newNodes) {
    const lv = levels.get(n.id)!
    if (!levelGroups.has(lv)) levelGroups.set(lv, [])
    levelGroups.get(lv)!.push(n.id)
  }

  function reduceCrossings() {
    const sortedLevels = Array.from(levelGroups.keys()).sort((a, b) => a - b)
    for (let iter = 0; iter < 5; iter++) {
      for (let i = 1; i < sortedLevels.length; i++) {
        const prevGroup = levelGroups.get(sortedLevels[i - 1])!
        const currGroup = levelGroups.get(sortedLevels[i])!

        const barycenters = new Map<string, number>()
        for (const nid of currGroup) {
          let sum = 0
          let count = 0
          for (const pid of parents.get(nid) || []) {
            const idx = prevGroup.indexOf(pid)
            if (idx >= 0) { sum += idx; count++ }
          }
          barycenters.set(nid, count > 0 ? sum / count : currGroup.indexOf(nid))
        }
        currGroup.sort((a, b) => (barycenters.get(a) ?? 0) - (barycenters.get(b) ?? 0))
      }
    }
  }

  reduceCrossings()

  const NODE_W = 300
  const NODE_H = 90
  const GAP_X = 50
  const GAP_Y = 50

  const sortedLevels = Array.from(levelGroups.keys()).sort((a, b) => a - b)

  for (const lv of sortedLevels) {
    const group = levelGroups.get(lv)!
    const totalW = group.length * NODE_W + (group.length - 1) * GAP_X
    const startX = -totalW / 2

    group.forEach((nid, col) => {
      positions.set(nid, {
        x: startX + col * (NODE_W + GAP_X),
        y: lv * (NODE_H + GAP_Y),
      })
    })
  }

  return positions
}

const vfNodes = ref<Node[]>([])
const vfEdges = ref<Edge[]>([])

function applyLayout(newNodes: NodeDTO[], newEdges: EdgeDTO[]) {
  const layoutPositions = buildLayout(newNodes, newEdges)

  vfNodes.value = newNodes.map(n => {
    const pos = layoutPositions.get(n.id) || { x: 0, y: 0 }
    return {
      id: n.id,
      type: 'scene-node',
      position: pos,
      data: {
        label: n.name,
        nodeType: n.type,
        icon: getNodeIcon(n.type),
        typeLabel: getNodeTypeLabel(n.type),
        originalNode: n,
      },
    } as Node
  })

  vfEdges.value = newEdges.map(e => {
    let markerEnd: string | { type: MarkerType; color: string } = MarkerType.ArrowClosed
    let strokeColor = 'var(--text-tertiary)'
    let edgeType = 'smoothstep'

    if (e.condition === '__if_true__') {
      markerEnd = { type: MarkerType.ArrowClosed, color: '#2ecc71' }
      strokeColor = '#2ecc71'
    } else if (e.condition === '__if_false__') {
      markerEnd = { type: MarkerType.ArrowClosed, color: '#e74c3c' }
      strokeColor = '#e74c3c'
    }

    return {
      id: e.id,
      source: e.from_node,
      target: e.to_node,
      type: edgeType,
      data: {
        condition: e.condition,
        edgeId: e.id,
      },
      label: (e.condition === '__if_true__' ? 'TRUE' : e.condition === '__if_false__' ? 'FALSE' : '') || undefined,
      labelStyle: { fontSize: '10px', fontWeight: 700, fill: strokeColor },
      labelBgStyle: { fill: 'var(--bg-secondary)' },
      labelBgPadding: [4, 8] as [number, number],
      labelBgBorderRadius: 6,
      animated: false,
      style: { stroke: strokeColor, strokeWidth: 2 },
      markerEnd,
    }
  })
}

watch([() => props.nodes, () => props.edges], ([newNodes, newEdges]) => {
  applyLayout(newNodes, newEdges)
}, { immediate: true })

function autoLayout() {
  applyLayout(props.nodes, props.edges)
  setTimeout(() => fitView({ padding: 0.2 }), 50)
  showToast('布局已重新排列', 'success')
}

function generateYaml(): string {
  const lines: string[] = []
  lines.push('# DAG 场景配置导出')
  lines.push('')

  lines.push('nodes:')
  for (const n of props.nodes) {
    let configStr = ''
    try {
      const parsed = JSON.parse(n.config || '{}')
      configStr = JSON.stringify(parsed).replace(/"/g, "'")
    } catch {
      configStr = n.config || '{}'
    }
    lines.push(`  - id: ${n.id}`)
    lines.push(`    name: ${n.name}`)
    lines.push(`    type: ${n.type}`)
    lines.push(`    config: ${configStr}`)
    lines.push(`    loop_count: ${n.loop_count}`)
    lines.push('')
  }

  lines.push('edges:')
  for (const e of props.edges) {
    const cond = e.condition || '(default)'
    lines.push(`  - from: ${e.from_node}`)
    lines.push(`    to: ${e.to_node}`)
    lines.push(`    condition: ${cond}`)
    lines.push('')
  }

  return lines.join('\n')
}

async function copyYaml() {
  const yaml = generateYaml()
  try {
    await navigator.clipboard.writeText(yaml)
    showToast('YAML 已复制到剪贴板', 'success')
  } catch {
    showToast('复制失败，请手动选择文本', 'error')
  }
}

function exportYaml() {
  const yaml = generateYaml()
  const blob = new Blob([yaml], { type: 'text/yaml;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `dag-config-${Date.now()}.yaml`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
  showToast('YAML 文件已导出', 'success')
}

const toastVisible = ref(false)
const toastMsg = ref('')
const toastType = ref<'success' | 'error'>('success')

function showToast(msg: string, type: 'success' | 'error' = 'success') {
  toastMsg.value = msg
  toastType.value = type
  toastVisible.value = true
  setTimeout(() => { toastVisible.value = false }, 2000)
}

let positionSaveTimer: ReturnType<typeof setTimeout> | null = null

function onNodesChange(changes: NodeChange[]) {
  for (const change of changes) {
    if (change.type === 'position' && change.position) {
      if (positionSaveTimer) clearTimeout(positionSaveTimer)
      positionSaveTimer = setTimeout(() => {
        emit('nodePositionUpdate', change.id!, change.position!.x, change.position!.y)
      }, 800)
    }
  }
}

function onNodeClick({ node }: { node: Node }) {
  emit('nodeSelect', (node.data?.originalNode as NodeDTO) || null)
}

function onPaneClick() {
  emit('nodeSelect', null)
}

function onConnect(params: Connection) {
  emit('addEdge', params.source!, params.target!)
}

const showEdgeMenu = ref(false)
const selectedEdgeId = ref('')
const edgeMenuPos = ref({ x: 0, y: 0 })

function onEdgeClick({ event, edge }: any) {
  showEdgeMenu.value = true
  selectedEdgeId.value = edge.id
  edgeMenuPos.value = { x: event.clientX, y: event.clientY }
  setTimeout(() => {
    document.addEventListener('click', closeEdgeMenu, { once: true })
  }, 10)
}

function closeEdgeMenu() {
  showEdgeMenu.value = false
  selectedEdgeId.value = ''
}

function deleteSelectedEdge() {
  emit('deleteEdge', selectedEdgeId.value)
  closeEdgeMenu()
}

function minimapNodeStroke(color: any) {
  return color === '#1e1f31' ? 'var(--accent-primary)' : color
}

function minimapNodeColor(node: any) {
  const t = (node?.data?.nodeType as string) || ''
  if (t === 'setup') return 'var(--accent-success)'
  if (t === 'http') return 'var(--accent-primary)'
  if (t === 'delay') return 'var(--accent-warning)'
  if (t === 'if-else') return '#e67e22'
  if (t === 'teardown') return 'var(--accent-danger)'
  return 'var(--text-tertiary)'
}
</script>

<style scoped>
.dag-flow-wrapper {
  width: 100%;
  height: 100%;
  min-height: 420px;
  position: relative;
  border-radius: var(--radius-md);
  overflow: visible;
  padding-bottom: 60px;
}

.vue-flow-canvas {
  width: 100%;
  height: 100%;
  background: var(--bg-secondary);
}

.dag-toolbar {
  position: absolute;
  top: 10px;
  right: 10px;
  z-index: 200;
  display: flex;
  align-items: center;
  gap: 4px;
  background: var(--bg-card);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-md);
  padding: 4px;
}

.toolbar-btn {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 10px;
  border: none;
  background: transparent;
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
  border-radius: var(--radius-sm);
  white-space: nowrap;
  transition: all 0.15s ease;
}

.toolbar-btn:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.toolbar-btn:active {
  transform: scale(0.96);
}

.toolbar-divider {
  width: 1px;
  height: 20px;
  background: var(--border-primary);
  margin: 0 2px;
}

.dag-toast {
  position: absolute;
  top: 48px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 300;
  padding: 8px 18px;
  border-radius: var(--radius-md);
  font-size: 13px;
  font-weight: 500;
  pointer-events: none;
  animation: toastIn 0.2s ease-out;
  white-space: nowrap;
}

.dag-toast.success {
  background: rgba(46, 204, 113, 0.92);
  color: #fff;
}

.dag-toast.error {
  background: rgba(231, 76, 60, 0.92);
  color: #fff;
}

@keyframes toastIn {
  from { opacity: 0; transform: translateX(-50%) translateY(-8px); }
  to { opacity: 1; transform: translateX(-50%) translateY(0); }
}

.edge-context-menu {
  position: fixed;
  z-index: 500;
  background: var(--bg-card);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-lg);
  padding: 6px;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.edge-context-menu button {
  padding: 6px 14px;
  border: none;
  background: transparent;
  color: var(--text-primary);
  font-size: 12px;
  cursor: pointer;
  border-radius: var(--radius-sm);
  text-align: left;
  white-space: nowrap;
}

.edge-context-menu button:hover {
  background: var(--bg-hover);
  color: var(--accent-danger);
}
</style>

<style>
.vue-flow__background {
  opacity: 0.35;
}

.vue-flow__controls {
  background: var(--bg-card) !important;
  border: 1px solid var(--border-primary) !important;
  border-radius: var(--radius-md) !important;
  box-shadow: var(--shadow-md) !important;
  overflow: hidden;
  bottom: 12px !important;
  left: 12px !important;
}

.vue-flow__controls-button {
  background: var(--bg-tertiary) !important;
  border-bottom: 1px solid var(--border-primary) !important;
  fill: var(--text-secondary) !important;
}

.vue-flow__controls-button:hover {
  background: var(--bg-hover) !important;
  fill: var(--text-primary) !important;
}

.vue-flow__minimap {
  background: var(--bg-card) !important;
  border: 1px solid var(--border-primary) !important;
  border-radius: var(--radius-md) !important;
  box-shadow: var(--shadow-md) !important;
}

.vue-flow__edge-path {
  stroke-width: 2;
}

.vue-flow__handle {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--accent-primary);
  border: 2px solid var(--bg-card);
  transition: all 0.15s ease;
}

.vue-flow__handle:hover {
  transform: scale(1.4);
  background: var(--accent-info);
}

.vue-flow__connection-line {
  stroke: var(--accent-primary);
  stroke-width: 2;
  stroke-dasharray: 5;
}

.vue-flow__edge-textbg {
  rx: 6;
  ry: 6;
  fill-opacity: 0.85;
}

.vue-flow__edge-textwrapper .vue-flow__edge-text {
  font-size: 10px;
  font-weight: 700;
}
</style>
