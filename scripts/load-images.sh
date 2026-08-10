#!/usr/bin/env sh
set -eu

CLUSTER=${CLUSTER:-delivery-lab}
TAG=${TAG:-local}

k3d image import -c "${CLUSTER}" "food-delivery-control-api:${TAG}" "food-delivery-dashboard:${TAG}"
