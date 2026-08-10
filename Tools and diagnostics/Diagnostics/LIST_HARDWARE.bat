@echo off
chcp 65001 >nul
setlocal
set "TOOLS=%~dp0.."
set "BRIDGE=%TOOLS%\Bridge"
set "USB=%BRIDGE%\EnhancedPS5DualSenseHapticsUSB.exe"
set "BT=%BRIDGE%\EnhancedPS5DualSenseHapticsBluetooth.exe"

echo ============================================================
echo DualSense USB probe
echo ============================================================
"%USB%" --probe
echo.
echo ============================================================
echo DualSense Bluetooth probe
echo ============================================================
"%BT%" --probe
echo.
echo ============================================================
echo Audio endpoints visible to the USB bridge
echo ============================================================
"%USB%" --list-audio
echo.
echo ============================================================
echo Windows Plug and Play entries related to DualSense / Sony
echo ============================================================
powershell -NoProfile -ExecutionPolicy Bypass -Command "Get-PnpDevice -PresentOnly -ErrorAction SilentlyContinue ^| Where-Object { $_.FriendlyName -match 'DualSense|Wireless Controller|Sony Interactive Entertainment' -or $_.InstanceId -match 'VID_054C' } ^| Sort-Object Class,FriendlyName ^| Format-Table -AutoSize Class,Status,FriendlyName,InstanceId"
echo.
pause
