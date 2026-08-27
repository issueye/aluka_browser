@echo off
rem ============================================================
rem  gio-browser build script for Windows CMD.
rem  Target platform is Windows only (WebView2 based app).
rem
rem  NOTE: keep this file pure ASCII on purpose -- non-ASCII
rem  batch files can mis-parse depending on the console
rem  codepage. Chinese-friendly output lives in build.sh / README.
rem
rem  Usage:
rem    build.bat                 Release build (default): hides console window
rem    build.bat --dev           Dev build: keeps console window for logs
rem    build.bat --skip-test     Skip unit tests
rem    build.bat --test-only     Run tests only, no binary output
rem    build.bat --clean         Remove build artifacts then exit
rem    build.bat --help          Show help
rem
rem  Environment variables:
rem    OUT_DIR                   Output directory, default "build"
rem ============================================================
setlocal EnableExtensions

cd /d "%~dp0"

set "BIN_NAME=gio-browser.exe"
if not defined OUT_DIR set "OUT_DIR=build"
set "BUILD_MODE=release"
set "RUN_TEST=1"
set "TEST_ONLY=0"
set "DO_CLEAN=0"

:parse_args
if "%~1"=="" goto :args_done
if /i "%~1"=="--dev"        ( set "BUILD_MODE=dev" & shift & goto :parse_args )
if /i "%~1"=="--skip-test"  ( set "RUN_TEST=0" & shift & goto :parse_args )
if /i "%~1"=="--test-only"  ( set "TEST_ONLY=1" & shift & goto :parse_args )
if /i "%~1"=="--clean"      ( set "DO_CLEAN=1" & shift & goto :parse_args )
if /i "%~1"=="-h"           goto :usage
if /i "%~1"=="--help"       goto :usage
echo [ERROR] Unknown option: %~1
call :usage
exit /b 1
:args_done

if "%DO_CLEAN%"=="1" goto :do_clean

rem ---- Environment checks ----
where go >nul 2>nul || goto :err_no_go
for /f "delims=" %%v in ('go version') do echo [build] %%v

for /f "tokens=3" %%p in ('findstr /c:"github.com/aluka-lang/aluka =>" go.mod') do set "ALUKA_DIR=%%p"
if not defined ALUKA_DIR goto :err_aluka_cfg
if not exist "%ALUKA_DIR%\" goto :err_aluka_missing
echo [OK] aluka engine sources ready: %ALUKA_DIR%

rem ---- Unit tests ----
if "%TEST_ONLY%"=="1" goto :run_tests
if not "%RUN_TEST%"=="1" (
    echo [WARN] Unit tests skipped by request.
    goto :do_build
)
:run_tests
echo [build] Running unit tests: go test ./...
go test ./... || goto :err_test
echo [OK] All tests passed.
if "%TEST_ONLY%"=="1" (
    echo [OK] --test-only finished.
    exit /b 0
)

rem ---- Build ----
:do_build
if not exist "%OUT_DIR%" mkdir "%OUT_DIR%"
set "LDFLAGS=-s -w"
if "%BUILD_MODE%"=="release" set "LDFLAGS=%LDFLAGS% -H windowsgui"
echo [build] Building mode=%BUILD_MODE% output=%OUT_DIR%\%BIN_NAME%
go build -trimpath -ldflags "%LDFLAGS%" -o "%OUT_DIR%\%BIN_NAME%" . || goto :err_build

for %%F in ("%OUT_DIR%\%BIN_NAME%") do set "BIN_SIZE=%%~zF"
echo [OK] Build finished: %OUT_DIR%\%BIN_NAME%  %BIN_SIZE% bytes
if "%BUILD_MODE%"=="release" echo [build] Hint: release build hides the console window; use --dev to see logs.
echo [build] WebView2 Runtime is required at runtime; user data lives under %APPDATA%\gio-browser\
exit /b 0

rem ---- Subcommands and error branches ----

:usage
echo Usage: build.bat [options]
echo   --dev          Dev build: keeps the console window for log output
echo   --skip-test    Skip unit tests
echo   --test-only    Run tests only, do not produce a binary
echo   --clean        Remove the build output directory then exit
echo   --help         Show this help
exit /b 0

:do_clean
if exist "%OUT_DIR%" (
    rmdir /s /q "%OUT_DIR%"
    echo [OK] Cleaned build directory: %OUT_DIR%
) else (
    echo [build] Nothing to clean, directory missing: %OUT_DIR%
)
exit /b 0

:err_no_go
echo [ERROR] "go" command not found. Install Go 1.25+ from https://go.dev/dl/
exit /b 1

:err_aluka_cfg
echo [ERROR] Missing replace entry for github.com/aluka-lang/aluka in go.mod.
exit /b 1

:err_aluka_missing
echo [ERROR] aluka engine local sources not found: %ALUKA_DIR%
echo [ERROR] Fix it either way below:
echo        1. Clone the aluka language repo into that path; or
echo        2. Edit the replace block in go.mod to point at your local checkout.
exit /b 1

:err_test
echo [ERROR] Tests failed. Fix them or pass --skip-test to force the build.
exit /b 1

:err_build
echo [ERROR] go build failed. Check compiler errors above.
exit /b 1
