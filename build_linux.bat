@echo off
setlocal enabledelayedexpansion
echo ==========================================
echo       Noyo Linux Build Tool
echo ==========================================

cd /d "%~dp0"

rem Lightweight resources use a separate output and never erase existing models.
if not defined NOYO_PACKAGE_MODE set NOYO_PACKAGE_MODE=bundled
if /I not "%NOYO_PACKAGE_MODE%"=="light" if /I not "%NOYO_PACKAGE_MODE%"=="bundled" (
    echo Error: NOYO_PACKAGE_MODE must be light or bundled.
    exit /b 1
)
set RESOURCE_OUTPUT=backend\linux
if /I "%NOYO_PACKAGE_MODE%"=="light" set RESOURCE_OUTPUT=backend\linux-light
if /I "%NOYO_PACKAGE_MODE%"=="light" (
    for /f "delims=" %%F in ('dir /b /a "%RESOURCE_OUTPUT%\models" 2^>nul') do (
        echo Error: lightweight output models must be empty. Use a fresh output directory.
        exit /b 1
    )
    set ORT_AVAILABLE=
    for %%L in (libonnxruntime.so libonnxruntime-cpu.so libonnxruntime-cuda.so libonnxruntime-gpu.so) do (
        if exist "backend\lib\%%L" set ORT_AVAILABLE=1
    )
    if not defined ORT_AVAILABLE (
        echo Error: lightweight Pro packages require baseline ONNX Runtime libraries in backend\lib.
        exit /b 1
    )
)

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

echo [3/3] Syncing Resources to %RESOURCE_OUTPUT%/...
cd /d "%~dp0"
if not exist "%RESOURCE_OUTPUT%\lib" mkdir "%RESOURCE_OUTPUT%\lib"
if not exist "%RESOURCE_OUTPUT%\models" mkdir "%RESOURCE_OUTPUT%\models"
if not exist "%RESOURCE_OUTPUT%\data" mkdir "%RESOURCE_OUTPUT%\data"

attrib -r "%RESOURCE_OUTPUT%\*.*" /s 2>nul

if exist "backend\lib\*.so*" (
    attrib -r "%RESOURCE_OUTPUT%\lib\*.so*" 2>nul
    copy /y "backend\lib\*.so*" "%RESOURCE_OUTPUT%\lib\" >nul
    if errorlevel 1 exit /b !errorlevel!
)
if /I "%NOYO_PACKAGE_MODE%"=="bundled" (
    xcopy /s /e /y /q "backend\models\*" "%RESOURCE_OUTPUT%\models\" >nul
    if errorlevel 2 exit /b !errorlevel!
) else (
    copy /y "backend\linux\noyo-linux-amd64-pro" "%RESOURCE_OUTPUT%\" >nul
    if errorlevel 1 exit /b !errorlevel!
)

echo ==========================================
echo Build Linux Success!
echo Output Directory: %~dp0%RESOURCE_OUTPUT%\
echo ==========================================
endlocal
exit /b 0
