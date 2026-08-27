@echo off
setlocal
cd /d "%~dp0\..\Bridge"
echo ============================================================
echo DualSense extended-input diagnostic - USB
echo ============================================================
echo.
echo Close the normal Bridge first and connect the controller by USB.
echo Test: 1-finger tap, 2-finger tap, then left/right/up/down movement
echo with one finger and with two fingers.
echo.
echo The output separates raw absolute positions from derived motion.
echo.
EnhancedPS5DualSenseHapticsUSB.exe --test-extended-inputs
set "RC=%ERRORLEVEL%"
echo.
echo Exit code: %RC%
pause
exit /b %RC%
