# Quick Start Guide - Windows

Get up and running in 5 minutes!

**✨ Fresh Project** - This is a clean, fresh installation ready to use.

---

## ⚡ Quick Setup (3 Steps)

### 1. Install Go
- Download: https://go.dev/dl/
- Install: Run `.msi` installer
- Verify: `go version` in PowerShell

### 2. Install Tesseract OCR
- Download: https://github.com/UB-Mannheim/tesseract/wiki
- Install: Run installer, check "Add to PATH"
- Verify: `tesseract --version` in PowerShell

### 3. Run the Extractor
```powershell
cd "C:\path\to\email-extractor.go.v1.2"
go mod download
go run main.go ocr_extraction.go distributed_processing.go machine.go --yes
```

**Replace `C:\path\to\` with your actual project directory path.**

**Note**: This is a fresh project - no compiled binaries or output files. Ready to use!

---

## 📝 Domain File Setup

1. Edit `domains/domains.txt`
2. Add domains (one per line):
   ```
   example.com
   test.com
   another-domain.com
   ```
3. Save the file

---

## 🎯 That's It!

This is a **fresh, clean project** ready for your first extraction!

The tool will:
- ✅ Use `domains/domains.txt` automatically (already configured)
- ✅ Extract emails from HTML and images
- ✅ Show progress in real-time
- ✅ Save results to `output/` folder (empty and ready)

**Project Status**: ✅ Clean - No compiled binaries, no output files, no auto-generated files - ready for fresh start!

---

## 📖 Full Guide

For detailed instructions, see: `private-docs/WINDOWS_SETUP_GUIDE.md`

---

**Ready? Run:**
```powershell
go run main.go ocr_extraction.go distributed_processing.go machine.go --yes
```

