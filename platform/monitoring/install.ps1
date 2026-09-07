param([string]$Context = 'k3d-teko-k8s', [string]$ChartVersion = '88.1.3')
$ErrorActionPreference = 'Stop'
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts --force-update
if ($LASTEXITCODE -ne 0) { throw 'Helm-Repository nicht erreichbar.' }
helm upgrade --install monitoring prometheus-community/kube-prometheus-stack --kube-context $Context --namespace monitoring --create-namespace --version $ChartVersion --values (Join-Path $PSScriptRoot 'values-light.yaml') --wait --timeout 8m
if ($LASTEXITCODE -ne 0) { throw 'Monitoring-Installation fehlgeschlagen.' }
