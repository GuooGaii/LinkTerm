@echo off
cd /d "%~dp0"
echo Building Terminal Client for Windows...
go build -o terminal.exe
if %errorlevel% equ 0 (
    echo Build successful! Run terminal.exe to start.
) else (
    echo Build failed!
    exit /b 1
)
