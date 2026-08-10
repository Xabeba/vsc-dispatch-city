package events

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/teko/food-delivery/internal/model"
)

// OrderCreated is the payload of order.created.
type OrderCreated struct {
	Order      model.Order      `json:"order"`
	Customer   model.Customer   `json:"customer"`
	Restaurant model.Restaurant `json:"restaurant"`
}

// OrderAccepted is the payload of order.accepted and order.rejected.
type OrderAccepted struct {
	Order      model.Order      `json:"order"`
	Customer   model.Customer   `json:"customer"`
	Restaurant model.Restaurant `json:"restaurant"`
}

// CustomerRegistered is emitted when a stable customer producer starts.
type CustomerRegistered struct {
	Customer model.Customer `json:"customer"`
}

// CourierRegistered is emitted when a stable courier pod joins the fleet.
type CourierRegistered struct {
	Courier model.Courier `json:"courier"`
}

// CourierAssigned is the payload of courier.assigned.
type CourierAssigned struct {
	Order   model.Order   `json:"order"`
	Courier model.Courier `json:"courier"`
}

// CourierLocation is the payload of courier.location.updated.
type CourierLocation struct {
	OrderID  string        `json:"order_id"`
	Courier  model.Courier `json:"courier"`
	Progress float64       `json:"progress"`
}

// OrderPickedUp marks the transition from pickup travel to customer delivery.
type OrderPickedUp struct {
	Order   model.Order   `json:"order"`
	Courier model.Courier `json:"courier"`
}

// OrderDelivered is the payload of order.delivered.
type OrderDelivered struct {
	Order   model.Order   `json:"order"`
	Courier model.Courier `json:"courier"`
}

// New wraps a typed payload in the common event envelope.
func New(eventType, correlationID, causationID, source string, payload any) (model.EventEnvelope, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return model.EventEnvelope{}, fmt.Errorf("marshal %s payload: %w", eventType, err)
	}
	return model.EventEnvelope{
		ID:            uuid.NewString(),
		Type:          eventType,
		Version:       1,
		OccurredAt:    time.Now().UTC(),
		CorrelationID: correlationID,
		CausationID:   causationID,
		Source:        source,
		Payload:       raw,
	}, nil
}

// RoutingKey returns the topic routing key for an event.
func RoutingKey(event model.EventEnvelope) string {
	if event.Type != "order.created" {
		return event.Type
	}
	var payload OrderCreated
	if err := json.Unmarshal(event.Payload, &payload); err != nil || payload.Restaurant.ID == "" {
		return event.Type + ".unknown"
	}
	return event.Type + "." + payload.Restaurant.ID
}

// Decode unmarshals an event payload into a concrete type.
func Decode[T any](event model.EventEnvelope) (T, error) {
	var value T
	if err := json.Unmarshal(event.Payload, &value); err != nil {
		return value, fmt.Errorf("decode %s payload: %w", event.Type, err)
	}
	return value, nil
}
