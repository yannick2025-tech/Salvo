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
      :nodes-draggable="!isRunning"
      :nodes-connectable="!isRunning"
      :elements-selectable="true"
      :edges-updatable="!isRunning"
      class="vue-flow-canvas"
      ref="vueFlowRef"
      @node-click="onNodeClick"
      @connect="onConnect"
      @edge-update="onEdgeUpdate"
      @edge-click="onEdgeClick"
      @nodes-change="onNodesChange"
      @pane-click="onPaneClick"
    >
      <Background :gap="20" :size="1" pattern-color="var(--border-primary)" />
      <Controls position="bottom-left" :show-interactive="false" />
      <MiniMap :node-stroke-color="minimapNodeStroke as any" :node-color="minimapNodeColor as any" pannable zoomable />

      <template #node-scene-node="nodeProps">
        <SceneNode
          v-bind="nodeProps"
          :execution-status="getNodeExecStatus(nodeProps.id)"
          :chain-status="getNodeChainStatus(nodeProps.id)"
          :loop-progress="getNodeLoopProgress(nodeProps.id)"
          :view-mode="execViewMode"
          @edit="$emit('edit', $event)"
          @delete="$emit('deleteNode', $event)"
        />
      </template>
    </VueFlow>

    <!-- Execution view mode toolbar (shown when running) -->
    <div v-if="isRunning" class="exec-toolbar">
      <button :class="['exec-tab', { active: execViewMode === 'aggregate' }]" @click="switchView('aggregate')">聚合视图</button>
      <button :class="['exec-tab', { active: execViewMode === 'chain' }]" @click="switchView('chain')">单链路视图</button>
      <CustomSelect
        v-show="execViewMode === 'chain'"
        v-model="localSelectedChain"
        :options="chainOptions"
        placeholder="选择链路"
        font-size="11px"
        @update:modelValue="onChainSelect"
      />
      <div v-if="wsConnected" class="ws-indicator connected" title="WebSocket 已连接">●</div>
      <div v-else class="ws-indicator disconnected" title="WebSocket 已断开">●</div>
    </div>

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
import ELK from 'elkjs/lib/elk.bundled.js'
import SceneNode from './DagSceneNode.vue'
import { getDagIcon } from './dagIcons'
import type { NodeDTO, EdgeDTO } from '@/types'
import { useExecutionWs } from '@/composables/useExecutionWs'
import { useExecutionStatus, type ViewMode } from '@/composables/useExecutionStatus'
import { getTraceByRun } from '@/api/trace'
import CustomSelect from '@/components/CustomSelect.vue'
import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'
import '@vue-flow/controls/dist/style.css'
import '@vue-flow/minimap/dist/style.css'

const props = defineProps<{
  nodes: NodeDTO[]
  edges: EdgeDTO[]
  dataSources?: { name: string; columns: string[]; rows: Record<string, string>[] }[]
  isRunning?: boolean
  runId?: string
}>()

const emit = defineEmits<{
  edit: [node: NodeDTO]
  deleteNode: [id: string]
  addEdge: [from: string, to: string, condition?: string, sourceHandle?: string, targetHandle?: string]
  updateEdge: [edgeId: string, newSource: string, newTarget: string, newSourceHandle?: string, newTargetHandle?: string]
  deleteEdge: [id: string]
  nodeSelect: [node: NodeDTO | null]
  nodePositionUpdate: [id: string, x: number, y: number]
}>()

const nodeTypes: Record<string, any> = {
  'scene-node': markRaw(SceneNode),
}

const { fitView } = useVueFlow()

// ===== Execution status composables =====
const { spanUpdates, isConnected: wsConnected, connect: wsConnect, disconnect: wsDisconnect } = useExecutionWs()
const {
  aggregateStatus,
  chainStatuses,
  loopProgress,
  viewMode: execViewMode,
  selectedChainId,
  computeAggregateStatus,
  switchView,
  selectChain,
  initFromSpans,
  version: execVersion,
} = useExecutionStatus(spanUpdates)

