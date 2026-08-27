@echo off
chcp 65001 >nul
setlocal
for %%I in ("%~dp0..") do set "ROOT=%%~fI"
set "BRIDGE=%ROOT%\Bridge"
set "USB=%BRIDGE%\EnhancedPS5DualSenseHapticsUSB.exe"
set "BT=%BRIDGE%\EnhancedPS5DualSenseHapticsBluetooth.exe"

if not exist "%USB%" (
  echo ERROR: USB Bridge executable not found:
  echo %USB%
  pause
  exit /b 2
)
if not exist "%BT%" (
  echo ERROR: Bluetooth Bridge executable not found:
  echo %BT%
  pause
  exit /b 2
)

pushd "%BRIDGE%"
echo ============================================================
echo DualSense USB probe
echo ============================================================
"%USB%" --probe
echo USB probe exit code: %ERRORLEVEL%
echo.
echo ============================================================
echo DualSense Bluetooth probe
echo ============================================================
"%BT%" --probe
echo Bluetooth probe exit code: %ERRORLEVEL%
echo.
echo ============================================================
echo Audio endpoints visible to the USB bridge
echo ============================================================
"%USB%" --list-audio
popd
echo.
echo ============================================================
echo Windows Plug and Play entries related to DualSense / Sony
echo ============================================================
powershell.exe -NoProfile -ExecutionPolicy Bypass -Command "Get-PnpDevice -PresentOnly -ErrorAction SilentlyContinue ^| Where-Object { $_.FriendlyName -match 'DualSense|Wireless Controller|Sony Interactive Entertainment' -or $_.InstanceId -match 'VID_054C' } ^| Sort-Object Class,FriendlyName ^| Format-Table -AutoSize Class,Status,FriendlyName,InstanceId"
echo.
pause
