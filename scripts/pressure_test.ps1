# 压力测试脚本

param(
    [int]$TotalRequests = 1000,
    [int]$Concurrent = 10,
    [string]$Url = "http://localhost:8080/kamaitachi/api/data/v1/snapshot"
)

Write-Host "===========================================`n" -ForegroundColor Cyan
Write-Host "  Kamaitachi API 压力测试`n" -ForegroundColor Cyan
Write-Host "===========================================`n" -ForegroundColor Cyan
Write-Host "测试URL: $Url"
Write-Host "总请求数: $TotalRequests"
Write-Host "并发数: $Concurrent"
Write-Host ""

# 测试请求体
$body = @{
    ids = "operating_income,parent_holder_net_profit"
    subjects = "33:00000009,33:00082582,33:01000729"
    field = "operating_income"
    order = -1
    offset = 0
    limit = 10
} | ConvertTo-Json

# 记录开始时间
$startTime = Get-Date

# 成功和失败计数器
$successCount = 0
$failCount = 0
$totalLatency = 0

# 使用RunspacePool实现并发
$runspacePool = [runspacefactory]::CreateRunspacePool(1, $Concurrent)
$runspacePool.Open()
$jobs = @()

Write-Host "开始测试..." -ForegroundColor Yellow
Write-Host ""

# 创建进度跟踪
$completed = 0

# 创建测试任务
for ($i = 0; $i -lt $TotalRequests; $i++) {
    $powershell = [powershell]::Create().AddScript({
        param($url, $body)
        
        $sw = [System.Diagnostics.Stopwatch]::StartNew()
        try {
            $response = Invoke-RestMethod -Uri $url -Method POST -Body $body -ContentType "application/json" -TimeoutSec 30
            $sw.Stop()
            
            return @{
                Success = $true
                Latency = $sw.ElapsedMilliseconds
                StatusCode = 0
            }
        } catch {
            $sw.Stop()
            return @{
                Success = $false
                Latency = $sw.ElapsedMilliseconds
                Error = $_.Exception.Message
            }
        }
    }).AddArgument($Url).AddArgument($body)
    
    $powershell.RunspacePool = $runspacePool
    $jobs += @{
        Pipe = $powershell
        Status = $powershell.BeginInvoke()
    }
}

# 等待所有任务完成并收集结果
$latencies = @()

foreach ($job in $jobs) {
    try {
        $result = $job.Pipe.EndInvoke($job.Status)
        
        if ($result.Success) {
            $successCount++
            $latencies += $result.Latency
            $totalLatency += $result.Latency
        } else {
            $failCount++
        }
        
        $completed++
        
        # 每100个请求显示一次进度
        if ($completed % 100 -eq 0) {
            $progress = [math]::Round(($completed / $TotalRequests) * 100, 2)
            Write-Host "进度: $completed/$TotalRequests ($progress%)" -ForegroundColor Cyan
        }
    } catch {
        $failCount++
        $completed++
    }
    
    $job.Pipe.Dispose()
}

$runspacePool.Close()
$runspacePool.Dispose()

# 计算总耗时
$endTime = Get-Date
$duration = ($endTime - $startTime).TotalSeconds

# 计算统计数据
$qps = [math]::Round($successCount / $duration, 2)
$avgLatency = if ($successCount -gt 0) { [math]::Round($totalLatency / $successCount, 2) } else { 0 }

# 计算百分位延迟
$latencies = $latencies | Sort-Object
$p50 = if ($latencies.Count -gt 0) { $latencies[[math]::Floor($latencies.Count * 0.5)] } else { 0 }
$p95 = if ($latencies.Count -gt 0) { $latencies[[math]::Floor($latencies.Count * 0.95)] } else { 0 }
$p99 = if ($latencies.Count -gt 0) { $latencies[[math]::Floor($latencies.Count * 0.99)] } else { 0 }
$maxLatency = if ($latencies.Count -gt 0) { ($latencies | Measure-Object -Maximum).Maximum } else { 0 }

# 输出结果
Write-Host ""
Write-Host "===========================================`n" -ForegroundColor Cyan
Write-Host "  测试结果`n" -ForegroundColor Cyan
Write-Host "===========================================`n" -ForegroundColor Cyan

Write-Host "总请求数: $TotalRequests"
Write-Host "成功数: $successCount" -ForegroundColor Green
Write-Host "失败数: $failCount" -ForegroundColor $(if($failCount -gt 0){"Red"}else{"Green"})
Write-Host "总耗时: $([math]::Round($duration, 2)) 秒"
Write-Host ""
Write-Host "QPS: $qps" -ForegroundColor $(if($qps -ge 1000){"Green"}else{"Yellow"})
Write-Host ""
Write-Host "延迟统计 (ms):"
Write-Host "  平均: $avgLatency"
Write-Host "  P50: $p50"
Write-Host "  P95: $p95"
Write-Host "  P99: $p99"
Write-Host "  最大: $maxLatency"
Write-Host ""

# 评估结果
if ($qps -ge 1000) {
    Write-Host "结论: 性能达标！QPS >= 1000" -ForegroundColor Green
} elseif ($qps -ge 500) {
    Write-Host "结论: 性能良好，QPS >= 500" -ForegroundColor Yellow
} else {
    Write-Host "结论: 性能需要优化，QPS < 500" -ForegroundColor Red
}

Write-Host ""
Write-Host "===========================================`n" -ForegroundColor Cyan

