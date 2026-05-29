@echo off
setlocal enabledelayedexpansion

:: ===========================================================================
:: code-detector -- Windows Build Script
:: Usage: build.bat [target]
:: ===========================================================================

set BINARY=code-detector.exe
set OUTPUT_DIR=build

echo.
echo ===== code-detector - Build Tool =====
echo.

if "%1"=="clean" goto :clean
if "%1"=="test"  goto :test
if "%1"=="vet"   goto :vet
if "%1"=="run"   goto :run
if "%1"=="dev"   goto :dev
if "%1"=="help"  goto :help
if "%1"==""      goto :all
goto :all

:: --- Full Build ---
:all
echo [BUILD] Running full build...
call :vet
echo.
echo [BUILD] Compiling binary...
if not exist %OUTPUT_DIR% mkdir %OUTPUT_DIR%
go build -ldflags="-s -w" -o %OUTPUT_DIR%\%BINARY% ./cmd/scanner
if errorlevel 1 (
    echo [ERROR] Build failed
    exit /b 1
)
echo [OK] Build success: %OUTPUT_DIR%\%BINARY%
for %%I in (%OUTPUT_DIR%\%BINARY%) do echo [INFO] Size: %%~zI bytes
goto :eof

:: --- Dev Build ---
:dev
echo [DEV] Quick build...
go build -ldflags="-s -w" -o %BINARY% ./cmd/scanner 2>&1
if errorlevel 1 (
    echo [ERROR] Build failed
    exit /b 1
)
echo [OK] %BINARY% built
goto :eof

:: --- Test ---
:test
echo [TEST] Running go vet...
go vet ./cmd/scanner ./internal/...
echo [OK] go vet passed
echo.
echo [TEST] Running go test...
go test -v -count=1 ./...
echo [TEST] Tests complete
goto :eof

:: --- Vet ---
:vet
echo [LINT] Running go vet...
go vet ./cmd/scanner ./internal/...
echo [OK] go vet passed
goto :eof

:: --- Clean ---
:clean
echo [CLEAN] Cleaning artifacts...
if exist %OUTPUT_DIR% rmdir /S /Q %OUTPUT_DIR%
if exist %BINARY% del %BINARY%
echo [OK] Clean complete
goto :eof

:: --- Run ---
:run
echo [RUN] Building and scanning...
if not exist %OUTPUT_DIR% mkdir %OUTPUT_DIR%
go build -ldflags="-s -w" -o %OUTPUT_DIR%\%BINARY% ./cmd/scanner
if errorlevel 1 (
    echo [ERROR] Build failed
    exit /b 1
)
echo [OK] Build success, starting scan...
%OUTPUT_DIR%\%BINARY% %*
goto :eof

:: --- Help ---
:help
echo Usage: build.bat [target]
echo.
echo Targets:
echo   (blank)    Full build (lint + test + build)
echo   build      Compile to build\ directory
echo   dev        Quick compile to current directory
echo   test       Run vet + test
echo   vet        Run go vet only
echo   clean      Clean artifacts
echo   run [args] Build and run (e.g. build.bat run -h)
echo   help       Show this help
echo.
echo Examples:
echo   build.bat
echo   build.bat run -lang go,py -db scaned_db/result.db ./testdata
echo   build.bat run -graph ./myproject
goto :eof
