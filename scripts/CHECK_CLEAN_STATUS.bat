@echo off
echo ========================================
echo   PROJECT CLEAN STATUS CHECK
echo ========================================
echo.

set CLEAN=true

REM Check for compiled binaries
echo [1/6] Checking for compiled binaries...
dir *.exe >nul 2>&1
if %errorlevel% == 0 (
    echo    [X] Found .exe files
    set CLEAN=false
) else (
    echo    [OK] No compiled binaries
)

REM Check output files
echo [2/6] Checking output directory...
dir output\*.txt >nul 2>&1
if %errorlevel% == 0 (
    echo    [X] Found output files
    set CLEAN=false
) else (
    echo    [OK] Output directory is clean
)

REM Check .git folder
echo [3/6] Checking .git folder...
if exist .git (
    echo    [!] .git folder exists (optional to remove)
) else (
    echo    [OK] No .git folder
)

REM Check required files
echo [4/6] Checking required files...
set MISSING=
if not exist main.go set MISSING=%MISSING% main.go
if not exist config.json set MISSING=%MISSING% config.json
if not exist go.mod set MISSING=%MISSING% go.mod
if not exist README.md set MISSING=%MISSING% README.md

if defined MISSING (
    echo    [X] Missing files:%MISSING%
    set CLEAN=false
) else (
    echo    [OK] All required files present
)

REM Check documentation
echo [5/6] Checking documentation...
if not exist QUICK_START_WINDOWS.md (
    echo    [X] Missing QUICK_START_WINDOWS.md
    set CLEAN=false
) else (
    echo    [OK] Quick start guide present
)

if not exist docs\WINDOWS_SETUP_GUIDE.md (
    echo    [X] Missing docs\WINDOWS_SETUP_GUIDE.md
    set CLEAN=false
) else (
    echo    [OK] Windows setup guide present
)

REM Check domain file
echo [6/6] Checking domain files...
if exist domains\domains.txt (
    echo    [OK] domains.txt found
) else (
    echo    [!] domains.txt not found (will be created on first run)
)

echo.
echo ========================================
if "%CLEAN%"=="true" (
    echo   STATUS: CLEAN FOR FRESH START
    echo ========================================
    echo.
    echo Ready to run:
    echo   go run main.go ocr_extraction.go distributed_processing.go machine.go --yes
) else (
    echo   STATUS: NEEDS CLEANUP
    echo ========================================
    echo.
    echo Run cleanup: scripts\CLEAN_PROJECT.bat
)
echo.
pause

