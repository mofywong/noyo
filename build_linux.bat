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

rem 2. Build Pro Edition (using WSL with CGO and $ORIGIN RPATH)
echo --- Compiling Pro Edition (noyo-linux-amd64-pro) ---
set NOYO_EDITION=pro
call node scripts\sync-pro.mjs
if errorlevel 1 exit /b !errorlevel!
rem WSL inherits the current project directory; keep shell quoting in a shell script.
wsl -d Ubuntu -- bash scripts/build-linux-pro.sh
if %errorlevel% neq 0 (
    echo Error: Pro edition build in WSL failed. Resources were not packaged.
    exit /b %errorlevel%
)

echo [3/3] Syncing Resources to backend/linux/...
cd /d "%~dp0"
if not exist "backend\linux\lib" mkdir "backend\linux\lib"
if not exist "backend\linux\models" mkdir "backend\linux\models"
if not exist "backend\linux\data" mkdir "backend\linux\data"

attrib -r "backend\linux\*.*" /s 2>nul

if errorlevel 1 exit /b !errorlevel!

if exist "backend\lib\*.so" (
    attrib -r "backend\linux\lib\*.so" 2>nul
    copy /y "backend\lib\*.so" "backend\linux\lib\" >nul
    if errorlevel 1 exit /b !errorlevel!
)
xcopy /s /e /y /q "backend\models\*" "backend\linux\models\" >nul
if errorlevel 2 exit /b !errorlevel!

echo ==========================================
echo Build Linux Success!
echo Output Directory: %~dp0backend\linux\
echo ==========================================
endlocal
exit /b 0
