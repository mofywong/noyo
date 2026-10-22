@echo off
setlocal
echo ==========================================
echo       Noyo Build Script
echo ==========================================

set TARGET=%1
if "%TARGET%"=="" set TARGET=windows

set EDITION=%2
if "%EDITION%"=="" (
    if defined NOYO_PRO_DIR (
        set EDITION=all
    ) else if exist "..\noyo-pro" (
        set EDITION=all
    ) else (
        set EDITION=community
    )
)

set BUILD_YOLO=%3
if "%BUILD_YOLO%"=="" set BUILD_YOLO=1

set BUILD_ONNXRUNTIME=%4
if "%BUILD_ONNXRUNTIME%"=="" set BUILD_ONNXRUNTIME=1

if not "%BUILD_YOLO%"=="0" if not "%BUILD_YOLO%"=="1" (
    echo Error: build_yolo must be 0 or 1
    exit /b 1
)

if not "%BUILD_ONNXRUNTIME%"=="0" if not "%BUILD_ONNXRUNTIME%"=="1" (
    echo Error: build_onnxruntime must be 0 or 1
    exit /b 1
)

if "%BUILD_YOLO%"=="0" set BUILD_ONNXRUNTIME=0

if "%BUILD_YOLO%"=="1" if "%BUILD_ONNXRUNTIME%"=="0" (
    echo Error: build_yolo=1 requires build_onnxruntime=1
    exit /b 1
)

echo Target OS: %TARGET%
echo Target Edition: %EDITION%
echo Build YOLO: %BUILD_YOLO%
echo Build ONNX Runtime: %BUILD_ONNXRUNTIME%

rem Setup Version
if "%VERSION%"=="" (
    for /f "tokens=*" %%i in ('git describe --tags --always 2^>nul') do set VERSION=%%i
)
if "%VERSION%"=="" set VERSION=v1.0.0
echo Building version: %VERSION%

if /I "%EDITION%"=="all" (
    call :build_edition community ""
    if errorlevel 1 exit /b 1
    call :build_edition pro "-pro"
    if errorlevel 1 exit /b 1
) else if /I "%EDITION%"=="pro" (
    call :build_edition pro "-pro"
    if errorlevel 1 exit /b 1
) else (
    call :build_edition community ""
    if errorlevel 1 exit /b 1
)

echo ==========================================
echo Build Success!
if /I "%EDITION%"=="all" (
    echo Binaries with and without -pro suffix are in backend/
) else if /I "%EDITION%"=="pro" (
    echo Binaries with -pro suffix are in backend/
) else (
    echo Binaries are in backend/
)
echo ==========================================
pause
goto :eof

:build_edition
setlocal
set EDITION_NAME=%~1
set BIN_SUFFIX=%~2

echo ==========================================
echo       Building %EDITION_NAME% Edition
echo ==========================================

set NOYO_EDITION=%EDITION_NAME%
set NOYO_BUILD_YOLO=%BUILD_YOLO%

echo [1/4] Installing screen engine dependencies...
if not exist "integrations\goview\app\node_modules" (
    pushd integrations\goview\app
    call npm ci --no-audit --no-fund
    if %errorlevel% neq 0 (
        popd
        echo Error: screen engine dependency install failed.
        exit /b %errorlevel%
    )
    popd
)

echo [2/4] Building Frontend...
cd frontend
call npm install
if %errorlevel% neq 0 exit /b %errorlevel%
call npm run build
if %errorlevel% neq 0 exit /b %errorlevel%
if not exist "..\backend\dist\index.html" (
    echo Error: Frontend build did not produce backend\dist\index.html.
    exit /b 1
)
cd ..

node scripts\build-goview.mjs --verify-only
if %errorlevel% neq 0 exit /b %errorlevel%

echo [3/4] Skipping copy (Vite builds to backend/dist directly)...

echo [4/4] Building Backend...
cd backend
set CGO_ENABLED=0
rem The Pro voice assistant links sherpa-onnx through cgo, so the Pro build always needs a C
rem toolchain - independently of the YOLO / ONNX Runtime toggles.
if /I "%EDITION_NAME%"=="pro" set CGO_ENABLED=1
set GOARCH=amd64
set LDFLAGS=-w -s -X "noyo/core/system.Version=%VERSION%"

if /I "%TARGET%"=="windows" goto build_windows
if /I "%TARGET%"=="linux" goto build_linux
if /I "%TARGET%"=="mac" goto build_mac
if /I "%TARGET%"=="all" goto build_all
goto build_windows

:build_windows
echo Building for Windows...
set GOOS=windows
go build -ldflags "%LDFLAGS%" -o noyo-windows-amd64%BIN_SUFFIX%.exe .
if %errorlevel% neq 0 exit /b %errorlevel%
goto end_backend

:build_linux
echo Building for Linux...
set GOOS=linux
go build -ldflags "%LDFLAGS%" -o noyo-linux-amd64%BIN_SUFFIX% .
if %errorlevel% neq 0 exit /b %errorlevel%
goto end_backend

:build_mac
echo Building for Mac (Darwin)...
set GOOS=darwin
go build -ldflags "%LDFLAGS%" -o noyo-darwin-amd64%BIN_SUFFIX% .
if %errorlevel% neq 0 exit /b %errorlevel%
goto end_backend

:build_all
echo Building for Windows...
set GOOS=windows
go build -ldflags "%LDFLAGS%" -o noyo-windows-amd64%BIN_SUFFIX%.exe .
if %errorlevel% neq 0 exit /b %errorlevel%

echo Building for Linux...
set GOOS=linux
go build -ldflags "%LDFLAGS%" -o noyo-linux-amd64%BIN_SUFFIX% .
if %errorlevel% neq 0 exit /b %errorlevel%

echo Building for Mac...
set GOOS=darwin
go build -ldflags "%LDFLAGS%" -o noyo-darwin-amd64%BIN_SUFFIX% .
if %errorlevel% neq 0 exit /b %errorlevel%
goto end_backend

:end_backend
cd ..
endlocal
exit /b 0
