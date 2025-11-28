# Utility Modules

Email Extractor includes utility modules to enhance functionality and performance.

---

## 🚀 machine.go - Auto Performance Optimization

**Purpose**: Automatically optimizes the extractor for your system hardware.

### Features

- **System Analysis**
  - CPU cores and usage detection
  - Memory (total and available) analysis
  - Network speed and latency measurement

- **Automatic Optimization**
  - **Concurrency**: Adjusts worker count based on CPU cores
  - **Rate Limits**: Optimizes requests per second based on network speed
  - **Timeouts**: Adjusts timeouts based on network latency
  - **Batch Sizes**: Optimizes batch sizes based on available memory
  - **Delays**: Adjusts request delays for optimal performance

### How It Works

1. **Analyzes System** on startup:
   - CPU cores and current usage
   - Total and available memory
   - Network speed and latency

2. **Optimizes Configuration**:
   - Calculates optimal concurrency based on CPU
   - Adjusts rate limits for network capacity
   - Sets timeouts based on network latency
   - Optimizes batch sizes for available memory

3. **No Manual Configuration Needed**:
   - Works optimally on any system
   - Automatically adapts to hardware
   - Ensures best performance without tuning

### Usage

The utility is **automatically enabled** and runs on every startup:

```bash
go run main.go ocr_extraction.go distributed_processing.go machine.go --yes
```

The optimization happens automatically - no configuration needed!

### Benefits

- ✅ **Best Performance**: Optimized for your specific hardware
- ✅ **No Manual Tuning**: Works out of the box
- ✅ **Resource Efficient**: Uses optimal resource allocation
- ✅ **Adaptive**: Adjusts to system changes

---

## 🔄 distributed_processing.go - Distributed Processing Framework

**Purpose**: Framework for distributed processing across multiple machines using Redis.

### Features

- **Redis Integration** (planned):
  - Queue-based job distribution
  - Worker registration and management
  - Job status tracking

- **Horizontal Scaling**:
  - Distribute domains across multiple workers
  - Process large domain lists faster
  - Load balancing across machines

### Current Status

⚠️ **Framework Only** - Redis integration not yet implemented

- Functions are defined but not actively used
- Disabled by default (`distributed_mode: false` in config.json)
- Placeholder for future distributed processing needs

### How to Enable (When Implemented)

1. **Install Redis** server
2. **Update config.json**:
   ```json
   {
     "distributed_mode": true,
     "redis_url": "redis://localhost:6379",
     "worker_id": "worker-1",
     "queue_name": "email_extraction"
   }
   ```
3. **Run multiple workers** on different machines

### Future Benefits

- ⚡ **Faster Processing**: Multiple machines working in parallel
- 📈 **Scalability**: Add more workers as needed
- 🔄 **Resilience**: Jobs can be retried if worker fails
- 📊 **Progress Tracking**: Centralized job status

---

## 📊 Utility Comparison

| Utility | Status | Auto-Enabled | Purpose |
|---------|--------|--------------|---------|
| `machine.go` | ✅ Active | Yes | Auto-optimize performance |
| `distributed_processing.go` | ⚠️ Framework | No | Distributed scaling (future) |

---

## 🎯 When to Use

### Use `machine.go` (Always)
- ✅ Enabled by default
- ✅ Automatically optimizes performance
- ✅ No configuration needed
- ✅ Works on any system

### Use `distributed_processing.go` (Future)
- ⚠️ When you need to process very large domain lists
- ⚠️ When you have multiple machines available
- ⚠️ When Redis integration is implemented
- ⚠️ When you need horizontal scaling

---

## 📖 Related Documentation

- [Installation Guide](INSTALLATION.md)
- [Usage Guide](USAGE.md)
- [Features](FEATURES.md)
- [Project Structure](PROJECT_STRUCTURE.md)

---

**Author:** Dr.Anach | **Telegram:** [@dranach](https://t.me/dranach)

