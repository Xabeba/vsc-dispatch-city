param(
    [string]$BaseUrl = "http://localhost:15672"
)

$ErrorActionPreference = "Stop"
$EventId = [guid]::NewGuid().ToString()
$OrderId = "lab-idempotency-$((Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmss'))"
$Now = (Get-Date).ToUniversalTime().ToString("o")
$Token = [Convert]::ToBase64String([Text.Encoding]::ASCII.GetBytes("delivery:delivery"))
$Headers = @{ Authorization = "Basic $Token" }
$PublishUrl = "$BaseUrl/api/exchanges/%2F/food.events/publish"

$Event = [ordered]@{
    event_id = $EventId
    event_type = "order.created"
    event_version = 1
    occurred_at = $Now
    correlation_id = $OrderId
    source = "block-06-lab"
    payload = [ordered]@{
        order = [ordered]@{ id = $OrderId; customer_id = "lab-customer"; restaurant_id = "restaurant-pizza"; status = "created"; created_at = $Now; updated_at = $Now; progress = 0 }
        customer = [ordered]@{ id = "lab-customer"; name = "Lab"; position = @{ x = 20; y = 8 }; status = "active" }
        restaurant = [ordered]@{ id = "restaurant-pizza"; name = "Luna Pizza"; cuisine = "Pizza"; position = @{ x = 4; y = 4 }; status = "online"; replicas = 1; ready_replicas = 1 }
    }
} | ConvertTo-Json -Depth 8 -Compress

$Request = @{
    properties = @{ delivery_mode = 2 }
    routing_key = "order.created.restaurant-pizza"
    payload = $Event
    payload_encoding = "string"
} | ConvertTo-Json -Depth 5 -Compress

1..2 | ForEach-Object {
    $Response = Invoke-RestMethod -Uri $PublishUrl -Method Post -Headers $Headers -ContentType "application/json" -Body $Request
    if (-not $Response.routed) { throw "RabbitMQ hat die Nachricht nicht geroutet." }
}

Write-Host "Dieselbe Nachricht wurde zweimal publiziert."
Write-Host "EVENT_ID=$EventId"
Write-Host "ORDER_ID=$OrderId"
