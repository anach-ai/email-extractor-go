@echo off
echo.
echo ========================================
echo  TESSERACT OCR VERIFICATION
echo ========================================
echo.

echo Checking Tesseract installation...
echo.

where tesseract >nul 2>&1
if %errorlevel% == 0 (
    echo [OK] Tesseract is in PATH!
    echo.
    echo Version:
    tesseract --version
    echo.
    echo Location:
    where tesseract
    echo.
    echo ========================================
    echo [SUCCESS] Tesseract is ready to use!
    echo ========================================
    echo.
    echo You can now test email extraction with:
    echo   go run main.go ocr_extraction.go distributed_processing.go --test http://3elrappresentanze.it/
    echo.
) else (
    echo [WARNING] Tesseract not found in PATH.
    echo.
    if exist "C:\Program Files\Tesseract-OCR\tesseract.exe" (
        echo [OK] Tesseract found at: C:\Program Files\Tesseract-OCR\tesseract.exe
        echo [INFO] You may need to add it to PATH or restart your terminal.
        echo.
        echo To add to PATH:
        echo 1. Search for "Environment Variables" in Windows
        echo 2. Edit "Path" in System variables
        echo 3. Add: C:\Program Files\Tesseract-OCR
        echo 4. Restart terminal
        echo.
    ) else (
        echo [ERROR] Tesseract not found. Please reinstall.
        echo.
    )
)

echo.
pause

