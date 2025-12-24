# 启动3节点集群（简化版）
# 启动3节点集群（简化版）

Write-Host "==========================================="
Write-Host "启动3节点集群"
Write-Host "==========================================="

# 停止旧进程
Write-Host "`n[1] 清理旧进程..." -ForegroundColor Yellow
Get-Process -Name master,slave,server -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Get-Job | Remove-Job -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 2

# 启动Master节点
Write-Host "`n[2] 启动Master节点 (端口8080)..." -ForegroundColor Yellow
Start-Process -FilePath ".\bin\master.exe" -ArgumentList "-db data/master.db" -WorkingDirectory "." -WindowStyle Minimized
Start-Sleep -Seconds 3

# 启动Slave1 (端口8081)
Write-Host "`n[3] 启动Slave节点1 (端口8081)..." -ForegroundColor Yellow
Start-Process -FilePath ".\bin\slave.exe" -ArgumentList "-config conf/slave1.ini -db data/slave1.db" -WorkingDirectory "." -WindowStyle Minimized
Start-Sleep -Seconds 2

# 启动Slave2 (端口8082)
Write-Host "`n[4] 启动Slave节点2 (端口8082)..." -ForegroundColor Yellow
Start-Process -FilePath ".\bin\slave.exe" -ArgumentList "-config conf/slave2.ini -db data/slave2.db" -WorkingDirectory "." -WindowStyle Minimized
Start-Sleep -Seconds 2

Write-Host "`n==========================================="
Write-Host "集群已启动" -ForegroundColor Green
Write-Host "==========================================="
Write-Host "Master:  http://localhost:8080"
Write-Host "Slave1:  http://localhost:8081"
Write-Host "Slave2:  http://localhost:8082"
Write-Host ""
Write-Host "等待5秒后自动开始测试..." -ForegroundColor Yellow
Start-Sleep -Seconds 5

