import { ref, watch, shallowRef, type Ref } from 'vue'
import type { SpanUpdateEvent, SpanStatsEvent } from './useExecutionWs'
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

// Per (chain, node) cumulative state. Pass/fail/skip only grow; runningIdx
// tracks in-flight iterations; appliedIndex is the highest iteration index
// already counted (-1 = nothing), used to dedup the three data sources
// (REST init, WS snapshot, WS incremental events).
interface ChainNodeState {
  pass: number
  fail: number
  skip: number
  runningIdx: Set<number>
  appliedIndex: number
}

export type ViewMode = 'aggregate' | 'chain'

export function useExecutionStatus(
  spanUpdates: Ref<SpanUpdateEvent[]>,
  statsEvents: Ref<SpanStatsEvent[]>,
) {
  const aggregateStatus = shallowRef<Map<string, AggregateCounts>>(new Map())
  const chainStatuses = shallowRef<Map<string, Map<string, ChainNodeState>>>(new Map())
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
      case 'canceled':
        return 'skip'
      case 'running':
        return 'running'
      default:
        return 'idle'
    }
  }

  function getOrCreateState(chainId: string, nodeId: string): ChainNodeState {
    let chainMap = chainStatuses.value.get(chainId)
    if (!chainMap) {
      chainMap = new Map()
      chainStatuses.value.set(chainId, chainMap)
    }
    let state = chainMap.get(nodeId)
    if (!state) {
      state = { pass: 0, fail: 0, skip: 0, runningIdx: new Set(), appliedIndex: -1 }
      chainMap.set(nodeId, state)
    }
    return state
  }

  // Recompute the aggregate counts of a node across all chains.
  function recomputeAggregate(nodeId: string) {
    const counts: AggregateCounts = { pass: 0, fail: 0, skip: 0, running: 0, idle: 0 }
    for (const [, nodes] of chainStatuses.value) {
      const s = nodes.get(nodeId)
      if (!s) continue
      counts.pass += s.pass
      counts.fail += s.fail
      counts.skip += s.skip
      counts.running += s.runningIdx.size
    }
    aggregateStatus.value.set(nodeId, counts)
  }

  function processEvent(event: SpanUpdateEvent) {
    const nodeId = event.node_id
    const chainId = event.chain_id
    const status = statusFromEvent(event)
    const idx = event.loop_index ?? 0
    const state = getOrCreateState(chainId, nodeId)

    if (status === 'running') {
      // Ignore replays of iterations that already finished.
      if (idx <= state.appliedIndex) return
      state.runningIdx.add(idx)
    } else if (status === 'pass' || status === 'fail' || status === 'skip') {
      // Cumulative: terminal results only grow. Iterations at or below
      // appliedIndex were already counted (e.g. per-iteration event
      // followed by the span-level Finish broadcast).
      if (idx <= state.appliedIndex) return
      state.appliedIndex = idx
      state.runningIdx.delete(idx)
      if (status === 'pass') state.pass++
      else if (status === 'fail') state.fail++
      else state.skip++
    } else {
      // Unknown status: ignore.
      return
    }

    // Update loop progress (idempotent via Math.max)
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

    recomputeAggregate(nodeId)
    // Trigger reactivity for shallowRef
    bumpVersion()
  }

  // Apply a span_stats snapshot: replaces the full state of one
  // (chain, node). A snapshot older than what we already applied
  // (last_index < appliedIndex — can happen when an incremental event
  // arrives before the subscribe snapshot) is ignored to avoid rollback.
  function processStatsEvent(event: SpanStatsEvent) {
    const { chain_id: chainId, node_id: nodeId } = event
    const lastIndex = event.last_index ?? -1
    const existing = chainStatuses.value.get(chainId)?.get(nodeId)
    if (existing && existing.appliedIndex > lastIndex) return

    const state = getOrCreateState(chainId, nodeId)
    state.pass = event.pass ?? 0
    state.fail = event.fail ?? 0
    state.skip = event.skip ?? 0
    state.runningIdx = new Set(event.running_idx ?? [])
    state.appliedIndex = lastIndex

    // Rebuild loop progress from the snapshot.
    if (lastIndex >= 0) {
      let chainLoopMap = loopProgress.value.get(chainId)
      if (!chainLoopMap) {
        chainLoopMap = new Map()
        loopProgress.value.set(chainId, chainLoopMap)
      }
      const currentIndex = lastIndex + 1 // loop_index is 0-based
      const existingProgress = chainLoopMap.get(nodeId)
      if (!existingProgress) {
        chainLoopMap.set(nodeId, { current: currentIndex, total: currentIndex })
      } else {
        existingProgress.current = Math.max(existingProgress.current, currentIndex)
        existingProgress.total = Math.max(existingProgress.total, currentIndex)
      }
    }

    recomputeAggregate(nodeId)
    bumpVersionImmediate()
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
    { immediate: true },
  )

  // Watch for new span_stats snapshots and process them
  let lastProcessedStatsIndex = 0
  watch(
    () => statsEvents.value.length,
    () => {
      const events = statsEvents.value
      while (lastProcessedStatsIndex < events.length) {
        processStatsEvent(events[lastProcessedStatsIndex])
        lastProcessedStatsIndex++
      }
    },
    { immediate: true },
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
        idle: 0,
      }

      if (agg) {
        if (agg.running > 0) {
          badge.status = 'running'
        } else if (agg.fail > 0) {
          // Cumulative semantics: any failure keeps the node marked red,
          // even when later iterations succeeded.
          badge.status = 'fail'
        } else if (agg.pass > 0) {
          badge.status = 'pass'
        } else if (agg.skip > 0) {
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

  // Conservative fallback from persisted spans (REST trace): only seeds
  // (chain, node) pairs that have no live state yet (appliedIndex = -1),
  // so a late REST response never overwrites the WS snapshot or events
  // that already arrived. Spans hold the node's final outcome only, so
  // the final status is counted once at iteration 0.
  function initFromSpans(spans: SpanDTO[]) {
    for (const span of spans) {
      const chainId = span.chain_id || 'default'
      const nodeId = span.node_id

      const existing = chainStatuses.value.get(chainId)?.get(nodeId)
      if (existing && existing.appliedIndex >= 0) continue

      const status = spanStatusFromSpan(span.status)
      if (status !== 'pass' && status !== 'fail' && status !== 'skip') continue

      const state = getOrCreateState(chainId, nodeId)
      if (state.appliedIndex >= 0) continue
      state.appliedIndex = 0
      state.runningIdx = new Set()
      if (status === 'pass') state.pass = 1
      else if (status === 'fail') state.fail = 1
      else state.skip = 1
    }

    // Recompute aggregates for all touched nodes
    const allNodeIds = new Set<string>()
    for (const [, nodes] of chainStatuses.value) {
      for (const nodeId of nodes.keys()) {
        allNodeIds.add(nodeId)
      }
    }
    for (const nodeId of allNodeIds) {
      recomputeAggregate(nodeId)
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
