#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
CONTEXT=${CONTEXT:-k3d-teko-k8s}
CHART_VERSION=${CHART_VERSION:-0.29.0}

helm repo add cnpg https://cloudnative-pg.github.io/charts --force-update
helm upgrade --install cnpg cnpg/cloudnative-pg \
  --kube-context "${CONTEXT}" \
  --namespace cnpg-system \
  --create-namespace \
  --version "${CHART_VERSION}" \
  --values "${SCRIPT_DIR}/values-course.yaml" \
  --wait \
  --timeout 5m

kubectl --context "${CONTEXT}" wait \
  --for=condition=Established crd/clusters.postgresql.cnpg.io \
  --timeout=120s

printf 'CloudNativePG Chart %s ist im Kontext %s bereit.\n' "${CHART_VERSION}" "${CONTEXT}"
