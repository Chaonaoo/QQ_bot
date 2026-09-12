@echo off
chcp 65001 >nul
title QQ-Bot Watchdog (keep this window open)
cd /d "%~dp0"
echo Starting watchdog... keep this window OPEN.
echo The watchdog auto-restarts the bot if it hangs or crashes.
echo.
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\watchdog.ps1"
pause
