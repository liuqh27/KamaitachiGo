# 启动 Kamaitachi Finance API 服务器

param(
    [int]$Port = 8080,
    [string]$DbPath = "./data/finance_test.db",
    [switch]$Debug
)

Write-Host "===========================================`n" -ForegroundColor Cyan
Write-Host "  Kamaitachi Finance API Server`n" -ForegroundColor Cyan
Write-Host "===========================================`n" -ForegroundColor Cyan

# 检查数据库文件是否存在
if (!(Test-Path $DbPath)) {
    Write-Host "错误: 数据库文件不存在: $DbPath" -ForegroundColor Red
    Write-Host "请先运行导入工具导入数据" -ForegroundColor Yellow
    exit 1
}

# 构建启动参数
$args = @("-db", $DbPath, "-port", $Port)
if ($Debug) {
    $args += "-debug"
}

Write-Host "数据库: $DbPath" -ForegroundColor Green
Write-Host "端口: $Port" -ForegroundColor Green
Write-Host "调试模式: $(if($Debug){'开启'}else{'关闭'})" -ForegroundColor Green
Write-Host "`n正在启动服务器...`n" -ForegroundColor Yellow

# 启动服务器
& "cmd/server/server.exe" $args