const chainIds = computed(() => {
  // Depend on version to react to shallowRef changes
  void execVersion.value
  return Array.from(chainStatuses.value.keys())
})

// Map chain_id to a readable label (Chain #1, Chain #2, etc.)
const chainLabelMap = computed(() => {
  void execVersion.value
  const map = new Map<string, string>()
  const ids = Array.from(chainStatuses.value.keys())
  ids.forEach((id, index) => {
    map.set(id, `Chain #${index + 1}`)
  })
  return map
})

// Options for CustomSelect
const chainOptions = computed(() => {
  return debouncedChainIds.value.map(cid => ({
    value: cid,
    label: chainLabelMap.value.get(cid) || cid,
  }))
})

// Debounced chainIds: only update when user is not actively selecting
// or when the list genuinely changes (new chain added)
const debouncedChainIds = ref<string[]>([])
let chainIdsTimer: ReturnType<typeof setTimeout> | null = null

watch(chainIds, (newIds) => {
  // Always update immediately if the list size changed (new chain added)
  if (newIds.length !== debouncedChainIds.value.length) {
    debouncedChainIds.value = newIds
    return
  }
  // Otherwise debounce to avoid frequent re-renders during WS event bursts
  if (chainIdsTimer) clearTimeout(chainIdsTimer)
  chainIdsTimer = setTimeout(() => {
    debouncedChainIds.value = newIds
  }, 300)
}, { immediate: true })

// Local ref for select binding, synced with composable's selectedChainId
const localSelectedChain = ref('')

// Sync: when composable's selectedChainId changes, update local
watch(selectedChainId, (val) => {
  localSelectedChain.value = val || ''
}, { immediate: true })

function onChainSelect(value: string | number) {
  if (value) {
    selectChain(String(value))
  }
}

// Compute node badges for current nodes
const nodeBadges = computed(() => {
  void execVersion.value // depend on version for shallowRef reactivity
  return computeAggregateStatus(props.nodes)
})

function getNodeExecStatus(nodeId: string) {
  if (!props.isRunning) return undefined
  void execVersion.value // depend on version for shallowRef reactivity
  return aggregateStatus.value.get(nodeId) || undefined
}

function getNodeChainStatus(nodeId: string) {
  if (!props.isRunning || execViewMode.value !== 'chain' || !selectedChainId.value) return undefined
  void execVersion.value // depend on version for shallowRef reactivity
  const chainMap = chainStatuses.value.get(selectedChainId.value)
  if (!chainMap) return undefined
  return chainMap.get(nodeId) ?? null
}

function getNodeLoopProgress(nodeId: string) {
  if (!props.isRunning || !selectedChainId.value) return undefined
  void execVersion.value // depend on version for shallowRef reactivity
  const chainLoopMap = loopProgress.value.get(selectedChainId.value)
  if (!chainLoopMap) return undefined
  return chainLoopMap.get(nodeId) || undefined
}

// Connect/disconnect WS based on runId, and load initial trace data
watch(() => props.runId, async (newRunId) => {
  if (newRunId) {
    wsConnect(newRunId)
    // Load existing trace data to initialize status
    try {
      const resp = await getTraceByRun(newRunId)
      if (resp.code === 0 && resp.data?.spans) {
        initFromSpans(resp.data.spans)
      }
    } catch { /* trace may not exist yet */ }
  } else {
    wsDisconnect()
  }
}, { immediate: true })

// Active chain edges for single chain view edge styling
const activeChainEdgeIds = computed(() => {
  void execVersion.value // depend on version for shallowRef reactivity
  if (execViewMode.value !== 'chain' || !selectedChainId.value) return new Set<string>()
  // In chain mode, all edges connected to nodes in the selected chain are "active"
  const chainNodeIds = new Set<string>()
  const chainMap = chainStatuses.value.get(selectedChainId.value)
  if (chainMap) {
    for (const nodeId of chainMap.keys()) {
      chainNodeIds.add(nodeId)
    }
  }
  const activeIds = new Set<string>()
  for (const e of props.edges) {
    if (chainNodeIds.has(e.from_node) && chainNodeIds.has(e.to_node)) {
      activeIds.add(e.id)
    }
  }
  return activeIds
})

