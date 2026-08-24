package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// Metrics provides lightweight Prometheus counters and health endpoints.
type Metrics struct {
	Service   string
	Published atomic.Uint64
	Consumed  atomic.Uint64
	Failures  atomic.Uint64
	Ready     atomic.Bool
}

// Run serves metrics and health endpoints until the context is cancelled.
func (m *Metrics) Run(ctx context.Context, addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, _ *http.Request) {
		if !m.Ready.Load() {
			http.Error(w, `{"status":"not_ready"}`, http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "food_delivery_events_published_total{service=%q} %d\n", m.Service, m.Published.Load())
		fmt.Fprintf(w, "food_delivery_events_consumed_total{service=%q} %d\n", m.Service, m.Consumed.Load())
		fmt.Fprintf(w, "food_delivery_failures_total{service=%q} %d\n", m.Service, m.Failures.Load())
	})
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 3 * time.Second}

	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("telemetry server: %w", err)
		}
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown telemetry server: %w", err)
		}
		return nil
	case err := <-errCh:
		return err
	}
}
