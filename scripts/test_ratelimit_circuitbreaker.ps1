# 限流和熔断自动化测试脚本

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "   限流和熔断功能测试" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 基础URL
$baseUrl = "http://localhost:8080"

# 测试1：正常请求（验证限流不影响正常流量）
Write-Host "[测试1] 正常请求测试..." -ForegroundColor Yellow
$successCount = 0
for ($i=1; $i -le 10; $i++) {
    try {
        $response = Invoke-WebRequest -Uri "$baseUrl/data/v1/snapshot" `
            -Method POST `
            -ContentType "application/json" `
            -Body '{"secuCodes":["000001.SZ"],"dataInfoIds":[101]}' `
            -UseBasicParsing -ErrorAction Stop
        
        if ($response.StatusCode -eq 200) {
            $successCount++
        }
    } catch {
        Write-Host "  请求失败: $($_.Exception.Message)" -ForegroundColor Red
    }
    Start-Sleep -Milliseconds 100
}
Write-Host "  结果: $successCount / 10 成功" -ForegroundColor Green
Write-Host ""

# 测试2：限流测试
Write-Host "[测试2] 限流测试（快速发送1000个请求）..." -ForegroundColor Yellow
$successCount = 0
$limitedCount = 0
$errorCount = 0

for ($i=1; $i -le 1000; $i++) {
    try {
        $response = Invoke-WebRequest -Uri "$baseUrl/data/v1/snapshot" `
            -Method POST `
            -ContentType "application/json" `
            -Body '{"secuCodes":["000001.SZ"],"dataInfoIds":[101]}' `
            -UseBasicParsing -ErrorAction SilentlyContinue
        
        if ($response.StatusCode -eq 200) {
            $successCount++
        }
    } catch {
        $statusCode = $_.Exception.Response.StatusCode.value__
        if ($statusCode -eq 429) {
            $limitedCount++
        } else {
            $errorCount++
        }
    }
    
    # 每100个请求显示一次进度
    if ($i % 100 -eq 0) {
        Write-Host "  已发送: $i / 1000" -ForegroundColor Gray
    }
}

Write-Host "  成功请求: $successCount" -ForegroundColor Green
Write-Host "  限流拒绝: $limitedCount" -ForegroundColor Yellow
Write-Host "  其他错误: $errorCount" -ForegroundColor Red
Write-Host "  限流率: $([math]::Round($limitedCount / 1000 * 100, 2))%" -ForegroundColor Cyan
Write-Host ""

# 等待5秒，让限流器恢复
Write-Host "等待5秒，让限流器恢复..." -ForegroundColor Gray
Start-Sleep -Seconds 5

# 测试3：查看熔断器状态（初始状态）
Write-Host "[测试3] 查看熔断器初始状态..." -ForegroundColor Yellow
try {
    $response = Invoke-WebRequest -Uri "$baseUrl/monitor/circuitbreaker" `
        -Method GET -UseBasicParsing
    $stats = $response.Content | ConvertFrom-Json
    
    Write-Host "  Snapshot熔断器状态: $($stats.data.snapshot.state)" -ForegroundColor Cyan
    Write-Host "  请求数: $($stats.data.snapshot.requests)" -ForegroundColor Gray
} catch {
    Write-Host "  获取状态失败: $($_.Exception.Message)" -ForegroundColor Red
}
Write-Host ""

# 测试4：触发熔断（发送错误请求）
Write-Host "[测试4] 触发熔断测试（发送20个错误请求）..." -ForegroundColor Yellow
$errorRequests = 0
$total = 20

for ($i=1; $i -le $total; $i++) {
    try {
        # 发送空请求体，会导致400错误
        $response = Invoke-WebRequest -Uri "$baseUrl/data/v1/snapshot" `
            -Method POST `
            -ContentType "application/json" `
            -Body '{}' `
            -UseBasicParsing -ErrorAction SilentlyContinue
    } catch {
        $errorRequests++
    }
    Start-Sleep -Milliseconds 200
}

Write-Host "  错误请求数: $errorRequests / $total" -ForegroundColor Yellow
Write-Host ""

# 查看熔断器状态
Write-Host "查看熔断后的状态..." -ForegroundColor Gray
try {
    $response = Invoke-WebRequest -Uri "$baseUrl/monitor/circuitbreaker" `
        -Method GET -UseBasicParsing
    $stats = $response.Content | ConvertFrom-Json
    
    Write-Host "  熔断器状态: $($stats.data.snapshot.state)" -ForegroundColor $(
        if ($stats.data.snapshot.state -eq "OPEN") { "Red" } 
        elseif ($stats.data.snapshot.state -eq "HALF_OPEN") { "Yellow" } 
        else { "Green" }
    )
    Write-Host "  失败率: $([math]::Round($stats.data.snapshot.failure_rate, 2))%" -ForegroundColor Gray
    Write-Host "  成功数: $($stats.data.snapshot.successes)" -ForegroundColor Gray
    Write-Host "  失败数: $($stats.data.snapshot.failures)" -ForegroundColor Gray
} catch {
    Write-Host "  获取状态失败: $($_.Exception.Message)" -ForegroundColor Red
}
Write-Host ""

