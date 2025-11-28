@echo off
echo.
echo ========================================
echo  TESSERACT OCR INSTALLATION
echo ========================================
echo.

REM Check if already installed
echo Checking for existing Tesseract installation...
where tesseract >nul 2>&1
if %errorlevel% == 0 (
    echo [OK] Tesseract is already installed!
    tesseract --version
    echo.
    pause
    exit /b 0
)

if exist "C:\Program Files\Tesseract-OCR\tesseract.exe" (
    echo [OK] Tesseract found at: C:\Program Files\Tesseract-OCR\tesseract.exe
    echo.
    pause
    exit /b 0
)

echo [INFO] Tesseract not found. Installing...
echo.

REM Try Chocolatey first
where choco >nul 2>&1
if %errorlevel% == 0 (
    echo [OK] Chocolatey found. Installing Tesseract via Chocolatey...
    echo.
    choco install tesseract --yes
    if %errorlevel% == 0 (
        echo.
        echo [SUCCESS] Tesseract installed via Chocolatey!
        echo.
        echo Refreshing PATH...
        call refreshenv
        echo.
        echo Verifying installation...
        where tesseract
        tesseract --version
        echo.
        pause
        exit /b 0
    )
) else (
    echo [INFO] Chocolatey not found. Skipping Chocolatey installation.
    echo.
)

REM Manual installation instructions
echo ========================================
echo  MANUAL INSTALLATION REQUIRED
echo ========================================
echo.
echo Please install Tesseract OCR manually:
echo.
echo 1. Download from: https://github.com/UB-Mannheim/tesseract/wiki
echo    Or direct: https://digi.bib.uni-mannheim.de/tesseract/
echo.
echo 2. Download file: tesseract-ocr-w64-setup-5.4.0.20240619.exe
echo.
echo 3. Run the installer:
echo    - Install to: C:\Program Files\Tesseract-OCR\
echo    - Check "Add to PATH" if available
echo.
echo 4. After installation, restart this terminal and verify:
echo    tesseract --version
echo.
echo ========================================
echo.

REM Try to open download page
echo Opening download page in browser...
start https://github.com/UB-Mannheim/tesseract/wiki

pause

