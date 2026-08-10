@echo off
setlocal
set "LOGDIR=%~dp0Logs"
if not exist "%LOGDIR%" mkdir "%LOGDIR%"
start "" explorer "%LOGDIR%"
