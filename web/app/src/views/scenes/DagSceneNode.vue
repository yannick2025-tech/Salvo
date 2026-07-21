<template>
  <div :class="['scene-node', data.nodeType, chainStatusClass, { selected, expanded: isGroupExpanded || isWhileExpanded }]" :style="expandableNodeStyle">
    <Handle v-if="data.nodeType !== 'timer'" type="target" id="t-top" :position="Position.Top" class="handle-target handle-top" />
    <Handle v-if="data.nodeType !== 'timer'" type="target" id="t-bottom" :position="Position.Bottom" class="handle-target handle-bottom" />

    <!-- Aggregated view badges (top-right) -->
    <div v-if="executionStatus && viewMode === 'aggregate'" class="exec-badges">
      <span v-if="executionStatus.pass > 0" class="exec-badge pass">✓{{ executionStatus.pass }}</span>
      <span v-if="executionStatus.fail > 0" class="exec-badge fail">✗{{ executionStatus.fail }}</span>
      <span v-if="executionStatus.skip > 0" class="exec-badge skip">⊘{{ executionStatus.skip }}</span>
      <span v-if="executionStatus.running > 0" class="exec-badge running">⟳{{ executionStatus.running }}</span>
      <span v-if="executionStatus.idle > 0" class="exec-badge idle">◦{{ executionStatus.idle }}</span>
    </div>

    <!-- Single chain status indicator (top-right) -->
    <div v-if="chainStatus && viewMode === 'chain'" :class="['exec-status-dot', chainStatus]">
      <span v-if="chainStatus === 'pass'">✓</span>
      <span v-else-if="chainStatus === 'fail'">✗</span>
      <span v-else-if="chainStatus === 'skip'">⊘</span>
      <span v-else-if="chainStatus === 'running'">⟳</span>
    </div>

    <div class="node-body" @click.stop="toggleExpand">
      <div :class="['node-icon-wrap', data.nodeType]">
        <span class="node-icon" v-html="data.icon"></span>
      </div>
      <div class="node-content">
        <span class="node-label">{{ data.label }}</span>
        <div class="node-meta-row">
          <span :class="['type-badge', data.nodeType]">{{ data.typeLabel }}</span>
          <span v-if="data.loopCount && data.loopCount > 1" class="loop-badge">x{{ data.loopCount }}</span>
          <span v-if="isWhileType && whileStepCount > 0 && !isWhileExpanded" class="steps-badge">{{ whileStepCount }}步</span>
        </div>
      </div>
      <button v-if="isGroupType && hasChildren" class="action-btn expand-btn expand-btn-group" @click.stop="toggleExpand" :title="isGroupExpanded ? '折叠' : '展开'">
        {{ isGroupExpanded ? '−' : '+' }}
      </button>
      <button v-if="isWhileType && whileStepCount > 0" class="action-btn expand-btn expand-btn-while" @click.stop="toggleExpand" :title="isWhileExpanded ? '折叠' : '展开步骤'">
        {{ isWhileExpanded ? '−' : '+' }}
      </button>
      <div class="node-actions">
        <button class="action-btn edit" @click.stop="$emit('edit', data.originalNode)" title="编辑">✎</button>
        <button class="action-btn del" @click.stop="$emit('delete', id)" title="删除">✕</button>
      </div>
    </div>

    <!-- Loop progress badge (bottom-right) -->
    <div v-if="loopProgress" class="loop-progress-badge">L{{ loopProgress.current }}/{{ loopProgress.total }}</div>

    <div v-if="isGroupType && isGroupExpanded && hasChildren" ref="groupChildrenRef" class="group-children" @click.stop>
      <div
        v-for="(child, idx) in sortedChildNodes"
        :key="child.id"
        :class="['group-child-row', { 'child-selected': selectedChildId === child.id }]"
      >
        <div
          class="group-child-node"
          @click.stop="selectChild(child)"
          @dblclick.stop="$emit('edit', child)"
        >
          <span :class="['child-icon', child.type]" v-html="getChildIcon(child.type)"></span>
          <span class="child-name">{{ child.name }}</span>
          <span :class="['child-type-badge', child.type]">{{ getNodeTypeLabel(child.type) }}</span>
        </div>
        <div v-if="idx < sortedChildNodes.length - 1" class="edge-arrow-down">
          <svg viewBox="0 0 20 24" xmlns="http://www.w3.org/2000/svg">
            <line x1="10" y1="0" x2="10" y2="18" stroke="#8b949e" stroke-width="1.5" />
            <polyline points="5,14 10,20 15,14" fill="none" stroke="#8b949e" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
          </svg>
        </div>
      </div>
      <div class="resize-handle" @mousedown.prevent="startResize">↘</div>
    </div>

    <div v-if="isWhileType && isWhileExpanded && whileStepCount > 0" class="while-steps" @click.stop>
      <div class="while-loop-indicator">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#8e44ad" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 1l4 4-4 4"/><path d="M3 11V9a4 4 0 014-4h14"/><path d="M7 23l-4-4 4-4"/><path d="M21 13v2a4 4 0 01-4 4H3"/></svg>
        <span class="while-loop-label">循环体</span>
      </div>
      <div v-for="(step, idx) in whileStepsList" :key="idx" class="while-step-row">
        <div class="while-step-node">
          <span :class="['child-icon', step.type]" v-html="getChildIcon(step.type)"></span>
          <span class="child-name">{{ step.name }}</span>
          <span :class="['child-type-badge', step.type]">{{ getNodeTypeLabel(step.type) }}</span>
        </div>
        <div v-if="idx < whileStepCount - 1" class="edge-arrow-down">
          <svg viewBox="0 0 20 24" xmlns="http://www.w3.org/2000/svg">
            <line x1="10" y1="0" x2="10" y2="18" stroke="rgba(142,68,173,0.5)" stroke-width="1.5" />
            <polyline points="5,14 10,20 15,14" fill="none" stroke="rgba(142,68,173,0.5)" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
          </svg>
        </div>
      </div>
    </div>

    <Handle type="source" id="s-bottom" :position="Position.Bottom" class="handle-source handle-bottom" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onUnmounted } from 'vue'
