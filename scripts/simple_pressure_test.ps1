# 简单压力测试脚本

param(
    [int]$Requests = 1000,
    [int]$Concurrent = 10
)

$url = "http://localhost:8080/kamaitachi/api/data/v1/snapshot"
$body = '{"ids":"operating_income","subjects":"33:00000009,33:00082582","field":"operating_income","order":-1,"limit":10}'

Write-Host "==========================================="
Write-Host "Kamaitachi API Pressure Test"
Write-Host "==========================================="
Write-Host "URL: $url"
Write-Host "Total Requests: $Requests"
Write-Host "Concurrent: $Concurrent"
Write-Host ""

$startTime = Get-Date
$success = 0
$failed = 0
$latencies = @()

Write-Host "Testing..." -ForegroundColor Yellow

# 创建RunspacePool
$pool = [runspacefactory]::CreateRunspacePool(1, $Concurrent)
$pool.Open()
$jobs = @()

# 创建任务
for ($i = 0; $i -lt $Requests; $i++) {
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
    if ($done % 100 -eq 0) {
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

Write-Host ""
Write-Host "==========================================="
Write-Host "Results"
Write-Host "==========================================="
Write-Host "Total: $Requests"
Write-Host "Success: $success" -ForegroundColor Green
Write-Host "Failed: $failed" -ForegroundColor $(if($failed -gt 0){"Red"}else{"Green"})
Write-Host "Duration: $([math]::Round($duration, 2)) sec"
Write-Host ""
Write-Host "QPS: $qps" -ForegroundColor $(if($qps -ge 1000){"Green"}elseif($qps -ge 500){"Yellow"}else{"Red"})
Write-Host ""
Write-Host "Latency (ms):"
Write-Host "  Avg: $avg"
Write-Host "  P50: $p50"
Write-Host "  P95: $p95"
Write-Host "  P99: $p99"
Write-Host ""
Write-Host "==========================================="

