@echo off
chcp 65001 >nul
setlocal
set "EXE=%~dp0..\Bridge\EnhancedPS5DualSenseHapticsUSB.exe"
"%EXE%" --list-audio
pause
