@echo off
echo.
echo ========================================
echo  Add Tesseract to PATH
echo ========================================
echo.

set TESSERACT_PATH=C:\Program Files\Tesseract-OCR
set TESSERACT_EXE=%TESSERACT_PATH%\tesseract.exe

if not exist "%TESSERACT_EXE%" (
    echo [ERROR] Tesseract not found at: %TESSERACT_EXE%
    echo Please check your installation.
    echo.
    pause
    exit /b 1
)

echo [OK] Tesseract found at: %TESSERACT_EXE%
echo.
echo Adding to PATH (requires administrator privileges)...
echo.

REM Check if already in PATH
echo %PATH% | findstr /C:"%TESSERACT_PATH%" >nul
if %errorlevel% == 0 (
    echo [OK] Tesseract is already in PATH!
    echo.
) else (
    echo [INFO] Adding Tesseract to system PATH...
    echo.
    echo NOTE: This will require administrator privileges.
    echo.
    
    REM Try using setx (doesn't require admin for user PATH)
    setx PATH "%PATH%;%TESSERACT_PATH%" >nul 2>&1
    
    if %errorlevel% == 0 (
        echo [SUCCESS] Tesseract added to PATH!
        echo.
    ) else (
        echo [WARNING] Could not add to PATH automatically.
        echo.
        echo Please add manually:
        echo   1. Press Win+R, type: sysdm.cpl
        echo   2. Click "Environment Variables"
        echo   3. Under "System variables", select "Path"
        echo   4. Click "Edit" and add: %TESSERACT_PATH%
        echo   5. Click OK and restart terminal
        echo.
    )
)

echo ========================================
echo.
echo IMPORTANT: Restart your terminal for PATH changes to take effect.
echo.
echo However, the email extractor can still find Tesseract at:
echo   %TESSERACT_EXE%
echo.
echo ========================================
echo.
pause

