<script setup lang="ts">
import {
  Activity,
  Bike,
  Boxes,
  CircleDot,
  Map as MapIcon,
  Network,
  PackagePlus,
  Pause,
  Play,
  LocateFixed,
  RotateCcw,
  Store,
  UtensilsCrossed,
  Users,
  ZoomIn,
  ZoomOut,
} from '@lucide/vue'
import type { SelectedEntity } from '~/types/delivery'

const { snapshot, events, connection, lastError, start, pause, reset, createOrder } = useDeliveryApi()
const view = ref<'city' | 'system'>('city')
const selected = ref<SelectedEntity>()
const cityStage = ref<{ zoomIn: () => void; zoomOut: () => void; resetCamera: () => void }>()
const { data: uiRuntime } = await useFetch<{ instance: string }>('/ui-instance', { default: () => ({ instance: 'dashboard' }) })

const statusLabel: Record<string, string> = {
  created: 'Eingegangen',
  accepted: 'In Zubereitung',
  courier_to_restaurant: 'Kurier zur Abholung',
  picked_up: 'Wird abgeholt',
  in_transit: 'Unterwegs',
  delivered: 'Geliefert',
  failed: 'Fehlgeschlagen',
}

const eventLabel = (type: string) => ({
  'order.created': 'Bestellung eingegangen',
  'order.accepted': 'Restaurant hat angenommen',
  'courier.assigned': 'Kurier fährt zum Restaurant',
  'order.picked_up': 'Bestellung abgeholt',
  'courier.location.updated': 'Position aktualisiert',
  'order.delivered': 'Bestellung geliefert',
}[type] || type)

const shortId = (id: string) => id.slice(0, 7).toUpperCase()
const eventTime = (date: string) => new Intl.DateTimeFormat('de-CH', { hour: '2-digit', minute: '2-digit', second: '2-digit' }).format(new Date(date))
</script>

