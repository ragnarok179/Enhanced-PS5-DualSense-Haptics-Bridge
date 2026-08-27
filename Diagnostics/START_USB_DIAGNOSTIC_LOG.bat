@echo off
chcp 65001 >nul
setlocal
for %%I in ("%~dp0..") do set "ROOT=%%~fI"
set "BRIDGE=%ROOT%\Bridge"
set "EXE=%BRIDGE%\EnhancedPS5DualSenseHapticsUSB.exe"
set "LOGDIR=%~dp0Logs"
if not exist "%EXE%" goto :missing
if not exist "%LOGDIR%" mkdir "%LOGDIR%"
for /f %%I in ('powershell.exe -NoProfile -Command "Get-Date -Format yyyyMMdd_HHmmss"') do set "STAMP=%%I"
set "LOG=%LOGDIR%\USB_%STAMP%.log"
echo Writing USB bridge output to:
echo %LOG%
echo.
echo Diagnostic mode includes native BeamNG collision sample selection/decoding, controller-speaker state and transport errors.
echo.
pushd "%BRIDGE%"
"%EXE%" --diagnostic-status > "%LOG%" 2>&1
set "CODE=%ERRORLEVEL%"
popd
echo.
echo Log saved to: %LOG%
pause
exit /b %CODE%
:missing
echo ERROR: USB Bridge executable not found:
echo %EXE%
pause
exit /b 2
