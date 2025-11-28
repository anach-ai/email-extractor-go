@echo off
echo ================================================
echo  EMAIL EXTRACTOR - RUN WITH ALL FEATURES
echo ================================================
echo.
echo Features Enabled:
echo   - Smart Email Validation
echo   - Obfuscation Detection (20+ techniques)
echo   - Contact Page Discovery (99%% coverage)
echo   - Memory Optimization (Streaming)
echo   - Rate Limiting
echo   - Progress Tracking
echo.
echo ================================================
echo.

REM Check if domains file exists
if not exist "..\domains\domains.txt" (
    echo WARNING: domains\domains.txt not found!
    echo Using domains file from config.json...
    echo.
)

REM Run the application
echo Starting extraction...
echo.
cd ..
go run main.go ocr_extraction.go distributed_processing.go --yes

if %ERRORLEVEL% NEQ 0 (
    echo.
    echo ERROR: Application failed!
    pause
    exit /b 1
)

echo.
echo ================================================
echo  EXTRACTION COMPLETE
echo ================================================
echo.
echo Results saved to: output\extracted_emails.txt
echo.
pause

