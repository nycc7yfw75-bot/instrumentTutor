@echo off
rem Starts HEADLINER with every new gig rolling a tough crowd (for testing).
cd /d "%~dp0"
set HEADLINER_FORCE_TOUGH=1
start "" musictutor.exe
