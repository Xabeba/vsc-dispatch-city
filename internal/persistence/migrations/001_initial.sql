CREATE TABLE IF NOT EXISTS restaurants (
    id text PRIMARY KEY,
    name text NOT NULL,
    cuisine text NOT NULL,
    x double precision NOT NULL,
    y double precision NOT NULL,
    status text NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS customers (
    id text PRIMARY KEY,
    name text NOT NULL,
    x double precision NOT NULL,
    y double precision NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS couriers (
    id text PRIMARY KEY,
    name text NOT NULL,
    x double precision NOT NULL,
    y double precision NOT NULL,
    status text NOT NULL,
    order_id text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS orders (
    id text PRIMARY KEY,
    customer_id text NOT NULL,
    restaurant_id text NOT NULL,
    courier_id text NOT NULL DEFAULT '',
    status text NOT NULL,
    progress double precision NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS orders_status_idx ON orders (status);
CREATE INDEX IF NOT EXISTS orders_updated_at_idx ON orders (updated_at DESC);

CREATE TABLE IF NOT EXISTS order_events (
    event_id uuid PRIMARY KEY,
    order_id text NOT NULL,
    event_type text NOT NULL,
    source text NOT NULL,
    occurred_at timestamptz NOT NULL,
    payload jsonb NOT NULL
);

CREATE INDEX IF NOT EXISTS order_events_order_idx ON order_events (order_id, occurred_at);

CREATE TABLE IF NOT EXISTS processed_events (
    event_id uuid PRIMARY KEY,
    event_type text NOT NULL,
    processed_at timestamptz NOT NULL DEFAULT now()
);
