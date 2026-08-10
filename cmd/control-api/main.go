package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/teko/food-delivery/internal/api"
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
	server := api.NewServer(":"+env("PORT", "8080"), engine, logger)

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
