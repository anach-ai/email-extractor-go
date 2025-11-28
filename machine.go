/*
 * Email Extractor - Performance Optimization Module
 * 
 * Author: Dr.Anach
 * Telegram: @dranach
 */

package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	psnet "github.com/shirou/gopsutil/net"
)

// SystemPerformance represents system performance metrics
type SystemPerformance struct {
	CPUCores           int
	CPUUsage           float64
	TotalMemoryMB      uint64
	AvailableMemoryMB  uint64
	MemoryUsagePercent float64
	NetworkSpeed       float64 // Mbps (estimated)
	NetworkLatency     time.Duration
}

// OptimizeConfig optimizes configuration based on system performance
func OptimizeConfig(config *Config) (*Config, *SystemPerformance, error) {
	perf, err := AnalyzeSystemPerformance()
	if err != nil {
		return config, nil, fmt.Errorf("failed to analyze system performance: %v", err)
	}

	optimized := *config // Copy original config

	// Optimize concurrency based on CPU cores and available memory
	optimized.Concurrency = calculateOptimalConcurrency(perf)

	// Optimize rate limit based on network speed
	optimized.RateLimitPerSecond = calculateOptimalRateLimit(perf)

	// Optimize timeout based on network latency
	optimized.Timeout = calculateOptimalTimeout(perf)

	// Optimize max parallel requests based on resources
	optimized.MaxParallelRequests = calculateOptimalMaxParallelRequests(perf)

	// Optimize batch size based on available memory
	optimized.BatchSize = calculateOptimalBatchSize(perf)

	// Optimize delays based on network performance
	optimized.MinDelayBetweenRequests = calculateOptimalMinDelay(perf)
	optimized.MaxDelayBetweenRequests = calculateOptimalMaxDelay(perf)

	return &optimized, perf, nil
}

// AnalyzeSystemPerformance analyzes CPU, memory, and network performance
func AnalyzeSystemPerformance() (*SystemPerformance, error) {
	perf := &SystemPerformance{}

	// Get CPU information
	cpuInfo, err := cpu.Info()
	if err != nil {
		return nil, fmt.Errorf("failed to get CPU info: %v", err)
	}
	if len(cpuInfo) > 0 {
		perf.CPUCores = int(cpuInfo[0].Cores)
		if perf.CPUCores == 0 {
			perf.CPUCores = runtime.NumCPU() // Fallback to runtime
		}
	} else {
		perf.CPUCores = runtime.NumCPU()
	}

	// Get current CPU usage (average over 1 second)
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err == nil && len(cpuPercent) > 0 {
		perf.CPUUsage = cpuPercent[0]
	}

	// Get memory information
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory info: %v", err)
	}
	perf.TotalMemoryMB = vmStat.Total / 1024 / 1024
	perf.AvailableMemoryMB = vmStat.Available / 1024 / 1024
	perf.MemoryUsagePercent = vmStat.UsedPercent

	// Analyze network performance
	networkSpeed, latency := analyzeNetworkPerformance()
	perf.NetworkSpeed = networkSpeed
	perf.NetworkLatency = latency

	return perf, nil
}

// analyzeNetworkPerformance estimates network speed and latency
func analyzeNetworkPerformance() (speedMbps float64, latency time.Duration) {
	// Test latency to a reliable server (Google DNS)
	start := time.Now()
	conn, err := net.DialTimeout("tcp", "8.8.8.8:53", 3*time.Second)
	if err == nil {
		latency = time.Since(start)
		conn.Close()
	} else {
		latency = 100 * time.Millisecond // Default fallback
	}

	// Estimate network speed based on interface statistics
	// This is a rough estimate - actual speed would require bandwidth testing
	interfaces, err := psnet.Interfaces()
	if err == nil {
		for _, iface := range interfaces {
			// Look for active network interface (not loopback)
			// Check if it's loopback by name or flags
			isLoopback := strings.Contains(strings.ToLower(iface.Name), "loopback") ||
				strings.Contains(strings.ToLower(iface.Name), "lo") ||
				len(iface.Flags) == 0

			if !isLoopback && len(iface.Addrs) > 0 {
				// Estimate based on interface type (rough heuristic)
				// Ethernet/Gigabit: ~100-1000 Mbps
				// WiFi: ~10-100 Mbps
				// Mobile: ~1-10 Mbps
				if strings.Contains(strings.ToLower(iface.Name), "ethernet") ||
					strings.Contains(strings.ToLower(iface.Name), "eth") {
					speedMbps = 100.0 // Conservative estimate for Ethernet
				} else if strings.Contains(strings.ToLower(iface.Name), "wifi") ||
					strings.Contains(strings.ToLower(iface.Name), "wlan") ||
					strings.Contains(strings.ToLower(iface.Name), "wireless") {
					speedMbps = 20.0 // Conservative estimate for WiFi
				} else {
					speedMbps = 10.0 // Default conservative estimate
				}
				break
			}
		}
	}

	// If no interface found, use conservative defaults
	if speedMbps == 0 {
		speedMbps = 10.0 // Default to 10 Mbps
	}

	// Adjust based on latency (lower latency = potentially faster connection)
	if latency < 20*time.Millisecond {
		speedMbps *= 1.5 // Fast connection
	} else if latency > 200*time.Millisecond {
		speedMbps *= 0.5 // Slow connection
	}

	return speedMbps, latency
}

