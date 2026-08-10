<script setup lang="ts">
import { Application, Assets, Container, Graphics, Sprite, Text, Texture } from 'pixi.js'
import type { Courier, Position, SelectedEntity, Snapshot } from '~/types/delivery'

const props = defineProps<{ snapshot: Snapshot }>()
const emit = defineEmits<{ select: [entity: SelectedEntity] }>()
const host = ref<HTMLDivElement>()
const renderState = ref('pending')
const renderError = ref('')

const tileWidth = 82
const tileHeight = 41
const gridSize = 21
const roadSpacing = 4
const cityHeight = gridSize * tileHeight
const cityWidth = gridSize * tileWidth

interface MarkerState {
  container: Container
  targetX: number
  targetY: number
  caption: Text
  status: Text
  signature: string
}

let app: Application | undefined
let cityRoot: Container | undefined
let entityLayer: Container | undefined
let routeLayer: Graphics | undefined
let ambienceLayer: Container | undefined
let pan = { x: 0, y: 0 }
let scale = 0.44
let dragging = false
let dragOrigin = { x: 0, y: 0 }
const markers = new Map<string, MarkerState>()
const textures = new Map<string, Texture>()
const streetLights: Graphics[] = []

function iso(x: number, y: number) {
  return { x: (x - y) * tileWidth * 0.5, y: (x + y) * tileHeight * 0.5 }
}

function tilePolygon(point: Position) {
  return [
    point.x, point.y,
    point.x + tileWidth / 2, point.y + tileHeight / 2,
    point.x, point.y + tileHeight,
    point.x - tileWidth / 2, point.y + tileHeight / 2,
  ]
}

function shade(color: number, factor: number) {
  const red = Math.round(((color >> 16) & 0xff) * factor)
  const green = Math.round(((color >> 8) & 0xff) * factor)
  const blue = Math.round((color & 0xff) * factor)
  return (red << 16) | (green << 8) | blue
}

function isRoadCoordinate(value: number) {
  return Math.abs(value - Math.round(value / roadSpacing) * roadSpacing) < 0.01
}

function snapToRoad(position: Position): Position {
  if (isRoadCoordinate(position.x) || isRoadCoordinate(position.y)) return position
  const xRoad = Math.round(position.x / roadSpacing) * roadSpacing
  const yRoad = Math.round(position.y / roadSpacing) * roadSpacing
  return Math.abs(position.x - xRoad) <= Math.abs(position.y - yRoad)
    ? { x: xRoad, y: position.y }
    : { x: position.x, y: yRoad }
}

function roadPath(from: Position, to: Position): Position[] {
  const start = snapToRoad(from)
  const target = snapToRoad(to)
  if (Math.hypot(start.x - target.x, start.y - target.y) < 0.01) return [start]
  const corner = isRoadCoordinate(start.y)
    ? { x: target.x, y: start.y }
    : { x: start.x, y: target.y }
  const path = [start]
  if (Math.hypot(corner.x - start.x, corner.y - start.y) > 0.01 && Math.hypot(corner.x - target.x, corner.y - target.y) > 0.01) path.push(corner)
  path.push(target)
  return path
}

function drawTile(layer: Container, x: number, y: number) {
  const point = iso(x, y)
  const verticalRoad = x % roadSpacing === 0
  const horizontalRoad = y % roadSpacing === 0
  const road = verticalRoad || horizontalRoad
  const intersection = verticalRoad && horizontalRoad
  const color = road ? (intersection ? 0x252d29 : 0x29332e) : (x + y) % 2 === 0 ? 0x405143 : 0x38483e
  const tile = new Graphics()
    .poly(tilePolygon(point))
    .fill({ color })
    .stroke({ color: road ? 0x4b5851 : 0x2a352f, width: 1, alpha: 0.95 })
  layer.addChild(tile)

  if (!road) return
  const markings = new Graphics()
  if (intersection) {
    for (let stripe = -2; stripe <= 2; stripe++) {
      const offset = stripe * 7
      markings.moveTo(point.x - 20 + offset, point.y + 10 + offset * 0.45)
        .lineTo(point.x - 10 + offset, point.y + 15 + offset * 0.45)
    }
    markings.stroke({ color: 0xe6e9df, width: 2, alpha: 0.42 })
  } else if (verticalRoad) {
    markings.moveTo(point.x, point.y + 7).lineTo(point.x, point.y + tileHeight - 7)
      .stroke({ color: 0xe8cc55, width: 1.2, alpha: 0.7 })
  } else {
    markings.moveTo(point.x - tileWidth / 2 + 13, point.y + tileHeight / 2)
      .lineTo(point.x + tileWidth / 2 - 13, point.y + tileHeight / 2)
      .stroke({ color: 0xe8cc55, width: 1.2, alpha: 0.7 })
  }

  if (intersection && (x + y) % 8 === 0) {
    const light = new Graphics().circle(point.x + 23, point.y + 9, 2.5).fill({ color: 0x4ee0c3, alpha: 0.85 })
    streetLights.push(light)
    layer.addChild(light)
  }
}

