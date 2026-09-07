@echo off
chcp 65001 >nul
setlocal
set "RELEASE=V1.42"
title Enhanced PS5 DualSense Haptics %RELEASE% - Motion Inputs USB
set "SCRIPT=%~dp0Run-MotionInputDiagnostic.ps1"
if not exist "%SCRIPT%" goto :missing
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT%" -Mode USB
set "CODE=%ERRORLEVEL%"
echo.
pause
exit /b %CODE%
:missing
echo ERROR: diagnostic PowerShell script not found:
echo %SCRIPT%
pause
exit /b 2
