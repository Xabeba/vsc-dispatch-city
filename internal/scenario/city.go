package scenario

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/teko/food-delivery/internal/events"
	"github.com/teko/food-delivery/internal/model"
)

const (
	// CityGridSize is the number of tiles along one side of Dispatch City.
	CityGridSize = 21
	// RoadSpacing places a connected road on every fourth row and column.
	RoadSpacing = 4
)

// Restaurants is the deterministic city configuration shared by producers.
var Restaurants = []model.Restaurant{
	{ID: "restaurant-pizza", Name: "Luna Pizza", Cuisine: "Pizza", Position: model.Position{X: 4, Y: 4}, Status: "online", Replicas: 1, ReadyReplicas: 1},
	{ID: "restaurant-bowl", Name: "Green Bowl", Cuisine: "Bowls", Position: model.Position{X: 16, Y: 4}, Status: "online", Replicas: 1, ReadyReplicas: 1},
	{ID: "restaurant-curry", Name: "Curry Circuit", Cuisine: "Curry", Position: model.Position{X: 12, Y: 16}, Status: "online", Replicas: 1, ReadyReplicas: 1},
}

var customerPositions = []model.Position{
	{X: 20, Y: 8}, {X: 0, Y: 16}, {X: 16, Y: 20}, {X: 8, Y: 0},
	{X: 4, Y: 12}, {X: 12, Y: 8}, {X: 20, Y: 16}, {X: 8, Y: 20},
}

var courierPositions = []model.Position{
	{X: 0, Y: 4}, {X: 8, Y: 8}, {X: 20, Y: 12}, {X: 12, Y: 20},
	{X: 4, Y: 16}, {X: 16, Y: 0}, {X: 8, Y: 20}, {X: 20, Y: 4},
}

var customerNames = []string{"Sam", "Aya", "Leo", "Nora", "Ivy", "Milo", "Zoé", "Finn"}
var courierNames = []string{"Mia", "Noah", "Lina", "Elio", "Juna", "Timo", "Ada", "Nico"}

// NewOrder creates a repeatable demo order for the given sequence number.
func NewOrder(sequence uint64, source string) (model.Order, model.EventEnvelope, error) {
	return NewOrderForCustomer(sequence, source, CustomerForOrdinal(int(sequence)))
}

// NewOrderForCustomer creates an order for a stable customer agent.
func NewOrderForCustomer(sequence uint64, source string, customer model.Customer) (model.Order, model.EventEnvelope, error) {
	restaurant := Restaurants[int(sequence-1)%len(Restaurants)]
	now := time.Now().UTC()
	order := model.Order{
		ID:           uuid.NewString(),
		CustomerID:   customer.ID,
		RestaurantID: restaurant.ID,
		Status:       "created",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	event, err := events.New("order.created", order.ID, "", source, events.OrderCreated{Order: order, Customer: customer, Restaurant: restaurant})
	return order, event, err
}

// CustomerForOrdinal returns the stable visual identity for one simulator pod.
func CustomerForOrdinal(ordinal int) model.Customer {
	index := positiveIndex(ordinal-1, len(customerPositions))
	return model.Customer{
		ID:       fmt.Sprintf("customer-agent-%d", ordinal),
		Name:     customerNames[index],
		Position: customerPositions[index],
		Status:   "active",
	}
}

// CourierForOrdinal returns the stable fleet identity for one StatefulSet pod.
func CourierForOrdinal(ordinal int) model.Courier {
	index := positiveIndex(ordinal-1, len(courierPositions))
	return model.Courier{
		ID:       fmt.Sprintf("courier-%d", ordinal),
		Name:     courierNames[index],
		Position: courierPositions[index],
		Status:   "idle",
	}
}

// OrdinalFromPodName converts a StatefulSet pod name ending in -0 to ordinal 1.
func OrdinalFromPodName(podName string) int {
	lastDash := strings.LastIndexByte(podName, '-')
	if lastDash < 0 || lastDash == len(podName)-1 {
		return 1
	}
	value, err := strconv.Atoi(podName[lastDash+1:])
	if err != nil || value < 0 {
		return 1
	}
	return value + 1
}

// RoadPath returns orthogonal waypoints that never leave the connected road grid.
func RoadPath(from, to model.Position) []model.Position {
	from = SnapToRoad(from)
	to = SnapToRoad(to)
	path := []model.Position{from}
	if samePosition(from, to) {
		return path
	}

	var corner model.Position
	if isRoadCoordinate(from.Y) {
		corner = model.Position{X: to.X, Y: from.Y}
	} else {
		corner = model.Position{X: from.X, Y: to.Y}
	}
	if !samePosition(corner, from) && !samePosition(corner, to) {
		path = append(path, corner)
	}
	return append(path, to)
}

// SnapToRoad moves an arbitrary point to its nearest road tile.
func SnapToRoad(position model.Position) model.Position {
	if isRoadCoordinate(position.X) || isRoadCoordinate(position.Y) {
		return clampPosition(position)
	}
	xRoad := nearestRoad(position.X)
	yRoad := nearestRoad(position.Y)
	if math.Abs(position.X-xRoad) <= math.Abs(position.Y-yRoad) {
		position.X = xRoad
	} else {
		position.Y = yRoad
	}
	return clampPosition(position)
}

// IsOnRoad reports whether a point is on a horizontal or vertical road.
func IsOnRoad(position model.Position) bool {
	return isRoadCoordinate(position.X) || isRoadCoordinate(position.Y)
}

func nearestRoad(value float64) float64 {
	return math.Round(value/RoadSpacing) * RoadSpacing
}

func isRoadCoordinate(value float64) bool {
	return math.Abs(value-nearestRoad(value)) < 0.001
}

func clampPosition(position model.Position) model.Position {
	maximum := float64(CityGridSize - 1)
	position.X = math.Max(0, math.Min(maximum, position.X))
	position.Y = math.Max(0, math.Min(maximum, position.Y))
	return position
}

func samePosition(left, right model.Position) bool {
	return math.Abs(left.X-right.X) < 0.001 && math.Abs(left.Y-right.Y) < 0.001
}

func positiveIndex(value, size int) int {
	if value < 0 {
		return 0
	}
	return value % size
}
