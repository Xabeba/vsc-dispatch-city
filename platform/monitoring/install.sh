#!/usr/bin/env sh
set -eu

CONTEXT=${CONTEXT:-k3d-teko-k8s}
CHART_VERSION=${CHART_VERSION:-88.1.3}
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts --force-update
helm upgrade --install monitoring prometheus-community/kube-prometheus-stack \
  --kube-context "${CONTEXT}" \
  --namespace monitoring \
  --create-namespace \
  --version "${CHART_VERSION}" \
  --values "${ROOT}/values-light.yaml" \
  --wait \
  --timeout 8m