import { Handle, Position, useVueFlow } from '@vue-flow/core'
import { getDagIcon } from './dagIcons'
import type { NodeDTO, EdgeDTO } from '@/types'
import type { AggregateCounts, NodeStatus, LoopProgress, ViewMode } from '@/composables/useExecutionStatus'

const props = defineProps<{
  id: string
  data: {
    label: string
    nodeType: string
    icon: string
    typeLabel: string
    loopCount?: number
    originalNode: any
    childNodes?: NodeDTO[]
    internalEdges?: EdgeDTO[]
    whileSteps?: { name: string; type: string }[]
  }
  selected?: boolean
  executionStatus?: AggregateCounts
  chainStatus?: NodeStatus | null
  loopProgress?: LoopProgress
  viewMode?: ViewMode
}>()

const emit = defineEmits<{
  edit: [node: any]
  delete: [id: string]
}>()

const { updateNodeDimensions } = useVueFlow()

const isGroupExpanded = ref(false)
const isWhileExpanded = ref(false)
const selectedChildId = ref<string | null>(null)
const groupChildrenRef = ref<HTMLDivElement | null>(null)

const isGroupType = computed(() => props.data.nodeType === 'group')
const isWhileType = computed(() => props.data.nodeType === 'while')

const chainStatusClass = computed(() => {
  if (!props.chainStatus || props.viewMode !== 'chain') return ''
  return `exec-chain-${props.chainStatus}`
})

const hasChildren = computed(() =>
  isGroupType.value &&
  Array.isArray(props.data.childNodes) &&
  props.data.childNodes.length > 0
)

const whileStepsList = computed(() => props.data.whileSteps || [])
const whileStepCount = computed(() => whileStepsList.value.length)

function toggleExpand() {
  if (isGroupType.value && hasChildren.value) {
    isGroupExpanded.value = !isGroupExpanded.value
    selectedChildId.value = null
    nextTick(() => refreshDimensions())
  } else if (isWhileType.value && whileStepCount.value > 0) {
    isWhileExpanded.value = !isWhileExpanded.value
    nextTick(() => refreshDimensions())
  }
}

function selectChild(child: NodeDTO) {
  selectedChildId.value = selectedChildId.value === child.id ? null : child.id
}

const sortedChildNodes = computed(() => props.data.childNodes || [])

const expandableNodeStyle = computed(() => {
  if (isGroupType.value && isGroupExpanded.value && hasChildren.value) {
    return { minWidth: '320px', maxWidth: '600px', width: 'auto', overflow: 'visible' }
  }
  if (isWhileType.value && isWhileExpanded.value && whileStepCount.value > 0) {
    return { minWidth: '300px', maxWidth: '500px', width: 'auto', overflow: 'visible' }
  }
  return {}
})

