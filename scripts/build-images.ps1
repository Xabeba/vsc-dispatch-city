param(
    [string]$Tag = "local"
)

$ErrorActionPreference = "Stop"
$Root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$Services = @("control-api", "customer-simulator", "restaurant-worker", "courier-simulator", "order-worker")

foreach ($Service in $Services) {
    docker build -t "food-delivery-${Service}:${Tag}" --build-arg "SERVICE=${Service}" -f (Join-Path $Root "build/go-service.Dockerfile") $Root
    if ($LASTEXITCODE -ne 0) { throw "Image-Build fuer $Service fehlgeschlagen." }
}

docker build -t "food-delivery-dashboard:${Tag}" (Join-Path $Root "apps/dashboard")
if ($LASTEXITCODE -ne 0) { throw "Image-Build fuer dashboard fehlgeschlagen." }
