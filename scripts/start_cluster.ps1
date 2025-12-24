# 启动Kamaitachi分布式集群

Write-Host "==========================================="
Write-Host "Kamaitachi Cluster Startup"
Write-Host "==========================================="

# 停止旧进程
Write-Host "`n[1] 清理旧进程..." -ForegroundColor Yellow
Get-Process | Where-Object {$_.Name -match "master|slave|gateway"} | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 2

# 检查etcd
Write-Host "`n[2] 检查etcd..." -ForegroundColor Yellow
$etcdRunning = $false
try {
    $response = Invoke-RestMethod -Uri "http://localhost:2379/version" -TimeoutSec 2 -ErrorAction Stop
    Write-Host "etcd已运行" -ForegroundColor Green
    $etcdRunning = $true
} catch {
    Write-Host "etcd未运行，请先启动etcd" -ForegroundColor Red
    Write-Host "提示: 运行 etcd.exe 启动etcd服务" -ForegroundColor Yellow
    exit 1
}

# 启动Master节点
Write-Host "`n[3] 启动Master节点 (端口8080)..." -ForegroundColor Yellow
Start-Process -FilePath ".\bin\master.exe" -WorkingDirectory "." -WindowStyle Minimized
Start-Sleep -Seconds 3

# 检查Master是否启动成功
try {
    $response = Invoke-RestMethod -Uri "http://localhost:8080/health" -TimeoutSec 2
    Write-Host "Master启动成功" -ForegroundColor Green
} catch {
    Write-Host "Master启动失败" -ForegroundColor Red
    exit 1
}

# 启动Slave节点1
Write-Host "`n[4] 启动Slave节点1 (端口8081)..." -ForegroundColor Yellow
# 复制slave配置
Copy-Item "conf/slave.ini" "conf/slave1.ini" -Force
Start-Process -FilePath ".\bin\slave.exe" -ArgumentList "-config conf/slave1.ini" -WorkingDirectory "." -WindowStyle Minimized
Start-Sleep -Seconds 2

# 启动Slave节点2 (端口8082)
Write-Host "`n[5] 启动Slave节点2 (端口8082)..." -ForegroundColor Yellow
# 创建slave2配置
$slave2Config = Get-Content "conf/slave.ini"
$slave2Config = $slave2Config -replace "port = 8081", "port = 8082"
$slave2Config = $slave2Config -replace "service_addr = localhost:8081", "service_addr = localhost:8082"
$slave2Config = $slave2Config -replace "slave_snapshot.dat", "slave2_snapshot.dat"
$slave2Config | Set-Content "conf/slave2.ini"
Start-Process -FilePath ".\bin\slave.exe" -ArgumentList "-config conf/slave2.ini" -WorkingDirectory "." -WindowStyle Minimized
Start-Sleep -Seconds 2

# 启动Gateway
Write-Host "`n[6] 启动Gateway (端口9000)..." -ForegroundColor Yellow
Start-Process -FilePath ".\bin\gateway.exe" -WorkingDirectory "." -WindowStyle Minimized
Start-Sleep -Seconds 3

# 检查Gateway是否启动成功
try {
    $response = Invoke-RestMethod -Uri "http://localhost:9000/health" -TimeoutSec 2
    Write-Host "Gateway启动成功" -ForegroundColor Green
} catch {
    Write-Host "Gateway启动失败，这是正常的，可能Gateway代码还需要更新" -ForegroundColor Yellow
}

# 显示集群状态
Write-Host "`n==========================================="
Write-Host "集群状态"
Write-Host "==========================================="
Write-Host "Master:  http://localhost:8080" -ForegroundColor Green
Write-Host "Slave1:  http://localhost:8081" -ForegroundColor Green
Write-Host "Slave2:  http://localhost:8082" -ForegroundColor Green
Write-Host "Gateway: http://localhost:9000" -ForegroundColor Yellow

Write-Host "`n提示:"
Write-Host "- 使用Gateway地址进行测试以获得负载均衡效果"
Write-Host "- 运行测试: .\simple_pressure_test.ps1"
Write-Host "- 停止集群: Get-Process | Where {`$_.Name -match 'master|slave|gateway'} | Stop-Process"
Write-Host ""

