package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/teko/food-delivery/internal/appenv"
	"github.com/teko/food-delivery/internal/events"
	"github.com/teko/food-delivery/internal/messaging"
	"github.com/teko/food-delivery/internal/model"
	"github.com/teko/food-delivery/internal/scenario"
	"github.com/teko/food-delivery/internal/telemetry"
	"github.com/teko/food-delivery/internal/workerutil"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	url := appenv.String("RABBITMQ_URL", "amqp://delivery:delivery@localhost:5672/")
	podName := appenv.String("POD_NAME", "courier-simulator-0")
	courier := scenario.CourierForOrdinal(scenario.OrdinalFromPodName(podName))
	courier.PodName = podName
	metrics := &telemetry.Metrics{Service: "courier-simulator"}
	go workerutil.RunTelemetry(ctx, cancel, metrics, logger)
	publisher, err := messaging.NewPublisher(ctx, url, logger)
	if err != nil {
		logger.Error("connect publisher", "error", err)
		return
	}
	defer workerutil.ClosePublisher(publisher, logger)

	registered, err := events.New("courier.registered", courier.ID, "", "courier-simulator/"+podName, events.CourierRegistered{Courier: courier})
	if err != nil {
		logger.Error("create courier registration", "error", err)
		return
	}
	if err := publisher.Publish(ctx, registered); err != nil {
		logger.Error("publish courier registration", "error", err)
		return
	}
	metrics.Published.Add(1)
	metrics.Ready.Store(true)

	config := messaging.ConsumerConfig{Queue: "courier-dispatch", Bindings: []string{"order.accepted"}, Workers: 1, Prefetch: 1}
	err = messaging.Consume(ctx, url, config, logger, func(ctx context.Context, event model.EventEnvelope) error {
		payload, err := events.Decode[events.OrderAccepted](event)
		if err != nil {
			metrics.Failures.Add(1)
			return err
		}
		metrics.Consumed.Add(1)
		courier.Status = "to_restaurant"
		courier.OrderID = payload.Order.ID
		payload.Order.CourierID = courier.ID
		payload.Order.Status = "courier_to_restaurant"
		payload.Order.Progress = 0
		payload.Order.UpdatedAt = time.Now().UTC()

		assigned, err := events.New("courier.assigned", payload.Order.ID, event.ID, "courier-simulator/"+podName, events.CourierAssigned{Order: payload.Order, Courier: courier})
		if err != nil {
			return err
		}
		if err := publisher.Publish(ctx, assigned); err != nil {
			return err
		}
		metrics.Published.Add(1)

		if err := travel(ctx, publisher, metrics, &courier, payload.Order.ID, payload.Restaurant.Position, "to_restaurant", 0, 0.45, assigned.ID, podName); err != nil {
			return err
		}

		courier.Status = "picking_up"
		payload.Order.Status = "picked_up"
		payload.Order.Progress = 0.5
		payload.Order.UpdatedAt = time.Now().UTC()
		pickedUp, err := events.New("order.picked_up", payload.Order.ID, assigned.ID, "courier-simulator/"+podName, events.OrderPickedUp{Order: payload.Order, Courier: courier})
		if err != nil {
			return err
		}
		if err := publisher.Publish(ctx, pickedUp); err != nil {
			return err
		}
		metrics.Published.Add(1)
		if err := waitFor(ctx, appenv.DurationMS("PICKUP_DELAY_MS", 900)); err != nil {
			return err
		}

		courier.Status = "to_customer"
		payload.Order.Status = "in_transit"
		if err := travel(ctx, publisher, metrics, &courier, payload.Order.ID, payload.Customer.Position, "to_customer", 0.5, 0.98, pickedUp.ID, podName); err != nil {
			return err
		}

		courier.Position = payload.Customer.Position
		courier.Status = "idle"
		courier.OrderID = ""
		payload.Order.Status = "delivered"
		payload.Order.Progress = 1
		payload.Order.UpdatedAt = time.Now().UTC()
		delivered, err := events.New("order.delivered", payload.Order.ID, pickedUp.ID, "courier-simulator/"+podName, events.OrderDelivered{Order: payload.Order, Courier: courier})
		if err != nil {
			return err
		}
		if err := publisher.Publish(ctx, delivered); err != nil {
			metrics.Failures.Add(1)
			return err
		}
		metrics.Published.Add(1)
		logger.Info("order delivered", "order_id", payload.Order.ID, "courier_id", courier.ID)
		return nil
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("courier consumer stopped", "error", err)
	}
}

func travel(
	ctx context.Context,
	publisher *messaging.Publisher,
	metrics *telemetry.Metrics,
	courier *model.Courier,
	orderID string,
	target model.Position,
	phase string,
	progressStart float64,
	progressEnd float64,
	causationID string,
	podName string,
) error {
	path := scenario.RoadPath(courier.Position, target)
	totalDistance := pathDistance(path)
	if totalDistance == 0 {
		courier.Position = target
		return nil
	}
	travelled := 0.0
	for waypointIndex := 1; waypointIndex < len(path); waypointIndex++ {
		waypoint := path[waypointIndex]
		for {
			dx := waypoint.X - courier.Position.X
			dy := waypoint.Y - courier.Position.Y
			distance := math.Hypot(dx, dy)
			if distance < 0.001 {
				courier.Position = waypoint
				break
			}
			if err := waitFor(ctx, appenv.DurationMS("COURIER_TICK_MS", 450)); err != nil {
				return err
			}
			step := math.Min(1, distance)
			courier.Position.X += dx / distance * step
			courier.Position.Y += dy / distance * step
			courier.Status = phase
			travelled += step
			progress := progressStart + (progressEnd-progressStart)*math.Min(1, travelled/totalDistance)
			location, err := events.New("courier.location.updated", orderID, causationID, "courier-simulator/"+podName, events.CourierLocation{OrderID: orderID, Courier: *courier, Progress: progress})
			if err != nil {
				return err
			}
			if err := publisher.Publish(ctx, location); err != nil {
				metrics.Failures.Add(1)
				return err
			}
			metrics.Published.Add(1)
			causationID = location.ID
		}
	}
	courier.Position = target
	return nil
}

func pathDistance(path []model.Position) float64 {
	total := 0.0
	for index := 1; index < len(path); index++ {
		total += math.Hypot(path[index].X-path[index-1].X, path[index].Y-path[index-1].Y)
	}
	return total
}

func waitFor(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("courier interrupted: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}
