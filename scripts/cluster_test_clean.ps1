# 改进的集群压力测试（自动清理资源）

param(
    [int]$Requests = 2000,
    [int]$Concurrent = 50
)

# 清理旧的Job和资源
Write-Host "清理旧资源..." -ForegroundColor Yellow
Get-Job | Remove-Job -Force -ErrorAction SilentlyContinue
[System.GC]::Collect()
[System.GC]::WaitForPendingFinalizers()
Start-Sleep -Milliseconds 500

$nodes = @("http://localhost:8080", "http://localhost:8081", "http://localhost:8082")
$body = '{"ids":"operating_income","subjects":"33:00000009,33:00082582","field":"operating_income","order":-1,"limit":10}'

Write-Host "==========================================="
Write-Host "Cluster Test (Clean)"
Write-Host "==========================================="
Write-Host "Nodes: $($nodes.Length)"
Write-Host "Requests: $Requests"
Write-Host "Concurrent: $Concurrent"
Write-Host ""

# 检查节点
Write-Host "Nodes:" -ForegroundColor Cyan
foreach ($node in $nodes) {
    try {
        Invoke-RestMethod -Uri "$node/health" -TimeoutSec 1 | Out-Null
        Write-Host "  $node - OK" -ForegroundColor Green
    } catch {
        Write-Host "  $node - FAIL" -ForegroundColor Red
    }
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

# 创建任务
for ($i = 0; $i -lt $Requests; $i++) {
    $nodeIndex = $i % $nodes.Length
    $nodeUrl = $nodes[$nodeIndex]
    
    $ps = [powershell]::Create().AddScript({
        param($u, $b)
        $sw = [Diagnostics.Stopwatch]::StartNew()
        try {
            $r = Invoke-RestMethod -Uri "$u/kamaitachi/api/data/v1/snapshot" -Method POST -Body $b -ContentType "application/json" -TimeoutSec 10
            $sw.Stop()
            return @{ok=$true; ms=$sw.ElapsedMilliseconds}
        } catch {
            $sw.Stop()
            return @{ok=$false; ms=$sw.ElapsedMilliseconds}
        }
    }).AddArgument($nodeUrl).AddArgument($body)
    
    $ps.RunspacePool = $pool
    $jobs += @{ps=$ps; handle=$ps.BeginInvoke()}
}

# 收集结果
$done = 0
foreach ($job in $jobs) {
    try {
        $result = $job.ps.EndInvoke($job.handle)
        if ($result.ok) {
            $success++
            $latencies += $result.ms
        } else {
            $failed++
        }
    } catch {
        $failed++
    }
    
    # 立即清理PowerShell对象
    $job.ps.Dispose()
    
    $done++
    if ($done % 200 -eq 0) {
        Write-Host "Progress: $done/$Requests"
    }
}

# 关闭并清理RunspacePool
$pool.Close()
$pool.Dispose()

# 强制垃圾回收
[System.GC]::Collect()

$endTime = Get-Date
$duration = ($endTime - $startTime).TotalSeconds
$qps = [math]::Round($success / $duration, 2)

# 计算延迟
$avg = if($success -gt 0) { [math]::Round(($latencies | Measure-Object -Average).Average, 2) } else { 0 }
$latencies = $latencies | Sort-Object
$p50 = if($latencies.Count -gt 0) { $latencies[[math]::Floor($latencies.Count * 0.5)] } else { 0 }
$p95 = if($latencies.Count -gt 0) { $latencies[[math]::Floor($latencies.Count * 0.95)] } else { 0 }
$p99 = if($latencies.Count -gt 0) { $latencies[[math]::Floor($latencies.Count * 0.99)] } else { 0 }

Write-Host ""
Write-Host "==========================================="
Write-Host "Results"
Write-Host "==========================================="
Write-Host "Total: $Requests"
Write-Host "Success: $success" -ForegroundColor Green
Write-Host "Failed: $failed" -ForegroundColor $(if($failed -gt 0){"Red"}else{"Green"})
Write-Host "Duration: $([math]::Round($duration, 2)) sec"
Write-Host ""
Write-Host "QPS: $qps" -ForegroundColor $(if($qps -ge 250){"Green"}elseif($qps -ge 200){"Yellow"}else{"Red"})
Write-Host ""
Write-Host "Latency (ms):"
Write-Host "  Avg: $avg"
Write-Host "  P50: $p50"
Write-Host "  P95: $p95"
Write-Host "  P99: $p99"
Write-Host ""
Write-Host "==========================================="

