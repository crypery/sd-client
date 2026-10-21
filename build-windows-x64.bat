@echo off
setlocal
cd /d "%~dp0"

set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0

echo Building mini-scan for windows/amd64 ...
go build -trimpath -ldflags "-s -w" -o sd-client.exe .
if errorlevel 1 (
    echo [ERROR] windows x64 build failed
    exit /b 1
)

echo [OK] built mini-scan-windows-amd64.exe ^(windows/amd64^)
endlocal
