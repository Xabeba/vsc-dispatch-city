param(
    [string]$Cluster = "teko-k8s",
    [string]$Tag = "local"
)

$ErrorActionPreference = "Stop"
$Images = @(
    "food-delivery-control-api:${Tag}",
    "food-delivery-customer-simulator:${Tag}",
    "food-delivery-restaurant-worker:${Tag}",
    "food-delivery-courier-simulator:${Tag}",
    "food-delivery-order-worker:${Tag}",
    "food-delivery-dashboard:${Tag}"
)

k3d image import -c $Cluster $Images
if ($LASTEXITCODE -ne 0) { throw "Image-Import in den Cluster $Cluster fehlgeschlagen." }
