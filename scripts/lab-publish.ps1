param(
    [ValidateSet("valid", "invalid")]
    [string]$Mode = "valid",
    [int]$Count = 8,
    [string]$BaseUrl = "http://localhost:15672"
)

$ErrorActionPreference = "Stop"
$Token = [Convert]::ToBase64String([Text.Encoding]::ASCII.GetBytes("delivery:delivery"))
$Headers = @{ Authorization = "Basic $Token" }
$PublishUrl = "$BaseUrl/api/exchanges/%2F/food.events/publish"

function Publish-LabMessage([string]$RoutingKey, [string]$Payload) {
    $Request = @{
        properties = @{ delivery_mode = 2 }
        routing_key = $RoutingKey
        payload = $Payload
        payload_encoding = "string"
    } | ConvertTo-Json -Depth 5 -Compress
    $Response = Invoke-RestMethod -Uri $PublishUrl -Method Post -Headers $Headers -ContentType "application/json" -Body $Request
    if (-not $Response.routed) { throw "RabbitMQ hat die Nachricht nicht geroutet." }
}

if ($Mode -eq "invalid") {
    Publish-LabMessage "order.created.restaurant-pizza" "this-is-not-json"
    Write-Host "Eine ungueltige Nachricht wurde publiziert."
    exit 0
}

$Now = (Get-Date).ToUniversalTime().ToString("o")
$Stamp = (Get-Date).ToUniversalTime().ToString("yyyyMMddTHHmmss")
1..$Count | ForEach-Object {
    $OrderId = "lab-pizza-$Stamp-$PID-$_"
    $Event = [ordered]@{
        event_id = "event-$OrderId"
        event_type = "order.created"
        event_version = 1
        occurred_at = $Now
        correlation_id = $OrderId
        source = "block-05-lab"
        payload = [ordered]@{
            order = [ordered]@{ id = $OrderId; customer_id = "lab-customer"; restaurant_id = "restaurant-pizza"; status = "created"; created_at = $Now; updated_at = $Now; progress = 0 }
            customer = [ordered]@{ id = "lab-customer"; name = "Lab"; position = @{ x = 20; y = 8 }; status = "active" }
            restaurant = [ordered]@{ id = "restaurant-pizza"; name = "Luna Pizza"; cuisine = "Pizza"; position = @{ x = 4; y = 4 }; status = "online"; replicas = 1; ready_replicas = 1 }
        }
    } | ConvertTo-Json -Depth 8 -Compress
    Publish-LabMessage "order.created.restaurant-pizza" $Event
}
Write-Host "$Count gueltige Pizza-Events wurden publiziert."
