@echo off
chcp 65001 >nul
echo.
echo ========================================================================
echo   EMAIL EXTRACTION - LARGE DOMAIN LIST
echo ========================================================================
echo.
echo This will process the full Hetzner_domains.txt file.
echo.
echo Features enabled:
echo   ✅ Auto-optimization (based on your system)
echo   ✅ OCR email extraction from images
echo   ✅ Email categorization by department
echo   ✅ Phone number filtering
echo   ✅ Smart validation
echo   ✅ Memory optimized (streaming mode)
echo.
echo The system will automatically optimize settings for your hardware.
echo.
echo ========================================================================
echo.

cd /d "%~dp0"

REM Check if domain file exists
if not exist "domains\Hetzner_domains.txt" (
    echo [ERROR] Domain file not found: domains\Hetzner_domains.txt
    echo.
    pause
    exit /b 1
)

REM Count domains
for /f %%i in ('find /c /v "" ^< domains\Hetzner_domains.txt') do set DOMAIN_COUNT=%%i
echo [INFO] Found %DOMAIN_COUNT% domains to process
echo.

REM Build executable for faster execution
echo [1/2] Building executable...
go build -o email-extractor.exe main.go ocr_extraction.go distributed_processing.go machine.go
if %errorlevel% neq 0 (
    echo.
    echo [ERROR] Build failed!
    echo.
    pause
    exit /b 1
)

echo [2/2] Starting extraction...
echo.
echo This may take a while depending on the number of domains.
echo Progress will be shown in real-time.
echo.
echo Press Ctrl+C to stop gracefully (partial results will be saved).
echo.

REM Run the extraction with auto-optimization
email-extractor.exe --yes

echo.
echo ========================================================================
echo   EXTRACTION COMPLETE
echo ========================================================================
echo.
echo Check results in the output directory:
echo   - output/extracted_emails.txt (all emails)
echo   - output/categorized_*.txt (by department)
echo   - output/email_categories_summary.txt
echo   - output/resolved_domains.txt
echo   - output/unresolved_domains.txt
echo.
pause

