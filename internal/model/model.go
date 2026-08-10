package model

import (
	"encoding/json"
	"time"
)

// Position is a point on the simulated city grid.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Restaurant is a configured order destination.
type Restaurant struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Cuisine       string   `json:"cuisine"`
	Position      Position `json:"position"`
	Status        string   `json:"status"`
	Replicas      int      `json:"replicas"`
	ReadyReplicas int      `json:"ready_replicas"`
}

// Customer represents the delivery destination of an order.
type Customer struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Position Position `json:"position"`
	Status   string   `json:"status,omitempty"`
	PodName  string   `json:"pod_name,omitempty"`
}

// Courier is a moving delivery vehicle.
type Courier struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Position Position `json:"position"`
	Status   string   `json:"status"`
	OrderID  string   `json:"order_id,omitempty"`
	PodName  string   `json:"pod_name,omitempty"`
}

// Order is the current projection of a delivery order.
type Order struct {
	ID           string    `json:"id"`
	CustomerID   string    `json:"customer_id"`
	RestaurantID string    `json:"restaurant_id"`
	CourierID    string    `json:"courier_id,omitempty"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Progress     float64   `json:"progress"`
}

// EventEnvelope is the versioned contract shared by every service.
type EventEnvelope struct {
	ID            string          `json:"event_id"`
	Type          string          `json:"event_type"`
	Version       int             `json:"event_version"`
	OccurredAt    time.Time       `json:"occurred_at"`
	CorrelationID string          `json:"correlation_id"`
	CausationID   string          `json:"causation_id,omitempty"`
	Source        string          `json:"source"`
	Payload       json.RawMessage `json:"payload"`
}

// Component is a reduced technical view of a Kubernetes workload.
type Component struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	Ready      int    `json:"ready"`
	Desired    int    `json:"desired"`
	Detail     string `json:"detail"`
	Category   string `json:"category"`
	EntityKind string `json:"entity_kind,omitempty"`
	EntityID   string `json:"entity_id,omitempty"`
}

// Stats contains the counters shown in the operations header.
type Stats struct {
	ActiveOrders int `json:"active_orders"`
	Delivered    int `json:"delivered"`
	Events       int `json:"events"`
	ReadyPods    int `json:"ready_pods"`
	TotalPods    int `json:"total_pods"`
}

// Snapshot is the complete state used to hydrate the dashboard.
type Snapshot struct {
	Mode        string       `json:"mode"`
	Running     bool         `json:"running"`
	Tick        uint64       `json:"tick"`
	Instance    string       `json:"instance"`
	Restaurants []Restaurant `json:"restaurants"`
	Customers   []Customer   `json:"customers"`
	Couriers    []Courier    `json:"couriers"`
	Orders      []Order      `json:"orders"`
	Components  []Component  `json:"components"`
	Stats       Stats        `json:"stats"`
}
