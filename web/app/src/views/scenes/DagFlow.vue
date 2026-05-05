<template>
  <div class="dag-flow-wrapper" ref="wrapperRef">
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
      :min-zoom="0.15"
      :max-zoom="2"
      :nodes-draggable="true"
      :nodes-connectable="true"
      :elements-selectable="true"
      class="vue-flow-canvas"
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

    <div v-if="showEdgeMenu && selectedEdgeId" class="edge-context-menu" :style="{ top: edgeMenuPos.y + 'px', left: edgeMenuPos.x + 'px' }">
      <button @click="deleteSelectedEdge">删除连线</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, markRaw, watch, onMounted, nextTick } from 'vue'
import { VueFlow } from '@vue-flow/core'
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

const wrapperRef = ref<HTMLElement | null>(null)

onMounted(() => {
  nextTick(() => injectArrowMarkers())
})

function injectArrowMarkers() {
  const svg = wrapperRef.value?.querySelector('.vue-flow svg')
  if (!svg) return
  if (svg.querySelector('#vf-arrowhead')) return
  const defs = document.createElementNS('http://www.w3.org/2000/svg', 'defs')

  const colors = [
    { id: 'vf-arrowhead', fill: 'var(--text-tertiary)' },
    { id: 'vf-arrowhead-true', fill: '#2ecc71' },
    { id: 'vf-arrowhead-false', fill: '#e74c3c' },
  ]

  for (const c of colors) {
    const marker = document.createElementNS('http://www.w3.org/2000/svg', 'marker')
    marker.setAttribute('id', c.id)
    marker.setAttribute('viewBox', '0 0 10 10')
    marker.setAttribute('refX', '9')
    marker.setAttribute('refY', '5')
    marker.setAttribute('markerWidth', '7')
    marker.setAttribute('markerHeight', '7')
    marker.setAttribute('orient', 'auto-start-reverse')
    const path = document.createElementNS('http://www.w3.org/2000/svg', 'path')
    path.setAttribute('d', 'M 0 0 L 10 5 L 0 10 z')
    path.setAttribute('fill', c.fill)
    marker.appendChild(path)
    defs.appendChild(marker)
  }

  svg.insertBefore(defs, svg.firstChild)
}

const defaultEdgeOptions = computed(() => ({
  type: 'smoothstep',
  style: { stroke: 'var(--text-tertiary)', strokeWidth: 2 },
  markerEnd: 'url(#arrowhead)',
}))

function parsePosition(posStr: string): { x: number; y: number } {
  try {
    const p = JSON.parse(posStr || '{}')
    return { x: p.x || 0, y: p.y || 0 }
  } catch {
    return { x: 0, y: 0 }
  }
}

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
  const hasCustomPosition = newNodes.some(n => {
    const pos = parsePosition(n.position)
    return pos.x !== 0 || pos.y !== 0
  })
  if (hasCustomPosition) {
    for (const n of newNodes) {
      positions.set(n.id, parsePosition(n.position))
    }
    return positions
  }

  const nodeIds = new Set(newNodes.map(n => n.id))
  const children = new Map<string, Set<string>>()
  const parents = new Map<string, Set<string>>()

  for (const n of newNodes) {
    children.set(n.id, new Set())
    parents.set(n.id, new Set())
  }

  for (const e of newEdges) {
    if (nodeIds.has(e.from_node) && nodeIds.has(e.to_node)) {
      children.get(e.from_node)!.add(e.to_node)
      parents.get(e.to_node)!.add(e.from_node)
    }
  }

  const levels = new Map<string, number>()
  const visited = new Set<string>()

  function assignLevel(nodeId: string): number {
    if (levels.has(nodeId)) return levels.get(nodeId)!
    if (visited.has(nodeId)) {
      levels.set(nodeId, 0)
      return 0
    }
    visited.add(nodeId)

    const parentIds = parents.get(nodeId)!
    if (parentIds.size === 0) {
      levels.set(nodeId, 0)
      return 0
    }

    let maxParentLevel = -1
    for (const pid of parentIds) {
      maxParentLevel = Math.max(maxParentLevel, assignLevel(pid))
    }
    const level = maxParentLevel + 1
    levels.set(nodeId, level)
    return level
  }

  for (const n of newNodes) {
    assignLevel(n.id)
  }

  const levelGroups = new Map<number, string[]>()
  for (const n of newNodes) {
    const lv = levels.get(n.id) ?? 0
    if (!levelGroups.has(lv)) levelGroups.set(lv, [])
    levelGroups.get(lv)!.push(n.id)
  }

  const NODE_W = 300
  const NODE_H = 90
  const GAP_X = 60
  const GAP_Y = 40
  const MAX_COLS = 4

  const sortedLevels = Array.from(levelGroups.keys()).sort((a, b) => a - b)

  for (const lv of sortedLevels) {
    const group = levelGroups.get(lv)!
    group.sort((a, b) => {
      const idxA = newNodes.findIndex(n => n.id === a)
      const idxB = newNodes.findIndex(n => n.id === b)
      return idxA - idxB
    })

    const cols = Math.min(group.length, MAX_COLS)
    const rows = Math.ceil(group.length / cols)
    const gridW = cols * NODE_W + (cols - 1) * GAP_X
    const startX = -gridW / 2

    group.forEach((nid, idx) => {
      const col = idx % cols
      const row = Math.floor(idx / cols)
      positions.set(nid, {
        x: startX + col * (NODE_W + GAP_X),
        y: lv * (NODE_H + GAP_Y) + row * (NODE_H * 0.15),
      })
    })
  }

  return positions
}

const vfNodes = ref<Node[]>([])
const vfEdges = ref<Edge[]>([])

watch([() => props.nodes, () => props.edges], ([newNodes, newEdges]) => {
  const layoutPositions = buildLayout(newNodes, newEdges)

  vfNodes.value = newNodes.map(n => {
    const pos = layoutPositions.get(n.id) || parsePosition(n.position)
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
    }
  })

  vfEdges.value = newEdges.map(e => {
    let markerEndId = 'url(#vf-arrowhead)'
    let strokeColor = 'var(--text-tertiary)'
    if (e.condition === '__if_true__') {
      markerEndId = 'url(#vf-arrowhead-true)'
      strokeColor = '#2ecc71'
    } else if (e.condition === '__if_false__') {
      markerEndId = 'url(#vf-arrowhead-false)'
      strokeColor = '#e74c3c'
    }
    return {
      id: e.id,
      source: e.from_node,
      target: e.to_node,
      type: 'smoothstep',
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
      markerEnd: markerEndId,
    }
  })
}, { immediate: true })

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
}

.vue-flow-canvas {
  width: 100%;
  height: 100%;
  background: var(--bg-secondary);
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
