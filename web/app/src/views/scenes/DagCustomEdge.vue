<template>
  <g class="custom-edge-group">
    <BaseEdge :path="edgePath.path" :marker-end="markerEnd" :style="edgeStyle" />
    <foreignObject v-if="showLabel && labelValid" :x="labelX - 28" :y="labelY - 10" width="56" height="20" class="edge-label-fo">
      <div :class="['edge-label', conditionClass]">{{ displayLabel }}</div>
    </foreignObject>
  </g>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { BaseEdge, getSmoothStepPath } from '@vue-flow/core'

const props = withDefaults(defineProps<{
  id: string
  sourceX: number
  sourceY: number
  targetX: number
  targetY: number
  sourcePosition: any
  targetPosition: any
  sourceHandleId?: string
  targetHandleId?: string
  data?: { condition?: string; edgeId?: string }
  markerEnd?: string
  style?: any
  selected?: boolean
}>(), {
  markerEnd: undefined,
  style: () => ({}),
})

const showLabel = computed(() => !!props.data?.condition)

const conditionClass = computed(() => {
  const c = props.data?.condition || ''
  if (c === '__if_true__') return 'true-edge'
  if (c === '__if_false__') return 'false-edge'
  return 'normal-edge'
})

const displayLabel = computed(() => {
  const c = props.data?.condition || ''
  if (c === '__if_true__') return 'TRUE'
  if (c === '__if_false__') return 'FALSE'
  return c || ''
})

const edgeStyle = computed(() => {
  const base: any = props.style || {}
  const c = props.data?.condition || ''
  let stroke = 'var(--text-tertiary)'
  if (c === '__if_true__') stroke = '#2ecc71'
  else if (c === '__if_false__') stroke = '#e74c3c'
  return { ...base, stroke, strokeWidth: 2 }
})

const edgePath = computed(() => {
  const sx = Number(props.sourceX) || 0
  const sy = Number(props.sourceY) || 0
  const tx = Number(props.targetX) || 0
  const ty = Number(props.targetY) || 0
  return getSmoothStepPath({
    sourceX: sx,
    sourceY: sy,
    sourcePosition: props.sourcePosition,
    targetX: tx,
    targetY: ty,
    targetPosition: props.targetPosition,
    borderRadius: 12,
  })
})

const rawLabelX = computed(() => Number(edgePath.value.labelX) || 0)
const rawLabelY = computed(() => Number(edgePath.value.labelY) || 0)

const labelValid = computed(() => {
  const x = rawLabelX.value
  const y = rawLabelY.value
  return !isNaN(x) && !isNaN(y) && isFinite(x) && isFinite(y)
})

const labelX = computed(() => labelValid.value ? rawLabelX.value : 0)
const labelY = computed(() => labelValid.value ? rawLabelY.value : 0)
</script>

<style scoped>
.custom-edge-group { pointer-events: all; }
.edge-label-fo { overflow: visible; }
.edge-label {
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.06em;
  padding: 1px 7px;
  border-radius: 6px;
  text-align: center;
  white-space: nowrap;
  line-height: 16px;
}
.true-edge {
  background: rgba(46,204,113,0.15);
  color: #2ecc71;
  border: 1px solid rgba(46,204,113,0.25);
}
.false-edge {
  background: rgba(231,76,60,0.11);
  color: #e74c3c;
  border: 1px solid rgba(231,76,60,0.22);
}
.normal-edge {
  background: var(--bg-tertiary);
  color: var(--text-tertiary);
  border: 1px solid var(--border-primary);
}
</style>
