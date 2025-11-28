@echo off
chcp 65001 >nul
echo.
echo ========================================================================
echo   EMAIL EXTRACTION TEST - 100 DOMAINS
echo ========================================================================
echo.
echo Starting extraction with 100 domains...
echo.
echo Features enabled:
echo   ✅ Email extraction from HTML
echo   ✅ OCR extraction from images
echo   ✅ Phone number filtering
echo   ✅ Smart validation
echo   ✅ Contact page discovery
echo.
echo This will process all 100 domains. Please wait...
echo.
echo ========================================================================
echo.

cd /d "%~dp0"

REM Build first for faster execution
echo [1/2] Building executable...
go build -o email-extractor.exe main.go ocr_extraction.go distributed_processing.go
if %errorlevel% neq 0 (
    echo.
    echo [ERROR] Build failed!
    echo.
    pause
    exit /b 1
)

echo [2/2] Starting extraction...
echo.

REM Run the extraction
email-extractor.exe --yes

echo.
echo ========================================================================
echo   EXTRACTION COMPLETE
echo ========================================================================
echo.
echo Check results in the output directory:
echo   - output/emails.txt
echo   - output/resolved_domains.txt
echo   - output/unresolved_domains.txt
echo.
pause

