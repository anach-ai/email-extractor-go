# Project Structure

Overview of the Email Extractor project structure.

---

## 📁 Directory Structure

```
email-extractor.go.v1.2/
│
├── 📄 Core Source Files
│   ├── main.go                          # Main application (with author signature)
│   ├── ocr_extraction.go                # OCR functionality (with author signature)
│   ├── machine.go                       # Performance optimization (with author signature)
│   └── distributed_processing.go        # Distributed processing (with author signature)
│
├── 📋 Configuration & Setup
│   ├── config.json                      # Main configuration file
│   ├── go.mod                           # Go module definition
│   └── .gitignore                       # Git ignore rules
│
├── 📚 Documentation
│   ├── README.md                        # Main README (GitHub-friendly)
│   ├── QUICK_START_WINDOWS.md          # Quick start guide
│   ├── LICENSE                          # MIT License
│   ├── CONTRIBUTING.md                  # Contribution guidelines
│   │
│   └── docs/                            # GitHub-friendly documentation
│       ├── INSTALLATION.md              # Installation guide
│       ├── USAGE.md                     # Usage guide
│       ├── FEATURES.md                  # Features list
│       ├── CHANGELOG.md                 # Version history
│       └── PROJECT_STRUCTURE.md         # This file
│
├── 📊 Data Files
│   └── data/                            # Filter and configuration data
│       ├── user_agents.txt              # 63 modern user agents (2025)
│       ├── referer_links.txt            # 186 referer URLs
│       ├── contact_lang.txt             # 469 keywords, 11 languages
│       ├── unwanted_doms.txt            # 173K disposable domains
│       ├── email_bad_extensions.txt     # 197 file extensions
│       ├── page_bad_extensions.txt      # 93 page extensions
│       └── email_categories.txt         # 10 email categories
│
├── 🌐 Domain Lists
│   └── domains/                         # Domain input files
│       ├── domains.txt                  # Default domain list
│       ├── test_100_domains.txt         # Test domain list
│       └── Hetzner_domains.txt          # Example domain list
│
├── 📤 Output Directory
│   └── output/                          # Results (empty in fresh install)
│       ├── extracted_emails.txt         # All extracted emails
│       ├── resolved_domains.txt         # Successful domains
│       ├── unresolved_domains.txt       # Failed domains
│       ├── categorized_*.txt            # Categorized emails
│       └── email_categories_summary.txt # Category summary
│
├── 🛠️ Utility Scripts
│   └── scripts/                         # Helper scripts
│       ├── install_tesseract.bat        # Tesseract installer
│       ├── verify_tesseract.bat         # Tesseract verifier
│       └── ... (other utility scripts)
│
└── 🔧 GitHub Templates
    └── .github/
        └── ISSUE_TEMPLATE/
            ├── bug_report.md            # Bug report template
            └── feature_request.md       # Feature request template
```

---

## 📄 File Descriptions

### Core Files

- **`main.go`** - Main application logic, HTTP client, email extraction
- **`ocr_extraction.go`** - OCR functionality for image email extraction

### Utility Modules

- **`machine.go`** - **Performance Optimization Utility**
  - Automatically analyzes system resources (CPU, memory, network)
  - Optimizes concurrency, rate limits, timeouts based on hardware
  - Adjusts batch sizes and delays for optimal performance
  - Ensures best performance on any system without manual tuning

- **`distributed_processing.go`** - **Distributed Processing Utility**
  - Framework for Redis-based distributed processing
  - Enables horizontal scaling across multiple workers
  - Queue-based job distribution (future feature)
  - Currently disabled by default (set `distributed_mode: true` to enable)

### Configuration

- **`config.json`** - Application configuration (concurrency, timeouts, paths)
- **`go.mod`** - Go module dependencies
- **`go.sum`** - Dependency checksums (auto-generated on `go mod download`)

### Documentation

- **`README.md`** - Main project README (GitHub-friendly)
- **`QUICK_START_WINDOWS.md`** - Quick start guide for Windows
- **`docs/`** - Public documentation for GitHub

---

## 🔒 Author Information

All source files include author signature:

```go
/*
 * Email Extractor
 * 
 * Author: Dr.Anach
 * Telegram: @dranach
 */
```

Version output also shows:
```
Email Extractor v1.3
Author: Dr.Anach
Telegram: @dranach
```

---

## 📊 File Counts

- **Source Files**: 4 Go files
- **Data Files**: 7 data files
- **Documentation**: 5 files in docs/
- **Scripts**: 11 utility scripts

---

**Author:** Dr.Anach | **Telegram:** [@dranach](https://t.me/dranach)

