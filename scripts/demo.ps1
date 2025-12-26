# demo.ps1
# This script demonstrates the performance improvement of the KamaitachiGo system
# after implementing consistent hashing and data-aware caching.
#
# It performs two benchmark runs:
# 1. Low Hit Rate Scenario: Simulates random requests (or very few repeated requests)
#    to show a baseline or pre-optimization state where cache hit is low.
# 2. High Hit Rate Scenario: Uses the -repeat flag to send a fixed set of requests
#    repeatedly, demonstrating the effectiveness of the optimized cache.

# --- Configuration ---
$GatewayPort = 9000
$MasterPort = 8080
$Slave1Port = 8081
$Slave2Port = 8082
$EtcdPort = 2379

$BenchmarkRequests = 10000 # Total requests for each benchmark run
$BenchmarkConcurrent = 50  # Concurrent workers for benchmark
$LowHitRepeat = 1          # For low hit rate, generate only 1 unique request
$HighHitRepeat = 10        # For high hit rate, generate 10 unique requests and repeat them

# --- Temporary files for build output ---
$tmpBuildOut = Join-Path $PSScriptRoot "tmp_build_output.txt"
$tmpBuildErr = Join-Path $PSScriptRoot "tmp_build_error.txt"

# --- Helper Functions ---
function Stop-KamaitachiProcesses {
    Write-Host "`nStopping any running KamaitachiGo processes and clearing ports..." -ForegroundColor Yellow
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

    # Also kill by name, in case processes are not listening on the expected ports or for benchmark.exe
    Get-Process | Where-Object {$_.ProcessName -match "master|slave|gateway|benchmark_scenarios"} | Stop-Process -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 2 # Give processes time to terminate

    # Clear log files
    Write-Host "  Clearing old log files..." -ForegroundColor DarkGray
    Get-ChildItem -Path "logs/*.log", "logs/*.err.log", "logs/*.txt" -ErrorAction SilentlyContinue | Remove-Item -Force
}

