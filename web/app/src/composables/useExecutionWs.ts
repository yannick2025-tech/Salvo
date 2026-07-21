import { ref, onUnmounted } from 'vue'

export interface SpanUpdateEvent {
  type: 'span_update'
  run_id: string
  chain_id: string
  node_id: string
  status: string
  duration_ns: number
  error?: string
  loop_index?: number
}

export interface SubscribeMessage {
  type: 'subscribe'
  run_id: string
}

type WsInMessage = SpanUpdateEvent

const MAX_BACKOFF = 30_000
const BASE_DELAY = 1_000
const GRACE_PERIOD = 5_000

function getWsUrl(): string {
  const loc = window.location
  const protocol = loc.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${loc.host}/ws`
}

export function useExecutionWs() {
  const spanUpdates = ref<SpanUpdateEvent[]>([])
  const isConnected = ref(false)

  let ws: WebSocket | null = null
  let currentRunId: string | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let graceTimer: ReturnType<typeof setTimeout> | null = null
  let attempt = 0
  let intentionalClose = false

  function clearTimers() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    if (graceTimer) {
      clearTimeout(graceTimer)
      graceTimer = null
    }
  }

  function scheduleReconnect() {
    if (intentionalClose) return
    const delay = Math.min(BASE_DELAY * Math.pow(2, attempt), MAX_BACKOFF)
    attempt++
    reconnectTimer = setTimeout(() => {
      if (currentRunId && !intentionalClose) {
        doConnect(currentRunId)
      }
    }, delay)
  }

  function handleMessage(event: MessageEvent) {
    try {
      const msg: WsInMessage = JSON.parse(event.data)
      if (msg.type === 'span_update') {
        spanUpdates.value = [...spanUpdates.value, msg]
      }
    } catch {
      // ignore malformed messages
    }
  }

  function doConnect(runId: string) {
    if (ws) {
      ws.close()
      ws = null
    }

    const url = getWsUrl()
    ws = new WebSocket(url)

    ws.onopen = () => {
      isConnected.value = true
      attempt = 0
      // Subscribe once connected
      const sub: SubscribeMessage = { type: 'subscribe', run_id: runId }
      ws!.send(JSON.stringify(sub))
    }

    ws.onmessage = handleMessage

    ws.onclose = () => {
      isConnected.value = false
      ws = null
      scheduleReconnect()
    }

    ws.onerror = () => {
      // onclose will fire after onerror, reconnect logic is handled there
    }
  }

  function connect(runId: string) {
    intentionalClose = false
    currentRunId = runId
    clearTimers()
    doConnect(runId)
  }

  function disconnect() {
    intentionalClose = true
    currentRunId = null
    clearTimers()

    if (ws) {
      const socket = ws
      ws = null

      // Grace period: allow final messages to arrive before closing
      graceTimer = setTimeout(() => {
        if (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING) {
          socket.close()
        }
      }, GRACE_PERIOD)
    }

    isConnected.value = false
  }

  onUnmounted(() => {
    disconnect()
  })

  return {
    spanUpdates,
    isConnected,
    connect,
    disconnect,
  }
}
