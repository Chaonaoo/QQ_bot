# ============================================================
#  QQ 模仿机器人 - 看门狗（守护进程）
#  作用：每 30 秒检查一次服务健康状态，发现异常自动重启
#    - Go 服务：不仅检查进程，还发 HTTP 健康检查
#             （进程活着但卡死不响应时，也会被判定为异常并重启）
#    - NoneBot：检查 bot.py 进程是否存活
#  用法：双击 watchdog.bat，或在 PowerShell 中运行本脚本
#  停止：关闭看门狗窗口即可（不影响已启动的服务）
# ============================================================
$ErrorActionPreference = "SilentlyContinue"

$root   = Split-Path -Parent $PSScriptRoot
$goDir  = Join-Path $root "go-service"
$botDir = Join-Path $root "bot"
$goLogDir  = Join-Path $goDir "logs"
$botLogDir = Join-Path $botDir "logs"
New-Item -ItemType Directory -Force -Path $goLogDir  | Out-Null
New-Item -ItemType Directory -Force -Path $botLogDir | Out-Null

$watchLog  = Join-Path $goLogDir "watchdog.log"
$healthUrl = "http://127.0.0.1:8080/api/stats"
$interval  = 30   # 检查间隔（秒）

function Write-Log($msg) {
    $line = "{0}  {1}" -f (Get-Date -Format "yyyy-MM-dd HH:mm:ss"), $msg
    Write-Host $line
    Add-Content -Path $watchLog -Value $line -Encoding UTF8
}

function Start-GoService {
    $goExe = Join-Path $goDir "mimic-server.exe"
    Start-Process -FilePath $goExe -WorkingDirectory $goDir -WindowStyle Hidden `
        -RedirectStandardOutput (Join-Path $goLogDir "go-service.log") `
        -RedirectStandardError  (Join-Path $goLogDir "go-service.err.log")
}

function Restart-GoService($reason) {
    Write-Log "[重启Go] 原因：$reason"
    Get-Process mimic-server | Stop-Process -Force
    Start-Sleep -Seconds 2
    Start-GoService
    Start-Sleep -Seconds 2
}

function Start-Bot {
    $pythonExe = (Get-Command python).Source
    Start-Process -FilePath $pythonExe -ArgumentList "bot.py" -WorkingDirectory $botDir -WindowStyle Hidden `
        -RedirectStandardOutput (Join-Path $botLogDir "nonebot.log") `
        -RedirectStandardError  (Join-Path $botLogDir "nonebot.err.log")
}

function Test-GoHealth {
    # 进程存在 + HTTP 能响应，才算健康
    $proc = Get-Process mimic-server
    if (-not $proc) { return "no_process" }
    try {
        $resp = Invoke-WebRequest -Uri $healthUrl -TimeoutSec 8 -UseBasicParsing
        if ($resp.StatusCode -eq 200) { return "ok" }
        return "bad_status"
    } catch {
        return "no_response"   # 进程活着但卡死/不响应
    }
}

function Test-BotAlive {
    $p = Get-CimInstance Win32_Process -Filter "Name='python.exe'" |
        Where-Object { $_.CommandLine -like '*bot.py*' }
    return [bool]$p
}

Write-Log "========== 看门狗启动 =========="
Write-Log "监控间隔：$interval 秒；健康检查：$healthUrl"

# 首次确保服务在运行
if ((Test-GoHealth) -ne "ok") { Restart-GoService "初始检查未通过" }
if (-not (Test-BotAlive))      { Write-Log "[启动Bot] 初始检查未运行"; Start-Bot; Start-Sleep -Seconds 4 }

while ($true) {
    Start-Sleep -Seconds $interval

    # --- 检查 Go 服务 ---
    $goState = Test-GoHealth
    switch ($goState) {
        "ok"          { }
        "no_process"  { Restart-GoService "进程不存在" }
        "no_response" { Restart-GoService "进程卡死无响应(HTTP超时)" }
        "bad_status"  { Restart-GoService "HTTP状态异常" }
    }

    # --- 检查 NoneBot ---
    if (-not (Test-BotAlive)) {
        Write-Log "[重启Bot] 进程不存在"
        Start-Bot
        Start-Sleep -Seconds 4
    }

    # 每分钟输出一次心跳（健康时）
    if ((Get-Date).Second -lt $interval) {
        $go2 = Test-GoHealth
        $bot2 = Test-BotAlive
        Write-Log ("心跳：Go={0}, Bot={1}" -f $go2, $(if ($bot2) {"alive"} else {"down"}))
    }
}
