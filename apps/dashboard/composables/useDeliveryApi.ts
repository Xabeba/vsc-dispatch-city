import type { EventEnvelope, Snapshot } from '~/types/delivery'

const emptySnapshot = (): Snapshot => ({
  mode: 'standalone',
  running: false,
  tick: 0,
  instance: 'connecting',
  restaurants: [],
  customers: [],
  couriers: [],
  orders: [],
  components: [],
  stats: { active_orders: 0, delivered: 0, events: 0, ready_pods: 0, total_pods: 0 },
})

export function useDeliveryApi() {
  const config = useRuntimeConfig()
  const apiBase = config.public.apiBase as string
  const snapshot = useState<Snapshot>('delivery-snapshot', emptySnapshot)
  const events = useState<EventEnvelope[]>('delivery-events', () => [])
  const connection = useState<'connecting' | 'live' | 'offline'>('delivery-connection', () => 'connecting')
  const lastError = useState<string>('delivery-error', () => '')
  let eventSource: EventSource | undefined
  let refreshTimer: ReturnType<typeof setTimeout> | undefined
  let pollTimer: ReturnType<typeof setInterval> | undefined

  const endpoint = (path: string) => `${apiBase}${path}`

  async function refreshSnapshot() {
    try {
      snapshot.value = await $fetch<Snapshot>(endpoint('/api/v1/snapshot'))
      lastError.value = ''
    } catch (error) {
      connection.value = 'offline'
      lastError.value = error instanceof Error ? error.message : 'Snapshot nicht erreichbar'
    }
  }

  function scheduleRefresh() {
    if (refreshTimer) return
    refreshTimer = setTimeout(async () => {
      refreshTimer = undefined
      await refreshSnapshot()
    }, 180)
  }

  function connect() {
    if (!import.meta.client) return
    connection.value = 'connecting'
    eventSource = new EventSource(endpoint('/api/v1/events'))
    eventSource.onopen = () => {
      connection.value = 'live'
    }
    eventSource.onmessage = (message) => {
      try {
        const event = JSON.parse(message.data) as EventEnvelope
        const previous = event.event_type === 'courier.location.updated'
          ? events.value.filter(item => item.event_type !== 'courier.location.updated' || item.source !== event.source)
          : events.value
        events.value = [event, ...previous].slice(0, 40)
        scheduleRefresh()
      } catch {
        lastError.value = 'Ein Event konnte nicht gelesen werden.'
      }
    }
    eventSource.onerror = () => {
      connection.value = 'offline'
    }
  }

  async function command(path: string) {
    await $fetch(endpoint(path), { method: 'POST' })
    await refreshSnapshot()
  }

  onMounted(async () => {
    await refreshSnapshot()
    connect()
    pollTimer = setInterval(refreshSnapshot, 5000)
  })

  onBeforeUnmount(() => {
    eventSource?.close()
    if (refreshTimer) clearTimeout(refreshTimer)
    if (pollTimer) clearInterval(pollTimer)
  })

  return {
    snapshot,
    events,
    connection,
    lastError,
    refreshSnapshot,
    start: () => command('/api/v1/simulation/start'),
    pause: () => command('/api/v1/simulation/pause'),
    reset: () => command('/api/v1/simulation/reset'),
    createOrder: () => command('/api/v1/orders'),
  }
}
