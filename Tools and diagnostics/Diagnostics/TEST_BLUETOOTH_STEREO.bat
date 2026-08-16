@echo off
setlocal EnableExtensions DisableDelayedExpansion
chcp 65001 >nul
call "%~dp0_DIAGNOSTIC_ENV.bat"
set "EXE=%DPH_BRIDGE%\EnhancedPS5DualSenseHapticsBluetooth.exe"
echo ============================================================
echo Bluetooth stereo hardware test - LEFT / RIGHT / CENTER
echo ============================================================
echo Close BeamNG, DSX, DS4Windows, Steam Input controller remapping and other DualSense bridges first.
echo.
if not exist "%EXE%" goto :missing
pushd "%DPH_BRIDGE%" || goto :patherror
"%EXE%" --test --protocol-36
set "CODE=%ERRORLEVEL%"
popd
echo.
echo Exit code: %CODE%
pause
exit /b %CODE%
:missing
echo ERROR: Bluetooth Bridge executable not found:
echo "%EXE%"
pause
exit /b 2
:patherror
echo ERROR: Could not open Bridge folder:
echo "%DPH_BRIDGE%"
pause
exit /b 3
