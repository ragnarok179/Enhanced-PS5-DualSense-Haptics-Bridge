@echo off
chcp 65001 >nul
setlocal
set "EXE=%~dp0..\Bridge\EnhancedPS5DualSenseHapticsUSB.exe"
echo USB stereo hardware test: left, right, center.
echo Close other software that can take control of the DualSense before testing.
"%EXE%" --test-stereo
set "CODE=%ERRORLEVEL%"
pause
exit /b %CODE%