function drawBuilding(layer: Container, x: number, y: number, height: number, color: number) {
  const point = iso(x, y)
  const width = 27 + ((x * 3 + y) % 8)
  const roofHeight = 14 + ((x + y) % 4)
  const topY = point.y + tileHeight / 2 - height
  const building = new Graphics()
  building.poly([point.x, topY, point.x + width, topY + roofHeight, point.x, topY + roofHeight * 2, point.x - width, topY + roofHeight])
    .fill({ color })
    .stroke({ color: 0x111713, width: 1.2 })
  building.poly([point.x - width, topY + roofHeight, point.x, topY + roofHeight * 2, point.x, point.y + tileHeight / 2 + 9, point.x - width, point.y + tileHeight / 2 - 7])
    .fill({ color: shade(color, 0.72) })
  building.poly([point.x + width, topY + roofHeight, point.x, topY + roofHeight * 2, point.x, point.y + tileHeight / 2 + 9, point.x + width, point.y + tileHeight / 2 - 7])
    .fill({ color: shade(color, 0.5) })

  const floors = Math.max(1, Math.floor(height / 22))
  for (let floor = 0; floor < floors; floor++) {
    const windowY = topY + roofHeight * 2 + 10 + floor * 15
    building.poly([point.x - width + 7, windowY - 3, point.x - 4, windowY + 7, point.x - 4, windowY + 12, point.x - width + 7, windowY + 2])
      .fill({ color: 0x9dd9d1, alpha: 0.34 })
    building.poly([point.x + 7, windowY + 7, point.x + width - 6, windowY - 2, point.x + width - 6, windowY + 3, point.x + 7, windowY + 12])
      .fill({ color: 0xf0d86c, alpha: 0.32 })
  }
  if ((x * 7 + y) % 5 === 0) {
    building.rect(point.x - 2, topY - 12, 4, 13).fill({ color: 0x65746c })
    building.circle(point.x, topY - 13, 3).fill({ color: 0xf26f5e })
  }
  layer.addChild(building)
}

function drawPark(layer: Container, x: number, y: number) {
  const point = iso(x, y)
  const park = new Graphics()
  park.poly(tilePolygon(point)).fill({ color: 0x426b4d }).stroke({ color: 0x6f8b62, width: 1 })
  for (let tree = 0; tree < 3; tree++) {
    const offsetX = (tree - 1) * 14
    const offsetY = tree % 2 === 0 ? 16 : 25
    park.rect(point.x + offsetX - 1, point.y + offsetY, 3, 10).fill({ color: 0x3a3124 })
    park.circle(point.x + offsetX, point.y + offsetY - 3, 7).fill({ color: tree % 2 === 0 ? 0x73a860 : 0x5f9257 })
  }
  layer.addChild(park)
}

function drawDistrictLabel(layer: Container, label: string, x: number, y: number) {
  const point = iso(x, y)
  const text = new Text({ text: label, style: { fontFamily: 'Barlow Condensed', fontSize: 16, fontWeight: '600', fill: 0xcbd3ca, letterSpacing: 0 } })
  text.anchor.set(0.5)
  text.alpha = 0.22
  text.position.set(point.x, point.y + 18)
  layer.addChild(text)
}

