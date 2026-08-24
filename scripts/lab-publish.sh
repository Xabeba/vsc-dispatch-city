#!/usr/bin/env sh
set -eu

MODE=${1:-valid}
COUNT=${2:-8}
BASE_URL=${RABBITMQ_MANAGEMENT_URL:-http://localhost:15672}
PUBLISH_URL="$BASE_URL/api/exchanges/%2F/food.events/publish"

publish() {
  ROUTING_KEY=$1
  PAYLOAD=$2
  ESCAPED=$(printf '%s' "$PAYLOAD" | sed 's/\\/\\\\/g; s/"/\\"/g')
  REQUEST=$(printf '{"properties":{"delivery_mode":2},"routing_key":"%s","payload":"%s","payload_encoding":"string"}' "$ROUTING_KEY" "$ESCAPED")
  RESPONSE=$(curl -fsS -u delivery:delivery -H 'content-type: application/json' -d "$REQUEST" "$PUBLISH_URL")
  case "$RESPONSE" in
    *'"routed":true'*) ;;
    *) echo "RabbitMQ hat die Nachricht nicht geroutet: $RESPONSE" >&2; exit 1 ;;
  esac
}

case "$MODE" in
  valid)
    STAMP=$(date -u +%Y%m%dT%H%M%S)
    NOW=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    INDEX=1
    while [ "$INDEX" -le "$COUNT" ]; do
      ORDER_ID="lab-pizza-$STAMP-$$-$INDEX"
      EVENT=$(printf '{"event_id":"event-%s","event_type":"order.created","event_version":1,"occurred_at":"%s","correlation_id":"%s","source":"block-05-lab","payload":{"order":{"id":"%s","customer_id":"lab-customer","restaurant_id":"restaurant-pizza","status":"created","created_at":"%s","updated_at":"%s","progress":0},"customer":{"id":"lab-customer","name":"Lab","position":{"x":20,"y":8},"status":"active"},"restaurant":{"id":"restaurant-pizza","name":"Luna Pizza","cuisine":"Pizza","position":{"x":4,"y":4},"status":"online","replicas":1,"ready_replicas":1}}}' "$ORDER_ID" "$NOW" "$ORDER_ID" "$ORDER_ID" "$NOW" "$NOW")
      publish "order.created.restaurant-pizza" "$EVENT"
      INDEX=$((INDEX + 1))
    done
    echo "$COUNT gueltige Pizza-Events wurden publiziert."
    ;;
  invalid)
    publish "order.created.restaurant-pizza" "this-is-not-json"
    echo "Eine ungueltige Nachricht wurde publiziert."
    ;;
  *)
    echo "Verwendung: $0 valid [anzahl] | invalid" >&2
    exit 1
    ;;
esac
