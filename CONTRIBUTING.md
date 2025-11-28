# Contributing to Email Extractor

Thank you for your interest in contributing! This document provides guidelines and instructions for contributing to Email Extractor.

---

## Getting Started

1. **Fork the repository**
2. **Clone your fork:**
   ```bash
   git clone <your-fork-url>
   cd email-extractor.go.v1.2
   ```

3. **Create a branch:**
   ```bash
   git checkout -b feature/your-feature-name
   ```

---

## Development Guidelines

### Code Style

- Follow Go formatting standards
- Use meaningful variable names
- Add comments for complex logic
- Keep functions focused and concise

### Testing

Before submitting:

1. Test your changes:
   ```bash
   go run main.go ocr_extraction.go distributed_processing.go machine.go --test example.com
   ```

2. Run with a small domain list:
   ```bash
   go run main.go ocr_extraction.go distributed_processing.go machine.go --yes
   ```

---

## Submission Process

1. **Commit your changes:**
   ```bash
   git commit -m "Description of your changes"
   ```

2. **Push to your fork:**
   ```bash
   git push origin feature/your-feature-name
   ```

3. **Create a Pull Request:**
   - Describe your changes
   - Reference any related issues
   - Include test results if applicable

---

## Areas for Contribution

- 🐛 Bug fixes
- ⚡ Performance improvements
- 📚 Documentation improvements
- 🌍 Additional language support
- 🔍 Enhanced email detection patterns
- 🎯 New features

---

## Questions?

Contact: **Dr.Anach** | **Telegram:** [@dranach](https://t.me/dranach)

---

Thank you for contributing! 🎉

