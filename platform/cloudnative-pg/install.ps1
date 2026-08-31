param(
    [string]$Context = "k3d-teko-k8s",
    [string]$ChartVersion = "0.29.0"
)

$ErrorActionPreference = "Stop"
$Values = Join-Path $PSScriptRoot "values-course.yaml"

helm repo add cnpg https://cloudnative-pg.github.io/charts --force-update
if ($LASTEXITCODE -ne 0) { throw "Das CloudNativePG Helm Repository konnte nicht hinzugefuegt werden." }

helm upgrade --install cnpg cnpg/cloudnative-pg `
    --kube-context $Context `
    --namespace cnpg-system `
    --create-namespace `
    --version $ChartVersion `
    --values $Values `
    --wait `
    --timeout 5m
if ($LASTEXITCODE -ne 0) { throw "Die CloudNativePG Installation ist fehlgeschlagen." }

kubectl --context $Context wait --for=condition=Established crd/clusters.postgresql.cnpg.io --timeout=120s
if ($LASTEXITCODE -ne 0) { throw "Die CloudNativePG Cluster-CRD ist nicht bereit." }

Write-Host "CloudNativePG Chart $ChartVersion ist im Kontext $Context bereit."
