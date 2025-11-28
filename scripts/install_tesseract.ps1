# Tesseract OCR Installation Script
Write-Host "`n📦 TESSERACT OCR INSTALLATION`n" -ForegroundColor Cyan
Write-Host "=" * 70 -ForegroundColor Cyan

# Check if already installed
Write-Host "`n🔍 Checking if Tesseract is already installed...`n" -ForegroundColor Yellow
$tesseractPaths = @(
    "tesseract",
    "C:\Program Files\Tesseract-OCR\tesseract.exe",
    "C:\Program Files (x86)\Tesseract-OCR\tesseract.exe"
)

$found = $false
foreach ($path in $tesseractPaths) {
    if ($path -eq "tesseract") {
        $result = Get-Command tesseract -ErrorAction SilentlyContinue
        if ($result) {
            Write-Host "✅ Tesseract is already installed!" -ForegroundColor Green
            Write-Host "   Location: $($result.Source)" -ForegroundColor White
            $version = & tesseract --version 2>&1 | Select-Object -First 1
            Write-Host "   Version: $version`n" -ForegroundColor White
            $found = $true
            break
        }
    } else {
        if (Test-Path $path) {
            Write-Host "✅ Tesseract is already installed!" -ForegroundColor Green
            Write-Host "   Location: $path`n" -ForegroundColor White
            $found = $true
            break
        }
    }
}

if ($found) {
    Write-Host "✅ Tesseract is ready to use!`n" -ForegroundColor Green
    exit 0
}

Write-Host "❌ Tesseract not found. Installing...`n" -ForegroundColor Yellow

# Try Chocolatey first
$choco = Get-Command choco -ErrorAction SilentlyContinue
if ($choco) {
    Write-Host "📦 Method 1: Using Chocolatey...`n" -ForegroundColor Cyan
    try {
        Write-Host "Installing Tesseract OCR via Chocolatey...`n" -ForegroundColor Yellow
        choco install tesseract --yes
        Write-Host "`n✅ Installation via Chocolatey complete!`n" -ForegroundColor Green
        
        # Refresh environment
        $env:Path = [System.Environment]::GetEnvironmentVariable("Path","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path","User")
        
        # Verify
        Start-Sleep -Seconds 2
        $tesseract = Get-Command tesseract -ErrorAction SilentlyContinue
        if ($tesseract) {
            Write-Host "✅ Tesseract installed successfully!" -ForegroundColor Green
            Write-Host "   Location: $($tesseract.Source)`n" -ForegroundColor White
            exit 0
        }
    } catch {
        Write-Host "❌ Chocolatey installation failed: $_`n" -ForegroundColor Red
    }
} else {
    Write-Host "⚠️  Chocolatey not found. Trying direct download...`n" -ForegroundColor Yellow
}

# Method 2: Direct download
Write-Host "📥 Method 2: Direct Download...`n" -ForegroundColor Cyan
$downloadUrl = "https://digi.bib.uni-mannheim.de/tesseract/tesseract-ocr-w64-setup-5.4.0.20240619.exe"
$installerPath = "$env:TEMP\tesseract-installer.exe"
$installDir = "C:\Program Files\Tesseract-OCR"

try {
    Write-Host "Downloading Tesseract installer...`n" -ForegroundColor Yellow
    Write-Host "URL: $downloadUrl" -ForegroundColor Gray
    Write-Host "Saving to: $installerPath`n" -ForegroundColor Gray
    
    Invoke-WebRequest -Uri $downloadUrl -OutFile $installerPath -UseBasicParsing -ErrorAction Stop
    
    Write-Host "✅ Download complete!`n" -ForegroundColor Green
    Write-Host "Starting installer...`n" -ForegroundColor Yellow
    Write-Host "Please follow the installer prompts.`n" -ForegroundColor Yellow
    Write-Host "Recommended settings:" -ForegroundColor Cyan
    Write-Host "  • Installation path: $installDir" -ForegroundColor White
    Write-Host "  • ✅ Add to PATH (if available)`n" -ForegroundColor White
    
    # Run installer (silent install if possible)
    Start-Process -FilePath $installerPath -Wait
    
    Write-Host "`n✅ Installation complete!`n" -ForegroundColor Green
    
    # Clean up
    if (Test-Path $installerPath) {
        Remove-Item $installerPath -Force
    }
    
    # Refresh PATH
    $env:Path = [System.Environment]::GetEnvironmentVariable("Path","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path","User")
    
} catch {
    Write-Host "`n❌ Automatic download failed: $_`n" -ForegroundColor Red
    Write-Host "Please install manually:" -ForegroundColor Yellow
    Write-Host "1. Download from: https://github.com/UB-Mannheim/tesseract/wiki" -ForegroundColor White
    Write-Host "2. Run the installer" -ForegroundColor White
    Write-Host "3. Install to: $installDir`n" -ForegroundColor White
}

# Final verification
Write-Host "`n🔍 Verifying installation...`n" -ForegroundColor Cyan
Start-Sleep -Seconds 3

$tesseract = $null
foreach ($path in $tesseractPaths) {
    if ($path -eq "tesseract") {
        $tesseract = Get-Command tesseract -ErrorAction SilentlyContinue
        if ($tesseract) { break }
    } else {
        if (Test-Path $path) {
            $tesseract = @{ Source = $path }
            break
        }
    }
}

if ($tesseract) {
    Write-Host "✅ Tesseract installed successfully!" -ForegroundColor Green
    Write-Host "   Location: $($tesseract.Source)" -ForegroundColor White
    
    # Try to get version
    try {
        $versionOutput = & $tesseract.Source --version 2>&1 | Select-Object -First 1
        Write-Host "   Version: $versionOutput" -ForegroundColor White
    } catch {
        Write-Host "   (Version check skipped)" -ForegroundColor Gray
    }
    
    Write-Host "`n✅ Ready to extract emails from images!`n" -ForegroundColor Green
} else {
    Write-Host "⚠️  Tesseract not found. Please:" -ForegroundColor Yellow
    Write-Host "   1. Restart your terminal/PowerShell" -ForegroundColor White
    Write-Host "   2. Or manually add Tesseract to PATH" -ForegroundColor White
    Write-Host "   3. Or re-run this script`n" -ForegroundColor White
}

Write-Host "=" * 70`n" -ForegroundColor Cyan

