package simulation

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/teko/food-delivery/internal/events"
	"github.com/teko/food-delivery/internal/model"
	"github.com/teko/food-delivery/internal/scenario"
)

const maxVisibleOrders = 18

type trackedOrder struct {
	order model.Order
	age   int
}

// Engine owns the deterministic standalone simulation and its subscribers.
type Engine struct {
	mu          sync.RWMutex
	mode        string
	instance    string
	persistent  bool
	running     bool
	tick        uint64
	eventCount  int
	delivered   int
	sequence    int
	restaurants []model.Restaurant
	customers   map[string]model.Customer
	couriers    map[string]model.Courier
	orders      map[string]*trackedOrder
	components  []model.Component
	subscribers map[chan model.EventEnvelope]struct{}
}

// NewEngine returns an initialized simulation with a stable city layout.
func NewEngine(mode, instance string) *Engine {
	restaurants := append([]model.Restaurant(nil), scenario.Restaurants...)
	couriers := make(map[string]model.Courier, 4)
	for ordinal := 1; ordinal <= 4; ordinal++ {
		courier := scenario.CourierForOrdinal(ordinal)
		couriers[courier.ID] = courier
	}
	return &Engine{
		mode:        mode,
		instance:    instance,
		running:     true,
		restaurants: restaurants,
		customers:   map[string]model.Customer{},
		couriers:    couriers,
		orders:      map[string]*trackedOrder{},
		subscribers: map[chan model.EventEnvelope]struct{}{},
	}
}

// Run advances the world until the context is cancelled.
func (e *Engine) Run(ctx context.Context, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("simulation stopped: %w", ctx.Err())
		case <-ticker.C:
			e.advance()
		}
	}
}

// SetRunning pauses or resumes the simulation.
func (e *Engine) SetRunning(running bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.running = running
}

// SetPersistent marks PostgreSQL as an active dependency in the system view.
func (e *Engine) SetPersistent(persistent bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.persistent = persistent
}

// ReplaceComponents switches the system view from planned to live cluster data.
func (e *Engine) ReplaceComponents(components []model.Component) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.components = append([]model.Component(nil), components...)
	e.reconcileScaledEntitiesLocked(components)
}

// Hydrate replaces the durable projection after an API restart.
func (e *Engine) Hydrate(snapshot model.Snapshot) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.customers = make(map[string]model.Customer, len(snapshot.Customers))
	for _, customer := range snapshot.Customers {
		e.customers[customer.ID] = customer
	}
	if len(snapshot.Couriers) > 0 {
		e.couriers = make(map[string]model.Courier, len(snapshot.Couriers))
		for _, courier := range snapshot.Couriers {
			e.couriers[courier.ID] = courier
		}
	}
	e.orders = make(map[string]*trackedOrder, len(snapshot.Orders))
	e.delivered = 0
	for _, order := range snapshot.Orders {
		e.orders[order.ID] = &trackedOrder{order: order}
		if order.Status == "delivered" {
			e.delivered++
		}
	}
}

// Reset clears all orders while retaining the configured city.
func (e *Engine) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.resetLocked()
}

// CreateOrder inserts a manual order and emits order.created.
func (e *Engine) CreateOrder() model.Order {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.createOrderLocked()
}

