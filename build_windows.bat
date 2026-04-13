@echo off
setlocal
echo ==========================================
echo       Noyo Build Script
echo ==========================================

set TARGET=%1
if "%TARGET%"=="" set TARGET=windows

set EDITION=%2
if "%EDITION%"=="" (
    if exist "..\noyo-pro" (
        set EDITION=all
    ) else (
        set EDITION=community
    )
)

echo Target OS: %TARGET%
echo Target Edition: %EDITION%

rem Setup Version
if "%VERSION%"=="" (
    for /f "tokens=*" %%i in ('git describe --tags --always 2^>nul') do set VERSION=%%i
)
if "%VERSION%"=="" set VERSION=v1.0.0
echo Building version: %VERSION%

if /I "%EDITION%"=="all" (
    call :build_edition community ""
    if %errorlevel% neq 0 exit /b %errorlevel%
    call :build_edition pro "-pro"
    if %errorlevel% neq 0 exit /b %errorlevel%
) else if /I "%EDITION%"=="pro" (
    call :build_edition pro "-pro"
    if %errorlevel% neq 0 exit /b %errorlevel%
) else (
    call :build_edition community ""
    if %errorlevel% neq 0 exit /b %errorlevel%
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

echo [1/3] Building Frontend...
cd frontend
call npm install
if %errorlevel% neq 0 exit /b %errorlevel%
call npm run build
if %errorlevel% neq 0 exit /b %errorlevel%
cd ..

echo [2/3] Skipping copy (Vite builds to backend/dist directly)...

echo [3/3] Building Backend...
cd backend
set CGO_ENABLED=0
set GOARCH=amd64
set LDFLAGS=-X "noyo/core/system.Version=%VERSION%"

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
