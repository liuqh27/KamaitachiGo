$baseUrl = "http://localhost:8080"
$body = @{
    ids = "operating_income,parent_holder_net_profit"
    subjects = "33:00000009"
    field = "operating_income"
    order = -1
    offset = 0
    limit = 1
    timestamp = 0
} | ConvertTo-Json

Write-Host "Sending identical request twice to test cache hit..."
for ($i=1; $i -le 2; $i++) {
    try {
        $resp = Invoke-RestMethod -Uri "$baseUrl/kamaitachi/api/data/v1/snapshot" -Method POST -Body $body -ContentType "application/json"
        Write-Host ("Request {0}: OK - returned {1}" -f $i, $resp.data.Length)
    } catch {
        Write-Host ("Request {0} failed: {1}" -f $i, $_)
    }
}

Write-Host "\nFetching stats..."
$stats = Invoke-RestMethod -Uri "$baseUrl/kamaitachi/api/data/v1/stats" -Method GET
$stats | ConvertTo-Json -Depth 4