function Check-Health {
    param(
        [string]$Url,
        [string]$ServiceName
    )
    for ($i = 0; $i -lt 10; $i++) {
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
    Write-Host "  $ServiceName ($Url) failed to start." -ForegroundColor Red
    return $false
}

function Get-GatewayStats {
    $statsUrl = "http://localhost:$GatewayPort/kamaitachi/api/data/v1/stats"
    Write-Host "`nCollecting Gateway Stats from $statsUrl..." -ForegroundColor Cyan
    try {
        $stats = Invoke-RestMethod -Uri $statsUrl -TimeoutSec 5 -ErrorAction Stop
        Write-Host ($stats | ConvertTo-Json -Depth 5)
        return $stats
    } catch {
        Write-Host "Failed to get gateway stats: $($_.Exception.Message)" -ForegroundColor Red
        return $null
    }
}

# --- Main Script ---

# 1. Stop any existing processes
Stop-KamaitachiProcesses

# 2. Check Etcd
Write-Host "`nChecking etcd..." -ForegroundColor Yellow
try {
    Invoke-RestMethod -Uri "http://localhost:$EtcdPort/version" -TimeoutSec 2 -ErrorAction Stop | Out-Null
    Write-Host "  etcd is running." -ForegroundColor Green
} catch {
    Write-Host "  etcd is NOT running. Please start etcd (e.g., by running etcd.exe) and try again." -ForegroundColor Red
    exit 1
}

# 3. Build Components
Write-Host "`nBuilding KamaitachiGo components..." -ForegroundColor Yellow
if (-not (Test-Path "bin")) {
    New-Item -ItemType Directory -Force -Path "bin" | Out-Null
    Write-Host "Created 'bin' directory." -ForegroundColor DarkGray
}
if (-not (Test-Path "logs")) {
    New-Item -ItemType Directory -Force -Path "logs" | Out-Null
    Write-Host "Created 'logs' directory." -ForegroundColor DarkGray
}

go build -v -o bin/master cmd/master/main.go
if ($LASTEXITCODE -ne 0) { Write-Host "Master build failed. Exiting." -ForegroundColor Red; exit 1 }
go build -v -o bin/slave cmd/slave/main.go
if ($LASTEXITCODE -ne 0) { Write-Host "Slave build failed. Exiting." -ForegroundColor Red; exit 1 }
go build -v -o bin/gateway cmd/gateway/main.go
if ($LASTEXITCODE -ne 0) { Write-Host "Gateway build failed. Exiting." -ForegroundColor Red; exit 1 }
go build -v -o bin/benchmark_scenarios.exe tools/benchmark_scenarios.go > $tmpBuildOut 2> $tmpBuildErr
if ($LASTEXITCODE -ne 0) { Write-Host "Benchmark build failed. Exiting." -ForegroundColor Red; exit 1 }

if (-not (Test-Path "bin/benchmark_scenarios.exe")) {
    Write-Host "Error: bin/benchmark_scenarios.exe not found after build! This indicates a problem with 'go build'." -ForegroundColor Red
    Write-Host "`n--- go build STDOUT ---" -ForegroundColor Red
    Get-Content $tmpBuildOut
    Write-Host "`n--- go build STDERR ---" -ForegroundColor Red
    Get-Content $tmpBuildErr
    Remove-Item $tmpBuildOut, $tmpBuildErr -ErrorAction SilentlyContinue
    exit 1
}
Remove-Item $tmpBuildOut, $tmpBuildErr -ErrorAction SilentlyContinue
Write-Host "All components built successfully." -ForegroundColor Green

# 4. Generate Sample Data
Write-Host "`nGenerating sample database (data/sample.db)..." -ForegroundColor Yellow
go run tools/generate_sample_db.go
Write-Host "Sample data generated." -ForegroundColor Green

# 5. Start Cluster
Write-Host "`nStarting KamaitachiGo cluster (Master, Slaves, Gateway)..." -ForegroundColor Yellow
Start-Process -FilePath ".\bin\master.exe" -ArgumentList @("-db", "data/master.db") -WindowStyle Minimized
Start-Process -FilePath ".\bin\slave.exe" -ArgumentList @("-config", "conf/slave.ini", "-db", "data/slave1.db") -WindowStyle Minimized
Start-Process -FilePath ".\bin\slave.exe" -ArgumentList @("-config", "conf/slave2.ini", "-db", "data/slave2.db") -WindowStyle Minimized
Start-Process -FilePath ".\bin\gateway.exe" -ArgumentList @("-config", "conf/gateway.ini") -WindowStyle Minimized

# 6. Wait for services to be ready
Check-Health "http://localhost:$MasterPort/health" "Master" | Out-Null
Check-Health "http://localhost:$Slave1Port/health" "Slave1" | Out-Null
Check-Health "http://localhost:$Slave2Port/health" "Slave2" | Out-Null
Check-Health "http://localhost:$GatewayPort/health" "Gateway" | Out-Null
Write-Host "`nAll services should be up." -ForegroundColor Green

# Optional: Reset cache stats before each run for cleaner comparison
# This would require adding a /resetStats endpoint to slave and gateway
# For now, we just let them accumulate and check current state.

# --- Scenario 1: Low Hit Rate (simulate pre-optimization or highly random queries) ---
Write-Host "`n=======================================================" -ForegroundColor White
Write-Host "  Scenario 1: Low Hit Rate (Highly Random Query Pattern)" -ForegroundColor White
Write-Host "=======================================================" -ForegroundColor White
Write-Host "  Running benchmark with -repeat $LowHitRepeat (few unique requests)" -ForegroundColor Cyan

& ".\bin\benchmark_scenarios.exe" `
    -scenario 1 `
    -requests $BenchmarkRequests `
    -concurrent $BenchmarkConcurrent `
    -target "http://localhost:$GatewayPort/data" `
    -repeat $LowHitRepeat

Write-Host "`nBenchmark for Low Hit Rate Scenario completed." -ForegroundColor Green
$stats1 = Get-GatewayStats
if ($stats1) {
    Write-Host "`n--- Expected: Low Cache Hit Rate (e.g., < 20%) ---" -ForegroundColor Yellow
}
Start-Sleep -Seconds 5 # Give some time for stats to update

# --- Scenario 2: High Hit Rate (demonstrate optimized cache) ---
Write-Host "`n=======================================================" -ForegroundColor White
Write-Host "  Scenario 2: High Hit Rate (Repeatable Query Pattern)" -ForegroundColor White
Write-Host "=======================================================" -ForegroundColor White
Write-Host "  Running benchmark with -repeat $HighHitRepeat (fixed, repeated unique requests)" -ForegroundColor Cyan

& ".\bin\benchmark_scenarios.exe" `
    -scenario 1 `
    -requests $BenchmarkRequests `
    -concurrent $BenchmarkConcurrent `
    -target "http://localhost:$GatewayPort/data" `
    -repeat $HighHitRepeat

Write-Host "`nBenchmark for High Hit Rate Scenario completed." -ForegroundColor Green
$stats2 = Get-GatewayStats
if ($stats2) {
    Write-Host "`n--- Expected: High Cache Hit Rate (e.g., > 70%) ---" -ForegroundColor Yellow
}
Start-Sleep -Seconds 5 # Give some time for stats to update

# --- Final Cleanup ---
Write-Host "`n=======================================================" -ForegroundColor White
Write-Host "  Demo Completed. Cleaning up processes." -ForegroundColor White
Write-Host "=======================================================" -ForegroundColor White
Stop-KamaitachiProcesses
Write-Host "`nDemo script finished." -ForegroundColor Green