function drawCity() {
  if (!cityRoot) return
  const ground = new Container()
  const buildings = new Container()
  ambienceLayer = new Container()
  cityRoot.addChild(ground, buildings, ambienceLayer)
  for (let x = 0; x < gridSize; x++) {
    for (let y = 0; y < gridSize; y++) drawTile(ground, x, y)
  }

  const restaurantTiles = new Set(props.snapshot.restaurants.map(restaurant => `${Math.round(restaurant.position.x)}:${Math.round(restaurant.position.y)}`))
  const colors = [0xd9d6c7, 0xaec3b5, 0xd1aaa1, 0x9fbfbd, 0xd2bd61, 0x8fa2b4, 0xb59eb7]
  for (let x = 1; x < gridSize - 1; x++) {
    for (let y = 1; y < gridSize - 1; y++) {
      if (x % roadSpacing === 0 || y % roadSpacing === 0 || restaurantTiles.has(`${x}:${y}`)) continue
      if ((x * 5 + y * 3) % 13 === 0) {
        drawPark(buildings, x, y)
        continue
      }
      if ((x + y) % 3 === 0) continue
      drawBuilding(buildings, x, y, 30 + ((x * 17 + y * 11) % 72), colors[(x * 2 + y) % colors.length]!)
    }
  }

  drawDistrictLabel(ambienceLayer, 'NORTH QUAY', 4, 1)
  drawDistrictLabel(ambienceLayer, 'MARKET LOOP', 16, 9)
  drawDistrictLabel(ambienceLayer, 'SOUTH YARD', 7, 19)

  routeLayer = new Graphics()
  entityLayer = new Container()
  entityLayer.sortableChildren = true
  cityRoot.addChild(routeLayer, entityLayer)
}

function createMarker(
  id: string,
  textureName: 'restaurant' | 'customer' | 'courier',
  label: string,
  detail: string,
  kind: SelectedEntity['kind'],
  signature: string,
  replicas = 1,
  readyReplicas = replicas,
) {
  if (!entityLayer) return undefined
  const container = new Container()
  container.eventMode = 'static'
  container.cursor = 'pointer'
  container.on('pointertap', () => emit('select', { kind, id, label, detail } as SelectedEntity))

  const activeColor = textureName === 'courier' ? 0x4ee0c3 : textureName === 'customer' ? 0xe8cc55 : 0xf26f5e
  const shadow = new Graphics().ellipse(0, 10, textureName === 'restaurant' ? 24 : 17, 7).fill({ color: 0x07100b, alpha: 0.45 })
  const halo = new Graphics().circle(0, 1, textureName === 'restaurant' ? 28 : 20).stroke({ color: activeColor, width: 1, alpha: 0.28 })
  const sprite = new Sprite(textures.get(textureName) ?? Texture.WHITE)
  sprite.anchor.set(0.5, 0.86)
  sprite.width = textureName === 'restaurant' ? 56 : textureName === 'courier' ? 42 : 34
  sprite.height = textureName === 'restaurant' ? 63 : textureName === 'courier' ? 34 : 41

  if (textureName === 'restaurant') {
    const pods = new Graphics()
    const visibleReplicas = Math.min(replicas, 8)
    for (let replica = 0; replica < visibleReplicas; replica++) {
      const row = Math.floor(replica / 4)
      const column = replica % 4
      const x = -25 + column * 17
      const y = 25 + row * 10
      pods.roundRect(x, y, 13, 7, 1).fill({ color: replica < readyReplicas ? 0x8cd56e : 0x526059 }).stroke({ color: 0x101412, width: 1 })
    }
    container.addChild(pods)
  }

  const caption = new Text({ text: label, style: { fontFamily: 'IBM Plex Sans', fontSize: 11, fontWeight: '600', fill: 0xf2f5ef, stroke: { color: 0x101412, width: 4 } } })
  caption.anchor.set(0.5, 0)
  caption.position.set(0, textureName === 'restaurant' ? 38 : 18)
  const status = new Text({ text: detail.toUpperCase(), style: { fontFamily: 'Barlow Condensed', fontSize: 9, fontWeight: '600', fill: activeColor, stroke: { color: 0x101412, width: 3 } } })
  status.anchor.set(0.5, 0)
  status.position.set(0, textureName === 'restaurant' ? 52 : 31)
  container.addChild(shadow, halo, sprite, caption, status)
  entityLayer.addChild(container)
  return { container, targetX: 0, targetY: 0, caption, status, signature } satisfies MarkerState
}

