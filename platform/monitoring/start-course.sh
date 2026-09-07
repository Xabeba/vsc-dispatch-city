#!/usr/bin/env sh
set -eu
CONTEXT=${CONTEXT:-k3d-teko-k8s}
CLUSTER=${CLUSTER:-teko-k8s}
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
kubectl --context "$CONTEXT" -n food-delivery get cluster food-delivery-db >/dev/null
docker build -t food-delivery-cluster-observer:local --build-arg SERVICE=cluster-observer -f "$ROOT/build/go-service.Dockerfile" "$ROOT"
k3d image import -c "$CLUSTER" food-delivery-cluster-observer:local
CONTEXT="$CONTEXT" sh "$ROOT/platform/monitoring/install.sh"
kubectl --context "$CONTEXT" apply -k "$ROOT/deploy/overlays/block-07-observability"
kubectl --context "$CONTEXT" -n food-delivery rollout status deployment/cluster-observer --timeout=180s
kubectl --context "$CONTEXT" -n food-delivery rollout status statefulset/rabbitmq --timeout=180s
kubectl --context "$CONTEXT" -n monitoring rollout status deployment/monitoring-grafana --timeout=180s
echo 'Monitoring bereit. Grafana per Port-Forward auf Port 3000 oeffnen.'
