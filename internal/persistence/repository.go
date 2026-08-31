package persistence

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/teko/food-delivery/internal/events"
	"github.com/teko/food-delivery/internal/model"
	"github.com/teko/food-delivery/internal/scenario"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Repository stores idempotency keys and the current read model.
type Repository struct {
	pool *pgxpool.Pool
}

// Connect waits for PostgreSQL and returns a connection pool.
func Connect(ctx context.Context, databaseURL string) (*Repository, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		pool, err := pgxpool.New(ctx, databaseURL)
		if err == nil {
			err = pool.Ping(ctx)
			if err == nil {
				return &Repository{pool: pool}, nil
			}
			pool.Close()
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("connect PostgreSQL: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

// Close releases all database connections.
func (r *Repository) Close() {
	r.pool.Close()
}

// Migrate applies all embedded idempotent SQL migrations.
func (r *Repository) Migrate(ctx context.Context) error {
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		script, err := migrations.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		if _, err := r.pool.Exec(ctx, string(script)); err != nil {
			return fmt.Errorf("apply migration %s: %w", entry.Name(), err)
		}
	}
	return nil
}

// Project applies an event once in a single transaction.
func (r *Repository) Project(ctx context.Context, event model.EventEnvelope) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin projection: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var eventID string
	err = tx.QueryRow(ctx, `
		INSERT INTO processed_events (event_id, event_type)
		VALUES ($1, $2)
		ON CONFLICT (event_id) DO NOTHING
		RETURNING event_id::text`, event.ID, event.Type).Scan(&eventID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim event %s: %w", event.ID, err)
	}

	if event.Type != "courier.location.updated" && event.Type != "courier.registered" && event.Type != "customer.registered" && !strings.HasPrefix(event.Type, "simulation.") {
		if _, err := tx.Exec(ctx, `
			INSERT INTO order_events (event_id, order_id, event_type, source, occurred_at, payload)
			VALUES ($1, $2, $3, $4, $5, $6)`, event.ID, event.CorrelationID, event.Type, event.Source, event.OccurredAt, event.Payload); err != nil {
			return false, fmt.Errorf("append order event: %w", err)
		}
	}

	if err := projectEvent(ctx, tx, event); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit projection: %w", err)
	}
	return true, nil
}

// LoadSnapshot reads the durable UI projection.
func (r *Repository) LoadSnapshot(ctx context.Context) (model.Snapshot, error) {
	snapshot := model.Snapshot{Restaurants: append([]model.Restaurant(nil), scenario.Restaurants...)}
	rows, err := r.pool.Query(ctx, `SELECT id, name, x, y FROM customers ORDER BY id`)
	if err != nil {
		return snapshot, fmt.Errorf("query customers: %w", err)
	}
	for rows.Next() {
		var customer model.Customer
		if err := rows.Scan(&customer.ID, &customer.Name, &customer.Position.X, &customer.Position.Y); err != nil {
			rows.Close()
			return snapshot, fmt.Errorf("scan customer: %w", err)
		}
		snapshot.Customers = append(snapshot.Customers, customer)
	}
	rows.Close()

	rows, err = r.pool.Query(ctx, `SELECT id, name, x, y, status, order_id FROM couriers ORDER BY id`)
	if err != nil {
		return snapshot, fmt.Errorf("query couriers: %w", err)
	}
	for rows.Next() {
		var courier model.Courier
		if err := rows.Scan(&courier.ID, &courier.Name, &courier.Position.X, &courier.Position.Y, &courier.Status, &courier.OrderID); err != nil {
			rows.Close()
			return snapshot, fmt.Errorf("scan courier: %w", err)
		}
		snapshot.Couriers = append(snapshot.Couriers, courier)
	}
	rows.Close()

	rows, err = r.pool.Query(ctx, `
		SELECT id, customer_id, restaurant_id, courier_id, status, progress, created_at, updated_at
		FROM orders ORDER BY updated_at DESC LIMIT 50`)
	if err != nil {
		return snapshot, fmt.Errorf("query orders: %w", err)
	}
	for rows.Next() {
		var order model.Order
		if err := rows.Scan(&order.ID, &order.CustomerID, &order.RestaurantID, &order.CourierID, &order.Status, &order.Progress, &order.CreatedAt, &order.UpdatedAt); err != nil {
			rows.Close()
			return snapshot, fmt.Errorf("scan order: %w", err)
		}
		snapshot.Orders = append(snapshot.Orders, order)
	}
	rows.Close()
	return snapshot, nil
}

