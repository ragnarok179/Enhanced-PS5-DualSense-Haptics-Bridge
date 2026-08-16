@echo off
setlocal EnableExtensions DisableDelayedExpansion
call "%~dp0_DIAGNOSTIC_ENV.bat"
if not exist "%DPH_LOGS%" mkdir "%DPH_LOGS%"
start "" explorer.exe "%DPH_LOGS%"
if errorlevel 1 (
  echo ERROR: Could not open:
  echo "%DPH_LOGS%"
  pause
)
