# Installation Guide

Complete installation guide for Email Extractor.

---

## System Requirements

- **OS**: Windows 10/11, macOS, or Linux
- **RAM**: 4GB minimum (8GB+ recommended)
- **Disk Space**: 500MB free space
- **Internet**: Stable connection required
- **Go**: Version 1.23 or higher
- **Tesseract OCR**: Latest version (optional, for image extraction)

---

## Step 1: Install Go

### Windows

1. Download: https://go.dev/dl/
2. Run the `.msi` installer
3. Keep default path: `C:\Program Files\Go`
4. Verify: `go version`

### macOS

```bash
brew install go
# or download from https://go.dev/dl/
```

### Linux

```bash
sudo apt update
sudo apt install golang-go
# or download from https://go.dev/dl/
```

### Verify Installation

```bash
go version
# Should show: go version go1.23.x
```

---

## Step 2: Install Tesseract OCR (Optional)

Tesseract is required for extracting emails from images.

### Windows

1. Download: https://github.com/UB-Mannheim/tesseract/wiki
2. Run installer to: `C:\Program Files\Tesseract-OCR`
3. Check "Add to PATH" during installation
4. Verify: `tesseract --version`

### macOS

```bash
brew install tesseract
```

### Linux

```bash
sudo apt install tesseract-ocr
```

---

## Step 3: Clone Repository

```bash
git clone <repository-url>
cd email-extractor.go.v1.2
```

---

## Step 4: Install Dependencies

```bash
go mod tidy
```

This will download all dependencies and generate the `go.sum` file.

---

## Step 5: Configure Domain List

1. Edit `domains/domains.txt`
2. Add domains (one per line):
   ```
   example.com
   test.com
   ```

---

## Step 6: Run the Extractor

```bash
go run main.go ocr_extraction.go distributed_processing.go machine.go --yes
```

---

## ✅ Installation Complete!

Your Email Extractor is now ready to use.

For detailed usage, see [USAGE.md](USAGE.md).

---

**Author:** Dr.Anach | **Telegram:** [@dranach](https://t.me/dranach)

