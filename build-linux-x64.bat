@echo off
setlocal
cd /d "%~dp0"

set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0

echo Building mini-scan for linux/amd64 ...
go build -trimpath -ldflags "-s -w" -o sd-client .
if errorlevel 1 (
    echo [ERROR] linux x64 build failed
    exit /b 1
)

echo [OK] built mini-scan-linux-amd64 ^(linux/amd64^)
endlocal
