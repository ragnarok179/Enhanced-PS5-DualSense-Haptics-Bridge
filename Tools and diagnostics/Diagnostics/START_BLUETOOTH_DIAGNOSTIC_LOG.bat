@echo off
chcp 65001 >nul
setlocal
set "TOOLS=%~dp0.."
set "BRIDGE=%TOOLS%\Bridge"
set "EXE=%BRIDGE%\EnhancedPS5DualSenseHapticsBluetooth.exe"
set "LOGDIR=%~dp0Logs"
if not exist "%LOGDIR%" mkdir "%LOGDIR%"
for /f %%I in ('powershell -NoProfile -Command "Get-Date -Format yyyyMMdd_HHmmss"') do set "STAMP=%%I"
set "LOG=%LOGDIR%\BLUETOOTH_%STAMP%.log"
echo Writing Bluetooth bridge output to:
echo %LOG%
echo.
pushd "%BRIDGE%"
powershell -NoProfile -ExecutionPolicy Bypass -Command "& '%EXE%' --protocol-36 --rgb-via-beamng 2^>^&1 ^| Tee-Object -FilePath '%LOG%'"
set "CODE=%ERRORLEVEL%"
popd
echo.
echo Log saved to: %LOG%
pause
exit /b %CODE%
