#!/usr/bin/env sh
set -eu

CLUSTER=${CLUSTER:-delivery-lab}
TAG=${TAG:-local}

k3d image import -c "${CLUSTER}" \
  "food-delivery-control-api:${TAG}" \
  "food-delivery-customer-simulator:${TAG}" \
  "food-delivery-restaurant-worker:${TAG}" \
  "food-delivery-courier-simulator:${TAG}" \
  "food-delivery-order-worker:${TAG}" \
  "food-delivery-dashboard:${TAG}"
