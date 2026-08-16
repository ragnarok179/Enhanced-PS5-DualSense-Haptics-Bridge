@echo off
setlocal EnableExtensions DisableDelayedExpansion
chcp 65001 >nul
call "%~dp0_DIAGNOSTIC_ENV.bat"
set "USB=%DPH_BRIDGE%\EnhancedPS5DualSenseHapticsUSB.exe"
set "BT=%DPH_BRIDGE%\EnhancedPS5DualSenseHapticsBluetooth.exe"

echo ============================================================
echo Enhanced PS5 DualSense Haptics - hardware diagnostics
echo ============================================================
echo Package root:  "%DPH_ROOT%"
echo Bridge folder: "%DPH_BRIDGE%"
echo.

if "%DPH_NESTED_INSTALL%"=="1" (
  echo WARNING: Nested Bridge installation detected.
  echo          Use a clean extraction before testing START_BRIDGE.exe.
  echo.
)

if not exist "%USB%" echo ERROR: Missing "%USB%"
if not exist "%BT%" echo ERROR: Missing "%BT%"
if not exist "%USB%" goto :end
if not exist "%BT%" goto :end

pushd "%DPH_BRIDGE%" || goto :patherror
echo [1/3] USB probe
"%USB%" --probe
set "USB_CODE=%ERRORLEVEL%"
echo USB exit code: %USB_CODE%
echo.

echo [2/3] Bluetooth probe
"%BT%" --probe
set "BT_CODE=%ERRORLEVEL%"
echo Bluetooth exit code: %BT_CODE%
echo.

echo [3/3] USB audio endpoints
"%USB%" --list-audio
set "AUDIO_CODE=%ERRORLEVEL%"
echo Audio-list exit code: %AUDIO_CODE%
popd

echo.
echo Windows PnP entries related to Sony / DualSense:
powershell.exe -NoProfile -ExecutionPolicy Bypass -Command "Get-PnpDevice -PresentOnly -ErrorAction SilentlyContinue | Where-Object { $_.FriendlyName -match 'DualSense|Wireless Controller|Sony Interactive Entertainment' -or $_.InstanceId -match 'VID_054C' } | Sort-Object Class,FriendlyName | Format-Table -AutoSize Class,Status,FriendlyName,InstanceId"
goto :end

:patherror
echo ERROR: Could not open Bridge folder.

:end
echo.
pause
