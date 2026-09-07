package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/teko/food-delivery/internal/appenv"
	"github.com/teko/food-delivery/internal/cluster"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	observer, err := cluster.NewInCluster(appenv.String("NAMESPACE", "food-delivery"))
	if err != nil {
		logger.Error("create cluster observer", "error", err)
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/components", func(w http.ResponseWriter, request *http.Request) {
		components, err := observer.Components(request.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(components); err != nil {
			logger.Error("encode components", "error", err)
		}
	})
	mux.HandleFunc("GET /health/live", ok)
	mux.HandleFunc("GET /health/ready", ok)
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, request *http.Request) {
		components, err := observer.Components(request.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		ready, desired := 0, 0
		for _, component := range components {
			ready += component.Ready
			desired += component.Desired
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "food_delivery_cluster_ready_pods %d\nfood_delivery_cluster_desired_pods %d\n", ready, desired)
	})
	server := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown HTTP server", "error", err)
		}
	case err := <-errCh:
		logger.Error("HTTP server stopped", "error", err)
	}
}

func ok(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintln(w, `{"status":"ok"}`)
}