const runningChainEdgeIds = computed(() => {
  void execVersion.value // depend on version for shallowRef reactivity
  if (execViewMode.value !== 'chain' || !selectedChainId.value) return new Set<string>()
  const chainMap = chainStatuses.value.get(selectedChainId.value)
  if (!chainMap) return new Set<string>()
  const runningNodeIds = new Set<string>()
  for (const [nodeId, status] of chainMap) {
    if (status === 'running') runningNodeIds.add(nodeId)
  }
  const runningIds = new Set<string>()
  for (const e of props.edges) {
    if (runningNodeIds.has(e.from_node) || runningNodeIds.has(e.to_node)) {
      runningIds.add(e.id)
    }
  }
  return runningIds
})

const defaultEdgeOptions = computed(() => ({
  type: 'smoothstep',
  style: { stroke: 'var(--text-tertiary)', strokeWidth: 2 },
  markerEnd: MarkerType.ArrowClosed,
}))

function getNodeTypeLabel(type: string) {
  const map: Record<string, string> = { setup: 'SETUP', http: 'HTTP', delay: 'DELAY', condition: 'COND', 'if-else': 'IF-ELSE', teardown: 'TEARDOWN', group: 'GROUP', timer: 'TIMER', while: 'WHILE', parallel: 'PARALLEL', sub_flow: 'SUBFLOW', loop: 'LOOP' }
  return map[type] || type.toUpperCase()
}

function getNodeIcon(type: string) {
  return getDagIcon(type)
}

const elk = new ELK()

function getNodeDimensions(n: NodeDTO): { width: number; height: number } {
  const DEFAULT_W = 280
  const DEFAULT_H = 56
  try {
    const cfg = JSON.parse(n.config || '{}')
    if (n.type === 'group' && cfg.node_ids && Array.isArray(cfg.node_ids)) {
      const childCount = cfg.node_ids.length
      // header (56) + children area (each child ~40px + 22px arrow) + padding
      return { width: 320, height: DEFAULT_H + childCount * 62 + 24 }
    }
    if (n.type === 'while' && cfg.steps && Array.isArray(cfg.steps)) {
      const stepCount = cfg.steps.length
      // header (56) + loop indicator (30) + steps (each step ~40px + 22px arrow) + padding
      return { width: 300, height: DEFAULT_H + stepCount * 62 + 54 }
    }
  } catch { /* ignore */ }
  return { width: DEFAULT_W, height: DEFAULT_H }
}

async function buildLayout(newNodes: NodeDTO[], newEdges: EdgeDTO[]): Promise<Map<string, { x: number; y: number }>> {
  const positions = new Map<string, { x: number; y: number }>()
  if (newNodes.length === 0) return positions

  const elkNodes = newNodes.map(n => {
    const dims = getNodeDimensions(n)
    return { id: n.id, width: dims.width, height: dims.height }
  })

  const nodeIds = new Set(newNodes.map(n => n.id))
  const elkEdges = newEdges
    .filter(e => nodeIds.has(e.from_node) && nodeIds.has(e.to_node))
    .map(e => ({ id: e.id, sources: [e.from_node], targets: [e.to_node] }))

  const graph = {
    id: 'root',
    layoutOptions: {
      'org.eclipse.elk.algorithm': 'layered',
      'org.eclipse.elk.direction': 'DOWN',
      'org.eclipse.elk.layered.nodePlacement.strategy': 'BRANDES_KOEPF',
      'org.eclipse.elk.layered.crossingMinimization.strategy': 'LAYER_SWEEP',
      'org.eclipse.elk.spacing.nodeNode': '70',
      'org.eclipse.elk.layered.spacing.nodeNodeBetweenLayers': '110',
      'org.eclipse.elk.layered.considerModelOrder.strategy': 'NODES_AND_EDGES',
      'org.eclipse.elk.edgeRouting': 'ORTHOGONAL',
      'org.eclipse.elk.layered.nodePlacement.favorStraightEdges': 'true',
      'org.eclipse.elk.hierarchyHandling': 'INCLUDE_CHILDREN',
    },
    children: elkNodes,
    edges: elkEdges,
  }

  const result = await elk.layout(graph)

  for (const child of result.children || []) {
    positions.set(child.id, {
      x: child.x || 0,
      y: child.y || 0,
    })
  }

  return positions
}

