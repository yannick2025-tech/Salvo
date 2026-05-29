<template>
  <div :class="['scene-node', data.nodeType, { selected, expanded: isGroupExpanded }]" :style="groupNodeStyle">
    <Handle v-if="data.nodeType !== 'timer'" type="target" id="t-top" :position="Position.Top" class="handle-target handle-top" />
    <Handle v-if="data.nodeType !== 'timer'" type="target" id="t-bottom" :position="Position.Bottom" class="handle-target handle-bottom" />

    <div class="node-body" @click.stop="toggleExpand">
      <div :class="['node-icon-wrap', data.nodeType]">
        <span class="node-icon">{{ data.icon }}</span>
      </div>
      <div class="node-content">
        <span class="node-label">{{ data.label }}</span>
        <div class="node-meta-row">
          <span :class="['type-badge', data.nodeType]">{{ data.typeLabel }}</span>
          <span v-if="data.loopCount && data.loopCount > 1" class="loop-badge">x{{ data.loopCount }}</span>
        </div>
      </div>
      <button v-if="isGroupType && hasChildren" class="action-btn expand-btn" @click.stop="toggleExpand" :title="isGroupExpanded ? '折叠' : '展开'">
        {{ isGroupExpanded ? '−' : '+' }}
      </button>
      <div class="node-actions">
        <button class="action-btn edit" @click.stop="$emit('edit', data.originalNode)" title="编辑">✎</button>
        <button class="action-btn del" @click.stop="$emit('delete', id)" title="删除">✕</button>
      </div>
    </div>

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
          <span :class="['child-icon', child.type]">{{ getChildIcon(child.type) }}</span>
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

    <Handle type="source" id="s-bottom" :position="Position.Bottom" class="handle-source handle-bottom" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onUnmounted } from 'vue'
import { Handle, Position, useVueFlow } from '@vue-flow/core'
import type { NodeDTO, EdgeDTO } from '@/types'

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
  }
  selected?: boolean
}>()

const emit = defineEmits<{
  edit: [node: any]
  delete: [id: string]
}>()

const { updateNodeDimensions } = useVueFlow()

const isGroupExpanded = ref(false)
const selectedChildId = ref<string | null>(null)
const groupChildrenRef = ref<HTMLDivElement | null>(null)

const isGroupType = computed(() => props.data.nodeType === 'group')

const hasChildren = computed(() =>
  isGroupType.value &&
  Array.isArray(props.data.childNodes) &&
  props.data.childNodes.length > 0
)

function toggleExpand() {
  if (!isGroupType.value || !hasChildren.value) return
  isGroupExpanded.value = !isGroupExpanded.value
  selectedChildId.value = null
  nextTick(() => refreshDimensions())
}

function selectChild(child: NodeDTO) {
  selectedChildId.value = selectedChildId.value === child.id ? null : child.id
}

const sortedChildNodes = computed(() => props.data.childNodes || [])

const groupNodeStyle = computed(() => {
  if (!isGroupType.value || !isGroupExpanded.value || !hasChildren.value) return {}
  return {
    minWidth: '320px',
    maxWidth: '600px',
    width: 'auto',
    overflow: 'visible',
  }
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
  const icons: { [key: string]: string } = {}
  icons['setup'] = '\u25B6'
  icons['http'] = '\u21C4'
  icons['delay'] = '\u23F1'
  icons['condition'] = '\u25C7'
  icons['if-else'] = 'Y'
  icons['teardown'] = '\u25A0'
  icons['group'] = '\u229E'
  icons['timer'] = '\u23F2'
  return icons[type] || '?'
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
</style>