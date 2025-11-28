# Add Tesseract to PATH
Write-Host "`n🔧 Adding Tesseract OCR to PATH...`n" -ForegroundColor Cyan

$tesseractPath = "C:\Program Files\Tesseract-OCR"
$tesseractExe = "$tesseractPath\tesseract.exe"

# Check if Tesseract exists
if (-not (Test-Path $tesseractExe)) {
    Write-Host "❌ Tesseract not found at: $tesseractExe" -ForegroundColor Red
    Write-Host "Please check your installation.`n" -ForegroundColor Yellow
    exit 1
}

Write-Host "✅ Tesseract found at: $tesseractExe`n" -ForegroundColor Green

# Get current PATH
$currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")

# Check if already in PATH
if ($currentPath -like "*$tesseractPath*") {
    Write-Host "✅ Tesseract is already in PATH!`n" -ForegroundColor Green
} else {
    Write-Host "Adding Tesseract to system PATH...`n" -ForegroundColor Yellow
    
    try {
        # Add to system PATH (requires admin)
        $newPath = $currentPath + ";$tesseractPath"
        [Environment]::SetEnvironmentVariable("Path", $newPath, "Machine")
        
        Write-Host "✅ Tesseract added to PATH successfully!`n" -ForegroundColor Green
        Write-Host "⚠️  Please restart your terminal for changes to take effect.`n" -ForegroundColor Yellow
    } catch {
        Write-Host "❌ Failed to add to system PATH: $_`n" -ForegroundColor Red
        Write-Host "Trying user PATH instead...`n" -ForegroundColor Yellow
        
        try {
            # Try user PATH (doesn't require admin)
            $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
            if ($userPath -notlike "*$tesseractPath*") {
                $newUserPath = $userPath + ";$tesseractPath"
                [Environment]::SetEnvironmentVariable("Path", $newUserPath, "User")
                Write-Host "✅ Tesseract added to user PATH!`n" -ForegroundColor Green
                Write-Host "⚠️  Please restart your terminal for changes to take effect.`n" -ForegroundColor Yellow
            }
        } catch {
            Write-Host "❌ Failed to add to PATH. Please add manually:`n" -ForegroundColor Red
            Write-Host "   $tesseractPath`n" -ForegroundColor White
        }
    }
}

# Also update current session PATH
$env:Path = [Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [Environment]::GetEnvironmentVariable("Path", "User")

# Verify
Write-Host "Verifying Tesseract is accessible...`n" -ForegroundColor Cyan
$tesseract = Get-Command tesseract -ErrorAction SilentlyContinue

if ($tesseract) {
    Write-Host "✅ Tesseract is now accessible in this session!`n" -ForegroundColor Green
    Write-Host "Version:" -ForegroundColor Cyan
    & tesseract --version | Select-Object -First 1
    Write-Host ""
} else {
    Write-Host "⚠️  Tesseract not yet accessible. Restart terminal and try again.`n" -ForegroundColor Yellow
    Write-Host "However, the email extractor will still find it at:`n" -ForegroundColor Cyan
    Write-Host "   $tesseractExe`n" -ForegroundColor White
}

