#!/usr/bin/env sh
set -eu

BASE_URL=${RABBITMQ_MANAGEMENT_URL:-http://localhost:15672}
PUBLISH_URL="$BASE_URL/api/exchanges/%2F/food.events/publish"
EVENT_ID=$(uuidgen | tr '[:upper:]' '[:lower:]')
STAMP=$(date -u +%Y%m%dT%H%M%S)
NOW=$(date -u +%Y-%m-%dT%H:%M:%SZ)
ORDER_ID="lab-idempotency-${STAMP}"

EVENT=$(printf '{"event_id":"%s","event_type":"order.created","event_version":1,"occurred_at":"%s","correlation_id":"%s","source":"block-06-lab","payload":{"order":{"id":"%s","customer_id":"lab-customer","restaurant_id":"restaurant-pizza","status":"created","created_at":"%s","updated_at":"%s","progress":0},"customer":{"id":"lab-customer","name":"Lab","position":{"x":20,"y":8},"status":"active"},"restaurant":{"id":"restaurant-pizza","name":"Luna Pizza","cuisine":"Pizza","position":{"x":4,"y":4},"status":"online","replicas":1,"ready_replicas":1}}}' "$EVENT_ID" "$NOW" "$ORDER_ID" "$ORDER_ID" "$NOW" "$NOW")
ESCAPED=$(printf '%s' "$EVENT" | sed 's/\\/\\\\/g; s/"/\\"/g')
REQUEST=$(printf '{"properties":{"delivery_mode":2},"routing_key":"order.created.restaurant-pizza","payload":"%s","payload_encoding":"string"}' "$ESCAPED")

INDEX=1
while [ "$INDEX" -le 2 ]; do
  RESPONSE=$(curl -fsS -u delivery:delivery -H 'content-type: application/json' -d "$REQUEST" "$PUBLISH_URL")
  case "$RESPONSE" in
    *'"routed":true'*) ;;
    *) echo "RabbitMQ hat die Nachricht nicht geroutet: $RESPONSE" >&2; exit 1 ;;
  esac
  INDEX=$((INDEX + 1))
done

printf 'Dieselbe Nachricht wurde zweimal publiziert.\nEVENT_ID=%s\nORDER_ID=%s\n' "$EVENT_ID" "$ORDER_ID"
