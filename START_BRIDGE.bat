@echo off
chcp 65001 >nul
setlocal
set "ROOT=%~dp0"
set "TOOLS=%ROOT%Tools and diagnostics"
set "BRIDGE=%TOOLS%\Bridge"
set "USB=%BRIDGE%\EnhancedPS5DualSenseHapticsUSB.exe"
set "BT=%BRIDGE%\EnhancedPS5DualSenseHapticsBluetooth.exe"

if not exist "%USB%" (
  echo ERROR: missing "%USB%".
  pause
  exit /b 2
)
if not exist "%BT%" (
  echo ERROR: missing "%BT%".
  pause
  exit /b 2
)

"%USB%" --probe >nul 2>&1
if not errorlevel 1 goto usb

"%BT%" --probe >nul 2>&1
if not errorlevel 1 goto bluetooth

echo ERROR: no compatible DualSense controller was detected over USB or Bluetooth.
echo Open "Tools and diagnostics\Diagnostics\LIST_HARDWARE.bat" for more information.
pause
exit /b 1

:usb
echo Enhanced PS5 DualSense Haptics - USB
pushd "%BRIDGE%"
"%USB%"
set "EXITCODE=%ERRORLEVEL%"
popd
goto end

:bluetooth
echo Enhanced PS5 DualSense Haptics - Bluetooth
pushd "%BRIDGE%"
"%BT%" --protocol-36 --rgb-via-beamng
set "EXITCODE=%ERRORLEVEL%"
popd

:end
if not "%EXITCODE%"=="0" (
  echo.
  echo The bridge exited with code %EXITCODE%.
  pause
)
exit /b %EXITCODE%
