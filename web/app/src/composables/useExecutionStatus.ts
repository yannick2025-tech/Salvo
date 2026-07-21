import { ref, watch, type Ref } from 'vue'
import type { SpanUpdateEvent } from './useExecutionWs'

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
  const aggregateStatus = ref<Map<string, AggregateCounts>>(new Map())
  const chainStatuses = ref<Map<string, Map<string, NodeStatus | null>>>(new Map())
  const loopProgress = ref<Map<string, Map<string, LoopProgress>>>(new Map())

  const viewMode = ref<ViewMode>('aggregate')
  const selectedChainId = ref<string | null>(null)

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

    // Trigger reactivity by reassigning
    chainStatuses.value = new Map(chainStatuses.value)

    // Update aggregate counts
    let agg = aggregateStatus.value.get(nodeId)
    if (!agg) {
      agg = { pass: 0, fail: 0, skip: 0, running: 0, idle: 0 }
    }

    // When a chain transitions status for this node, we need to decrement
    // the previous status for that chain. We track by scanning all chains.
    // For simplicity, recompute aggregate from chainStatuses.
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
    aggregateStatus.value = new Map(aggregateStatus.value)

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
      loopProgress.value = new Map(loopProgress.value)
    }
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
      if (badge.skip > 0) parts.push(`${badge.skip}⊘`)
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
  }
}
