# Push Email Extractor to GitHub
# Author: Dr.Anach | Telegram: @dranach

Write-Host ""
Write-Host "============================================================" -ForegroundColor Cyan
Write-Host "  Push Email Extractor to GitHub" -ForegroundColor Green
Write-Host "============================================================" -ForegroundColor Cyan
Write-Host ""

# Check if git is available
try {
    $null = Get-Command git -ErrorAction Stop
} catch {
    Write-Host "ERROR: Git is not installed or not in PATH" -ForegroundColor Red
    Write-Host "Please install Git from https://git-scm.com/downloads" -ForegroundColor Yellow
    Read-Host "Press Enter to exit"
    exit 1
}

# Check if repository exists
if (-not (Test-Path ".git")) {
    Write-Host "ERROR: This is not a Git repository" -ForegroundColor Red
    Write-Host "Please run this script from the project root directory" -ForegroundColor Yellow
    Read-Host "Press Enter to exit"
    exit 1
}

Write-Host "Step 1: Checking Git status..." -ForegroundColor Yellow
git status
Write-Host ""

Write-Host "Step 2: Enter your GitHub repository URL" -ForegroundColor Yellow
Write-Host "Example: https://github.com/YOUR_USERNAME/email-extractor-go.git" -ForegroundColor Gray
Write-Host ""
$repoUrl = Read-Host "GitHub Repository URL"

if ([string]::IsNullOrWhiteSpace($repoUrl)) {
    Write-Host "ERROR: Repository URL cannot be empty" -ForegroundColor Red
    Read-Host "Press Enter to exit"
    exit 1
}

Write-Host ""
Write-Host "Step 3: Adding remote origin..." -ForegroundColor Yellow
git remote remove origin 2>$null
git remote add origin $repoUrl
if ($LASTEXITCODE -ne 0) {
    Write-Host "ERROR: Failed to add remote" -ForegroundColor Red
    Read-Host "Press Enter to exit"
    exit 1
}

Write-Host ""
Write-Host "Step 4: Renaming branch to 'main' (if needed)..." -ForegroundColor Yellow
git branch -M main 2>$null

Write-Host ""
Write-Host "Step 5: Pushing to GitHub..." -ForegroundColor Yellow
Write-Host "This will push your code to: $repoUrl" -ForegroundColor Gray
Write-Host ""
$confirm = Read-Host "Do you want to continue? (Y/N)"

if ($confirm -ne "Y" -and $confirm -ne "y") {
    Write-Host "Cancelled." -ForegroundColor Yellow
    Read-Host "Press Enter to exit"
    exit 0
}

Write-Host ""
git push -u origin main

if ($LASTEXITCODE -eq 0) {
    Write-Host ""
    Write-Host "============================================================" -ForegroundColor Cyan
    Write-Host "  SUCCESS! Project pushed to GitHub!" -ForegroundColor Green
    Write-Host "============================================================" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "Repository URL: $repoUrl" -ForegroundColor White
    Write-Host ""
    Write-Host "Next steps:" -ForegroundColor Yellow
    Write-Host "  1. Visit your repository on GitHub" -ForegroundColor White
    Write-Host "  2. Add repository description and topics" -ForegroundColor White
    Write-Host "  3. Create a release (v1.3)" -ForegroundColor White
    Write-Host ""
} else {
    Write-Host ""
    Write-Host "============================================================" -ForegroundColor Cyan
    Write-Host "  ERROR: Failed to push to GitHub" -ForegroundColor Red
    Write-Host "============================================================" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "Possible issues:" -ForegroundColor Yellow
    Write-Host "  - Repository does not exist on GitHub (create it first)" -ForegroundColor White
    Write-Host "  - Authentication failed (check your GitHub credentials)" -ForegroundColor White
    Write-Host "  - Network connection issues" -ForegroundColor White
    Write-Host ""
    Write-Host "See GITHUB_SETUP.md for detailed instructions." -ForegroundColor Yellow
    Write-Host ""
}

Read-Host "Press Enter to exit"

