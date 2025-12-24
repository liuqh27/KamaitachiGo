# API测试脚本

$baseUrl = "http://localhost:8080"

Write-Host "==========================================="
Write-Host "开始测试 Kamaitachi Finance API"
Write-Host "==========================================="

# 1. 健康检查
Write-Host ""
Write-Host "[测试1] 健康检查"
Write-Host "--------------------------------------"
try {
    $response = Invoke-RestMethod -Uri "$baseUrl/health" -Method GET
    Write-Host "状态: OK" -ForegroundColor Green
    Write-Host ($response | ConvertTo-Json)
} catch {
    Write-Host "失败: $_" -ForegroundColor Red
}

# 2. 快照查询 - 指定证券
Write-Host ""
Write-Host "[测试2] 快照查询 - 指定证券"
Write-Host "--------------------------------------"
$snapshotBody = @{
    ids = "operating_income,parent_holder_net_profit"
    subjects = "33:000001,33:000002,33:000003"
    field = "operating_income"
    order = -1
    offset = 0
    limit = 5
    timestamp = 0
} | ConvertTo-Json

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/kamaitachi/api/data/v1/snapshot" -Method POST -Body $snapshotBody -ContentType "application/json"
    Write-Host "状态: OK" -ForegroundColor Green
    Write-Host "返回记录数: $($response.data.Length)"
    if ($response.data.Length -gt 0) {
        Write-Host "第一条记录示例:"
        Write-Host ($response.data[0] | ConvertTo-Json -Depth 5)
    }
} catch {
    Write-Host "失败: $_" -ForegroundColor Red
}

# 3. 快照查询 - 全市场
Write-Host ""
Write-Host "[测试3] 快照查询 - 全市场（topic）"
Write-Host "--------------------------------------"
$topicBody = @{
    ids = "operating_income,parent_holder_net_profit"
    topic = "stock_a_listing_pool"
    field = "operating_income"
    order = -1
    offset = 0
    limit = 10
    timestamp = 0
} | ConvertTo-Json

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/kamaitachi/api/data/v1/snapshot" -Method POST -Body $topicBody -ContentType "application/json"
    Write-Host "状态: OK" -ForegroundColor Green
    Write-Host "返回记录数: $($response.data.Length)"
    if ($response.data.Length -gt 0) {
        Write-Host "前3条记录:"
        for ($i = 0; $i -lt [Math]::Min(3, $response.data.Length); $i++) {
            Write-Host "  [$i] $($response.data[$i].subject.subject) - $($response.data[$i].subject.name)"
        }
    }
} catch {
    Write-Host "失败: $_" -ForegroundColor Red
}

# 4. 区间查询
Write-Host ""
Write-Host "[测试4] 区间查询"
Write-Host "--------------------------------------"
$periodBody = @{
    ids = "operating_income,parent_holder_net_profit"
    subjects = "33:000001,33:000002"
    from = 1609459200
    to = 1640995200
} | ConvertTo-Json

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/kamaitachi/api/data/v1/period" -Method POST -Body $periodBody -ContentType "application/json"
    Write-Host "状态: OK" -ForegroundColor Green
    Write-Host "返回证券数: $($response.data.Length)"
    if ($response.data.Length -gt 0) {
        Write-Host "第一个证券的数据点数: $($response.data[0].data.Length)"
    }
} catch {
    Write-Host "失败: $_" -ForegroundColor Red
}

# 5. 统计信息
Write-Host ""
Write-Host "[测试5] 统计信息"
Write-Host "--------------------------------------"
try {
    $response = Invoke-RestMethod -Uri "$baseUrl/kamaitachi/api/data/v1/stats" -Method GET
    Write-Host "状态: OK" -ForegroundColor Green
    Write-Host ($response | ConvertTo-Json)
} catch {
    Write-Host "失败: $_" -ForegroundColor Red
}

# 6. 监控 - 熔断器状态
Write-Host ""
Write-Host "[测试6] 监控 - 熔断器状态"
Write-Host "--------------------------------------"
try {
    $response = Invoke-RestMethod -Uri "$baseUrl/monitor/circuitbreaker" -Method GET
    Write-Host "状态: OK" -ForegroundColor Green
    Write-Host ($response | ConvertTo-Json)
} catch {
    Write-Host "失败: $_" -ForegroundColor Red
}

Write-Host ""
Write-Host "==========================================="
Write-Host "测试完成"
Write-Host "==========================================="
