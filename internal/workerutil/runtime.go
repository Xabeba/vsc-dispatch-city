package workerutil

import (
	"context"
	"errors"
	"log/slog"

	"github.com/teko/food-delivery/internal/messaging"
	"github.com/teko/food-delivery/internal/telemetry"
)

// RunTelemetry serves health and metrics and cancels the process on failure.
func RunTelemetry(ctx context.Context, cancel context.CancelFunc, metrics *telemetry.Metrics, logger *slog.Logger) {
	if err := metrics.Run(ctx, ":8080"); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("telemetry stopped", "error", err)
		cancel()
	}
}

// ClosePublisher logs a non-fatal AMQP close error.
func ClosePublisher(publisher *messaging.Publisher, logger *slog.Logger) {
	if err := publisher.Close(); err != nil {
		logger.Warn("close publisher", "error", err)
	}
}
