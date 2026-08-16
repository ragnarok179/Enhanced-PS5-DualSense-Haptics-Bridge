@echo off
for %%I in ("%~dp0..") do set "DPH_TOOLS=%%~fI"
for %%I in ("%DPH_TOOLS%\..") do set "DPH_ROOT=%%~fI"
for %%I in ("%DPH_ROOT%\..") do set "DPH_ROOT_PARENT=%%~fI"
for %%I in ("%DPH_ROOT%") do set "DPH_ROOT_NAME=%%~nxI"
for %%I in ("%DPH_ROOT_PARENT%") do set "DPH_ROOT_PARENT_NAME=%%~nxI"
set "DPH_BRIDGE=%DPH_TOOLS%\Bridge"
set "DPH_CONFIG=%DPH_TOOLS%\Config\feel_profile_v1.json"
set "DPH_LOGS=%~dp0Logs"
set "DPH_NESTED_INSTALL=0"
if /I "%DPH_ROOT_NAME%"=="Diagnostics" if /I "%DPH_ROOT_PARENT_NAME%"=="Tools and diagnostics" set "DPH_NESTED_INSTALL=1"
exit /b 0
