@echo off
setlocal
cd /d "%~dp0\..\Bridge"
echo ============================================================
echo DualSense Touchpad Windows Mouse - Bluetooth diagnostic
echo ============================================================
echo.
echo Close the normal Bridge first and connect the controller by Bluetooth.
echo BeamNG is not required. This test injects real Windows mouse events.
echo The detected Sony input report should be 0x31 / 78 bytes.
echo.
echo Test:
echo   1 finger       = move cursor
echo   1-finger tap   = left click
echo   2 fingers      = vertical / horizontal scroll
echo   2-finger tap   = right click
echo.
echo Mouse movement, taps and scrolling should react immediately.
echo Only SendInput failures are printed by the Bridge.
echo.
echo If SendInput fails while BeamNG is running as administrator, run this
echo diagnostic at the same integrity level. Press Ctrl+C to stop.
echo.
EnhancedPS5DualSenseHapticsBluetooth.exe --test-touchpad-mouse
set "RC=%ERRORLEVEL%"
echo.
echo Exit code: %RC%
pause
exit /b %RC%
