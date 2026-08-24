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
	metrics := &telemetry.Metrics{Service: "customer-simulator"}
	go workerutil.RunTelemetry(ctx, cancel, metrics, logger)

	publisher, err := messaging.NewPublisher(ctx, url, logger)
	if err != nil {
		logger.Error("connect publisher", "error", err)
		return
	}
	defer workerutil.ClosePublisher(publisher, logger)
	metrics.Ready.Store(true)
	podName := appenv.String("POD_NAME", "customer-simulator-0")
	ordinal := scenario.OrdinalFromPodName(podName)
	customer := scenario.CustomerForOrdinal(ordinal)
	customer.PodName = podName
	registered, err := events.New("customer.registered", customer.ID, "", "customer-simulator/"+podName, events.CustomerRegistered{Customer: customer})
	if err != nil {
		logger.Error("create customer registration", "error", err)
		return
	}
	if err := publisher.Publish(ctx, registered); err != nil {
		logger.Error("publish customer registration", "error", err)
		return
	}
	metrics.Published.Add(1)

	var running atomic.Bool
	running.Store(true)
	go func() {
		config := messaging.ConsumerConfig{Queue: "simulation-control." + podName, Bindings: []string{"simulation.#"}, Workers: 1, Prefetch: 8, AutoDelete: true}
		if err := messaging.Consume(ctx, url, config, logger, func(_ context.Context, event model.EventEnvelope) error {
			metrics.Consumed.Add(1)
			switch event.Type {
			case "simulation.started":
				running.Store(true)
			case "simulation.paused":
				running.Store(false)
			case "simulation.reset":
				running.Store(true)
			}
			return nil
		}); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("control consumer stopped", "error", err)
			cancel()
		}
	}()
	startDelay := time.Duration(ordinal-1) * appenv.DurationMS("START_STAGGER_MS", 5000)
	if startDelay > 0 {
		timer := time.NewTimer(startDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}

	ticker := time.NewTicker(appenv.DurationMS("ORDER_INTERVAL_MS", 15000))
	defer ticker.Stop()
	sequence := uint64(ordinal)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !running.Load() {
				continue
			}
			sequence++
			order, event, err := scenario.NewOrderForCustomer(sequence, "customer-simulator/"+podName, customer)
			if err != nil {
				metrics.Failures.Add(1)
				logger.Error("create order event", "error", err)
				continue
			}
			if err := publisher.Publish(ctx, event); err != nil {
				metrics.Failures.Add(1)
				logger.Error("publish order", "order_id", order.ID, "error", err)
				continue
			}
			metrics.Published.Add(1)
			logger.Info("order published", "order_id", order.ID, "restaurant_id", order.RestaurantID)
		}
	}
}
