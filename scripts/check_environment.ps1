Write-Host "========== Environment Check ==========" -ForegroundColor Cyan

# 1. Check Go
Write-Host "`n[1] Go Version:" -ForegroundColor Yellow
try {
    go version
    Write-Host "Go: OK" -ForegroundColor Green
} catch {
    Write-Host "Go: NOT INSTALLED" -ForegroundColor Red
}

# 2. Check etcd
Write-Host "`n[2] etcd Status:" -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "http://localhost:2379/version" -TimeoutSec 2
    Write-Host "etcd is running!" -ForegroundColor Green
    Write-Host "Version: $($response.etcdserver)" -ForegroundColor Cyan
} catch {
    Write-Host "etcd is NOT running!" -ForegroundColor Red
    Write-Host "Please start etcd first." -ForegroundColor Red
    Write-Host "Download: https://github.com/etcd-io/etcd/releases" -ForegroundColor Yellow
}

# 3. Check Go Dependencies
Write-Host "`n[3] Go Dependencies:" -ForegroundColor Yellow
try {
    go mod verify
    Write-Host "Dependencies: OK" -ForegroundColor Green
} catch {
    Write-Host "Dependencies: FAILED" -ForegroundColor Red
    Write-Host "Run: go mod download" -ForegroundColor Yellow
}

# 4. Check Compiled Binaries
Write-Host "`n[4] Compiled Binaries:" -ForegroundColor Yellow
$binaries = @("master.exe", "slave.exe", "benchmark.exe", "benchmark_scenarios.exe")
$allFound = $true
foreach ($bin in $binaries) {
    if (Test-Path "bin\$bin") {
        $size = [math]::Round((Get-Item "bin\$bin").Length / 1MB, 2)
        Write-Host "$bin : OK ($size MB)" -ForegroundColor Green
    } else {
        Write-Host "$bin : NOT FOUND" -ForegroundColor Red
        $allFound = $false
    }
}
if (-not $allFound) {
    Write-Host "`nRun build commands:" -ForegroundColor Yellow
    Write-Host "  go build -o bin\master.exe .\cmd\master\main.go" -ForegroundColor Cyan
    Write-Host "  go build -o bin\slave.exe .\cmd\slave\main.go" -ForegroundColor Cyan
    Write-Host "  go build -o bin\benchmark.exe .\tools\benchmark.go" -ForegroundColor Cyan
}

# 5. Check Database Files
Write-Host "`n[5] Database Files:" -ForegroundColor Yellow
if (Test-Path "data") {
    $dbs = Get-ChildItem data\*.db -ErrorAction SilentlyContinue
    if ($dbs) {
        foreach ($db in $dbs) {
            $size = [math]::Round($db.Length/1MB, 2)
            Write-Host "$($db.Name): $size MB" -ForegroundColor Green
        }
    } else {
        Write-Host "No database files found (will be created on first run)" -ForegroundColor Yellow
    }
} else {
    Write-Host "data/ directory not found" -ForegroundColor Yellow
}

# 6. Check Configuration Files
Write-Host "`n[6] Configuration Files:" -ForegroundColor Yellow
$configs = @("conf\master.ini", "conf\slave1.ini", "conf\slave2.ini")
foreach ($config in $configs) {
    if (Test-Path $config) {
        Write-Host "$config : OK" -ForegroundColor Green
    } else {
        Write-Host "$config : NOT FOUND" -ForegroundColor Red
    }
}

# 7. Check PowerShell Version
Write-Host "`n[7] PowerShell Version:" -ForegroundColor Yellow
Write-Host "PowerShell $($PSVersionTable.PSVersion)" -ForegroundColor Cyan
if ($PSVersionTable.PSVersion.Major -ge 7) {
    Write-Host "PowerShell: OK (v7+)" -ForegroundColor Green
} else {
    Write-Host "PowerShell: OK (v5+, but v7+ recommended)" -ForegroundColor Yellow
    Write-Host "Download PowerShell 7: https://github.com/PowerShell/PowerShell/releases" -ForegroundColor Cyan
}

Write-Host "`n========== Check Complete ==========" -ForegroundColor Cyan

# Summary
Write-Host "`n========== Summary ==========" -ForegroundColor Cyan
Write-Host "Ready to run:" -ForegroundColor Yellow
Write-Host "  1. .\start_3nodes.ps1        # Start 3-node cluster" -ForegroundColor Cyan
Write-Host "  2. .\bin\benchmark.exe -requests 5000 -concurrent 50 -nodes 3" -ForegroundColor Cyan
Write-Host "  3. See '测试流程手册.md' for detailed testing guide" -ForegroundColor Cyan

