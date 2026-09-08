@echo off
setlocal
for %%I in ("%~dp0Logs") do set "LOGDIR=%%~fI"
if not exist "%LOGDIR%" mkdir "%LOGDIR%"
start "" explorer.exe "%LOGDIR%"
