@echo off
chcp 65001 >nul
setlocal
set "BRIDGE=%~dp0..\Bridge"
echo ============================================================
echo Bluetooth profile
echo ============================================================
"%BRIDGE%\EnhancedPS5DualSenseHapticsBluetooth.exe" --show-feel-profile
echo.
echo ============================================================
echo USB profile
echo ============================================================
"%BRIDGE%\EnhancedPS5DualSenseHapticsUSB.exe" --show-feel-profile
pause
