package simulation

import (
	"testing"

	"github.com/teko/food-delivery/internal/model"
	"github.com/teko/food-delivery/internal/scenario"
)

func TestCreateOrderAddsActiveOrder(t *testing.T) {
	t.Parallel()
	engine := NewEngine("standalone", "test")

	order := engine.CreateOrder()
	snapshot := engine.Snapshot()

	if order.Status != "created" {
		t.Fatalf("status = %q, want created", order.Status)
	}
	if snapshot.Stats.ActiveOrders != 1 {
		t.Fatalf("active orders = %d, want 1", snapshot.Stats.ActiveOrders)
	}
	if len(snapshot.Customers) != 1 {
		t.Fatalf("customers = %d, want 1", len(snapshot.Customers))
	}
}

func TestResetClearsOrders(t *testing.T) {
	t.Parallel()
	engine := NewEngine("standalone", "test")
	engine.CreateOrder()

	engine.Reset()
	snapshot := engine.Snapshot()

	if len(snapshot.Orders) != 0 || len(snapshot.Customers) != 0 {
		t.Fatalf("reset retained state: %d orders, %d customers", len(snapshot.Orders), len(snapshot.Customers))
	}
}

func TestCourierDrivesToRestaurantThenCustomer(t *testing.T) {
	t.Parallel()
	engine := NewEngine("standalone", "test")
	order := engine.CreateOrder()

	engine.mu.Lock()
	tracked := engine.orders[order.ID]
	tracked.order.Status = "accepted"
	tracked.age = 2
	start := engine.couriers["courier-1"].Position
	engine.mu.Unlock()

	engine.advance()
	snapshot := engine.Snapshot()
	assigned := orderByID(t, snapshot, order.ID)
	if assigned.Status != "courier_to_restaurant" {
		t.Fatalf("status after assignment = %q, want courier_to_restaurant", assigned.Status)
	}
	courier := courierByID(t, snapshot, assigned.CourierID)
	if courier.Position != start {
		t.Fatalf("courier teleported from %+v to %+v", start, courier.Position)
	}

	sawPickup := false
	sawDeliveryLeg := false
	for step := 0; step < 120; step++ {
		engine.advance()
		snapshot = engine.Snapshot()
		currentOrder := orderByID(t, snapshot, order.ID)
		courier = courierByID(t, snapshot, currentOrder.CourierID)
		if !scenario.IsOnRoad(courier.Position) {
			t.Fatalf("courier left the road at %+v", courier.Position)
		}
		switch currentOrder.Status {
		case "picked_up":
			sawPickup = true
		case "in_transit":
			sawDeliveryLeg = true
		case "delivered":
			if !sawPickup || !sawDeliveryLeg {
				t.Fatalf("delivery skipped phases: pickup=%t delivery_leg=%t", sawPickup, sawDeliveryLeg)
			}
			customer := customerByID(t, snapshot, currentOrder.CustomerID)
			if courier.Position != customer.Position || courier.OrderID == currentOrder.ID {
				t.Fatalf("delivered courier = %+v, want persistent courier at customer %+v", courier, customer.Position)
			}
			return
		}
	}
	t.Fatal("order was not delivered within 120 simulation steps")
}

func TestClusterReplicaCountsChangeVisibleEntities(t *testing.T) {
	t.Parallel()
	engine := NewEngine("distributed", "test")
	engine.ReplaceComponents([]model.Component{
		{ID: "courier-simulator", Ready: 6, Desired: 6},
		{ID: "customer-simulator", Ready: 5, Desired: 5},
		{ID: "restaurant-pizza", Ready: 3, Desired: 3},
		{ID: "restaurant-bowl", Ready: 1, Desired: 1},
		{ID: "restaurant-curry", Ready: 1, Desired: 1},
	})

	snapshot := engine.Snapshot()
	if len(snapshot.Couriers) != 6 {
		t.Fatalf("couriers = %d, want 6", len(snapshot.Couriers))
	}
	if len(snapshot.Customers) != 5 {
		t.Fatalf("customers = %d, want 5", len(snapshot.Customers))
	}
	for _, restaurant := range snapshot.Restaurants {
		if restaurant.ID == "restaurant-pizza" && (restaurant.Replicas != 3 || restaurant.ReadyReplicas != 3) {
			t.Fatalf("pizza replicas = %d/%d, want 3/3", restaurant.ReadyReplicas, restaurant.Replicas)
		}
	}

	engine.ReplaceComponents([]model.Component{
		{ID: "courier-simulator", Ready: 2, Desired: 2},
		{ID: "customer-simulator", Ready: 1, Desired: 1},
	})
	snapshot = engine.Snapshot()
	if len(snapshot.Couriers) != 2 || len(snapshot.Customers) != 1 {
		t.Fatalf("scaled down entities = %d couriers, %d customers; want 2 and 1", len(snapshot.Couriers), len(snapshot.Customers))
	}
}

func orderByID(t *testing.T, snapshot model.Snapshot, id string) model.Order {
	t.Helper()
	for _, order := range snapshot.Orders {
		if order.ID == id {
			return order
		}
	}
	t.Fatalf("order %s not found", id)
	return model.Order{}
}

func courierByID(t *testing.T, snapshot model.Snapshot, id string) model.Courier {
	t.Helper()
	for _, courier := range snapshot.Couriers {
		if courier.ID == id {
			return courier
		}
	}
	t.Fatalf("courier %s not found", id)
	return model.Courier{}
}

func customerByID(t *testing.T, snapshot model.Snapshot, id string) model.Customer {
	t.Helper()
	for _, customer := range snapshot.Customers {
		if customer.ID == id {
			return customer
		}
	}
	t.Fatalf("customer %s not found", id)
	return model.Customer{}
}
