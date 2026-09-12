# ============================================================
#  QQ 模仿机器人 - 停止脚本
#  作用：干净地停止 Go 服务与 NoneBot 机器人
# ============================================================
$ErrorActionPreference = "SilentlyContinue"

Write-Host "正在停止 Go 服务 (mimic-server.exe)..." -ForegroundColor Yellow
$goProcs = Get-Process mimic-server
if ($goProcs) {
    $goProcs | Stop-Process -Force
    Write-Host "  已停止 Go 服务" -ForegroundColor Green
} else {
    Write-Host "  Go 服务未在运行" -ForegroundColor DarkGray
}

Write-Host "正在停止 NoneBot (bot.py)..." -ForegroundColor Yellow
$botProcs = Get-CimInstance Win32_Process -Filter "Name='python.exe'" |
    Where-Object { $_.CommandLine -like '*bot.py*' }
if ($botProcs) {
    $botProcs | ForEach-Object { Stop-Process -Id $_.ProcessId -Force }
    Write-Host "  已停止 NoneBot" -ForegroundColor Green
} else {
    Write-Host "  NoneBot 未在运行" -ForegroundColor DarkGray
}

Start-Sleep -Seconds 1
Write-Host "全部已停止。" -ForegroundColor Green