function refreshDimensions() {
  const el = document.querySelector(`div[data-id="${props.id}"]`) as HTMLDivElement | null
  if (el) updateNodeDimensions([{ id: props.id, nodeElement: el, forceUpdate: true }])
}

let resizing = false
let startY = 0
let startH = 0

function startResize(e: MouseEvent) {
  if (!groupChildrenRef.value) return
  resizing = true
  startY = e.clientY
  startH = groupChildrenRef.value.offsetHeight
  document.addEventListener('mousemove', onResizeMove)
  document.addEventListener('mouseup', onResizeEnd)
}

function onResizeMove(e: MouseEvent) {
  if (!resizing || !groupChildrenRef.value) return
  const delta = e.clientY - startY
  const newH = Math.max(120, startH + delta)
  groupChildrenRef.value.style.height = newH + 'px'
}

function onResizeEnd() {
  if (!resizing) return
  resizing = false
  document.removeEventListener('mousemove', onResizeMove)
  document.removeEventListener('mouseup', onResizeEnd)
  nextTick(refreshDimensions)
}

onUnmounted(() => {
  document.removeEventListener('mousemove', onResizeMove)
  document.removeEventListener('mouseup', onResizeEnd)
})

function getChildIcon(type: string): string {
  return getDagIcon(type)
}

function getNodeTypeLabel(type: string): string {
  const labels: { [key: string]: string } = {}
  labels['setup'] = 'SETUP'
  labels['http'] = 'HTTP'
  labels['delay'] = 'DELAY'
  labels['condition'] = 'COND'
  labels['if-else'] = 'IF-ELSE'
  labels['teardown'] = 'TEARDOWN'
  labels['group'] = 'GROUP'
  labels['timer'] = 'TIMER'
  labels['while'] = 'WHILE'
  labels['parallel'] = 'PARALLEL'
  labels['sub_flow'] = 'SUBFLOW'
  labels['loop'] = 'LOOP'
  return labels[type] || type.toUpperCase()
}
</script>

<style scoped>
.scene-node {
  min-width: 260px;
  max-width: 340px;
  background: var(--bg-card);
  border: 1.5px solid var(--border-primary);
  border-radius: 10px;
  box-shadow: var(--shadow-sm);
  transition: all 0.25s ease;
  font-family: inherit;
  position: relative;
}

.scene-node:hover {
  border-color: var(--accent-primary);
  box-shadow: 0 4px 16px rgba(88,166,255,0.12), var(--shadow-sm);
}

.scene-node.selected {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 3px rgba(88,166,255,0.18), 0 4px 20px rgba(88,166,255,0.15);
}