// ApplyEvent updates the distributed read model and forwards the original event.
func (e *Engine) ApplyEvent(event model.EventEnvelope) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	switch event.Type {
	case "customer.registered":
		payload, err := events.Decode[events.CustomerRegistered](event)
		if err != nil {
			return err
		}
		e.customers[payload.Customer.ID] = payload.Customer
	case "courier.registered":
		payload, err := events.Decode[events.CourierRegistered](event)
		if err != nil {
			return err
		}
		if current, ok := e.couriers[payload.Courier.ID]; ok {
			current.PodName = payload.Courier.PodName
			payload.Courier = current
		}
		e.couriers[payload.Courier.ID] = payload.Courier
	case "order.created":
		payload, err := events.Decode[events.OrderCreated](event)
		if err != nil {
			return err
		}
		e.upsertRestaurantLocked(payload.Restaurant)
		e.customers[payload.Customer.ID] = payload.Customer
		e.orders[payload.Order.ID] = &trackedOrder{order: payload.Order}
	case "order.accepted", "order.rejected":
		payload, err := events.Decode[events.OrderAccepted](event)
		if err != nil {
			return err
		}
		e.upsertRestaurantLocked(payload.Restaurant)
		e.customers[payload.Customer.ID] = payload.Customer
		e.orders[payload.Order.ID] = &trackedOrder{order: payload.Order}
	case "courier.assigned":
		payload, err := events.Decode[events.CourierAssigned](event)
		if err != nil {
			return err
		}
		e.orders[payload.Order.ID] = &trackedOrder{order: payload.Order}
		e.couriers[payload.Courier.ID] = payload.Courier
	case "courier.location.updated":
		payload, err := events.Decode[events.CourierLocation](event)
		if err != nil {
			return err
		}
		e.couriers[payload.Courier.ID] = payload.Courier
		if tracked, ok := e.orders[payload.OrderID]; ok {
			tracked.order.Progress = payload.Progress
			if payload.Courier.Status == "to_customer" {
				tracked.order.Status = "in_transit"
			}
			tracked.order.UpdatedAt = event.OccurredAt
		}
	case "order.picked_up":
		payload, err := events.Decode[events.OrderPickedUp](event)
		if err != nil {
			return err
		}
		e.orders[payload.Order.ID] = &trackedOrder{order: payload.Order}
		e.couriers[payload.Courier.ID] = payload.Courier
	case "order.delivered":
		payload, err := events.Decode[events.OrderDelivered](event)
		if err != nil {
			return err
		}
		if tracked, ok := e.orders[payload.Order.ID]; !ok || tracked.order.Status != "delivered" {
			e.delivered++
		}
		e.orders[payload.Order.ID] = &trackedOrder{order: payload.Order}
		e.couriers[payload.Courier.ID] = payload.Courier
	case "simulation.reset":
		e.resetLocked()
	case "simulation.started":
		e.running = true
	case "simulation.paused":
		e.running = false
	}

	e.eventCount++
	for subscriber := range e.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
	return nil
}

// Subscribe registers a buffered event subscriber.
func (e *Engine) Subscribe() (<-chan model.EventEnvelope, func()) {
	ch := make(chan model.EventEnvelope, 64)
	e.mu.Lock()
	e.subscribers[ch] = struct{}{}
	e.mu.Unlock()

	return ch, func() {
		e.mu.Lock()
		if _, ok := e.subscribers[ch]; ok {
			delete(e.subscribers, ch)
			close(ch)
		}
		e.mu.Unlock()
	}
}

