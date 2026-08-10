#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TAG=${TAG:-local}

docker build -t "food-delivery-control-api:${TAG}" --build-arg SERVICE=control-api -f "${ROOT}/build/go-service.Dockerfile" "${ROOT}"
docker build -t "food-delivery-dashboard:${TAG}" "${ROOT}/apps/dashboard"
