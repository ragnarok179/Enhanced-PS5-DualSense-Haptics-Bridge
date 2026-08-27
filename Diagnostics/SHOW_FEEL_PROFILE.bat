@echo off
chcp 65001 >nul
setlocal
for %%I in ("%~dp0..") do set "ROOT=%%~fI"
set "BRIDGE=%ROOT%\Bridge"
set "BT=%BRIDGE%\EnhancedPS5DualSenseHapticsBluetooth.exe"
set "USB=%BRIDGE%\EnhancedPS5DualSenseHapticsUSB.exe"
if not exist "%BT%" goto :missing
if not exist "%USB%" goto :missing
pushd "%BRIDGE%"
echo ============================================================
echo Bluetooth profile
echo ============================================================
"%BT%" --show-feel-profile
echo.
echo ============================================================
echo USB profile
echo ============================================================
"%USB%" --show-feel-profile
popd
pause
exit /b 0
:missing
echo ERROR: Bridge executable missing in:
echo %BRIDGE%
pause
exit /b 2
