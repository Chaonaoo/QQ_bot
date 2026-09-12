# ============================================================
#  QQ 模仿机器人 - 启动脚本
#  特性：
#    1. 先清理旧实例，避免 8080 端口占用导致「闪退」
#    2. 等待端口释放后再启动
#    3. 后台运行 + 输出重定向到日志文件（闪退也能查到原因）
#    4. 启动后做状态检查
# ============================================================
$ErrorActionPreference = "Stop"

# 项目根目录（scripts 的上一级）
$root   = Split-Path -Parent $PSScriptRoot
$goDir  = Join-Path $root "go-service"
$botDir = Join-Path $root "bot"
$goLogDir  = Join-Path $goDir "logs"
$botLogDir = Join-Path $botDir "logs"
New-Item -ItemType Directory -Force -Path $goLogDir  | Out-Null
New-Item -ItemType Directory -Force -Path $botLogDir | Out-Null

$goLogFile = Join-Path $goLogDir  "go-service.log"
$goErrFile = Join-Path $goLogDir  "go-service.err.log"
$botLogFile = Join-Path $botLogDir "nonebot.log"
$botErrFile = Join-Path $botLogDir "nonebot.err.log"

# ---- 1. 清理旧实例（关键：避免端口冲突闪退）----
Write-Host "[1/4] 清理旧实例..." -ForegroundColor Cyan
& (Join-Path $PSScriptRoot "stop-bot.ps1")

# ---- 2. 等待 8080 端口释放 ----
Write-Host "[2/4] 等待端口 8080 释放..." -ForegroundColor Cyan
$portFree = $false
for ($i = 0; $i -lt 12; $i++) {
    $conn = Get-NetTCPConnection -LocalPort 8080 -State Listen -ErrorAction SilentlyContinue
    if (-not $conn) { $portFree = $true; break }
    Start-Sleep -Milliseconds 500
}
if (-not $portFree) {
    Write-Host "  警告：端口 8080 仍被占用，Go 服务可能启动失败！" -ForegroundColor Red
    Get-NetTCPConnection -LocalPort 8080 -State Listen | ForEach-Object {
        Write-Host "    占用进程 PID: $($_.OwningProcess)" -ForegroundColor Red
    }
} else {
    Write-Host "  端口已空闲" -ForegroundColor Green
}

# ---- 3. 启动 Go 服务（后台 + 日志）----
Write-Host "[3/4] 启动 Go 服务..." -ForegroundColor Cyan
$goExe = Join-Path $goDir "mimic-server.exe"
if (-not (Test-Path $goExe)) {
    Write-Host "  错误：找不到 $goExe，请先编译 Go 服务！" -ForegroundColor Red
    exit 1
}
Start-Process -FilePath $goExe -WorkingDirectory $goDir -WindowStyle Hidden `
    -RedirectStandardOutput $goLogFile -RedirectStandardError $goErrFile
Start-Sleep -Seconds 2

# ---- 4. 启动 NoneBot（后台 + 日志）----
Write-Host "[4/4] 启动 NoneBot 机器人..." -ForegroundColor Cyan
$pythonExe = (Get-Command python -ErrorAction SilentlyContinue).Source
if (-not $pythonExe) {
    Write-Host "  错误：PATH 中找不到 python，请确认已安装并加入环境变量！" -ForegroundColor Red
} else {
    Start-Process -FilePath $pythonExe -ArgumentList "bot.py" -WorkingDirectory $botDir -WindowStyle Hidden `
        -RedirectStandardOutput $botLogFile -RedirectStandardError $botErrFile
    Start-Sleep -Seconds 4
}

# ---- 状态检查 ----
Write-Host ""
Write-Host "========== 启动状态 ==========" -ForegroundColor Green
$goProc = Get-Process mimic-server -ErrorAction SilentlyContinue
if ($goProc) {
    Write-Host "  Go 服务 : 运行中 (PID $($goProc.Id))" -ForegroundColor Green
} else {
    Write-Host "  Go 服务 : 未运行！请查看日志 -> $goErrFile" -ForegroundColor Red
}

$port = Get-NetTCPConnection -LocalPort 8080 -State Listen -ErrorAction SilentlyContinue
if ($port) {
    Write-Host "  端口8080: 已监听" -ForegroundColor Green
} else {
    Write-Host "  端口8080: 未监听！" -ForegroundColor Red
}

$botProc = Get-CimInstance Win32_Process -Filter "Name='python.exe'" -ErrorAction SilentlyContinue |
    Where-Object { $_.CommandLine -like '*bot.py*' }
if ($botProc) {
    Write-Host "  NoneBot : 运行中" -ForegroundColor Green
} else {
    Write-Host "  NoneBot : 未运行！请查看日志 -> $botErrFile" -ForegroundColor Red
}

Write-Host "==============================" -ForegroundColor Green
Write-Host "日志位置：" -ForegroundColor Yellow
Write-Host "  Go 服务 : $goLogFile"
Write-Host "  NoneBot : $botLogFile"
Write-Host ""
Write-Host "提示：若某项未运行，打开对应的 .err.log 文件即可看到闪退原因。" -ForegroundColor Yellow
