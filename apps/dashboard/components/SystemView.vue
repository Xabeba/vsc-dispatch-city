<script setup lang="ts">
import { Activity, Boxes, Database, Globe2, MessageSquareMore, Server } from '@lucide/vue'
import type { ComponentStatus } from '~/types/delivery'

defineProps<{ components: ComponentStatus[] }>()

const iconFor = (id: string) => {
  if (id.includes('dashboard')) return Globe2
  if (id.includes('rabbit')) return MessageSquareMore
  if (id.includes('postgre')) return Database
  if (id.includes('worker')) return Boxes
  if (id.includes('api')) return Server
  return Activity
}
</script>

<template>
  <section class="system-map" aria-label="Technische Systemansicht">
    <div class="system-map__backdrop">
      <span v-for="index in 14" :key="index" />
    </div>
    <div class="system-map__flow system-map__flow--one">HTTP / SSE</div>
    <div class="system-map__flow system-map__flow--two">AMQP events</div>
    <div class="system-map__flow system-map__flow--three">SQL projection</div>
    <article
      v-for="(component, index) in components"
      :key="component.id"
      class="system-node"
      :class="[`system-node--${component.category}`, { 'system-node--planned': component.status === 'planned' }]"
      :style="{ '--node-index': index }"
    >
      <component :is="iconFor(component.id)" :size="22" stroke-width="1.8" />
      <div>
        <span>{{ component.kind }}</span>
        <strong>{{ component.name }}</strong>
        <small>{{ component.detail }}</small>
      </div>
      <div class="replica-count" :class="{ 'replica-count--ok': component.ready === component.desired && component.desired > 0 }">
        {{ component.ready }}/{{ component.desired }}
      </div>
    </article>
  </section>
</template>
