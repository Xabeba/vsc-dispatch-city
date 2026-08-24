#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TAG=${TAG:-local}

for SERVICE in control-api customer-simulator restaurant-worker courier-simulator order-worker; do
  docker build -t "food-delivery-${SERVICE}:${TAG}" --build-arg SERVICE="${SERVICE}" -f "${ROOT}/build/go-service.Dockerfile" "${ROOT}"
done

docker build -t "food-delivery-dashboard:${TAG}" "${ROOT}/apps/dashboard"
