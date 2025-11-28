# Features

Comprehensive list of Email Extractor features.

---

## Core Features

### 🎯 Smart Email Extraction

- **HTML Parsing** - Extracts emails from HTML content
- **Image OCR** - Extracts emails from images using Tesseract
- **Text Node Processing** - Respects HTML structure to prevent concatenation
- **Multiple Pattern Matching** - Uses various regex patterns for detection

### 🔍 Validation & Filtering

- **MX Record Verification** - Validates email domains via DNS
- **Disposable Domain Filtering** - Filters out temporary email domains (173K+)
- **Extension Filtering** - Removes file paths and invalid extensions
- **Artifact Cleaning** - Removes phone numbers, time ranges, prefixes

### 🌍 Multilingual Support

- **11 Languages** - English, German, Spanish, French, Italian, Portuguese, Russian, Japanese, Korean, Chinese, Arabic
- **469 Contact Keywords** - Comprehensive keyword list for contact page detection

### 📊 Email Categorization

- **10 Categories** - Automatically categorizes emails by department
  - General, Sales, Support, Admin, Marketing
  - Technical, Finance, HR, Legal, Operations
- **Pattern Matching** - Uses keyword patterns for categorization

### ⚡ Performance

- **Concurrent Processing** - Parallel domain processing
- **Auto-Optimization** - Automatically optimizes settings based on system
- **Rate Limiting** - Configurable request rate limiting
- **DNS Caching** - Fast DNS pre-checks with caching

### 🔒 Advanced Features

- **Cloudflare Detection** - Detects and handles Cloudflare challenges
- **Browser Headers** - Enhanced HTTP headers for better stealth
- **Cookie Management** - Persistent cookie jar for sessions
- **Obfuscation Detection** - Detects 20+ email obfuscation techniques

### 📈 Progress Tracking

- **Real-time Progress Bar** - Visual progress with statistics
- **Detailed Statistics** - Emails, phones, success/failure rates
- **ETA Calculation** - Estimated time remaining
- **Cloudflare Tracking** - Tracks blocked domains

---

## Technical Features

- ✅ Go 1.23+ compatible
- ✅ Cross-platform (Windows, macOS, Linux)
- ✅ Memory efficient with streaming
- ✅ Graceful shutdown handling
- ✅ Comprehensive error handling
- ✅ Detailed logging

---

**Author:** Dr.Anach | **Telegram:** [@dranach](https://t.me/dranach)

