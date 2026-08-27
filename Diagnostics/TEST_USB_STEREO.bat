@echo off
chcp 65001 >nul
setlocal
for %%I in ("%~dp0..") do set "ROOT=%%~fI"
set "BRIDGE=%ROOT%\Bridge"
set "EXE=%BRIDGE%\EnhancedPS5DualSenseHapticsUSB.exe"
if not exist "%EXE%" goto :missing
pushd "%BRIDGE%"
echo USB stereo hardware test: left, right, center.
echo Close other software that can take control of the DualSense before testing.
"%EXE%" --test-stereo
set "CODE=%ERRORLEVEL%"
popd
pause
exit /b %CODE%
:missing
echo ERROR: USB Bridge executable not found:
echo %EXE%
pause
exit /b 2
