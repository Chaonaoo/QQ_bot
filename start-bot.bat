@echo off
chcp 65001 >nul
title QQ-Bot Start
cd /d "%~dp0"
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\start-bot.ps1"
echo.
pause
