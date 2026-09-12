@echo off
chcp 65001 >nul
title QQ-Bot Stop
cd /d "%~dp0"
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\stop-bot.ps1"
echo.
pause
