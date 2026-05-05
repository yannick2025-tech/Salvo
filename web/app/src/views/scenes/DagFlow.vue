<template>
  <div class="dag-flow-wrapper">
    <VueFlow
      v-model:nodes="vfNodes"
      v-model:edges="vfEdges"
      :node-types="nodeTypes"
      :edge-types="edgeTypes"
      :default-edge-options="defaultEdgeOptions"
      :connection-line-style="{ stroke: 'var(--accent-primary)', strokeWidth: 2 }"
      :snap-to-grid="true"
      :snap-grid="[16, 16]"
      fit-view-on-init
      :fit-view-options="{ padding: 0.3 }"
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
      <Controls position="bottom-left" />
      <MiniMap :node-stroke-color="minimapNodeStroke as any" :node-color="minimapNodeColor as any" pannable zoomable />

      <template #node-scene-node="props">
        <SceneNode v-bind="props" @edit="$emit('edit', $event)" @delete="$emit('deleteNode', $event)" />
      </template>

      <template #edge-custom="props">
        <CustomEdge v-bind="props" />
      </template>
    </VueFlow>

    <div v-if="showEdgeMenu && selectedEdgeId" class="edge-context-menu" :style="{ top: edgeMenuPos.y + 'px', left: edgeMenuPos.x + 'px' }">
      <button @click="deleteSelectedEdge">删除连线</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, markRaw, watch } from 'vue'
import { VueFlow } from '@vue-flow/core'
import { Background } from '@vue-flow/background'
import { Controls } from '@vue-flow/controls'
import { MiniMap } from '@vue-flow/minimap'
import type { Node, Edge, Connection, EdgeChange, NodeChange } from '@vue-flow/core'
import SceneNode from './DagSceneNode.vue'
import CustomEdge from './DagCustomEdge.vue'
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

const edgeTypes: Record<string, any> = {
  'custom': markRaw(CustomEdge),
}

const defaultEdgeOptions = computed(() => ({
  type: 'custom',
  style: { stroke: 'var(--text-tertiary)', strokeWidth: 2 },
  markerEnd: undefined,
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

const vfNodes = ref<Node[]>([])
const vfEdges = ref<Edge[]>([])

watch([() => props.nodes, () => props.edges], ([newNodes, newEdges]) => {
  vfNodes.value = newNodes.map((n, i) => {
    const pos = parsePosition(n.position)
    return {
      id: n.id,
      type: 'scene-node',
      position: pos.x !== 0 || pos.y !== 0 ? pos : { x: 50 + (i % 3) * 320, y: 50 + Math.floor(i / 3) * 120 },
      data: {
        label: n.name,
        nodeType: n.type,
        icon: getNodeIcon(n.type),
        typeLabel: getNodeTypeLabel(n.type),
        originalNode: n,
      },
    }
  })

  vfEdges.value = newEdges.map(e => ({
    id: e.id,
    source: e.from_node,
    target: e.to_node,
    type: 'default',
    data: {
      condition: e.condition,
      edgeId: e.id,
    },
    label: (e.condition === '__if_true__' ? 'TRUE' : e.condition === '__if_false__' ? 'FALSE' : '') || undefined,
    animated: false,
  }))
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
  overflow: hidden;
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
</style>