// Snapshot returns a sorted copy of the current state.
func (e *Engine) Snapshot() model.Snapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()

	activeCustomerIDs := make(map[string]struct{})
	for _, tracked := range e.orders {
		if tracked.order.Status != "delivered" && tracked.order.Status != "failed" {
			activeCustomerIDs[tracked.order.CustomerID] = struct{}{}
		}
	}
	customerReplicas := e.desiredReplicasLocked("customer-simulator")
	customers := make([]model.Customer, 0, len(e.customers))
	for _, customer := range e.customers {
		ordinal, agent := entityOrdinal(customer.ID, "customer-agent-")
		_, requiredForOrder := activeCustomerIDs[customer.ID]
		if e.mode == "distributed" && agent && customerReplicas >= 0 && ordinal > customerReplicas && !requiredForOrder {
			continue
		}
		if e.mode == "distributed" && agent && customerReplicas >= 0 && ordinal > customerReplicas {
			customer.Status = "draining"
		}
		if e.mode == "distributed" && !agent && !requiredForOrder {
			continue
		}
		customers = append(customers, customer)
	}
	sort.Slice(customers, func(i, j int) bool { return customers[i].ID < customers[j].ID })

	couriers := make([]model.Courier, 0, len(e.couriers))
	for _, courier := range e.couriers {
		couriers = append(couriers, courier)
	}
	sort.Slice(couriers, func(i, j int) bool { return couriers[i].ID < couriers[j].ID })

	orders := make([]model.Order, 0, len(e.orders))
	active := 0
	for _, tracked := range e.orders {
		orders = append(orders, tracked.order)
		if tracked.order.Status != "delivered" && tracked.order.Status != "failed" {
			active++
		}
	}
	sort.Slice(orders, func(i, j int) bool { return orders[i].CreatedAt.After(orders[j].CreatedAt) })
	if len(orders) > maxVisibleOrders {
		orders = orders[:maxVisibleOrders]
	}

	apiReplicas := 1
	postgresReady := 0
	postgresStatus := "planned"
	if e.persistent {
		apiReplicas = 2
		postgresReady = 2
		postgresStatus = "healthy"
	}
	components := []model.Component{
		{ID: "dashboard", Name: "Dashboard", Kind: "Deployment", Status: "healthy", Ready: 2, Desired: 2, Detail: "Nuxt + PixiJS", Category: "edge"},
		{ID: "control-api", Name: "Control API", Kind: "Deployment", Status: "healthy", Ready: apiReplicas, Desired: apiReplicas, Detail: e.mode + " / SSE", Category: "service"},
		{ID: "rabbitmq", Name: "RabbitMQ", Kind: "StatefulSet", Status: statusForMode(e.mode, "healthy", "planned"), Ready: readyForMode(e.mode), Desired: 1, Detail: "food.events", Category: "platform"},
		{ID: "customer-simulator", Name: "Customer Simulator", Kind: "Deployment", Status: statusForMode(e.mode, "healthy", "planned"), Ready: readyForMode(e.mode), Desired: readyForMode(e.mode), Detail: "order producer", Category: "service"},
		{ID: "restaurant-workers", Name: "Restaurant Workers", Kind: "Deployments", Status: statusForMode(e.mode, "healthy", "planned"), Ready: readyForMode(e.mode) * 3, Desired: readyForMode(e.mode) * 3, Detail: "3 restaurant queues", Category: "service"},
		{ID: "courier-simulator", Name: "Courier Simulator", Kind: "Deployment", Status: statusForMode(e.mode, "healthy", "planned"), Ready: readyForMode(e.mode), Desired: readyForMode(e.mode), Detail: "courier dispatch", Category: "service"},
		{ID: "order-worker", Name: "Order Worker", Kind: "Deployment", Status: statusForMode(e.mode, "healthy", "planned"), Ready: readyForMode(e.mode) * 2, Desired: readyForMode(e.mode) * 2, Detail: "event projection", Category: "service"},
		{ID: "postgres", Name: "PostgreSQL", Kind: "CNPG Cluster", Status: postgresStatus, Ready: postgresReady, Desired: 2, Detail: "primary / standby", Category: "data"},
	}
	if len(e.components) > 0 {
		components = append([]model.Component(nil), e.components...)
	}

	readyPods := 0
	totalPods := 0
	for _, component := range components {
		readyPods += component.Ready
		totalPods += component.Desired
	}

	return model.Snapshot{
		Mode:        e.mode,
		Running:     e.running,
		Tick:        e.tick,
		Instance:    e.instance,
		Restaurants: append([]model.Restaurant(nil), e.restaurants...),
		Customers:   customers,
		Couriers:    couriers,
		Orders:      orders,
		Components:  components,
		Stats: model.Stats{
			ActiveOrders: active,
			Delivered:    e.delivered,
			Events:       e.eventCount,
			ReadyPods:    readyPods,
			TotalPods:    totalPods,
		},
	}
}

func (e *Engine) advance() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running || e.mode == "distributed" {
		return
	}

	e.tick++
	if e.tick == 1 || e.tick%12 == 0 {
		e.createOrderLocked()
	}

	for _, tracked := range e.orders {
		tracked.age++
		switch tracked.order.Status {
		case "created":
			if tracked.age >= 3 {
				tracked.age = 0
				tracked.order.Status = "accepted"
				tracked.order.UpdatedAt = time.Now().UTC()
				e.emitLocked("order.accepted", tracked.order.ID, "restaurant-worker", tracked.order)
			}
		case "accepted":
			if courierID, ok := e.availableCourierLocked(); ok && tracked.age >= 2 {
				tracked.age = 0
				tracked.order.Status = "courier_to_restaurant"
				tracked.order.CourierID = courierID
				tracked.order.UpdatedAt = time.Now().UTC()
				courier := e.couriers[courierID]
				courier.Status = "to_restaurant"
				courier.OrderID = tracked.order.ID
				e.couriers[courierID] = courier
				e.emitLocked("courier.assigned", tracked.order.ID, "courier-simulator", map[string]any{"order": tracked.order, "courier": courier})
			}
		case "courier_to_restaurant":
			e.moveCourierToRestaurantLocked(tracked)
		case "picked_up":
			if tracked.age >= 2 {
				tracked.age = 0
				tracked.order.Status = "in_transit"
				courier := e.couriers[tracked.order.CourierID]
				courier.Status = "to_customer"
				e.couriers[courier.ID] = courier
			}
		case "in_transit":
			e.moveCourierToCustomerLocked(tracked)
		}
	}
}