.scene-node.setup { border-left: 3.5px solid #2ecc71; }
.scene-node.http { border-left: 3.5px solid var(--accent-primary); }
.scene-node.delay { border-left: 3.5px solid #f0ad4e; }
.scene-node.condition { border-left: 3.5px solid #9b59b6; }
.scene-node.if-else { border-left: 3.5px solid #e67e22; }
.scene-node.teardown { border-left: 3.5px solid #e74c3c; }
.scene-node.group { border-left: 3.5px solid #1abc9c; border-style: dashed; min-width: 300px; max-width: none; }
.scene-node.group.expanded { border-style: solid; border-color: rgba(26,188,156,0.4); background: var(--bg-secondary); }
.scene-node.timer { border-left: 3.5px solid #e84393; }
.scene-node.while { border-left: 3.5px solid #8e44ad; }
.scene-node.while.expanded { border-style: solid; border-color: rgba(142,68,173,0.4); background: var(--bg-secondary); }
.scene-node.parallel { border-left: 3.5px solid #16a085; }
.scene-node.sub_flow { border-left: 3.5px solid #2980b9; }
.scene-node.loop { border-left: 3.5px solid #d35400; }
.scene-node.generator { border-left: 3.5px solid #00bcd4; }

/* ====== Execution Status Overlays ====== */
/* Aggregated view badges */
.exec-badges {
  position: absolute;
  top: 6px;
  right: 6px;
  display: flex;
  gap: 3px;
  z-index: 10;
}
.exec-badge {
  font-size: 10px;
  font-weight: 700;
  padding: 1px 5px;
  border-radius: 4px;
  letter-spacing: 0.02em;
  line-height: 1.5;
}
.exec-badge.pass { background: rgba(46,204,113,0.18); color: #2ecc71; }
.exec-badge.fail { background: rgba(231,76,60,0.16); color: #e74c3c; }
.exec-badge.skip { background: rgba(241,196,15,0.16); color: #f1c40f; }
.exec-badge.running { background: rgba(0,229,255,0.14); color: var(--accent-primary); animation: exec-pulse 1.5s ease-in-out infinite; }
.exec-badge.idle { background: rgba(139,148,158,0.12); color: var(--text-tertiary); }

@keyframes exec-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

/* Single chain status indicator dot */
.exec-status-dot {
  position: absolute;
  top: 6px;
  right: 6px;
  width: 22px;
  height: 22px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  font-size: 11px;
  font-weight: 700;
  z-index: 10;
}
.exec-status-dot.pass { background: rgba(46,204,113,0.2); color: #2ecc71; }
.exec-status-dot.fail { background: rgba(231,76,60,0.2); color: #e74c3c; }
.exec-status-dot.skip { background: rgba(241,196,15,0.2); color: #f1c40f; }
.exec-status-dot.running { background: rgba(0,229,255,0.18); color: var(--accent-primary); animation: exec-pulse 1.5s ease-in-out infinite; }

/* Single chain view: node style overrides */
.scene-node.exec-chain-pass {
  border-left-color: #2ecc71 !important;
  background: rgba(46,204,113,0.06) !important;
}
.scene-node.exec-chain-fail {
  border-left-color: #e74c3c !important;
  background: rgba(231,76,60,0.06) !important;
}
.scene-node.exec-chain-skip {
  border-left-color: #f1c40f !important;
  opacity: 0.7;
}
.scene-node.exec-chain-running {
  animation: exec-border-pulse 1.5s ease-in-out infinite;
  border-left-color: var(--accent-primary) !important;
}
.scene-node.exec-chain-idle {
  opacity: 0.35;
}

@keyframes exec-border-pulse {
  0%, 100% { border-color: var(--accent-primary); box-shadow: 0 0 0 0 rgba(0,229,255,0); }
  50% { border-color: var(--accent-primary); box-shadow: 0 0 8px 2px rgba(0,229,255,0.3); }
}

/* Loop progress badge */
.loop-progress-badge {
  position: absolute;
  bottom: 6px;
  right: 6px;
  font-size: 10px;
  font-weight: 700;
  padding: 1px 6px;
  border-radius: 4px;
  background: rgba(142,68,173,0.18);
  color: #8e44ad;
  letter-spacing: 0.03em;
  z-index: 10;
}

.node-body {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  cursor: default;
}

.node-icon-wrap {
  width: 34px;
  height: 34px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  flex-shrink: 0;
  font-size: 14px;
}

.node-icon-wrap.setup { background: rgba(46,204,113,0.12); color: #2ecc71; }
.node-icon-wrap.http { background: rgba(88,166,255,0.1); color: var(--accent-primary); }
.node-icon-wrap.delay { background: rgba(240,173,78,0.13); color: #f0ad4e; }
.node-icon-wrap.condition { background: rgba(155,89,182,0.13); color: #9b59b6; }
.node-icon-wrap.if-else { background: rgba(230,126,34,0.13); color: #e67e22; }
.node-icon-wrap.teardown { background: rgba(231,76,60,0.11); color: #e74c3c; }
.node-icon-wrap.group { background: rgba(26,188,156,0.12); color: #1abc9c; }
.node-icon-wrap.timer { background: rgba(232,67,147,0.12); color: #e84393; }
.node-icon-wrap.while { background: rgba(142,68,173,0.12); color: #8e44ad; }
.node-icon-wrap.parallel { background: rgba(22,160,133,0.12); color: #16a085; }
.node-icon-wrap.sub_flow { background: rgba(41,128,185,0.12); color: #2980b9; }
.node-icon-wrap.loop { background: rgba(211,84,0,0.12); color: #d35400; }
.node-icon-wrap.generator { background: rgba(0,188,212,0.12); color: #00bcd4; }

.node-icon { font-size: 15px; line-height: 1; }

.node-content {
  display: flex;
  flex-direction: column;
  gap: 3px;
  flex: 1;
  min-width: 0;
}

.node-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  letter-spacing: -0.01em;
}

.node-meta-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.type-badge {
  font-size: 9.5px;
  font-weight: 700;
  letter-spacing: 0.08em;
  padding: 1.5px 7px;
  border-radius: 6px;
  width: fit-content;
}
.type-badge.setup { background: rgba(46,204,113,0.12); color: #2ecc71; }
.type-badge.http { background: rgba(88,166,255,0.1); color: var(--accent-primary); }
.type-badge.delay { background: rgba(240,173,78,0.13); color: #f0ad4e; }
.type-badge.condition { background: rgba(155,89,182,0.13); color: #9b59b6; }
.type-badge.if-else { background: rgba(230,126,34,0.13); color: #e67e22; }
.type-badge.teardown { background: rgba(231,76,60,0.11); color: #e74c3c; }
.type-badge.group { background: rgba(26,188,156,0.12); color: #1abc9c; }
.type-badge.timer { background: rgba(232,67,147,0.12); color: #e84393; }
.type-badge.while { background: rgba(142,68,173,0.12); color: #8e44ad; }
.type-badge.parallel { background: rgba(22,160,133,0.12); color: #16a085; }
.type-badge.sub_flow { background: rgba(41,128,185,0.12); color: #2980b9; }
.type-badge.loop { background: rgba(211,84,0,0.12); color: #d35400; }
.type-badge.generator { background: rgba(0,188,212,0.12); color: #00bcd4; }

.loop-badge {
  font-size: 9.5px;
  font-weight: 700;
  padding: 1.5px 6px;
  border-radius: 6px;
  background: rgba(255,152,0,0.15);
  color: #f57c00;
  letter-spacing: 0.04em;
}

.expand-btn {
  width: 26px;
  height: 26px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--border-primary);
  border-radius: 6px;
  background: transparent;
  color: var(--text-secondary);
  font-size: 14px;
  cursor: pointer;
  flex-shrink: 0;
  transition: all 0.15s ease;
}

.expand-btn:hover {
  background: rgba(26,188,156,0.1);
  color: #1abc9c;
  border-color: #1abc9c;
}

.expand-btn-while:hover {
  background: rgba(142,68,173,0.1);
  color: #8e44ad;
  border-color: #8e44ad;
}

.steps-badge {
  font-size: 9.5px;
  font-weight: 700;
  padding: 1.5px 6px;
  border-radius: 6px;
  background: rgba(142,68,173,0.12);
  color: #8e44ad;
  letter-spacing: 0.04em;
}

/* ====== While Steps Area ====== */
.while-steps {
  position: relative;
  margin: 4px 12px 10px;
  padding: 8px 12px 12px;
  border-top: 1px dashed rgba(142,68,173,0.3);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0;
  min-height: 60px;
  overflow: visible;
}

.while-loop-indicator {
  display: flex;
  align-items: center;
  gap: 5px;
  margin-bottom: 6px;
  opacity: 0.7;
}

.while-loop-label {
  font-size: 10px;
  font-weight: 600;
  color: #8e44ad;
  letter-spacing: 0.06em;
}

.while-step-row {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0;
  position: relative;
}

.while-step-node {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  background: var(--bg-card);
  border: 1px solid rgba(142,68,173,0.2);
  border-radius: 7px;
  box-shadow: 0 1px 4px rgba(142,68,173,0.06);
  min-width: 180px;
  max-width: 260px;
  box-sizing: border-box;
  transition: all 0.15s ease;
}

.while-step-node:hover {
  border-color: #8e44ad;
  box-shadow: 0 2px 8px rgba(142,68,173,0.12);
  transform: translateY(-1px);
}

.node-actions {
  display: flex;
  gap: 2px;
  opacity: 0;
  transition: opacity 0.15s ease;
  flex-shrink: 0;
}

.scene-node:hover .node-actions { opacity: 1; }

.action-btn {
  width: 26px;
  height: 26px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--text-tertiary);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.action-btn.edit:hover { background: rgba(88,166,255,0.12); color: var(--accent-primary); }
.action-btn.del:hover { background: rgba(248,81,73,0.1); color: var(--accent-danger); }

.handle-top { top: -6px !important; }
.handle-bottom { bottom: -6px !important; }

/* ====== Group Children Area ====== */
.group-children {
  position: relative;
  margin: 4px 12px 10px;
  padding: 8px 12px 16px;
  border-top: 1px dashed var(--border-tertiary);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0;
  min-height: 80px;
  overflow: visible;
}

.group-child-row {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0;
  position: relative;
}

.group-child-node {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 12px;
  background: var(--bg-card);
  border: 1px solid var(--border-primary);
  border-radius: 7px;
  box-shadow: 0 1px 4px rgba(0,0,0,0.08);
  cursor: pointer;
  min-width: 200px;
  max-width: 280px;
  box-sizing: border-box;
  transition: all 0.15s ease;
}

.group-child-node:hover {
  border-color: var(--accent-primary);
  box-shadow: 0 2px 8px rgba(88,166,255,0.15);
  transform: translateY(-1px);
}

.group-child-node.child-selected {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(88,166,255,0.25), 0 2px 10px rgba(88,166,255,0.18);
}

.edge-arrow-down {
  width: 20px;
  height: 22px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  margin: -1px 0;
}

.edge-arrow-down svg {
  width: 100%;
  height: 100%;
  display: block;
}

.resize-handle {
  position: absolute;
  bottom: 2px;
  right: 4px;
  width: 18px;
  height: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 10px;
  color: var(--text-tertiary);
  cursor: se-resize;
  opacity: 0;
  transition: opacity 0.15s;
  border-radius: 3px;
  user-select: none;
}

.group-children:hover .resize-handle {
  opacity: 0.5;
}

.resize-handle:hover {
  opacity: 1 !important;
  background: rgba(88,166,255,0.1);
  color: var(--accent-primary);
}

.child-icon {
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  flex-shrink: 0;
  font-size: 11px;
}
.child-icon.setup { background: rgba(46,204,113,0.12); color: #2ecc71; }
.child-icon.http { background: rgba(88,166,255,0.1); color: var(--accent-primary); }
.child-icon.delay { background: rgba(240,173,78,0.13); color: #f0ad4e; }
.child-icon.condition { background: rgba(155,89,182,0.13); color: #9b59b6; }
.child-icon.if-else { background: rgba(230,126,34,0.13); color: #e67e22; }
.child-icon.teardown { background: rgba(231,76,60,0.11); color: #e74c3c; }
.child-icon.group { background: rgba(26,188,156,0.12); color: #1abc9c; }
.child-icon.timer { background: rgba(232,67,147,0.12); color: #e84393; }
.child-icon.while { background: rgba(142,68,173,0.12); color: #8e44ad; }
.child-icon.parallel { background: rgba(22,160,133,0.12); color: #16a085; }
.child-icon.sub_flow { background: rgba(41,128,185,0.12); color: #2980b9; }
.child-icon.loop { background: rgba(211,84,0,0.12); color: #d35400; }
.child-icon.generator { background: rgba(0,188,212,0.12); color: #00bcd4; }

.child-name {
  font-size: 11.5px;
  font-weight: 500;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
  min-width: 0;
}

.child-type-badge {
  font-size: 8.5px;
  font-weight: 700;
  letter-spacing: 0.06em;
  padding: 1px 5px;
  border-radius: 5px;
  white-space: nowrap;
  flex-shrink: 0;
}
.child-type-badge.setup { background: rgba(46,204,113,0.12); color: #2ecc71; }
.child-type-badge.http { background: rgba(88,166,255,0.1); color: var(--accent-primary); }
.child-type-badge.delay { background: rgba(240,173,78,0.13); color: #f0ad4e; }
.child-type-badge.condition { background: rgba(155,89,182,0.13); color: #9b59b6; }
.child-type-badge.if-else { background: rgba(230,126,34,0.13); color: #e67e22; }
.child-type-badge.teardown { background: rgba(231,76,60,0.11); color: #e74c3c; }
.child-type-badge.group { background: rgba(26,188,156,0.12); color: #1abc9c; }
.child-type-badge.timer { background: rgba(232,67,147,0.12); color: #e84393; }
.child-type-badge.while { background: rgba(142,68,173,0.12); color: #8e44ad; }
.child-type-badge.parallel { background: rgba(22,160,133,0.12); color: #16a085; }
.child-type-badge.sub_flow { background: rgba(41,128,185,0.12); color: #2980b9; }
.child-type-badge.loop { background: rgba(211,84,0,0.12); color: #d35400; }
.child-type-badge.generator { background: rgba(0,188,212,0.12); color: #00bcd4; }
</style>

<style>
.vue-flow__node .vue-flow__handle {
  opacity: 0;
  transition: opacity 0.15s ease;
}

.vue-flow__node:hover .vue-flow__handle,
.vue-flow__connection-line-active ~ .vue-flow__node .vue-flow__handle,
.vue-flow__handle.connecting {
  opacity: 1;
}

.vue-flow__node.expanded-group {
  overflow: visible !important;
  z-index: 10;
}

.vue-flow__node.expanded {
  overflow: visible !important;
  z-index: 10;
}
</style>