import { ref, watch, shallowRef, type Ref } from 'vue'
import type { SpanUpdateEvent } from './useExecutionWs'
import type { SpanDTO } from '@/types'

export type NodeStatus = 'pass' | 'fail' | 'skip' | 'running' | 'idle'

export interface AggregateCounts {
  pass: number
  fail: number
  skip: number
  running: number
  idle: number
}

export interface LoopProgress {
  current: number
  total: number
}

export interface NodeBadge {
  status: NodeStatus
  label: string
  pass: number
  fail: number
  skip: number
  running: number
  idle: number
  loopCurrent?: number
  loopTotal?: number
}

export type ViewMode = 'aggregate' | 'chain'

export function useExecutionStatus(spanUpdates: Ref<SpanUpdateEvent[]>) {
  const aggregateStatus = shallowRef<Map<string, AggregateCounts>>(new Map())
  const chainStatuses = shallowRef<Map<string, Map<string, NodeStatus | null>>>(new Map())
  const loopProgress = shallowRef<Map<string, Map<string, LoopProgress>>>(new Map())

  const viewMode = ref<ViewMode>('aggregate')
  const selectedChainId = ref<string | null>(null)

  // Version counter to trigger dependent computeds without replacing Map objects
  const version = ref(0)

  // Throttle version bumps: batch rapid WS events into ~200ms intervals
  // to prevent excessive re-renders under high concurrency
  let pendingBump = false
  let bumpTimer: ReturnType<typeof setTimeout> | null = null

  function bumpVersion() {
    if (pendingBump) return // already scheduled
    pendingBump = true
    bumpTimer = setTimeout(() => {
      version.value++
      pendingBump = false
    }, 200)
  }

  function bumpVersionImmediate() {
    if (bumpTimer) { clearTimeout(bumpTimer); bumpTimer = null }
    pendingBump = false
    version.value++
  }

  function statusFromEvent(event: SpanUpdateEvent): NodeStatus {
    switch (event.status) {
      case 'ok':
      case 'success':
        return 'pass'
      case 'error':
      case 'fail':
        return 'fail'
      case 'skip':
      case 'skipped':
        return 'skip'
      case 'running':
        return 'running'
      default:
        return 'idle'
    }
  }

  function processEvent(event: SpanUpdateEvent) {
    const nodeId = event.node_id
    const chainId = event.chain_id
    const status = statusFromEvent(event)

    // Update chain status
    let chainMap = chainStatuses.value.get(chainId)
    if (!chainMap) {
      chainMap = new Map()
      chainStatuses.value.set(chainId, chainMap)
    }
    chainMap.set(nodeId, status)

    // Recompute aggregate from chainStatuses
    const counts: AggregateCounts = { pass: 0, fail: 0, skip: 0, running: 0, idle: 0 }
    for (const [, nodes] of chainStatuses.value) {
      const s = nodes.get(nodeId)
      if (s === 'pass') counts.pass++
      else if (s === 'fail') counts.fail++
      else if (s === 'skip') counts.skip++
      else if (s === 'running') counts.running++
      else counts.idle++
    }
    aggregateStatus.value.set(nodeId, counts)

    // Update loop progress
    if (event.loop_index !== undefined && event.loop_index !== null) {
      let chainLoopMap = loopProgress.value.get(chainId)
      if (!chainLoopMap) {
        chainLoopMap = new Map()
        loopProgress.value.set(chainId, chainLoopMap)
      }
      const existing = chainLoopMap.get(nodeId)
      const currentIndex = event.loop_index + 1 // loop_index is 0-based
      if (!existing) {
        chainLoopMap.set(nodeId, { current: currentIndex, total: currentIndex })
      } else {
        existing.current = Math.max(existing.current, currentIndex)
        existing.total = Math.max(existing.total, currentIndex)
      }
    }

    // Trigger reactivity for shallowRef
    bumpVersion()
  }

  // Watch for new span updates and process them
  let lastProcessedIndex = 0
  watch(
    () => spanUpdates.value.length,
    () => {
      const updates = spanUpdates.value
      while (lastProcessedIndex < updates.length) {
        processEvent(updates[lastProcessedIndex])
        lastProcessedIndex++
      }
    },
    { immediate: true }
  )

  function computeAggregateStatus(nodes: { id: string; loop_count?: number }[]): Map<string, NodeBadge> {
    const result = new Map<string, NodeBadge>()

    for (const node of nodes) {
      const agg = aggregateStatus.value.get(node.id)
      const badge: NodeBadge = {
        status: 'idle',
        label: '',
        pass: agg?.pass ?? 0,
        fail: agg?.fail ?? 0,
        skip: agg?.skip ?? 0,
        running: agg?.running ?? 0,
        idle: agg?.idle ?? 0,
      }

      if (agg) {
        if (agg.running > 0) {
          badge.status = 'running'
        } else if (agg.fail > 0) {
          badge.status = 'fail'
        } else if (agg.pass > 0 && agg.skip === 0 && agg.idle === 0) {
          badge.status = 'pass'
        } else if (agg.pass > 0) {
          badge.status = 'pass'
        } else if (agg.skip > 0 && agg.pass === 0 && agg.fail === 0) {
          badge.status = 'skip'
        } else {
          badge.status = 'idle'
        }
      }

      // Build label
      const parts: string[] = []
      if (badge.pass > 0) parts.push(`${badge.pass}✓`)
      if (badge.fail > 0) parts.push(`${badge.fail}✗`)
      if (badge.skip > 0) parts.push(`${badge.skip}>>`)
      if (badge.running > 0) parts.push(`${badge.running}⟳`)
      badge.label = parts.join(' ') || ''

      // Loop progress from selected chain or aggregate
      if (viewMode.value === 'chain' && selectedChainId.value) {
        const chainLoopMap = loopProgress.value.get(selectedChainId.value)
        const progress = chainLoopMap?.get(node.id)
        if (progress) {
          badge.loopCurrent = progress.current
          badge.loopTotal = progress.total
        }
      }

      result.set(node.id, badge)
    }

    return result
  }

  function switchView(mode: ViewMode) {
    viewMode.value = mode
    if (mode === 'aggregate') {
      selectedChainId.value = null
    }
  }

  function selectChain(chainId: string) {
    selectedChainId.value = chainId
    viewMode.value = 'chain'
    // Immediate bump so the UI updates right away when user selects a chain
    bumpVersionImmediate()
  }

  function initFromSpans(spans: SpanDTO[]) {
    for (const span of spans) {
      const chainId = span.chain_id || 'default'
      const nodeId = span.node_id
      const status = spanStatusFromSpan(span.status)

      // Update chain status
      let chainMap = chainStatuses.value.get(chainId)
      if (!chainMap) {
        chainMap = new Map()
        chainStatuses.value.set(chainId, chainMap)
      }
      chainMap.set(nodeId, status)
    }

    // Recompute aggregate
    const allNodeIds = new Set<string>()
    for (const [, nodes] of chainStatuses.value) {
      for (const nodeId of nodes.keys()) {
        allNodeIds.add(nodeId)
      }
    }
    for (const nodeId of allNodeIds) {
      const counts: AggregateCounts = { pass: 0, fail: 0, skip: 0, running: 0, idle: 0 }
      for (const [, nodes] of chainStatuses.value) {
        const s = nodes.get(nodeId)
        if (s === 'pass') counts.pass++
        else if (s === 'fail') counts.fail++
        else if (s === 'skip') counts.skip++
        else if (s === 'running') counts.running++
        else counts.idle++
      }
      aggregateStatus.value.set(nodeId, counts)
    }

    // Trigger reactivity for shallowRef immediately on init
    bumpVersionImmediate()
  }

  function spanStatusFromSpan(status: string): NodeStatus {
    switch (status) {
      case 'ok':
      case 'success':
        return 'pass'
      case 'error':
      case 'fail':
      case 'failed':
        return 'fail'
      case 'skip':
      case 'skipped':
      case 'canceled':
        return 'skip'
      case 'running':
        return 'running'
      default:
        return 'idle'
    }
  }

  return {
    aggregateStatus,
    chainStatuses,
    loopProgress,
    viewMode,
    selectedChainId,
    computeAggregateStatus,
    switchView,
    selectChain,
    initFromSpans,
    version,
  }
}
