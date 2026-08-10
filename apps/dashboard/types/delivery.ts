export interface Position {
  x: number
  y: number
}

export interface Restaurant {
  id: string
  name: string
  cuisine: string
  position: Position
  status: string
  replicas: number
  ready_replicas: number
}

export interface Customer {
  id: string
  name: string
  position: Position
  status?: string
  pod_name?: string
}

export interface Courier {
  id: string
  name: string
  position: Position
  status: string
  order_id?: string
  pod_name?: string
}

export interface Order {
  id: string
  customer_id: string
  restaurant_id: string
  courier_id?: string
  status: 'created' | 'accepted' | 'courier_to_restaurant' | 'picked_up' | 'in_transit' | 'delivered' | 'failed'
  created_at: string
  updated_at: string
  progress: number
}

export interface ComponentStatus {
  id: string
  name: string
  kind: string
  status: string
  ready: number
  desired: number
  detail: string
  category: string
  entity_kind?: 'restaurant' | 'courier' | 'customer'
  entity_id?: string
}

export interface Snapshot {
  mode: string
  running: boolean
  tick: number
  instance: string
  restaurants: Restaurant[]
  customers: Customer[]
  couriers: Courier[]
  orders: Order[]
  components: ComponentStatus[]
  stats: {
    active_orders: number
    delivered: number
    events: number
    ready_pods: number
    total_pods: number
  }
}

export interface EventEnvelope {
  event_id: string
  event_type: string
  event_version: number
  occurred_at: string
  correlation_id: string
  source: string
  payload: Record<string, unknown>
}

export type SelectedEntity =
  | { kind: 'restaurant'; id: string; label: string; detail: string }
  | { kind: 'courier'; id: string; label: string; detail: string }
  | { kind: 'customer'; id: string; label: string; detail: string }
