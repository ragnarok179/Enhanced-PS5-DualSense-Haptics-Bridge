@echo off
chcp 65001 >nul
setlocal
set "RELEASE=V1.42"
title Enhanced PS5 DualSense Haptics %RELEASE% - Audio Outputs
for %%I in ("%~dp0..") do set "ROOT=%%~fI"
set "BRIDGE=%ROOT%\Bridge"
set "EXE=%BRIDGE%\EnhancedPS5DualSenseHapticsUSB.exe"
if not exist "%EXE%" (
  echo ERROR: USB Bridge executable not found:
  echo %EXE%
  pause
  exit /b 2
)
echo Enhanced PS5 DualSense Haptics %RELEASE% - Audio output diagnostic
echo.
pushd "%BRIDGE%"
"%EXE%" --list-audio
set "CODE=%ERRORLEVEL%"
popd
pause
exit /b %CODE%
