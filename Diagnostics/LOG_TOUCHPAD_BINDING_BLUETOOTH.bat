@echo off
chcp 65001 >nul
setlocal
set "SCRIPT=%~dp0Run-TouchpadBindingDiagnostic.ps1"
if not exist "%SCRIPT%" goto :missing
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT%" -Mode Bluetooth
set "CODE=%ERRORLEVEL%"
echo.
pause
exit /b %CODE%
:missing
echo ERROR: diagnostic PowerShell script not found:
echo %SCRIPT%
pause
exit /b 2
