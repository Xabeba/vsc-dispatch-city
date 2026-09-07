#!/usr/bin/env sh
set -eu
CONTEXT=${CONTEXT:-k3d-teko-k8s}
echo 'Ein Pod bekommt 150 Sekunden kuenstliche CPU-Last. HPA in einem zweiten Terminal beobachten.'
kubectl --context "$CONTEXT" -n betrieb-lab exec deployment/lab-web -- timeout 150 sh -c 'while true; do true; done' || {
  CODE=$?
  test "$CODE" -eq 124 || test "$CODE" -eq 143 || exit "$CODE"
}
echo 'Last beendet. Der HPA skaliert nach seiner Wartezeit wieder herunter.'
