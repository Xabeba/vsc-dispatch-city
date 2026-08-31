package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/teko/food-delivery/internal/api"
	"github.com/teko/food-delivery/internal/events"
	"github.com/teko/food-delivery/internal/messaging"
	"github.com/teko/food-delivery/internal/model"
	"github.com/teko/food-delivery/internal/persistence"
	"github.com/teko/food-delivery/internal/scenario"
	"github.com/teko/food-delivery/internal/simulation"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	mode := env("APP_MODE", "standalone")
	instance := env("POD_NAME", "local")
	interval := time.Duration(envInt("TICK_MS", 500)) * time.Millisecond
	engine := simulation.NewEngine(mode, instance)
	var repository *persistence.Repository
	if databaseURL := env("DATABASE_URL", ""); databaseURL != "" {
		var err error
		repository, err = persistence.Connect(ctx, databaseURL)
		if err != nil {
			logger.Error("connect PostgreSQL", "error", err)
			return
		}
		defer repository.Close()
		engine.SetPersistent(true)
		if snapshot, err := repository.LoadSnapshot(ctx); err != nil {
			logger.Warn("initial database snapshot unavailable", "error", err)
		} else {
			engine.Hydrate(snapshot)
		}
		go refreshFromDatabase(ctx, repository, engine, logger)
	}
	commands := api.Commands(api.NewLocalCommands(engine))
	var publisher *messaging.Publisher
	if mode == "distributed" {
		var err error
		publisher, err = messaging.NewPublisher(ctx, env("RABBITMQ_URL", "amqp://delivery:delivery@localhost:5672/"), logger)
		if err != nil {
			logger.Error("create RabbitMQ publisher", "error", err)
			return
		}
		defer func() {
			if err := publisher.Close(); err != nil {
				logger.Warn("close RabbitMQ publisher", "error", err)
			}
		}()
		commands = &brokerCommands{publisher: publisher}
		go func() {
			config := messaging.ConsumerConfig{Queue: "live." + instance, Bindings: []string{"#"}, Workers: 1, Prefetch: 64, Exclusive: true, AutoDelete: true}
			if err := messaging.Consume(ctx, env("RABBITMQ_URL", "amqp://delivery:delivery@localhost:5672/"), config, logger, func(_ context.Context, event model.EventEnvelope) error {
				return engine.ApplyEvent(event)
			}); err != nil && !errors.Is(err, context.Canceled) {
				logger.Error("live event consumer stopped", "error", err)
				cancel()
			}
		}()
	}
	server := api.NewServer(":"+env("PORT", "8080"), engine, commands, logger)

	go func() {
		if err := engine.Run(ctx, interval); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("simulation stopped", "error", err)
			cancel()
		}
	}()
	go func() {
		logger.Info("control API started", "mode", mode, "interval", interval.String())
		if err := server.ListenAndServe(); err != nil {
			logger.Error("HTTP server stopped", "error", err)
			cancel()
		}
	}()

	<-ctx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}

func refreshFromDatabase(ctx context.Context, repository *persistence.Repository, engine *simulation.Engine, logger *slog.Logger) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snapshot, err := repository.LoadSnapshot(ctx)
			if err != nil {
				logger.Warn("refresh database snapshot", "error", err)
				continue
			}
			engine.Hydrate(snapshot)
		}
	}
}

type brokerCommands struct {
	publisher *messaging.Publisher
	sequence  atomic.Uint64
}

func (b *brokerCommands) Start(ctx context.Context) error {
	return b.publishCommand(ctx, "simulation.started")
}

func (b *brokerCommands) Pause(ctx context.Context) error {
	return b.publishCommand(ctx, "simulation.paused")
}

func (b *brokerCommands) Reset(ctx context.Context) error {
	return b.publishCommand(ctx, "simulation.reset")
}

func (b *brokerCommands) CreateOrder(ctx context.Context) (model.Order, error) {
	order, event, err := scenario.NewOrder(b.sequence.Add(1)+10_000, "control-api")
	if err != nil {
		return model.Order{}, err
	}
	if err := b.publisher.Publish(ctx, event); err != nil {
		return model.Order{}, err
	}
	return order, nil
}

func (b *brokerCommands) publishCommand(ctx context.Context, eventType string) error {
	event, err := events.New(eventType, "simulation", "", "control-api", map[string]string{"command": eventType})
	if err != nil {
		return err
	}
	return b.publisher.Publish(ctx, event)
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(env(key, strconv.Itoa(fallback)))
	if err != nil {
		return fallback
	}
	return value
}
