@echo off
chcp 65001 >nul
setlocal
set "SCRIPT=%~dp0Run-Diagnostic.ps1"
if not exist "%SCRIPT%" (
  echo ERROR: diagnostic files are incomplete. Reinstall or update the Bridge.
  pause
  exit /b 2
)
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT%"
exit /b %ERRORLEVEL%
