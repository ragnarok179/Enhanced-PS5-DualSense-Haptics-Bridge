@echo off
setlocal
cd /d "%~dp0\..\Bridge"
echo ============================================================
echo DualSense extended-input diagnostic - Bluetooth
echo ============================================================
echo.
echo Close the normal Bridge first and connect the controller by Bluetooth.
echo The first report should be Sony enhanced input 0x31 / 78 bytes.
echo Test: 1-finger tap, 2-finger tap, then left/right/up/down movement
echo with one finger and with two fingers.
echo.
echo The output separates raw absolute positions from derived motion.
echo.
EnhancedPS5DualSenseHapticsBluetooth.exe --test-extended-inputs
set "RC=%ERRORLEVEL%"
echo.
echo Exit code: %RC%
pause
exit /b %RC%