// calculateOptimalConcurrency calculates optimal concurrency based on CPU and memory
func calculateOptimalConcurrency(perf *SystemPerformance) int {
	// Base concurrency on CPU cores
	baseConcurrency := perf.CPUCores * 2

	// Adjust based on available memory (each goroutine uses ~5-10MB)
	// Allow up to 1GB for goroutines (conservative)
	maxMemoryBased := int(perf.AvailableMemoryMB / 10) // ~10MB per goroutine
	if maxMemoryBased < baseConcurrency {
		baseConcurrency = maxMemoryBased
	}

	// Adjust based on CPU usage (if CPU is busy, reduce concurrency)
	if perf.CPUUsage > 80 {
		baseConcurrency = int(float64(baseConcurrency) * 0.7) // Reduce by 30%
	} else if perf.CPUUsage < 20 {
		baseConcurrency = int(float64(baseConcurrency) * 1.3) // Increase by 30%
	}

	// Ensure reasonable bounds
	if baseConcurrency < 5 {
		baseConcurrency = 5 // Minimum
	}
	if baseConcurrency > 200 {
		baseConcurrency = 200 // Maximum (safety limit)
	}

	return baseConcurrency
}

// calculateOptimalRateLimit calculates optimal rate limit based on network speed
func calculateOptimalRateLimit(perf *SystemPerformance) float64 {
	// Base rate on network speed (requests per second)
	// Assume each request is ~10KB, so 1 Mbps = ~12 requests/sec
	baseRate := perf.NetworkSpeed * 12

	// Adjust based on latency (higher latency = lower rate)
	if perf.NetworkLatency > 200*time.Millisecond {
		baseRate *= 0.5 // Slow connection, reduce rate
	} else if perf.NetworkLatency < 50*time.Millisecond {
		baseRate *= 1.5 // Fast connection, increase rate
	}

	// Ensure reasonable bounds
	if baseRate < 10 {
		baseRate = 10 // Minimum 10 req/s
	}
	if baseRate > 200 {
		baseRate = 200 // Maximum 200 req/s (safety limit)
	}

	return baseRate
}

// calculateOptimalTimeout calculates optimal timeout based on network latency
func calculateOptimalTimeout(perf *SystemPerformance) int {
	// Base timeout: 3x the latency + buffer
	baseTimeout := int(perf.NetworkLatency.Seconds() * 3)
	baseTimeout += 5 // Add 5 second buffer

	// Adjust based on network speed (slower = longer timeout)
	if perf.NetworkSpeed < 5 {
		baseTimeout += 10 // Slow connection needs more time
	}

	// Ensure reasonable bounds
	if baseTimeout < 10 {
		baseTimeout = 10 // Minimum 10 seconds
	}
	if baseTimeout > 60 {
		baseTimeout = 60 // Maximum 60 seconds
	}

	return baseTimeout
}

// calculateOptimalMaxParallelRequests calculates optimal max parallel requests
func calculateOptimalMaxParallelRequests(perf *SystemPerformance) int {
	// Base on concurrency * 2 (allow some headroom)
	base := calculateOptimalConcurrency(perf) * 2

	// Adjust based on available memory
	maxMemoryBased := int(perf.AvailableMemoryMB / 5) // ~5MB per request
	if maxMemoryBased < base {
		base = maxMemoryBased
	}

	// Ensure reasonable bounds
	if base < 10 {
		base = 10
	}
	if base > 500 {
		base = 500
	}

	return base
}

// calculateOptimalBatchSize calculates optimal batch size based on memory
func calculateOptimalBatchSize(perf *SystemPerformance) int {
	// Base batch size on available memory
	// Each domain in batch uses ~1KB, so 100MB = ~100K domains
	// But we'll be conservative: use 1MB per 1000 domains
	baseBatch := int(perf.AvailableMemoryMB / 10) // ~10MB per 100 domains

	// Ensure reasonable bounds
	if baseBatch < 10 {
		baseBatch = 10 // Minimum
	}
	if baseBatch > 200 {
		baseBatch = 200 // Maximum
	}

	return baseBatch
}

// calculateOptimalMinDelay calculates optimal minimum delay
func calculateOptimalMinDelay(perf *SystemPerformance) float64 {
	// Base delay on network latency
	baseDelay := perf.NetworkLatency.Seconds() * 0.5

	// Adjust based on network speed
	if perf.NetworkSpeed < 5 {
		baseDelay *= 2 // Slow connection needs more delay
	}

	// Ensure reasonable bounds
	if baseDelay < 0.01 {
		baseDelay = 0.01 // Minimum 10ms
	}
	if baseDelay > 0.5 {
		baseDelay = 0.5 // Maximum 500ms
	}

	return baseDelay
}

// calculateOptimalMaxDelay calculates optimal maximum delay
func calculateOptimalMaxDelay(perf *SystemPerformance) float64 {
	// Max delay is 3x min delay
	minDelay := calculateOptimalMinDelay(perf)
	maxDelay := minDelay * 3

	// Ensure reasonable bounds
	if maxDelay < 0.05 {
		maxDelay = 0.05 // Minimum 50ms
	}
	if maxDelay > 2.0 {
		maxDelay = 2.0 // Maximum 2 seconds
	}

	return maxDelay
}

// TestNetworkSpeed performs a simple network speed test
func TestNetworkSpeed() (float64, error) {
	// Test download speed by fetching a small file
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, "GET", "http://www.google.com/favicon.ico", nil)
	if err != nil {
		return 0, err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// Read response
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	elapsed := time.Since(start)
	if elapsed.Seconds() == 0 {
		return 0, fmt.Errorf("zero elapsed time")
	}

	// Calculate speed (bytes per second to Mbps)
	bytesPerSecond := float64(len(data)) / elapsed.Seconds()
	megabitsPerSecond := (bytesPerSecond * 8) / (1024 * 1024)

	return megabitsPerSecond, nil
}
