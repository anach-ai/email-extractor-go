@echo off
echo ========================================
echo  CLEAN PROJECT - FRESH START
echo ========================================
echo.

REM Remove compiled binary
if exist "email-extractor.exe" (
    del /F /Q "email-extractor.exe"
    echo [OK] Removed compiled binary
)

REM Clean output files
if exist "output\*.txt" (
    del /F /Q "output\*.txt"
    echo [OK] Cleaned output files
)

REM Remove .git folder if exists (Git history)
if exist ".git" (
    echo.
    echo Warning: .git folder contains Git version control history.
    echo Do you want to remove it? (y/n)
    set /p remove_git=
    if /i "%remove_git%"=="y" (
        rmdir /S /Q ".git"
        echo [OK] Removed .git folder (Git history deleted)
    ) else (
        echo [SKIP] Keeping .git folder
    )
)

echo.
echo ========================================
echo  CLEANUP COMPLETE
echo ========================================
echo.
echo Project is now clean and ready for fresh start!
echo.
pause