const vfNodes = ref<Node[]>([])
const vfEdges = ref<Edge[]>([])
const handleOverrides = ref<Map<string, { sourceHandle: string; targetHandle: string }>>(new Map())

async function applyLayout(newNodes: NodeDTO[], newEdges: EdgeDTO[]) {
  const layoutPositions = await buildLayout(newNodes, newEdges)

  vfNodes.value = newNodes.map(n => {
    const pos = layoutPositions.get(n.id) || { x: 0, y: 0 }
    let loopCount = 1
    let childNodes: NodeDTO[] = []
    let whileSteps: { name: string; type: string }[] = []
    try {
      const cfg = JSON.parse(n.config || '{}')
      loopCount = cfg.loop_count || 1
      if (n.type === 'group' && cfg.node_ids && Array.isArray(cfg.node_ids)) {
        const childIdSet = new Set(cfg.node_ids as string[])
        const childMap = new Map(newNodes.filter(cn => childIdSet.has(cn.id)).map(cn => [cn.id, cn]))
        childNodes = (cfg.node_ids as string[]).map(id => childMap.get(id)).filter(Boolean) as NodeDTO[]
      }
      if (n.type === 'while' && cfg.steps && Array.isArray(cfg.steps)) {
        whileSteps = (cfg.steps as any[]).map(s => ({ name: s.name || '', type: s.type || 'http' }))
      }
    } catch { /* ignore */ }

    const baseData: Record<string, unknown> = {
      label: n.name,
      nodeType: n.type,
      icon: getNodeIcon(n.type),
      typeLabel: getNodeTypeLabel(n.type),
      loopCount,
      originalNode: n,
    }

    if (n.type === 'group') {
      baseData.childNodes = childNodes
    }
    if (n.type === 'while') {
      baseData.whileSteps = whileSteps
    }

    return {
      id: n.id,
      type: 'scene-node',
      position: pos,
      data: baseData,
    } as Node
  })

  vfEdges.value = newEdges.map(e => {
    let markerEnd: string | { type: MarkerType; color: string } = MarkerType.ArrowClosed
    let strokeColor = 'var(--text-tertiary)'
    let edgeType = 'smoothstep'
    let animated = false
    let edgeOpacity: number | undefined = undefined

    if (e.condition === '__if_true__') {
      markerEnd = { type: MarkerType.ArrowClosed, color: '#2ecc71' }
      strokeColor = '#2ecc71'
    } else if (e.condition === '__if_false__') {
      markerEnd = { type: MarkerType.ArrowClosed, color: '#e74c3c' }
      strokeColor = '#e74c3c'
    }

    // Single chain view edge styling
    if (props.isRunning && execViewMode.value === 'chain' && selectedChainId.value) {
      const isActive = activeChainEdgeIds.value.has(e.id)
      const isRunningEdge = runningChainEdgeIds.value.has(e.id)
      if (isRunningEdge) {
        animated = true
        strokeColor = 'var(--accent-primary)'
        markerEnd = { type: MarkerType.ArrowClosed, color: 'var(--accent-primary)' }
      } else if (!isActive) {
        edgeOpacity = 0.15
      }
    }

    const override = handleOverrides.value.get(e.id)
    const sourceHandle = override?.sourceHandle || 's-bottom'
    const targetHandle = override?.targetHandle || 't-top'

    return {
      id: e.id,
      source: e.from_node,
      target: e.to_node,
      sourceHandle,
      targetHandle,
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
      animated,
      style: { stroke: strokeColor, strokeWidth: 2, ...(edgeOpacity !== undefined ? { opacity: edgeOpacity } : {}) },
      markerEnd,
    } as Edge
  })
}