func projectEvent(ctx context.Context, tx pgx.Tx, event model.EventEnvelope) error {
	switch event.Type {
	case "customer.registered":
		payload, err := events.Decode[events.CustomerRegistered](event)
		if err != nil {
			return err
		}
		return upsertCustomer(ctx, tx, payload.Customer)
	case "courier.registered":
		payload, err := events.Decode[events.CourierRegistered](event)
		if err != nil {
			return err
		}
		return registerCourier(ctx, tx, payload.Courier)
	case "order.created":
		payload, err := events.Decode[events.OrderCreated](event)
		if err != nil {
			return err
		}
		if err := upsertRestaurant(ctx, tx, payload.Restaurant); err != nil {
			return err
		}
		if err := upsertCustomer(ctx, tx, payload.Customer); err != nil {
			return err
		}
		return upsertOrder(ctx, tx, payload.Order)
	case "order.accepted", "order.rejected":
		payload, err := events.Decode[events.OrderAccepted](event)
		if err != nil {
			return err
		}
		return upsertOrder(ctx, tx, payload.Order)
	case "courier.assigned":
		payload, err := events.Decode[events.CourierAssigned](event)
		if err != nil {
			return err
		}
		if err := upsertCourier(ctx, tx, payload.Courier); err != nil {
			return err
		}
		return upsertOrder(ctx, tx, payload.Order)
	case "courier.location.updated":
		payload, err := events.Decode[events.CourierLocation](event)
		if err != nil {
			return err
		}
		if err := upsertCourier(ctx, tx, payload.Courier); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			UPDATE orders
			SET progress=$2,
			    status=CASE WHEN $4='to_customer' THEN 'in_transit' ELSE status END,
			    updated_at=$3
			WHERE id=$1`, payload.OrderID, payload.Progress, event.OccurredAt, payload.Courier.Status)
		return err
	case "order.picked_up":
		payload, err := events.Decode[events.OrderPickedUp](event)
		if err != nil {
			return err
		}
		if err := upsertCourier(ctx, tx, payload.Courier); err != nil {
			return err
		}
		return upsertOrder(ctx, tx, payload.Order)
	case "order.delivered":
		payload, err := events.Decode[events.OrderDelivered](event)
		if err != nil {
			return err
		}
		if err := upsertCourier(ctx, tx, payload.Courier); err != nil {
			return err
		}
		return upsertOrder(ctx, tx, payload.Order)
	case "simulation.reset":
		if _, err := tx.Exec(ctx, `TRUNCATE orders, order_events`); err != nil {
			return fmt.Errorf("reset orders: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM customers WHERE id NOT LIKE 'customer-agent-%'`); err != nil {
			return fmt.Errorf("reset transient customers: %w", err)
		}
		_, err := tx.Exec(ctx, `UPDATE couriers SET status='idle', order_id='', updated_at=now()`)
		return err
	default:
		return nil
	}
}

func upsertRestaurant(ctx context.Context, tx pgx.Tx, restaurant model.Restaurant) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO restaurants (id, name, cuisine, x, y, status, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,now())
		ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, cuisine=EXCLUDED.cuisine, x=EXCLUDED.x, y=EXCLUDED.y, status=EXCLUDED.status, updated_at=now()`,
		restaurant.ID, restaurant.Name, restaurant.Cuisine, restaurant.Position.X, restaurant.Position.Y, restaurant.Status)
	return err
}

func upsertCustomer(ctx context.Context, tx pgx.Tx, customer model.Customer) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO customers (id, name, x, y, updated_at) VALUES ($1,$2,$3,$4,now())
		ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, x=EXCLUDED.x, y=EXCLUDED.y, updated_at=now()`,
		customer.ID, customer.Name, customer.Position.X, customer.Position.Y)
	return err
}

func upsertCourier(ctx context.Context, tx pgx.Tx, courier model.Courier) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO couriers (id, name, x, y, status, order_id, updated_at) VALUES ($1,$2,$3,$4,$5,$6,now())
		ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, x=EXCLUDED.x, y=EXCLUDED.y, status=EXCLUDED.status, order_id=EXCLUDED.order_id, updated_at=now()`,
		courier.ID, courier.Name, courier.Position.X, courier.Position.Y, courier.Status, courier.OrderID)
	return err
}

func registerCourier(ctx context.Context, tx pgx.Tx, courier model.Courier) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO couriers (id, name, x, y, status, order_id, updated_at) VALUES ($1,$2,$3,$4,$5,$6,now())
		ON CONFLICT (id) DO NOTHING`,
		courier.ID, courier.Name, courier.Position.X, courier.Position.Y, courier.Status, courier.OrderID)
	return err
}

func upsertOrder(ctx context.Context, tx pgx.Tx, order model.Order) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO orders (id, customer_id, restaurant_id, courier_id, status, progress, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (id) DO UPDATE SET courier_id=EXCLUDED.courier_id, status=EXCLUDED.status, progress=EXCLUDED.progress, updated_at=EXCLUDED.updated_at`,
		order.ID, order.CustomerID, order.RestaurantID, order.CourierID, order.Status, order.Progress, order.CreatedAt, order.UpdatedAt)
	return err
}