func (e *Engine) upsertRestaurantLocked(restaurant model.Restaurant) {
	for index := range e.restaurants {
		if e.restaurants[index].ID == restaurant.ID {
			e.restaurants[index] = restaurant
			return
		}
	}
	e.restaurants = append(e.restaurants, restaurant)
}

func (e *Engine) resetLocked() {
	e.tick = 0
	e.eventCount = 0
	e.delivered = 0
	e.running = true
	for id := range e.customers {
		if e.mode != "distributed" || !strings.HasPrefix(id, "customer-agent-") {
			delete(e.customers, id)
		}
	}
	e.orders = map[string]*trackedOrder{}
	for id, courier := range e.couriers {
		courier.Status = "idle"
		courier.OrderID = ""
		e.couriers[id] = courier
	}
}

func (e *Engine) createOrderLocked() model.Order {
	e.sequence++
	restaurant := e.restaurants[(e.sequence-1)%len(e.restaurants)]
	customerID := fmt.Sprintf("customer-%03d", e.sequence)
	positions := []model.Position{{X: 20, Y: 8}, {X: 0, Y: 16}, {X: 16, Y: 20}, {X: 8, Y: 0}, {X: 4, Y: 12}, {X: 12, Y: 8}}
	names := []string{"Sam", "Aya", "Leo", "Nora", "Ivy", "Milo"}
	position := positions[(e.sequence-1)%len(positions)]
	if position.X != 0 && position.X != scenario.CityGridSize-1 {
		position.X += rand.Float64()*0.3 - 0.15
	}
	position = scenario.SnapToRoad(position)
	customer := model.Customer{ID: customerID, Name: names[(e.sequence-1)%len(names)], Position: position}
	e.customers[customerID] = customer

	now := time.Now().UTC()
	order := model.Order{
		ID:           uuid.NewString(),
		CustomerID:   customer.ID,
		RestaurantID: restaurant.ID,
		Status:       "created",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	e.orders[order.ID] = &trackedOrder{order: order}
	e.emitLocked("order.created", order.ID, "customer-simulator", map[string]any{"order": order, "customer": customer, "restaurant": restaurant})
	return order
}

func (e *Engine) availableCourierLocked() (string, bool) {
	ids := make([]string, 0, len(e.couriers))
	for id := range e.couriers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if e.couriers[id].Status == "idle" {
			return id, true
		}
	}
	return "", false
}

func (e *Engine) moveCourierToRestaurantLocked(tracked *trackedOrder) {
	courier := e.couriers[tracked.order.CourierID]
	restaurant, ok := e.restaurantLocked(tracked.order.RestaurantID)
	if !ok {
		return
	}
	if e.moveCourierTowardsLocked(tracked, &courier, restaurant.Position, 0, 0.45) {
		courier.Position = restaurant.Position
		courier.Status = "picking_up"
		e.couriers[courier.ID] = courier
		tracked.age = 0
		tracked.order.Status = "picked_up"
		tracked.order.Progress = 0.5
		tracked.order.UpdatedAt = time.Now().UTC()
		e.emitLocked("order.picked_up", tracked.order.ID, "courier-simulator", map[string]any{"order": tracked.order, "courier": courier})
	}
}

func (e *Engine) moveCourierToCustomerLocked(tracked *trackedOrder) {
	courier := e.couriers[tracked.order.CourierID]
	customer, ok := e.customers[tracked.order.CustomerID]
	if !ok {
		return
	}
	if e.moveCourierTowardsLocked(tracked, &courier, customer.Position, 0.5, 0.98) {
		courier.Position = customer.Position
		courier.Status = "idle"
		courier.OrderID = ""
		e.couriers[courier.ID] = courier
		tracked.order.Status = "delivered"
		tracked.order.Progress = 1
		tracked.order.UpdatedAt = time.Now().UTC()
		e.delivered++
		e.emitLocked("order.delivered", tracked.order.ID, "courier-simulator", map[string]any{"order": tracked.order, "courier": courier})
	}
}

