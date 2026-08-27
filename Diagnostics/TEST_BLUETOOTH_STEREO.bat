@echo off
chcp 65001 >nul
setlocal
for %%I in ("%~dp0..") do set "ROOT=%%~fI"
set "BRIDGE=%ROOT%\Bridge"
set "EXE=%BRIDGE%\EnhancedPS5DualSenseHapticsBluetooth.exe"
if not exist "%EXE%" goto :missing
pushd "%BRIDGE%"
echo Bluetooth stereo hardware test: left, right, center.
echo Close BeamNG, Steam Input, DSX, DS4Windows and other DualSense bridges before testing.
"%EXE%" --test-stereo
set "CODE=%ERRORLEVEL%"
popd
pause
exit /b %CODE%
:missing
echo ERROR: Bluetooth Bridge executable not found:
echo %EXE%
pause
exit /b 2
