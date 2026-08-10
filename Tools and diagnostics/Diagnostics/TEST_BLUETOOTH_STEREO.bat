@echo off
chcp 65001 >nul
setlocal
set "EXE=%~dp0..\Bridge\EnhancedPS5DualSenseHapticsBluetooth.exe"
echo Bluetooth stereo hardware test: left, right, center.
echo Close BeamNG, Steam Input, DSX, DS4Windows and other DualSense bridges before testing.
"%EXE%" --test --protocol-36
set "CODE=%ERRORLEVEL%"
pause
exit /b %CODE%
