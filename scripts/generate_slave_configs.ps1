# Generate Slave Node Configuration Files
# Usage: .\generate_slave_configs.ps1 -Count 12

param(
    [int]$Count = 12
)

Write-Host "==========================================="
Write-Host "Generate Slave Node Configuration Files"
Write-Host "==========================================="
Write-Host "Configuration count: $Count (slave3 ~ slave$Count)" -ForegroundColor Cyan

# Ensure directory exists
$confDir = "conf"
if (-not (Test-Path $confDir)) {
    New-Item -ItemType Directory -Path $confDir | Out-Null
}

# Read template from slave1
$template = Get-Content "conf\slave.ini" -Raw

# Generate slave3 to slaveN configurations
for ($i = 3; $i -le $Count; $i++) {
    $port = 8080 + $i
    $configFile = "conf\slave$i.ini"
    
    # Replace port and service address
    $content = $template -replace "port = 8081", "port = $port"
    $content = $content -replace "service_addr = localhost:8081", "service_addr = localhost:$port"
    $content = $content -replace "slave_snapshot\.dat", "slave${i}_snapshot.dat"
    
    # Write to file
    $content | Set-Content $configFile -Encoding UTF8
    
    Write-Host "Created config file: $configFile (port: $port)" -ForegroundColor Green
}

Write-Host "`nConfiguration files generated successfully!" -ForegroundColor Green
Write-Host "Location: conf\slave3.ini ~ conf\slave$Count.ini"
