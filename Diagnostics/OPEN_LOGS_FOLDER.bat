@echo off
setlocal
set "RELEASE=V1.42"
title Enhanced PS5 DualSense Haptics %RELEASE% - Diagnostic Logs
for %%I in ("%~dp0Logs") do set "LOGDIR=%%~fI"
if not exist "%LOGDIR%" mkdir "%LOGDIR%"
start "" explorer.exe "%LOGDIR%"