function upsertMarker(
  id: string,
  textureName: 'restaurant' | 'customer' | 'courier',
  position: Position,
  label: string,
  detail: string,
  kind: SelectedEntity['kind'],
  signature: string,
  replicas = 1,
  readyReplicas = replicas,
) {
  let state = markers.get(id)
  if (state && state.signature !== signature) {
    state.container.destroy({ children: true })
    markers.delete(id)
    state = undefined
  }
  if (!state) {
    state = createMarker(id, textureName, label, detail, kind, signature, replicas, readyReplicas)
    if (!state) return
    const initial = iso(position.x, position.y)
    state.container.position.set(initial.x, initial.y + 14)
    markers.set(id, state)
  }
  const point = iso(position.x, position.y)
  state.targetX = point.x
  state.targetY = point.y + 14
  state.caption.text = label
  state.status.text = detail.toUpperCase()
  state.container.zIndex = Math.round((position.x + position.y) * 100)
  state.container.alpha = 1
}

function courierLabel(courier: Courier) {
  return {
    idle: 'Verfügbar',
    to_restaurant: 'Zur Abholung',
    picking_up: 'Holt ab',
    to_customer: 'Zum Kunden',
  }[courier.status] || courier.status
}

function renderEntities() {
  if (!entityLayer || !routeLayer) return
  const seen = new Set<string>()

  for (const restaurant of props.snapshot.restaurants) {
    seen.add(restaurant.id)
    const replicas = Math.max(0, restaurant.replicas || 0)
    const ready = Math.max(0, restaurant.ready_replicas || 0)
    upsertMarker(restaurant.id, 'restaurant', restaurant.position, restaurant.name, `${ready}/${replicas} Pods`, 'restaurant', `restaurant:${replicas}:${ready}`, replicas, ready)
  }
  for (const customer of props.snapshot.customers) {
    seen.add(customer.id)
    upsertMarker(customer.id, 'customer', customer.position, customer.name, customer.status === 'draining' ? 'Draining' : 'Kunden-Pod', 'customer', 'customer')
  }
  for (const courier of props.snapshot.couriers) {
    seen.add(courier.id)
    upsertMarker(courier.id, 'courier', courier.position, courier.name, courierLabel(courier), 'courier', 'courier')
  }

  for (const [id, state] of markers) {
    if (seen.has(id)) continue
    state.container.destroy({ children: true })
    markers.delete(id)
  }
  renderRoutes()
}

function renderRoutes() {
  if (!routeLayer) return
  routeLayer.clear()
  for (const courier of props.snapshot.couriers) {
    if (!courier.order_id || courier.status === 'idle') continue
    const order = props.snapshot.orders.find(item => item.id === courier.order_id)
    if (!order) continue
    const target = courier.status === 'to_restaurant' || courier.status === 'picking_up'
      ? props.snapshot.restaurants.find(item => item.id === order.restaurant_id)?.position
      : props.snapshot.customers.find(item => item.id === order.customer_id)?.position
    if (!target) continue
    const color = courier.status === 'to_restaurant' ? 0xe8cc55 : 0x4ee0c3
    const path = roadPath(courier.position, target)
    for (let index = 1; index < path.length; index++) drawDashedIsoLine(routeLayer, path[index - 1]!, path[index]!, color)
    const endpoint = iso(target.x, target.y)
    routeLayer.circle(endpoint.x, endpoint.y + 14, 10).stroke({ color, width: 2, alpha: 0.8 })
    routeLayer.circle(endpoint.x, endpoint.y + 14, 3).fill({ color, alpha: 0.85 })
  }
}

function drawDashedIsoLine(graphics: Graphics, from: Position, to: Position, color: number) {
  const start = iso(from.x, from.y)
  const end = iso(to.x, to.y)
  const dx = end.x - start.x
  const dy = end.y - start.y
  const distance = Math.hypot(dx, dy)
  const dash = 12
  const gap = 7
  for (let offset = 0; offset < distance; offset += dash + gap) {
    const finish = Math.min(offset + dash, distance)
    graphics.moveTo(start.x + dx * offset / distance, start.y + 14 + dy * offset / distance)
      .lineTo(start.x + dx * finish / distance, start.y + 14 + dy * finish / distance)
  }
  graphics.stroke({ color, width: 3, alpha: 0.82 })
}

