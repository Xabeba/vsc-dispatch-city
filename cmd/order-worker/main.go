package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/teko/food-delivery/internal/appenv"
	"github.com/teko/food-delivery/internal/messaging"
	"github.com/teko/food-delivery/internal/model"
	"github.com/teko/food-delivery/internal/telemetry"
	"github.com/teko/food-delivery/internal/workerutil"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	url := appenv.String("RABBITMQ_URL", "amqp://delivery:delivery@localhost:5672/")
	metrics := &telemetry.Metrics{Service: "order-worker"}
	metrics.Ready.Store(true)
	go workerutil.RunTelemetry(ctx, cancel, metrics, logger)

	seen := make(map[string]struct{})
	var seenMu sync.Mutex
	config := messaging.ConsumerConfig{Queue: "order-projection", Bindings: []string{"order.#", "courier.#", "customer.#", "simulation.#"}, Workers: 2, Prefetch: 16}
	err := messaging.Consume(ctx, url, config, logger, func(_ context.Context, event model.EventEnvelope) error {
		seenMu.Lock()
		_, duplicate := seen[event.ID]
		if !duplicate {
			seen[event.ID] = struct{}{}
		}
		seenMu.Unlock()
		if duplicate {
			logger.Warn("duplicate event ignored in memory", "event_id", event.ID)
			return nil
		}
		metrics.Consumed.Add(1)
		logger.Info("event projected in memory", "event_id", event.ID, "event_type", event.Type, "order_id", event.CorrelationID)
		return nil
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("order projection stopped", "error", err)
	}
}
