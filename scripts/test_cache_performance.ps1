# 测试缓存性能 - 混合查询模式

param(
    [int]$Requests = 2000,
    [int]$Concurrent = 50
)

$url = "http://localhost:8080/kamaitachi/api/data/v1/snapshot"

# 多种查询模式（模拟真实场景）
$queryTemplates = @(
    '{"ids":"operating_income","subjects":"33:00000009","field":"operating_income","order":-1,"limit":10}',
    '{"ids":"operating_income","subjects":"33:00082582","field":"operating_income","order":-1,"limit":10}',
    '{"ids":"operating_income","subjects":"33:01000729","field":"operating_income","order":-1,"limit":10}',
    '{"ids":"operating_income,parent_holder_net_profit","subjects":"33:00000009,33:00082582","field":"operating_income","order":-1,"limit":10}',
    '{"ids":"operating_income","topic":"stock_a_listing_pool","field":"operating_income","order":-1,"limit":20}',
    '{"ids":"parent_holder_net_profit","topic":"stock_a_listing_pool","field":"parent_holder_net_profit","order":-1,"limit":20}'
)

Write-Host "==========================================="
Write-Host "Cache Performance Test (Mixed Queries)"
Write-Host "==========================================="
Write-Host "Requests: $Requests"
Write-Host "Concurrent: $Concurrent"
Write-Host "Query Types: $($queryTemplates.Length)"
Write-Host ""

# 重置缓存统计
try {
    Invoke-RestMethod -Uri "http://localhost:8080/kamaitachi/api/data/v1/cache/reset" -Method POST | Out-Null
    Write-Host "Cache stats reset" -ForegroundColor Green
} catch {
    Write-Host "Failed to reset cache stats" -ForegroundColor Yellow
}

Write-Host "`nTesting..." -ForegroundColor Yellow

$startTime = Get-Date
$success = 0
$failed = 0
$latencies = @()

# 创建RunspacePool
$pool = [runspacefactory]::CreateRunspacePool(1, $Concurrent)
$pool.Open()
$jobs = @()

# 创建任务 - 使用多种查询模式
for ($i = 0; $i -lt $Requests; $i++) {
    $templateIndex = $i % $queryTemplates.Length
    $body = $queryTemplates[$templateIndex]
    
    $ps = [powershell]::Create().AddScript({
        param($u, $b)
        $sw = [Diagnostics.Stopwatch]::StartNew()
        try {
            $r = Invoke-RestMethod -Uri $u -Method POST -Body $b -ContentType "application/json" -TimeoutSec 10
            $sw.Stop()
            return @{ok=$true; ms=$sw.ElapsedMilliseconds}
        } catch {
            $sw.Stop()
            return @{ok=$false; ms=$sw.ElapsedMilliseconds}
        }
    }).AddArgument($url).AddArgument($body)
    
    $ps.RunspacePool = $pool
    $jobs += @{ps=$ps; handle=$ps.BeginInvoke()}
}

# 收集结果
$done = 0
foreach ($job in $jobs) {
    $result = $job.ps.EndInvoke($job.handle)
    if ($result.ok) {
        $success++
        $latencies += $result.ms
    } else {
        $failed++
    }
    $job.ps.Dispose()
    $done++
    if ($done % 200 -eq 0) {
        Write-Host "Progress: $done/$Requests"
    }
}

$pool.Close()
$pool.Dispose()

$endTime = Get-Date
$duration = ($endTime - $startTime).TotalSeconds
$qps = [math]::Round($success / $duration, 2)

# 计算延迟
$avg = if($success -gt 0) { [math]::Round(($latencies | Measure-Object -Average).Average, 2) } else { 0 }
$latencies = $latencies | Sort-Object
$p50 = if($latencies.Count -gt 0) { $latencies[[math]::Floor($latencies.Count * 0.5)] } else { 0 }
$p95 = if($latencies.Count -gt 0) { $latencies[[math]::Floor($latencies.Count * 0.95)] } else { 0 }
$p99 = if($latencies.Count -gt 0) { $latencies[[math]::Floor($latencies.Count * 0.99)] } else { 0 }

# 获取缓存统计
Write-Host "`nFetching cache stats..." -ForegroundColor Cyan
$cacheStats = $null
try {
    $stats = Invoke-RestMethod -Uri "http://localhost:8080/kamaitachi/api/data/v1/stats"
    $cacheStats = $stats.data.cache
} catch {
    Write-Host "Failed to fetch cache stats" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "==========================================="
Write-Host "Results"
Write-Host "==========================================="
Write-Host "Total: $Requests"
Write-Host "Success: $success" -ForegroundColor Green
Write-Host "Failed: $failed" -ForegroundColor $(if($failed -gt 0){"Red"}else{"Green"})
Write-Host "Duration: $([math]::Round($duration, 2)) sec"
Write-Host ""
Write-Host "QPS: $qps" -ForegroundColor $(if($qps -ge 300){"Green"}elseif($qps -ge 200){"Yellow"}else{"Red"})
Write-Host ""
Write-Host "Latency (ms):"
Write-Host "  Avg: $avg"
Write-Host "  P50: $p50"
Write-Host "  P95: $p95"
Write-Host "  P99: $p99"

if ($cacheStats) {
    Write-Host ""
    Write-Host "Cache Stats:"
    Write-Host "  Entries: $($cacheStats.entries)"
    Write-Host "  Hits: $($cacheStats.hits)"
    Write-Host "  Misses: $($cacheStats.misses)"
    Write-Host "  Hit Rate: $($cacheStats.hit_rate)" -ForegroundColor $(if([float]($cacheStats.hit_rate -replace '%','') -ge 80){"Green"}else{"Yellow"})
}

Write-Host ""
Write-Host "==========================================="

