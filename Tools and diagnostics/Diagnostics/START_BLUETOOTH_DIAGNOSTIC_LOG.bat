@echo off
setlocal EnableExtensions DisableDelayedExpansion
chcp 65001 >nul
call "%~dp0_DIAGNOSTIC_ENV.bat"
set "EXE=%DPH_BRIDGE%\EnhancedPS5DualSenseHapticsBluetooth.exe"

if not exist "%EXE%" goto :missing
if not exist "%DPH_LOGS%" mkdir "%DPH_LOGS%"
for /f %%I in ('powershell.exe -NoProfile -Command "Get-Date -Format yyyyMMdd_HHmmss"') do set "STAMP=%%I"
set "LOG=%DPH_LOGS%\BLUETOOTH_%STAMP%.log"
set "DPH_EXE=%EXE%"
set "DPH_LOG=%LOG%"

echo Bluetooth diagnostic log:
echo "%LOG%"
echo Press Ctrl+C to stop the Bridge.
echo.

pushd "%DPH_BRIDGE%" || goto :patherror
powershell.exe -NoProfile -ExecutionPolicy Bypass -Command "& $env:DPH_EXE --protocol-36 --rgb-via-beamng --diagnostic-status 2>&1 | Tee-Object -FilePath $env:DPH_LOG; exit $LASTEXITCODE"
set "CODE=%ERRORLEVEL%"
popd

echo.
echo Exit code: %CODE%
echo Log saved to: "%LOG%"
pause
exit /b %CODE%

:missing
echo ERROR: Bluetooth Bridge executable not found:
echo "%EXE%"
pause
exit /b 2

:patherror
echo ERROR: Could not open Bridge folder:
echo "%DPH_BRIDGE%"
pause
exit /b 3