watch([() => props.nodes, () => props.edges], ([newNodes, newEdges]) => {
  applyLayout(newNodes, newEdges)
}, { immediate: true })

// Update edge styles when view mode or selected chain changes
watch([execViewMode, selectedChainId], () => {
  if (!props.isRunning) return
  // Update edge styles in-place without re-running layout
  void execVersion.value // read current version to get latest data
  for (const edge of vfEdges.value) {
    if (execViewMode.value === 'chain' && selectedChainId.value) {
      const isActive = activeChainEdgeIds.value.has(edge.id)
      const isRunning = runningChainEdgeIds.value.has(edge.id)
      if (isRunning) {
        edge.style = { stroke: 'var(--accent-primary)', strokeWidth: 2, strokeDasharray: '8 4', animation: 'dash-flow 0.8s linear infinite' }
        edge.animated = true
      } else if (isActive) {
        edge.style = { stroke: 'var(--text-tertiary)', strokeWidth: 2 }
        edge.animated = false
      } else {
        edge.style = { stroke: 'var(--text-tertiary)', strokeWidth: 2, opacity: 0.15 }
        edge.animated = false
      }
    } else {
      // Reset to default style
      edge.style = { stroke: 'var(--text-tertiary)', strokeWidth: 2 }
      edge.animated = false
    }
  }
})

async function autoLayout() {
  await applyLayout(props.nodes, props.edges)
  setTimeout(() => fitView({ padding: 0.2 }), 50)
  showToast('布局已重新排列', 'success')
}

