package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"sync/atomic"
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
	restaurantID := appenv.String("RESTAURANT_ID", "restaurant-pizza")
	podName := appenv.String("POD_NAME", restaurantID+"-local")
	if _, ok := restaurantByID(restaurantID); !ok {
		logger.Error("unknown restaurant", "restaurant_id", restaurantID)
		return
	}
	metrics := &telemetry.Metrics{Service: "restaurant-worker"}
	go workerutil.RunTelemetry(ctx, cancel, metrics, logger)
	publisher, err := messaging.NewPublisher(ctx, url, logger)
	if err != nil {
		logger.Error("connect publisher", "error", err)
		return
	}
	defer workerutil.ClosePublisher(publisher, logger)
	metrics.Ready.Store(true)

	var handled atomic.Uint64
	config := messaging.ConsumerConfig{
		Queue:    "restaurant." + restaurantID,
		Bindings: []string{"order.created." + restaurantID},
		Workers:  1,
		Prefetch: 1,
	}
	err = messaging.Consume(ctx, url, config, logger, func(ctx context.Context, event model.EventEnvelope) error {
		payload, err := events.Decode[events.OrderCreated](event)
		if err != nil {
			metrics.Failures.Add(1)
			return err
		}
		metrics.Consumed.Add(1)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(appenv.DurationMS("PROCESSING_MS", 900)):
		}

		accepted := handled.Add(1)%9 != 0
		if accepted {
			payload.Order.Status = "accepted"
		} else {
			payload.Order.Status = "failed"
		}
		payload.Order.UpdatedAt = time.Now().UTC()
		eventType := "order.accepted"
		if !accepted {
			eventType = "order.rejected"
		}
		outgoing, err := events.New(eventType, payload.Order.ID, event.ID, "restaurant-worker/"+podName, events.OrderAccepted(payload))
		if err != nil {
			return err
		}
		if err := publisher.Publish(ctx, outgoing); err != nil {
			metrics.Failures.Add(1)
			return err
		}
		metrics.Published.Add(1)
		logger.Info("order processed", "order_id", payload.Order.ID, "restaurant_id", restaurantID, "result", eventType)
		return nil
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("restaurant consumer stopped", "error", err)
	}
}

func restaurantByID(id string) (model.Restaurant, bool) {
	for _, restaurant := range scenario.Restaurants {
		if restaurant.ID == id {
			return restaurant, true
		}
	}
	return model.Restaurant{}, false
}
