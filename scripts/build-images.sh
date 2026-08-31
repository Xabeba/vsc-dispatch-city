#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TAG=${TAG:-local}

docker build -t "food-delivery-control-api:${TAG}" --build-arg SERVICE=control-api -f "${ROOT}/build/go-service.Dockerfile" "${ROOT}"
for SERVICE in customer-simulator restaurant-worker courier-simulator order-worker; do
  docker build -t "food-delivery-${SERVICE}:${TAG}" --build-arg SERVICE="${SERVICE}" -f "${ROOT}/build/go-service.Dockerfile" "${ROOT}"
done
docker build -t "food-delivery-migrate:${TAG}" --build-arg SERVICE=migrate -f "${ROOT}/build/go-service.Dockerfile" "${ROOT}"

printf 'Block-6-Go-Images wurden mit Tag %s gebaut. Das Dashboard-Image bleibt aus Block 5 erhalten.\n' "${TAG}"