<template>
  <main class="ops-shell">
    <header class="topbar">
      <div class="brand-lockup">
        <div class="brand-mark"><UtensilsCrossed :size="22" /></div>
        <div>
          <span>TEKO DISTRIBUTED LAB</span>
          <h1>DISPATCH CITY</h1>
        </div>
      </div>

      <div class="headline-stats" aria-label="Live-Kennzahlen">
        <div><span>Aktiv</span><strong>{{ snapshot.stats.active_orders }}</strong></div>
        <div><span>Geliefert</span><strong>{{ snapshot.stats.delivered }}</strong></div>
        <div><span>Events</span><strong>{{ snapshot.stats.events }}</strong></div>
        <div><span>Pods</span><strong>{{ snapshot.stats.ready_pods }}/{{ snapshot.stats.total_pods }}</strong></div>
      </div>

      <div class="live-state" :class="`live-state--${connection}`">
        <CircleDot :size="16" />
        <span>{{ connection === 'live' ? 'LIVE' : connection === 'connecting' ? 'CONNECTING' : 'OFFLINE' }}</span>
        <small>{{ snapshot.mode }} · {{ uiRuntime.instance }}</small>
      </div>
    </header>

    <section class="workspace">
      <aside class="left-rail">
        <div class="rail-heading">
          <span>Flottenlage</span>
          <strong>{{ snapshot.instance }}</strong>
        </div>

        <section class="rail-section">
          <h2><Store :size="16" /> Restaurants</h2>
          <button
            v-for="restaurant in snapshot.restaurants"
            :key="restaurant.id"
            class="restaurant-row"
            type="button"
            @click="selected = { kind: 'restaurant', id: restaurant.id, label: restaurant.name, detail: restaurant.cuisine }"
          >
            <span class="restaurant-swatch" :class="`restaurant-swatch--${restaurant.cuisine.toLowerCase()}`" />
            <span><strong>{{ restaurant.name }}</strong><small>{{ restaurant.cuisine }}</small></span>
            <i :class="{ degraded: restaurant.ready_replicas < restaurant.replicas }">{{ restaurant.ready_replicas }}/{{ restaurant.replicas }} Pods</i>
          </button>
        </section>

        <section class="fleet-summary" aria-label="Skalierte Stadtentitaeten">
          <div><Bike :size="15" /><span>Kuriere</span><strong>{{ snapshot.couriers.length }}</strong></div>
          <div><Users :size="15" /><span>Kunden</span><strong>{{ snapshot.customers.length }}</strong></div>
        </section>

        <section class="rail-section order-list">
          <h2><Boxes :size="16" /> Bestellungen</h2>
          <div v-if="snapshot.orders.length === 0" class="empty-state">Noch keine Bestellungen</div>
          <div v-for="order in snapshot.orders.slice(0, 7)" :key="order.id" class="order-row">
            <div><strong>#{{ shortId(order.id) }}</strong><small>{{ statusLabel[order.status] }}</small></div>
            <span :class="`order-state order-state--${order.status}`" />
          </div>
        </section>

        <div v-if="selected" class="selection-panel">
          <span>{{ selected.kind }}</span>
          <strong>{{ selected.label }}</strong>
          <small>{{ selected.detail }}</small>
          <button type="button" aria-label="Auswahl schliessen" @click="selected = undefined">×</button>
        </div>
      </aside>

      <section class="main-stage">
        <div class="stage-toolbar">
          <div class="segmented-control" aria-label="Ansicht wählen">
            <button type="button" :class="{ active: view === 'city' }" @click="view = 'city'"><MapIcon :size="16" /> Stadt</button>
            <button type="button" :class="{ active: view === 'system' }" @click="view = 'system'"><Network :size="16" /> System</button>
          </div>
          <div class="stage-actions">
            <template v-if="view === 'city'">
              <button type="button" class="icon-button" title="Karte verkleinern" @click="cityStage?.zoomOut()"><ZoomOut :size="18" /></button>
              <button type="button" class="icon-button" title="Karte einpassen" @click="cityStage?.resetCamera()"><LocateFixed :size="18" /></button>
              <button type="button" class="icon-button" title="Karte vergrößern" @click="cityStage?.zoomIn()"><ZoomIn :size="18" /></button>
            </template>
            <button type="button" class="icon-button" :title="snapshot.running ? 'Simulation pausieren' : 'Simulation starten'" @click="snapshot.running ? pause() : start()">
              <Pause v-if="snapshot.running" :size="18" />
              <Play v-else :size="18" />
            </button>
            <button type="button" class="icon-button" title="Simulation zurücksetzen" @click="reset"><RotateCcw :size="18" /></button>
            <button type="button" class="command-button" @click="createOrder"><PackagePlus :size="17" /> Bestellung</button>
          </div>
        </div>

        <ClientOnly v-if="view === 'city'">
          <CityStage ref="cityStage" :snapshot="snapshot" @select="selected = $event" />
          <template #fallback><div class="stage-loading">Stadt wird aufgebaut…</div></template>
        </ClientOnly>
        <SystemView v-else :components="snapshot.components" />

        <div class="map-legend" aria-label="Kartenlegende">
          <span><Store :size="14" /> Restaurant</span>
          <span><Bike :size="14" /> Kurier</span>
          <span><CircleDot :size="12" /> Kunde</span>
        </div>
        <div v-if="lastError" class="error-toast">{{ lastError }}</div>
      </section>

      <aside class="event-rail">
        <div class="rail-heading">
          <span>Event Stream</span>
          <Activity :size="17" />
        </div>
        <div class="event-list">
          <article v-for="event in events" :key="event.event_id" class="event-item" :class="`event-item--${event.event_type.replaceAll('.', '-')}`">
            <time>{{ eventTime(event.occurred_at) }}</time>
            <div>
              <strong>{{ eventLabel(event.event_type) }}</strong>
              <span>{{ event.source }}</span>
              <small>#{{ shortId(event.correlation_id) }}</small>
            </div>
          </article>
          <div v-if="events.length === 0" class="event-waiting">
            <Activity :size="26" />
            <span>Warte auf Domain Events</span>
          </div>
        </div>
      </aside>
    </section>
  </main>
</template>
