@echo off
setlocal
powershell -ExecutionPolicy Bypass -File "%~dp0install-deps.ps1"
endlocal
