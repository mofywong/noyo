@echo off
setlocal enabledelayedexpansion
echo ==========================================
echo       Noyo Linux Build Tool
echo ==========================================

cd /d "%~dp0"

echo [1/3] Building Frontend Web UI...
cd frontend
call npm run build
if %errorlevel% neq 0 (
    echo Error: Frontend build failed.
    exit /b %errorlevel%
)
if not exist "..\backend\dist\index.html" (
    echo Error: Frontend build did not produce backend\dist\index.html.
    exit /b 1
)
cd ..

echo [2/3] Building Linux Backend Binaries...
rem 1. Build Community Edition (using Go cross compilation)
echo --- Compiling Community Edition (noyo-linux-amd64) ---
set NOYO_EDITION=community
call node scripts\sync-pro.mjs
cd backend
set CGO_ENABLED=0
set GOOS=linux
set GOARCH=amd64
go build -ldflags "-w -s" -o "linux\noyo-linux-amd64" .
if %errorlevel% neq 0 (
    echo Error: Community backend build failed.
    exit /b %errorlevel%
)
cd ..

rem 2. Build Pro Edition (using WSL with CGO and $ORIGIN RPATH)
echo --- Compiling Pro Edition (noyo-linux-amd64-pro) ---
set NOYO_EDITION=pro
call node scripts\sync-pro.mjs
wsl -d Ubuntu -- bash -c "cd /mnt/d/code/github/noyo/noyo/backend && export PATH=/usr/local/go/bin:/usr/bin:/bin && export CGO_ENABLED=1 && go build -ldflags '-w -s' -o 'linux/noyo-linux-amd64-pro' . && (which patchelf >/dev/null 2>&1 || (sudo apt-get update -qq && sudo apt-get install -y patchelf)) && patchelf --set-rpath '\$ORIGIN/lib:\$ORIGIN' linux/noyo-linux-amd64-pro"
if %errorlevel% neq 0 (
    echo Warning: Pro edition build in WSL failed.
)

echo [3/3] Syncing Resources to backend/linux/...
cd /d "%~dp0"
if not exist "backend\linux\lib" mkdir "backend\linux\lib"
if not exist "backend\linux\models" mkdir "backend\linux\models"
if not exist "backend\linux\data" mkdir "backend\linux\data"

attrib -r "backend\linux\*.*" /s 2>nul

if exist "backend\linux\noyo-linux-amd64-pro" (
    copy /y "backend\linux\noyo-linux-amd64-pro" "backend\linux\noyo" >nul
) else if exist "backend\linux\noyo-linux-amd64" (
    copy /y "backend\linux\noyo-linux-amd64" "backend\linux\noyo" >nul
)

if exist "backend\lib\*.so" (
    attrib -r "backend\linux\lib\*.so" 2>nul
    copy /y "backend\lib\*.so" "backend\linux\lib\" >nul
)
xcopy /s /e /y /q "backend\models\*" "backend\linux\models\" >nul

echo ==========================================
echo Build Linux Success!
echo Output Directory: %~dp0backend\linux\
echo ==========================================
endlocal