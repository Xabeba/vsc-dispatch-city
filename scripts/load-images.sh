#!/usr/bin/env sh
set -eu

CLUSTER=${CLUSTER:-teko-k8s}
TAG=${TAG:-local}

k3d image import -c "${CLUSTER}" \
  "food-delivery-control-api:${TAG}" \
  "food-delivery-customer-simulator:${TAG}" \
  "food-delivery-restaurant-worker:${TAG}" \
  "food-delivery-courier-simulator:${TAG}" \
  "food-delivery-order-worker:${TAG}" \
  "food-delivery-migrate:${TAG}" \
  "food-delivery-dashboard:${TAG}"

printf 'Block-6-Images wurden in den k3d-Cluster %s importiert.\n' "${CLUSTER}"
