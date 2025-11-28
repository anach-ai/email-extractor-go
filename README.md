# Email Extractor

A powerful, production-ready Go-based tool for extracting email addresses from websites with high accuracy and advanced features.

**Author:** Dr.Anach | **Telegram:** [@dranach](https://t.me/dranach)

---

## ⭐ Features

- 🔍 **Smart Email Validation** - MX record verification, disposable domain filtering
- 🎯 **Obfuscation Detection** - Detects 20+ email obfuscation techniques
- 📄 **Contact Page Discovery** - 99% accuracy in finding contact pages
- 🖼️ **OCR Image Extraction** - Extract emails from images using Tesseract
- ⚡ **High Performance** - Concurrent processing with automatic system optimization
- 🌍 **Multilingual Support** - 11 languages for contact page detection
- 📊 **Email Categorization** - Automatic categorization by department
- 🔒 **Cloudflare Detection** - Advanced bot detection and bypass strategies
- 📈 **Real-time Progress** - Detailed progress bar with statistics
- 🎨 **Clean Output** - Automatic artifact removal and email cleaning

---

## 🚀 Quick Start

### Prerequisites

- **Go** 1.23 or higher ([Download](https://go.dev/dl/))
- **Tesseract OCR** (optional, for image extraction) ([Download](https://github.com/UB-Mannheim/tesseract/wiki))

### Installation

1. **Clone the repository:**
   ```bash
   git clone <repository-url>
   cd email-extractor.go.v1.2
   ```

2. **Install dependencies:**
   ```bash
   go mod tidy
   ```
   
   This will download all dependencies and generate the `go.sum` file.

3. **Run the extractor:**
   ```bash
   go run main.go ocr_extraction.go distributed_processing.go machine.go --yes
   ```

The tool will automatically:
- ✅ Use `domains/domains.txt` from config
- ✅ Optimize settings for your system
- ✅ Extract emails from HTML and images
- ✅ Show real-time progress
- ✅ Save results to `output/` directory

---

## 📋 Configuration

Edit `config.json` to customize settings:

```json
{
  "domains_file_path": "./domains/domains.txt",
  "enable_ocr": true,
  "concurrency": 50,
  "rate_limit_per_second": 50,
  "timeout": 20
}
```

### Key Settings

- `domains_file_path` - Path to your domain list file
- `enable_ocr` - Enable/disable OCR email extraction from images
- `concurrency` - Number of concurrent workers (auto-optimized)
- `rate_limit_per_second` - Request rate limit (auto-optimized)
- `timeout` - Request timeout in seconds (auto-optimized)

---

## 💻 Usage

### Basic Extraction

```bash
go run main.go ocr_extraction.go distributed_processing.go machine.go --yes
```

### Test Single Domain

```bash
go run main.go ocr_extraction.go distributed_processing.go machine.go --test example.com
```

### Show Version

```bash
go run main.go ocr_extraction.go distributed_processing.go machine.go --version
```

---

## 📁 Project Structure

```
email-extractor.go.v1.2/
├── main.go                          # Main application
├── ocr_extraction.go                # OCR functionality
│
├── 🔧 Utility Modules
│   ├── machine.go                   # Auto system optimization utility
│   │   └── Analyzes CPU, memory, network & optimizes performance
│   └── distributed_processing.go    # Distributed processing utility
│       └── Redis-based scaling (future feature, disabled by default)
│
├── config.json                      # Configuration file
├── go.mod                           # Go module definition
├── README.md                        # This file
├── QUICK_START_WINDOWS.md          # Quick start guide
│
├── data/                            # Data files
│   ├── user_agents.txt              # Browser user agents (63 entries)
│   ├── referer_links.txt            # Referer URLs (186 entries)
│   ├── contact_lang.txt             # Contact keywords (469, 11 languages)
│   ├── unwanted_doms.txt            # Disposable domains (173K)
│   ├── email_bad_extensions.txt     # File extensions filter (197)
│   ├── page_bad_extensions.txt      # Page extensions filter (93)
│   └── email_categories.txt         # Email categorization (10 categories)
│
├── domains/                         # Domain lists
│   └── domains.txt                  # Default domain list
│
├── output/                          # Results directory
│   ├── extracted_emails.txt         # All extracted emails
│   ├── resolved_domains.txt         # Successful domains
│   ├── unresolved_domains.txt       # Failed domains
│   └── categorized_*.txt            # Categorized emails by department
│
└── docs/                            # GitHub-friendly documentation
    ├── INSTALLATION.md              # Installation guide
    └── USAGE.md                     # Usage guide
```

### Utility Modules Explained

#### 🚀 `machine.go` - Auto Performance Optimization

Automatically optimizes the extractor for your system:
- **CPU Analysis**: Detects cores and usage, optimizes concurrency
- **Memory Analysis**: Adjusts batch sizes based on available RAM
- **Network Analysis**: Measures latency and speed, optimizes timeouts and rate limits
- **Auto-Tuning**: No manual configuration needed - works optimally on any system

**Benefits**: Best performance on your hardware without manual tweaking.

#### 🔄 `distributed_processing.go` - Distributed Processing (Future)

Framework for scaling across multiple machines:
- **Redis Integration**: Queue-based job distribution
- **Horizontal Scaling**: Process domains across multiple workers
- **Status**: Framework ready, disabled by default (set `distributed_mode: true` in config.json to enable)

**Note**: Requires Redis server. Currently a placeholder for future distributed processing needs.

---

## 📊 Output Files

Results are saved to the `output/` directory:

### Main Files

- **`extracted_emails.txt`** - All unique emails found
- **`resolved_domains.txt`** - Domains successfully processed
- **`unresolved_domains.txt`** - Domains that failed

### Categorized Files

- **`categorized_general.txt`** - General inquiries
- **`categorized_sales.txt`** - Sales contacts
- **`categorized_support.txt`** - Support contacts
- **`categorized_admin.txt`** - Administrative contacts
- **`categorized_marketing.txt`** - Marketing contacts
- **`categorized_technical.txt`** - Technical contacts
- **`categorized_finance.txt`** - Finance contacts
- **`categorized_hr.txt`** - HR contacts
- **`categorized_legal.txt`** - Legal contacts
- **`categorized_operations.txt`** - Operations contacts
- **`categorized_other.txt`** - Other categories

---

## 🔧 Advanced Features

### Automatic System Optimization

The tool automatically optimizes settings based on your system:
- CPU cores and usage
- Available memory
- Network speed and latency

### OCR Email Extraction

Extract emails from images using Tesseract OCR:
- Automatic image detection
- Text cleaning and error correction
- Multiple email pattern matching

### Smart Email Cleaning

Automatic removal of artifacts:
- Phone number prefixes
- Time range prefixes
- URL/domain prefixes
- Text concatenation issues

### Cloudflare Detection

Advanced detection and handling of Cloudflare-protected sites.

---

## 📖 Documentation

- **[Quick Start Guide](QUICK_START_WINDOWS.md)** - Get started in 5 minutes
- **[Installation Guide](docs/INSTALLATION.md)** - Detailed installation instructions
- **[Usage Guide](docs/USAGE.md)** - Complete usage documentation
- **[Utilities Guide](docs/UTILITIES.md)** - Utility modules explained (`machine.go`, `distributed_processing.go`)

---

## ⚙️ Requirements

- **Go** 1.23 or higher
- **Tesseract OCR** (optional, for image extraction)
- **Internet connection**
- **Domain list file** (one domain per line)

---

## 🛠️ Development

### Building

```bash
go build -o email-extractor.exe main.go ocr_extraction.go distributed_processing.go machine.go
```

### Running Tests

```bash
go run main.go ocr_extraction.go distributed_processing.go machine.go --test example.com
```

---

## 📝 License

See LICENSE file for details.

---

## 👤 Author

**Dr.Anach**

- **Telegram:** [@dranach](https://t.me/dranach)

---

## 🙏 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

---

## ⚠️ Disclaimer

This tool is for educational and legitimate business purposes only. Always respect website terms of service and robots.txt files. Use responsibly and ethically.

---

**Version:** 1.3  
**Last Updated:** 2025-11-28
