param([string]$Context = 'k3d-teko-k8s', [string]$Cluster = 'teko-k8s')
$ErrorActionPreference = 'Stop'
$Root = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
kubectl --context $Context -n food-delivery get cluster food-delivery-db
if ($LASTEXITCODE -ne 0) { throw 'AB6 mit CloudNativePG muss bereit sein.' }
docker build -t food-delivery-cluster-observer:local --build-arg SERVICE=cluster-observer -f (Join-Path $Root 'build/go-service.Dockerfile') $Root
if ($LASTEXITCODE -ne 0) { throw 'Image-Build fehlgeschlagen.' }
k3d image import -c $Cluster food-delivery-cluster-observer:local
if ($LASTEXITCODE -ne 0) { throw 'Image-Import fehlgeschlagen.' }
& (Join-Path $PSScriptRoot 'install.ps1') -Context $Context
kubectl --context $Context apply -k (Join-Path $Root 'deploy/overlays/block-07-observability')
if ($LASTEXITCODE -ne 0) { throw 'Block-7-Overlay konnte nicht angewendet werden.' }
foreach ($Resource in @('deployment/cluster-observer', 'statefulset/rabbitmq')) {
    kubectl --context $Context -n food-delivery rollout status $Resource --timeout=180s
    if ($LASTEXITCODE -ne 0) { throw "Nicht bereit: $Resource" }
}
kubectl --context $Context -n monitoring rollout status deployment/monitoring-grafana --timeout=180s
if ($LASTEXITCODE -ne 0) { throw 'Grafana ist nicht bereit.' }
Write-Host 'Monitoring bereit. Grafana per Port-Forward auf Port 3000 oeffnen.'
