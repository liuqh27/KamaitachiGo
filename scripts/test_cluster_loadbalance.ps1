# 集群负载均衡测试

param(
    [int]$Requests = 500
)

Write-Host "==========================================="
Write-Host "Cluster Load Balance Test"
Write-Host "==========================================="

$body = '{"ids":"operating_income","subjects":"33:00000009,33:00082582","field":"operating_income","order":-1,"limit":10}'
$nodes = @("http://localhost:8080", "http://localhost:8081")

Write-Host "Available Nodes:" -ForegroundColor Cyan
foreach ($node in $nodes) {
    try {
        Invoke-RestMethod -Uri "$node/health" -TimeoutSec 1 | Out-Null
        Write-Host "  $node - OK" -ForegroundColor Green
    } catch {
        Write-Host "  $node - FAIL" -ForegroundColor Red
    }
}

Write-Host "`nTesting with $Requests requests (Round-Robin)..." -ForegroundColor Yellow

$startTime = Get-Date
$success = 0
$failed = 0

# 简单的轮询负载均衡
for ($i = 0; $i -lt $Requests; $i++) {
    $nodeIndex = $i % $nodes.Length
    $node = $nodes[$nodeIndex]
    
    try {
        $response = Invoke-RestMethod -Uri "$node/kamaitachi/api/data/v1/snapshot" -Method POST -Body $body -ContentType "application/json" -TimeoutSec 5
        if ($response.status_code -eq 0) {
            $success++
        } else {
            $failed++
        }
    } catch {
        $failed++
    }
    
    if (($i + 1) % 100 -eq 0) {
        Write-Host "Progress: $($i + 1)/$Requests"
    }
}

$endTime = Get-Date
$duration = ($endTime - $startTime).TotalSeconds
$qps = [math]::Round($success / $duration, 2)

Write-Host "`n==========================================="
Write-Host "Results"
Write-Host "==========================================="
Write-Host "Total: $Requests"
Write-Host "Success: $success" -ForegroundColor Green
Write-Host "Failed: $failed" -ForegroundColor $(if($failed -gt 0){"Red"}else{"Green"})
Write-Host "Duration: $([math]::Round($duration, 2)) sec"
Write-Host "QPS: $qps" -ForegroundColor $(if($qps -ge 200){"Green"}else{"Yellow"})
Write-Host "`nCluster QPS: $qps (with $($nodes.Length) nodes)"
Write-Host "==========================================="

