@echo off
REM Push Email Extractor to GitHub
REM Author: Dr.Anach | Telegram: @dranach

echo.
echo ============================================================
echo   Push Email Extractor to GitHub
echo ============================================================
echo.

REM Check if git is available
where git >nul 2>&1
if %errorlevel% neq 0 (
    echo ERROR: Git is not installed or not in PATH
    echo Please install Git from https://git-scm.com/downloads
    pause
    exit /b 1
)

REM Check if repository exists
if not exist ".git" (
    echo ERROR: This is not a Git repository
    echo Please run this script from the project root directory
    pause
    exit /b 1
)

echo Step 1: Checking Git status...
git status
echo.

echo Step 2: Enter your GitHub repository URL
echo Example: https://github.com/YOUR_USERNAME/email-extractor-go.git
echo.
set /p REPO_URL="GitHub Repository URL: "

if "%REPO_URL%"=="" (
    echo ERROR: Repository URL cannot be empty
    pause
    exit /b 1
)

echo.
echo Step 3: Adding remote origin...
git remote remove origin 2>nul
git remote add origin %REPO_URL%
if %errorlevel% neq 0 (
    echo ERROR: Failed to add remote
    pause
    exit /b 1
)

echo.
echo Step 4: Renaming branch to 'main' (if needed)...
git branch -M main 2>nul

echo.
echo Step 5: Pushing to GitHub...
echo This will push your code to: %REPO_URL%
echo.
set /p CONFIRM="Do you want to continue? (Y/N): "

if /i not "%CONFIRM%"=="Y" (
    echo Cancelled.
    pause
    exit /b 0
)

echo.
git push -u origin main

if %errorlevel% equ 0 (
    echo.
    echo ============================================================
    echo   SUCCESS! Project pushed to GitHub!
    echo ============================================================
    echo.
    echo Repository URL: %REPO_URL%
    echo.
) else (
    echo.
    echo ============================================================
    echo   ERROR: Failed to push to GitHub
    echo ============================================================
    echo.
    echo Possible issues:
    echo - Repository does not exist on GitHub (create it first)
    echo - Authentication failed (check your GitHub credentials)
    echo - Network connection issues
    echo.
    echo See GITHUB_SETUP.md for detailed instructions.
    echo.
)

pause

