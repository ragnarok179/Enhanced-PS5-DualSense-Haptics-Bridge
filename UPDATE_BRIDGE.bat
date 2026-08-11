@echo off
chcp 65001 >nul
setlocal

rem Resolve the repository folder without a trailing backslash.
rem Passing a quoted Windows path ending in "\\" to powershell.exe can leave
rem a stray quote in the argument on some systems, which makes GetFullPath fail.
for %%I in ("%~dp0.") do set "ROOT=%%~fI"
set "UPDATER=%ROOT%\Tools and diagnostics\Updater\Update-Bridge.ps1"

if not exist "%UPDATER%" (
  echo ERROR: missing "%UPDATER%".
  pause
  exit /b 2
)

echo Enhanced PS5 DualSense Haptics - Manual Updater
echo.
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%UPDATER%" -InstallRoot "%ROOT%"
set "EXITCODE=%ERRORLEVEL%"
echo.
if "%EXITCODE%"=="0" (
  echo Updater finished.
) else (
  echo Updater stopped with code %EXITCODE%.
)
pause
exit /b %EXITCODE%
