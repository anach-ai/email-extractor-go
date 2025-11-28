# Usage Guide

Complete usage guide for Email Extractor.

---

## Basic Usage

### Standard Extraction

```bash
go run main.go ocr_extraction.go distributed_processing.go machine.go --yes
```

This will:
- Use `domains/domains.txt` from config
- Automatically optimize settings for your system
- Extract emails from HTML and images
- Show real-time progress
- Save results to `output/` directory

---

## Test Mode

### Test Single Domain

```bash
go run main.go ocr_extraction.go distributed_processing.go machine.go --test example.com
```

### Test with URL

```bash
go run main.go ocr_extraction.go distributed_processing.go machine.go --test https://example.com
```

---

## Command Line Options

| Flag | Description |
|------|-------------|
| `--yes`, `-y` | Skip confirmation prompt |
| `--test <domain>` | Test mode for single domain |
| `--version`, `-v` | Show version information |
| `--help`, `-h` | Show help message |

---

## Configuration

Edit `config.json` to customize:

- `domains_file_path` - Path to domain list (default: `./domains/domains.txt`)
- `enable_ocr` - Enable OCR extraction (default: `true`)
- `concurrency` - Number of workers (auto-optimized)
- `rate_limit_per_second` - Request rate (auto-optimized)
- `timeout` - Request timeout in seconds (auto-optimized)

---

## Output Files

Results are saved to `output/` directory:

- `extracted_emails.txt` - All unique emails
- `resolved_domains.txt` - Successful domains
- `unresolved_domains.txt` - Failed domains
- `categorized_*.txt` - Emails by department

---

## Examples

### Basic Extraction

```bash
go run main.go ocr_extraction.go distributed_processing.go machine.go --yes
```

### Show Version

```bash
go run main.go ocr_extraction.go distributed_processing.go machine.go --version
```

Output:
```
Email Extractor v1.3
Author: Dr.Anach
Telegram: @dranach
```

---

## Building Executable

Instead of `go run`, build an executable:

```bash
go build -o email-extractor.exe main.go ocr_extraction.go distributed_processing.go machine.go
```

Then run:
```bash
./email-extractor.exe --yes
```

---

## Troubleshooting

### Issue: "go: command not found"

- Restart terminal after installing Go
- Verify Go is in PATH: `go version`

### Issue: "tesseract: command not found"

- Install Tesseract OCR
- Add to PATH or restart terminal

### Issue: "domains.txt not found"

- Create `domains/domains.txt` file
- Or update `config.json` with correct path

---

**For more help, see [INSTALLATION.md](INSTALLATION.md)**

---

**Author:** Dr.Anach | **Telegram:** [@dranach](https://t.me/dranach)

