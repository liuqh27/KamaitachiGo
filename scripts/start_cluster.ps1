# KamaitachiGo Cluster Startup Script
#
# 注意: 如果Go程序输出的中文在PowerShell中显示乱码,
# 请尝试在执行此脚本前运行以下命令:
# $OutputEncoding = [System.Text.Encoding]::UTF8

# --- Configuration ---
$MasterPort = 8080
$Slave1Port = 8081
$Slave2Port = 8082
$GatewayPort = 9000
$EtcdPort = 2379

# --- Helper Functions ---
function Stop-KamaitachiProcesses {
    Write-Host "`n[STEP 1/5] Stopping any running KamaitachiGo processes and clearing ports..." -ForegroundColor Yellow
    $ports = @($MasterPort, $Slave1Port, $Slave2Port, $GatewayPort)
    foreach ($port in $ports) {
        $pids = (netstat -ano | Select-String ":$port\s+LISTENING" | ForEach-Object { ($_ -split '\s+')[-1] } | Sort-Object -Unique)
        foreach ($pid in $pids) {
            if ($pid -ne $null -and $pid -ne "0") {
                Write-Host "  Killing process $pid on port $port..." -ForegroundColor DarkYellow
                try {
                    Stop-Process -Id $pid -Force -ErrorAction Stop
                } catch {
                    $errorMessage = $_.Exception.Message
                    Write-Host ("  Failed to kill process " + $pid + ": " + $errorMessage) -ForegroundColor Red
                }
            }
        }
    }
    # Also kill by name for safety
    Get-Process | Where-Object {$_.ProcessName -match "master|slave|gateway|benchmark_scenarios"} | Stop-Process -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 2
    Write-Host "  Cleanup complete." -ForegroundColor Green
}

function Check-Health {
    param(
        [string]$Url,
        [string]$ServiceName
    )
    Write-Host "  Checking status of $ServiceName..." -ForegroundColor DarkGray
    for ($i = 0; $i -lt 15; $i++) {
        try {
            $response = Invoke-RestMethod -Uri $Url -TimeoutSec 1 -ErrorAction Stop
            if ($response.status -eq "ok") {
                Write-Host "  $ServiceName ($Url) is UP." -ForegroundColor Green
                return $true
            }
        } catch {
            # Continue trying
        }
        Start-Sleep -Milliseconds 500
    }
    Write-Host "  $ServiceName ($Url) FAILED to start." -ForegroundColor Red
    return $false
}

# --- Main Script ---

# 1. Stop any existing processes
Stop-KamaitachiProcesses

# 2. Check Etcd
Write-Host "`n[STEP 2/5] Checking for etcd..." -ForegroundColor Yellow
try {
    Invoke-RestMethod -Uri "http://localhost:$EtcdPort/version" -TimeoutSec 2 -ErrorAction Stop | Out-Null
    Write-Host "  etcd is running." -ForegroundColor Green
} catch {
    Write-Host "  etcd is NOT running. Please start etcd (e.g., by running etcd.exe) and try again." -ForegroundColor Red
    exit 1
}

# 3. Build Components
Write-Host "`n[STEP 3/5] Building KamaitachiGo components..." -ForegroundColor Yellow
if (-not (Test-Path "bin")) {
    New-Item -ItemType Directory -Force -Path "bin" | Out-Null
}
go build -v -o bin/master.exe cmd/master/main.go
if ($LASTEXITCODE -ne 0) { Write-Host "Master build failed. Exiting." -ForegroundColor Red; exit 1 }
go build -v -o bin/slave.exe cmd/slave/main.go
if ($LASTEXITCODE -ne 0) { Write-Host "Slave build failed. Exiting." -ForegroundColor Red; exit 1 }
go build -v -o bin/gateway.exe cmd/gateway/main.go
if ($LASTEXITCODE -ne 0) { Write-Host "Gateway build failed. Exiting." -ForegroundColor Red; exit 1 }
go build -v -o bin/benchmark_scenarios.exe tools/benchmark_scenarios.go
if ($LASTEXITCODE -ne 0) { Write-Host "Benchmark tool build failed. Exiting." -ForegroundColor Red; exit 1 }
Write-Host "  All components built successfully to 'bin/' directory." -ForegroundColor Green

# 4. Start Cluster
Write-Host "`n[STEP 4/5] Starting KamaitachiGo cluster (Master, 2 Slaves, Gateway)..." -ForegroundColor Yellow
Start-Process -FilePath ".\bin\master.exe" -WindowStyle Minimized
Start-Process -FilePath ".\bin\slave.exe"   -ArgumentList "-config conf/slave1.ini", "-db data/slave1.db" -WindowStyle Minimized
Start-Process -FilePath ".\bin\slave.exe"   -ArgumentList "-config conf/slave2.ini", "-db data/slave2.db" -WindowStyle Minimized
Start-Process -FilePath ".\bin\gateway.exe" -ArgumentList "-config conf/gateway.ini" -WindowStyle Minimized
Write-Host "  All processes started in minimized windows." -ForegroundColor Green

# 5. Wait for services to be ready and verify
Write-Host "`n[STEP 5/5] Verifying services..." -ForegroundColor Yellow
$allUp = $true
if (-not (Check-Health "http://localhost:$MasterPort/health" "Master"))  { $allUp = $false }
if (-not (Check-Health "http://localhost:$Slave1Port/health" "Slave1"))  { $allUp = $false }
if (-not (Check-Health "http://localhost:$Slave2Port/health" "Slave2"))  { $allUp = $false }
if (-not (Check-Health "http://localhost:$GatewayPort/health" "Gateway")) { $allUp = $false }

if ($allUp) {
    Write-Host "`n==================================================" -ForegroundColor Green
    Write-Host "  KamaitachiGo Cluster is UP and READY!" -ForegroundColor Green
    Write-Host "=================================================="
    Write-Host "  Gateway is listening at: http://localhost:$GatewayPort"
    Write-Host "`n  To run a benchmark, use the following command:"
    Write-Host "  .\bin\benchmark_scenarios.exe -target http://localhost:9000/data -repeat 10" -ForegroundColor Cyan
} else {
    Write-Host "`n==================================================" -ForegroundColor Red
    Write-Host "  One or more services failed to start." -ForegroundColor Red
    Write-Host "  Please check the logs in the 'logs' directory." -ForegroundColor Red
    Write-Host "=================================================="
}
