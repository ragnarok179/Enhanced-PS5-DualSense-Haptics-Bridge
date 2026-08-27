@echo off
chcp 65001 >nul
setlocal
for %%I in ("%~dp0..") do set "ROOT=%%~fI"
set "BRIDGE=%ROOT%\Bridge"
set "EXE=%BRIDGE%\EnhancedPS5DualSenseHapticsUSB.exe"
if not exist "%EXE%" (
  echo ERROR: USB Bridge executable not found:
  echo %EXE%
  pause
  exit /b 2
)
pushd "%BRIDGE%"
"%EXE%" --list-audio
set "CODE=%ERRORLEVEL%"
popd
pause
exit /b %CODE%