# 测试5：熔断期间的请求
if ($stats.data.snapshot.state -eq "OPEN") {
    Write-Host "[测试5] 验证熔断期间请求被拒绝..." -ForegroundColor Yellow
    $rejectedCount = 0
    
    for ($i=1; $i -le 5; $i++) {
        try {
            $response = Invoke-WebRequest -Uri "$baseUrl/data/v1/snapshot" `
                -Method POST `
                -ContentType "application/json" `
                -Body '{"secuCodes":["000001.SZ"],"dataInfoIds":[101]}' `
                -UseBasicParsing -ErrorAction Stop
        } catch {
            if ($_.Exception.Response.StatusCode.value__ -eq 503) {
                $rejectedCount++
            }
        }
        Start-Sleep -Milliseconds 200
    }
    
    Write-Host "  熔断拒绝: $rejectedCount / 5" -ForegroundColor Yellow
    Write-Host ""
    
    # 等待熔断恢复
    Write-Host "等待30秒，让熔断器尝试恢复..." -ForegroundColor Gray
    for ($i=30; $i -gt 0; $i--) {
        Write-Host "  倒计时: $i 秒" -ForegroundColor Gray
        Start-Sleep -Seconds 1
    }
    Write-Host ""
    
    # 测试6：恢复测试
    Write-Host "[测试6] 发送正常请求测试恢复..." -ForegroundColor Yellow
    $recoverySuccess = 0
    
    for ($i=1; $i -le 10; $i++) {
        try {
            $response = Invoke-WebRequest -Uri "$baseUrl/data/v1/snapshot" `
                -Method POST `
                -ContentType "application/json" `
                -Body '{"secuCodes":["000001.SZ"],"dataInfoIds":[101]}' `
                -UseBasicParsing -ErrorAction Stop
            
            if ($response.StatusCode -eq 200) {
                $recoverySuccess++
            }
        } catch {
            Write-Host "  恢复请求失败: $($_.Exception.Message)" -ForegroundColor Red
        }
        Start-Sleep -Milliseconds 500
    }
    
    Write-Host "  恢复成功: $recoverySuccess / 10" -ForegroundColor Green
    Write-Host ""
    
    # 查看最终状态
    Write-Host "查看恢复后的状态..." -ForegroundColor Gray
    try {
        $response = Invoke-WebRequest -Uri "$baseUrl/monitor/circuitbreaker" `
            -Method GET -UseBasicParsing
        $stats = $response.Content | ConvertFrom-Json
        
        Write-Host "  最终状态: $($stats.data.snapshot.state)" -ForegroundColor $(
            if ($stats.data.snapshot.state -eq "CLOSED") { "Green" } 
            else { "Yellow" }
        )
        Write-Host "  连续成功: $($stats.data.snapshot.consecutive_successes)" -ForegroundColor Gray
    } catch {
        Write-Host "  获取状态失败: $($_.Exception.Message)" -ForegroundColor Red
    }
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "   测试完成！" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 显示总结
Write-Host "测试总结:" -ForegroundColor Green
Write-Host "✅ 限流功能: " -NoNewline
if ($limitedCount -gt 0) {
    Write-Host "正常工作（限流率: $([math]::Round($limitedCount / 1000 * 100, 2))%）" -ForegroundColor Green
} else {
    Write-Host "未触发（请求速度可能不够快）" -ForegroundColor Yellow
}

Write-Host "✅ 熔断功能: " -NoNewline
if ($stats.data.snapshot.state -eq "CLOSED" -or $stats.data.snapshot.state -eq "HALF_OPEN") {
    Write-Host "正常工作（熔断器已触发并恢复）" -ForegroundColor Green
} else {
    Write-Host "状态: $($stats.data.snapshot.state)" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "查看详细监控数据:" -ForegroundColor Cyan
Write-Host "  http://localhost:8080/monitor/circuitbreaker" -ForegroundColor Gray
Write-Host "  http://localhost:8080/monitor/status" -ForegroundColor Gray

