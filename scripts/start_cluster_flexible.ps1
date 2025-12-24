# Flexible Cluster Startup Script
# Usage: .\start_cluster_flexible.ps1 -Nodes 6

param(
    [int]$Nodes = 3,
    [switch]$Force
)

Write-Host "==========================================="
Write-Host "KamaitachiGo Cluster Startup"
Write-Host "==========================================="
Write-Host "Target nodes: $Nodes (1 Master + $($Nodes-1) Slaves)" -ForegroundColor Cyan

# Validate node count
if ($Nodes -lt 1 -or $Nodes -gt 15) {
    Write-Host "ERROR: Node count must be between 1-15" -ForegroundColor Red
    exit 1
}

# Clean old processes
Write-Host "`n[1] Cleaning old processes..." -ForegroundColor Yellow
Get-Process -Name master,slave,server -ErrorAction SilentlyContinue | Stop-Process -Force
Get-Job | Remove-Job -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 2
Write-Host "Cleanup completed" -ForegroundColor Green

# Check executables exist
if (-not (Test-Path "bin\master.exe")) {
    Write-Host "ERROR: bin\master.exe not found, please compile first" -ForegroundColor Red
    exit 1
}
if (-not (Test-Path "bin\slave.exe")) {
    Write-Host "ERROR: bin\slave.exe not found, please compile first" -ForegroundColor Red
    exit 1
}

# Start Master node
Write-Host "`n[2] Starting Master node (port:8080)..." -ForegroundColor Yellow
Start-Process -FilePath "bin\master.exe" -ArgumentList "-db data/master.db" -WorkingDirectory "." -WindowStyle Minimized
Start-Sleep -Seconds 3

# Verify Master started
try {
    $response = Invoke-RestMethod -Uri "http://localhost:8080/health" -TimeoutSec 2
    Write-Host "Master started successfully" -ForegroundColor Green
} catch {
    Write-Host "Master failed to start" -ForegroundColor Red
    exit 1
}

# Start Slave nodes
$slaveCount = $Nodes - 1
Write-Host "`n[3] Starting $slaveCount Slave nodes..." -ForegroundColor Yellow

for ($i = 1; $i -le $slaveCount; $i++) {
    $port = 8080 + $i
    $configFile = "conf\slave$i.ini"
    $dbFile = "data\slave$i.db"
    
    # Check config file
    if (-not (Test-Path $configFile)) {
        Write-Host "WARNING: $configFile not found, skipping" -ForegroundColor Yellow
        continue
    }
    
    # Ensure database file exists
    if (-not (Test-Path $dbFile)) {
        Copy-Item "data\master.db" $dbFile
        Write-Host "  Created database: $dbFile" -ForegroundColor Gray
    }
    
    Write-Host "  Starting Slave$i (port:$port)..." -ForegroundColor Cyan
    Start-Process -FilePath "bin\slave.exe" -ArgumentList "-config $configFile -db $dbFile" -WorkingDirectory "." -WindowStyle Minimized
    Start-Sleep -Seconds 1
}

Write-Host "`n[4] Waiting for nodes to be ready..." -ForegroundColor Yellow
Start-Sleep -Seconds 3

# Verify all nodes
Write-Host "`n[5] Verifying node status..." -ForegroundColor Yellow
$successCount = 0
$failCount = 0

# Verify Master
try {
    Invoke-RestMethod -Uri "http://localhost:8080/health" -TimeoutSec 2 | Out-Null
    Write-Host "  Master (8080): OK" -ForegroundColor Green
    $successCount++
} catch {
    Write-Host "  Master (8080): Failed" -ForegroundColor Red
    $failCount++
}

# Verify Slaves
for ($i = 1; $i -le $slaveCount; $i++) {
    $port = 8080 + $i
    try {
        Invoke-RestMethod -Uri "http://localhost:$port/health" -TimeoutSec 2 | Out-Null
        Write-Host "  Slave$i ($port): OK" -ForegroundColor Green
        $successCount++
    } catch {
        Write-Host "  Slave$i ($port): Failed" -ForegroundColor Red
        $failCount++
    }
}

# Summary
Write-Host "`n==========================================="
Write-Host "Cluster Startup Completed"
Write-Host "==========================================="
Write-Host "Success: $successCount nodes" -ForegroundColor Green
Write-Host "Failed: $failCount nodes" -ForegroundColor $(if ($failCount -gt 0) { "Red" } else { "Green" })

if ($failCount -eq 0) {
    Write-Host "`nReady for testing!" -ForegroundColor Green
    Write-Host "Run: .\bin\benchmark.exe -requests 10000 -concurrent 150 -nodes $Nodes"
} else {
    Write-Host "`nSome nodes failed to start, please check logs" -ForegroundColor Yellow
}

Write-Host "`nNode List:"
Write-Host "  Master:  http://localhost:8080"
for ($i = 1; $i -le $slaveCount; $i++) {
    $port = 8080 + $i
    Write-Host "  Slave$i : http://localhost:$port"
}
Write-Host ""
