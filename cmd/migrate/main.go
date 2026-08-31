package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/teko/food-delivery/internal/appenv"
	"github.com/teko/food-delivery/internal/persistence"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	repository, err := persistence.Connect(ctx, appenv.String("DATABASE_URL", "postgres://delivery:delivery@localhost:5432/delivery?sslmode=disable"))
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer repository.Close()
	if err := repository.Migrate(ctx); err != nil {
		logger.Error("apply migrations", "error", err)
		os.Exit(1)
	}
	logger.Info("database migrations applied")
}