function fitScale() {
  if (!host.value) return 0.44
  const widthFit = (host.value.clientWidth - 54) / cityWidth
  const heightFit = (host.value.clientHeight - 130) / cityHeight
  const minimum = host.value.clientWidth < 600 ? 0.4 : 0.36
  return Math.max(minimum, Math.min(0.72, widthFit * 1.15, heightFit * 1.15))
}

function applyCamera() {
  if (!cityRoot || !host.value) return
  const verticalCenter = (host.value.clientHeight - cityHeight * scale) / 2 + 24
  cityRoot.position.set(host.value.clientWidth / 2 + pan.x, Math.max(102, verticalCenter) + pan.y)
  cityRoot.scale.set(scale)
}

function resetCamera() {
  pan = { x: 0, y: 0 }
  scale = fitScale()
  applyCamera()
}

function zoomIn() {
  scale = Math.min(1.2, scale * 1.18)
  applyCamera()
}

function zoomOut() {
  scale = Math.max(0.24, scale / 1.18)
  applyCamera()
}

async function setup() {
  renderState.value = 'initializing'
  if (!host.value) throw new Error('City canvas host is unavailable')
  app = new Application()
  await app.init({ preference: 'webgl', resizeTo: host.value, antialias: true, backgroundAlpha: 0, autoDensity: true, resolution: Math.min(window.devicePixelRatio, 2) })
  host.value.appendChild(app.canvas)
  app.canvas.setAttribute('aria-label', 'Isometrische Live-Stadtkarte mit Kubernetes-skalierten Entitäten')

  const loaded = await Promise.all([
    Assets.load<Texture>('/sprites/restaurant.svg'),
    Assets.load<Texture>('/sprites/customer.svg'),
    Assets.load<Texture>('/sprites/courier.svg'),
  ])
  textures.set('restaurant', loaded[0])
  textures.set('customer', loaded[1])
  textures.set('courier', loaded[2])

  cityRoot = new Container()
  app.stage.addChild(cityRoot)
  drawCity()
  renderEntities()
  resetCamera()

  app.ticker.add(() => {
    const now = performance.now()
    for (const state of markers.values()) {
      state.container.x += (state.targetX - state.container.x) * 0.105
      state.container.y += (state.targetY - state.container.y) * 0.105
    }
    for (let index = 0; index < streetLights.length; index++) {
      streetLights[index]!.alpha = 0.48 + Math.sin(now * 0.002 + index) * 0.28
    }
  })

  app.canvas.addEventListener('wheel', onWheel, { passive: false })
  app.canvas.addEventListener('pointerdown', onPointerDown)
  window.addEventListener('pointermove', onPointerMove)
  window.addEventListener('pointerup', onPointerUp)
  window.addEventListener('resize', applyCamera)
  renderState.value = 'ready'
}

function onWheel(event: WheelEvent) {
  event.preventDefault()
  scale = Math.max(0.24, Math.min(1.2, scale - event.deltaY * 0.0007))
  applyCamera()
}

function onPointerDown(event: PointerEvent) {
  dragging = true
  dragOrigin = { x: event.clientX - pan.x, y: event.clientY - pan.y }
}

function onPointerMove(event: PointerEvent) {
  if (!dragging) return
  pan = { x: event.clientX - dragOrigin.x, y: event.clientY - dragOrigin.y }
  applyCamera()
}

function onPointerUp() {
  dragging = false
}

watch(() => props.snapshot, renderEntities, { deep: true })
defineExpose({ zoomIn, zoomOut, resetCamera })

onMounted(async () => {
  await nextTick()
  try {
    await setup()
  } catch (error) {
    renderState.value = 'failed'
    renderError.value = error instanceof Error ? error.message : 'PixiJS konnte nicht initialisiert werden.'
    console.error('CityStage initialization failed', error)
  }
})

onBeforeUnmount(() => {
  app?.canvas.removeEventListener('wheel', onWheel)
  app?.canvas.removeEventListener('pointerdown', onPointerDown)
  window.removeEventListener('pointermove', onPointerMove)
  window.removeEventListener('pointerup', onPointerUp)
  window.removeEventListener('resize', applyCamera)
  app?.destroy(true, { children: true, texture: false })
})
</script>

<template>
  <div ref="host" class="city-canvas" :data-render-state="renderState">
    <div v-if="renderError" class="canvas-error">{{ renderError }}</div>
  </div>
</template>
