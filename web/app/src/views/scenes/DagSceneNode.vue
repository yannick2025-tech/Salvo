<template>
  <div :class="['scene-node', data.nodeType, { selected: selected }]">
    <Handle v-if="data.nodeType !== 'timer'" type="target" id="t-top" :position="Position.Top" class="handle-target handle-top" />
    <Handle v-if="data.nodeType !== 'timer'" type="target" id="t-bottom" :position="Position.Bottom" class="handle-target handle-bottom" />
    <Handle v-if="data.nodeType !== 'timer'" type="target" id="t-left" :position="Position.Left" class="handle-target handle-left" />
    <Handle v-if="data.nodeType !== 'timer'" type="target" id="t-right" :position="Position.Right" class="handle-target handle-right" />

    <div class="node-body">
      <div :class="['node-icon-wrap', data.nodeType]">
        <span class="node-icon">{{ data.icon }}</span>
      </div>
      <div class="node-content">
        <span class="node-label">{{ data.label }}</span>
        <div class="node-meta-row">
          <span :class="['type-badge', data.nodeType]">{{ data.typeLabel }}</span>
          <span v-if="data.loopCount > 1" class="loop-badge">x{{ data.loopCount }}</span>
        </div>
      </div>
      <div class="node-actions">
        <button class="action-btn edit" @click.stop="$emit('edit', data.originalNode)" title="编辑">✎</button>
        <button class="action-btn del" @click.stop="$emit('delete', id)" title="删除">✕</button>
      </div>
    </div>

    <Handle type="source" id="s-top" :position="Position.Top" class="handle-source handle-top" />
    <Handle type="source" id="s-bottom" :position="Position.Bottom" class="handle-source handle-bottom" />
    <Handle type="source" id="s-left" :position="Position.Left" class="handle-source handle-left" />
    <Handle type="source" id="s-right" :position="Position.Right" class="handle-source handle-right" />
  </div>
</template>

<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core'

defineProps<{
  id: string
  data: {
    label: string
    nodeType: string
    icon: string
    typeLabel: string
    loopCount?: number
    originalNode: any
  }
  selected?: boolean
}>()

defineEmits<{
  edit: [node: any]
  delete: [id: string]
}>()
</script>

<style scoped>
.scene-node {
  min-width: 260px;
  max-width: 340px;
  background: var(--bg-card);
  border: 1.5px solid var(--border-primary);
  border-radius: 10px;
  box-shadow: var(--shadow-sm);
  transition: all 0.2s ease;
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
.scene-node.group { border-left: 3.5px solid #1abc9c; border-style: dashed; min-width: 300px; }
.scene-node.timer { border-left: 3.5px solid #e84393; }

.node-body {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
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
.handle-left { left: -6px !important; }
.handle-right { right: -6px !important; }
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
</style>