@echo off
chcp 65001 >nul
setlocal
set "ROOT=%~dp0"
start "" "steam://rungameid/284160"
timeout /t 1 /nobreak >nul
call "%ROOT%START_BRIDGE.bat"
exit /b %ERRORLEVEL%