function toYamlValue(val: any, indent: string): string {
  if (val === null || val === undefined) return 'null'
  if (typeof val === 'boolean') return val ? 'true' : 'false'
  if (typeof val === 'number') return String(val)
  if (typeof val === 'string') {
    if (val === '') return "''"
    if (/^[a-zA-Z0-9_\-.\/@$:{}[\]() ]+$/.test(val) && !/^(\d|true|false|null|yes|no|on|off)/i.test(val)) {
      return val
    }
    const escaped = val.replace(/'/g, "''")
    return `'${escaped}'`
  }
  if (Array.isArray(val)) {
    if (val.length === 0) return '[]'
    const items = val.map((item) => {
      const v = toYamlValue(item, indent + '  ')
      return `${indent}  - ${v}`
    })
    return '\n' + items.join('\n')
  }
  if (typeof val === 'object') {
    const keys = Object.keys(val)
    if (keys.length === 0) return '{}'
    const items = keys.map(k => {
      const v = toYamlValue(val[k], indent + '  ')
      if (typeof val[k] === 'object' && val[k] !== null && !Array.isArray(val[k]) && Object.keys(val[k]).length > 0) {
        return `${indent}  ${k}:\n${v}`
      }
      if (Array.isArray(val[k]) && val[k].length > 0) {
        return `${indent}  ${k}:${v}`
      }
      return `${indent}  ${k}: ${v}`
    })
    return items.join('\n')
  }
  return String(val)
}

function generateYaml(): string {
  const lines: string[] = []
  lines.push('# DAG 场景配置导出')
  lines.push('')

  const idToName = new Map<string, string>()
  for (const n of props.nodes) {
    idToName.set(n.id, n.name)
  }

  // Data sources section
  if (props.dataSources && props.dataSources.length > 0) {
    lines.push('data_sources:')
    for (const ds of props.dataSources) {
      lines.push(`  - name: ${toYamlValue(ds.name, '')}`)
      if (ds.columns && ds.columns.length > 0) {
        lines.push(`    columns:`)
        for (const col of ds.columns) {
          lines.push(`      - ${col}`)
        }
      }
      if (ds.rows && ds.rows.length > 0) {
        lines.push(`    rows:`)
        for (const row of ds.rows) {
          lines.push(`      - { ${Object.entries(row).map(([k, v]) => `${k}: ${toYamlValue(v, '')}`).join(', ')} }`)
        }
      }
      lines.push('')
    }
  }

  lines.push('nodes:')
  for (const n of props.nodes) {
    lines.push(`  - name: ${toYamlValue(n.name, '')}`)
    lines.push(`    type: ${n.type}`)
    try {
      const parsed = JSON.parse(n.config || '{}')
      // For group nodes, convert node_ids from IDs to names for readability
      if (n.type === 'group' && parsed.node_ids) {
        parsed.node_ids = (parsed.node_ids as string[]).map((id: string) => idToName.get(id) || id)
      }
      if (Object.keys(parsed).length > 0) {
        lines.push(`    config:`)
        for (const [k, v] of Object.entries(parsed)) {
          const yv = toYamlValue(v, '      ')
          if (typeof v === 'object' && v !== null && !Array.isArray(v) && Object.keys(v as object).length > 0) {
            lines.push(`      ${k}:`)
            lines.push(yv)
          } else if (Array.isArray(v) && (v as any[]).length > 0) {
            lines.push(`      ${k}:${yv}`)
          } else {
            lines.push(`      ${k}: ${yv}`)
          }
        }
      }
    } catch {
      lines.push(`    config: {}`)
    }
    if (n.loop_count && n.loop_count > 0) {
      lines.push(`    loop_count: ${n.loop_count}`)
    }
    lines.push('')
  }

  lines.push('edges:')
  for (const e of props.edges) {
    const fromName = idToName.get(e.from_node) || e.from_node
    const toName = idToName.get(e.to_node) || e.to_node
    const cond = e.condition || '(default)'
    lines.push(`  - from: ${toYamlValue(fromName, '')}`)
    lines.push(`    to: ${toYamlValue(toName, '')}`)
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
  emit('addEdge', params.source!, params.target!, undefined, params.sourceHandle || 's-bottom', params.targetHandle || 't-top')
}

function onEdgeUpdate({ edge, connection }: { edge: Edge; connection: Connection }) {
  const newSourceHandle = (connection.sourceHandle || edge.sourceHandle || 's-bottom') as string
  const newTargetHandle = (connection.targetHandle || edge.targetHandle || 't-top') as string
  const newSource = connection.source || edge.source
  const newTarget = connection.target || edge.target

  handleOverrides.value.set(edge.id, {
    sourceHandle: newSourceHandle,
    targetHandle: newTargetHandle,
  })

  vfEdges.value = vfEdges.value.map(e => {
    if (e.id === edge.id) {
      return {
        ...e,
        source: newSource,
        target: newTarget,
        sourceHandle: newSourceHandle,
        targetHandle: newTargetHandle,
        type: 'smoothstep',
      }
    }
    return e
  })

  if (newSource !== edge.source || newTarget !== edge.target) {
    emit('updateEdge', edge.id, newSource, newTarget, newSourceHandle, newTargetHandle)
  }
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
  if (t === 'while') return '#8e44ad'
  if (t === 'parallel') return '#16a085'
  if (t === 'sub_flow') return '#2980b9'
  if (t === 'loop') return '#d35400'
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

/* ===== Execution toolbar ===== */
.exec-toolbar {
  position: absolute;
  top: 10px;
  left: 10px;
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

.exec-tab {
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

.exec-tab:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.exec-tab.active {
  background: rgba(0,229,255,0.1);
  color: var(--accent-primary);
  font-weight: 600;
}



.ws-indicator {
  font-size: 10px;
  line-height: 1;
  padding: 0 6px;
}

.ws-indicator.connected {
  color: var(--accent-success);
}

.ws-indicator.disconnected {
  color: var(--accent-danger);
  animation: exec-pulse 1.5s ease-in-out infinite;
}

@keyframes exec-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
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