func (e *Engine) moveCourierTowardsLocked(tracked *trackedOrder, courier *model.Courier, target model.Position, progressStart, progressEnd float64) bool {
	path := scenario.RoadPath(courier.Position, target)
	if len(path) < 2 {
		return true
	}
	next := path[1]
	dx := next.X - courier.Position.X
	dy := next.Y - courier.Position.Y
	distance := math.Hypot(dx, dy)
	step := math.Min(0.62, distance)
	if distance > 0 {
		courier.Position.X += dx / distance * step
		courier.Position.Y += dy / distance * step
	}
	e.couriers[courier.ID] = *courier
	tracked.order.Progress = math.Min(progressEnd, math.Max(progressStart, tracked.order.Progress)+0.025)
	tracked.order.UpdatedAt = time.Now().UTC()
	e.emitLocked("courier.location.updated", tracked.order.ID, "courier-simulator", map[string]any{"order_id": tracked.order.ID, "courier": courier, "progress": tracked.order.Progress})
	return math.Hypot(target.X-courier.Position.X, target.Y-courier.Position.Y) < 0.2
}

func (e *Engine) reconcileScaledEntitiesLocked(components []model.Component) {
	if e.mode != "distributed" {
		return
	}
	desired := make(map[string]int)
	for _, component := range components {
		if component.Desired > desired[component.ID] {
			desired[component.ID] = component.Desired
		}
		if strings.HasPrefix(component.ID, "restaurant-") {
			for index := range e.restaurants {
				if e.restaurants[index].ID == component.ID {
					e.restaurants[index].Replicas = component.Desired
					e.restaurants[index].ReadyReplicas = component.Ready
					if component.Ready == 0 {
						e.restaurants[index].Status = "offline"
					} else {
						e.restaurants[index].Status = "online"
					}
				}
			}
		}
	}
	for ordinal := 1; ordinal <= desired["courier-simulator"]; ordinal++ {
		courier := scenario.CourierForOrdinal(ordinal)
		if _, ok := e.couriers[courier.ID]; !ok {
			e.couriers[courier.ID] = courier
		}
	}
	for id := range e.couriers {
		ordinal, managed := entityOrdinal(id, "courier-")
		if managed && ordinal > desired["courier-simulator"] {
			delete(e.couriers, id)
		}
	}
	for ordinal := 1; ordinal <= desired["customer-simulator"]; ordinal++ {
		customer := scenario.CustomerForOrdinal(ordinal)
		if _, ok := e.customers[customer.ID]; !ok {
			e.customers[customer.ID] = customer
		}
	}
}

func (e *Engine) desiredReplicasLocked(id string) int {
	if len(e.components) == 0 {
		return -1
	}
	desired := 0
	for _, component := range e.components {
		if component.ID == id && component.Desired > desired {
			desired = component.Desired
		}
	}
	return desired
}

func entityOrdinal(id, prefix string) (int, bool) {
	if !strings.HasPrefix(id, prefix) {
		return 0, false
	}
	ordinal, err := strconv.Atoi(strings.TrimPrefix(id, prefix))
	return ordinal, err == nil && ordinal > 0
}

func (e *Engine) restaurantLocked(id string) (model.Restaurant, bool) {
	for _, restaurant := range e.restaurants {
		if restaurant.ID == id {
			return restaurant, true
		}
	}
	return model.Restaurant{}, false
}

func (e *Engine) emitLocked(eventType, correlationID, source string, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	event := model.EventEnvelope{
		ID:            uuid.NewString(),
		Type:          eventType,
		Version:       1,
		OccurredAt:    time.Now().UTC(),
		CorrelationID: correlationID,
		Source:        source,
		Payload:       raw,
	}
	e.eventCount++
	for subscriber := range e.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
}

func statusForMode(mode, distributed, standalone string) string {
	if mode == "distributed" {
		return distributed
	}
	return standalone
}

func readyForMode(mode string) int {
	if mode == "distributed" {
		return 1
	}
	return 0
}
