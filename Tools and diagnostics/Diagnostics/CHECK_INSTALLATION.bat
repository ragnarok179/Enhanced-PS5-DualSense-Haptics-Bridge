@echo off
setlocal EnableExtensions DisableDelayedExpansion
chcp 65001 >nul
call "%~dp0_DIAGNOSTIC_ENV.bat"
set "USB=%DPH_BRIDGE%\EnhancedPS5DualSenseHapticsUSB.exe"
set "BT=%DPH_BRIDGE%\EnhancedPS5DualSenseHapticsBluetooth.exe"

echo ============================================================
echo Enhanced PS5 DualSense Haptics - installation check
echo ============================================================
echo Package root: "%DPH_ROOT%"
echo Diagnostics:  "%~dp0"
echo Bridge:       "%DPH_BRIDGE%"
echo.

if "%DPH_NESTED_INSTALL%"=="1" (
  echo WARNING: A complete Bridge package appears to be installed inside
  echo          "Tools and diagnostics\Diagnostics".
  echo.
  echo          Extract Enhanced_PS5_DualSense_Haptics_Bridge.zip into
  echo          a NEW EMPTY FOLDER so START_BRIDGE.exe is directly in
  echo          that folder.
  echo.
)

call :check "%USB%" "USB Bridge"
call :check "%BT%" "Bluetooth Bridge"
call :check "%DPH_CONFIG%" "Feel profile"

echo.
echo The profile commands below do not require a controller.
if exist "%USB%" "%USB%" --show-feel-profile
if exist "%BT%" "%BT%" --show-feel-profile
echo.
pause
exit /b 0

:check
if exist "%~1" (
  echo OK: %~2
) else (
  echo MISSING: %~2
  echo          "%~1"
)
exit /b 0
