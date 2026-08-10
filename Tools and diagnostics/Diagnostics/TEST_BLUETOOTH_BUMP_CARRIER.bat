@echo off
chcp 65001 >nul
setlocal
set "EXE=%~dp0..\Bridge\EnhancedPS5DualSenseHapticsBluetooth.exe"
echo Bluetooth suspension-bump carrier test.
echo This is a transport test and does not require BeamNG telemetry.
"%EXE%" --test-bump-carrier --protocol-36
set "CODE=%ERRORLEVEL%"
pause
exit /b %CODE%
