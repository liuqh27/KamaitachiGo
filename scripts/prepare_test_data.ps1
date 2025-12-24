# 准备选股测试数据
Write-Host "正在准备选股测试数据..." -ForegroundColor Green

# 插入多个股票的测试数据
$stocks = @(
    @{id="17:600606"; name="绿地控股"; income=51567755124.46; profit=82032393.66},
    @{id="33:000703"; name="恒逸石化"; income=31655506086.59; profit=413692260.32},
    @{id="33:002416"; name="爱施德"; income=21628731694.48; profit=167910610.45},
    @{id="17:601233"; name="桐昆股份"; income=21111384064.63; profit=579923332.09},
    @{id="33:300226"; name="上海钢联"; income=17541791066.96; profit=49155080.18}
)

foreach ($stock in $stocks) {
    $body = @{
        id = $stock.id
        data = @{
            "1672502400" = @($stock.profit, $stock.income, 300.1)
            "1672588800" = @($stock.profit * 1.1, $stock.income * 1.1, 302.3)
        }
        createTime = (Get-Date -Format "yyyy-MM-dd HH:mm:ss")
        updateTime = [DateTimeOffset]::Now.ToUnixTimeSeconds()
    } | ConvertTo-Json -Depth 10
    
    try {
        $response = Invoke-WebRequest -Uri "http://localhost:8080/data/v1/save?id=1" `
            -Method POST `
            -ContentType "application/json" `
            -Body $body `
            -UseBasicParsing
        
        Write-Host "已保存: $($stock.id) - $($stock.name)" -ForegroundColor Cyan
    } catch {
        Write-Host "保存失败: $($stock.id) - $_" -ForegroundColor Red
    }
    
    Start-Sleep -Milliseconds 100
}

Write-Host "`n测试数据准备完成！" -ForegroundColor Green

