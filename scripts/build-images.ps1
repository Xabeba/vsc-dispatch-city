param(
    [string]$Tag = "local"
)

$ErrorActionPreference = "Stop"
$Root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$Services = @("control-api", "customer-simulator", "restaurant-worker", "courier-simulator", "order-worker", "migrate")

foreach ($Service in $Services) {
    docker build -t "food-delivery-${Service}:${Tag}" --build-arg "SERVICE=${Service}" -f (Join-Path $Root "build/go-service.Dockerfile") $Root
    if ($LASTEXITCODE -ne 0) { throw "Image-Build fuer $Service fehlgeschlagen." }
}

Write-Host "Block-6-Go-Images wurden mit Tag $Tag gebaut. Das Dashboard-Image bleibt aus Block 5 erhalten."
