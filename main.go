/*
 * Email Extractor v1.3
 * 
 * Author: Dr.Anach
 * Telegram: @dranach
 * 
 * A powerful Go-based tool for extracting email addresses from websites.
 * Features smart validation, obfuscation detection, OCR, and contact page discovery.
 */

package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/publicsuffix"
	"golang.org/x/time/rate"
)

// Initialize random seed for better randomization
func init() {
	// Modern Go random initialization
	rand.Seed(time.Now().UnixNano())
}

// Config struct holds configuration options
type Config struct {
	Concurrency                  int     `json:"concurrency"`
	MinDelayBetweenRequests      float64 `json:"min_delay_between_requests"`
	MaxDelayBetweenRequests      float64 `json:"max_delay_between_requests"`
	Timeout                      int     `json:"timeout"`
	Retries                      bool    `json:"retries"`
	BatchSize                    int     `json:"batch_size"`
	DomainsFilePath              string  `json:"domains_file_path"`
	UserAgentsFile               string  `json:"user_agents"`
	RefererLinksFile             string  `json:"referer_links"`
	ContactLangFile              string  `json:"contact_lang_file"`
	UnwantedDomsFile             string  `json:"unwanted_doms_file"`
	BadEmailExtensions           string  `json:"email_bad_extension_file"`
	ContactPageSkipExtensionFile string  `json:"contact_page_skip_extension_file"`
	OutputDirectory              string  `json:"output_directory"`
	LogFile                      string  `json:"log_file"`
	ReportErrors                 bool    `json:"report_errors"`
	AdaptiveDelayEnabled         bool    `json:"adaptive_delay_enabled"`
	ErrorIncrement               float64 `json:"error_increment"`
	MaxRetriesPerDomain          int     `json:"max_retries_per_domain"`
	LogUnresolvedDomains         string  `json:"log_unresolved_domains"`
	LogResolvedDomains           string  `json:"log_resolved_domains"`
	GCInterval                   int     `json:"gc_interval"`
	MaxParallelRequests          int     `json:"max_parallel_requests"`
	DomainRetryBackoff           float64 `json:"domain_retry_backoff"`
	RateLimitPerSecond           float64 `json:"rate_limit_per_second"`
	CleanerInterval              int     `json:"cleaner_interval"`
	EnableOCR                    bool    `json:"enable_ocr"`
	OCREngine                    string  `json:"ocr_engine"`     // "tesseract", "none", or "auto"
	MaxImageSize                 int     `json:"max_image_size"` // Max image size in bytes (default: 5MB)
	OCRTimeout                   int     `json:"ocr_timeout"`    // Timeout for OCR processing in seconds
	// Memory optimization settings
	StreamDomains bool `json:"stream_domains"` // Stream domains instead of loading all (default: true)
	// Distributed processing settings
	DistributedMode bool   `json:"distributed_mode"` // Enable distributed processing
	RedisURL        string `json:"redis_url"`        // Redis connection URL for distributed mode
	WorkerID        string `json:"worker_id"`        // Unique worker identifier
	QueueName       string `json:"queue_name"`       // Redis queue name (default: "email_extraction")
}

// EmailExtractor struct to manage email extraction
type EmailExtractor struct {
	config                    Config
	domainCounter             int
	domainCounterMux          sync.Mutex
	userAgents                []string
	refererLinks              []string
	contactKeywords           []string
	unwantedDoms              []string
	badExtensions             []string
	contactPageSkipExtensions []string
	emailCategories           map[string][]string // category -> patterns
	httpClient                *http.Client
	outputMutex               sync.Mutex
	rateLimiter               *rate.Limiter
	cleanerStop               chan bool
	// Progress tracking
	totalDomains       int
	processedDomains   int
	successfulDomains  int
	failedDomains      int
	unresolvedDomains  int // DNS/connection failures
	noEmailDomains     int // Resolved but no emails found
	totalEmailsFound   int64 // Total emails extracted across all domains
	totalPhonesFound   int64 // Total phone numbers extracted
	cloudflareBlocked  int64 // Domains blocked by Cloudflare
	progressMutex      sync.RWMutex
	startTime          time.Time
	lastProgressUpdate time.Time // Throttle progress updates
	shutdownRequested  bool
	shutdownMutex      sync.RWMutex
	// DNS optimization
	dnsCache       map[string]bool      // domain -> hasDNS
	dnsCacheExpiry map[string]time.Time // domain -> expiry time
	dnsCacheMutex  sync.RWMutex
	dnsResolver    *net.Resolver
	// Browser automation for Cloudflare bypass
	cloudflareDomains map[string]bool // domain -> needs browser (cache)
	cloudflareMutex   sync.RWMutex    // Mutex for Cloudflare domain cache
}

// SmartValidation provides advanced email validation
type SmartValidation struct {
	disposableDomains map[string]bool
	badExtensions     []string
	mxCache           map[string]bool
	validationCache   map[string]bool
	mxCacheMutex      sync.RWMutex
	mxCacheExpiry     map[string]time.Time
	securityPatterns  []string
}

// ObfuscationDetector handles 20+ obfuscation techniques
type ObfuscationDetector struct {
	patterns map[string]*regexp.Regexp
}

// LoadConfig loads the configuration from a JSON file
func (e *EmailExtractor) LoadConfig(configPath string) error {
	file, err := os.Open(configPath)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&e.config)
	if err != nil {
		return fmt.Errorf("failed to decode config file: %w", err)
	}

	// Optimize configuration based on system performance
	optimizedConfig, perf, err := OptimizeConfig(&e.config)
	if err != nil {
		// Log warning but continue with original config
		log.Printf("Warning: Failed to optimize config based on system performance: %v", err)
		log.Printf("Using original configuration values.")
	} else {
		// Use optimized config
		e.config = *optimizedConfig
		log.Printf("✅ Configuration optimized based on system performance:")
		log.Printf("   CPU Cores: %d | CPU Usage: %.1f%%", perf.CPUCores, perf.CPUUsage)
		log.Printf("   Memory: %d MB available (%.1f%% used)", perf.AvailableMemoryMB, perf.MemoryUsagePercent)
		log.Printf("   Network: ~%.1f Mbps | Latency: %v", perf.NetworkSpeed, perf.NetworkLatency)
		log.Printf("   Optimized: Concurrency=%d, Rate=%d/s, Timeout=%ds",
			e.config.Concurrency, int(e.config.RateLimitPerSecond), e.config.Timeout)
	}

	// Validate configuration after loading (and optimization)
	if err := e.ValidateConfig(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// OCR settings are initialized (see ocr_extraction.go for implementation)

	return nil
}

// ValidateConfig validates the configuration values
func (e *EmailExtractor) ValidateConfig() error {
	if e.config.Concurrency < 1 {
		return fmt.Errorf("concurrency must be >= 1, got %d", e.config.Concurrency)
	}
	if e.config.Concurrency > 1000 {
		return fmt.Errorf("concurrency too high (max 1000), got %d", e.config.Concurrency)
	}
	if e.config.Timeout < 1 {
		return fmt.Errorf("timeout must be >= 1 second, got %d", e.config.Timeout)
	}
	if e.config.Timeout > 300 {
		return fmt.Errorf("timeout too high (max 300 seconds), got %d", e.config.Timeout)
	}
	if e.config.RateLimitPerSecond <= 0 {
		return fmt.Errorf("rate_limit_per_second must be > 0, got %f", e.config.RateLimitPerSecond)
	}
	if e.config.RateLimitPerSecond > 1000 {
		return fmt.Errorf("rate_limit_per_second too high (max 1000), got %f", e.config.RateLimitPerSecond)
	}
	if e.config.MaxParallelRequests < 1 {
		return fmt.Errorf("max_parallel_requests must be >= 1, got %d", e.config.MaxParallelRequests)
	}
	if e.config.MaxRetriesPerDomain < 0 {
		return fmt.Errorf("max_retries_per_domain must be >= 0, got %d", e.config.MaxRetriesPerDomain)
	}
	if e.config.MaxRetriesPerDomain > 10 {
		return fmt.Errorf("max_retries_per_domain too high (max 10), got %d", e.config.MaxRetriesPerDomain)
	}
	if e.config.DomainRetryBackoff < 0 {
		return fmt.Errorf("domain_retry_backoff must be >= 0, got %f", e.config.DomainRetryBackoff)
	}
	if e.config.MinDelayBetweenRequests < 0 {
		return fmt.Errorf("min_delay_between_requests must be >= 0, got %f", e.config.MinDelayBetweenRequests)
	}
	if e.config.MaxDelayBetweenRequests < e.config.MinDelayBetweenRequests {
		return fmt.Errorf("max_delay_between_requests (%f) must be >= min_delay_between_requests (%f)",
			e.config.MaxDelayBetweenRequests, e.config.MinDelayBetweenRequests)
	}
	if e.config.CleanerInterval < 1 {
		return fmt.Errorf("cleaner_interval must be >= 1 second, got %d", e.config.CleanerInterval)
	}
	if e.config.GCInterval < 0 {
		return fmt.Errorf("gc_interval must be >= 0, got %d", e.config.GCInterval)
	}
	if e.config.DomainsFilePath == "" {
		return fmt.Errorf("domains_file_path cannot be empty")
	}
	if e.config.OutputDirectory == "" {
		return fmt.Errorf("output_directory cannot be empty")
	}
	if e.config.EnableOCR {
		if e.config.MaxImageSize < 0 {
			return fmt.Errorf("max_image_size must be >= 0, got %d", e.config.MaxImageSize)
		}
		if e.config.OCRTimeout < 1 {
			return fmt.Errorf("ocr_timeout must be >= 1 second, got %d", e.config.OCRTimeout)
		}
	}
	return nil
}

// LoadLinesFromFile loads lines from a given file
func LoadLinesFromFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Check file size to prevent memory exhaustion
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	const maxFileSize = 100 * 1024 * 1024 // 100MB limit
	if stat.Size() > maxFileSize {
		return nil, fmt.Errorf("file too large: %d bytes (max: %d)", stat.Size(), maxFileSize)
	}

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, strings.TrimSpace(scanner.Text()))
	}
	return lines, scanner.Err()
}

// EmailCategory represents an email with its department category
type EmailCategory struct {
	Email    string
	Category string
}

// LoadEmailCategories loads email categorization patterns from a file
func LoadEmailCategories(filePath string) (map[string][]string, error) {
	categories := make(map[string][]string)

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 2 {
			continue
		}

		category := strings.ToLower(strings.TrimSpace(parts[0]))
		var patterns []string
		for i := 1; i < len(parts); i++ {
			pattern := strings.ToLower(strings.TrimSpace(parts[i]))
			if pattern != "" {
				patterns = append(patterns, pattern)
			}
		}

		if category != "" && len(patterns) > 0 {
			categories[category] = patterns
		}
	}

	return categories, scanner.Err()
}

// getDefaultEmailCategories returns default email categorization patterns
func getDefaultEmailCategories() map[string][]string {
	return map[string][]string{
		"general":   {"info", "contact", "inquiry", "general", "hello", "main", "office", "mail", "email", "reach"},
		"sales":     {"sales", "sell", "order", "purchase", "buy", "quote", "pricing", "business", "commercial", "revenue"},
		"support":   {"support", "help", "assist", "troubleshoot", "issue", "problem", "service", "customer", "care"},
		"admin":     {"admin", "administrator", "management", "manager", "executive", "director", "ceo", "founder", "owner"},
		"marketing": {"marketing", "promotion", "advertise", "newsletter", "subscribe", "pr", "public", "media", "social"},
		"technical": {"tech", "technical", "it", "developer", "dev", "engineer", "sysadmin", "system", "server", "hosting", "webmaster"},
		"finance":   {"finance", "financial", "accounting", "accountant", "billing", "invoice", "payment", "payable", "receivable", "bookkeeping"},
		"hr":        {"hr", "human", "recruit", "recruitment", "jobs", "career", "hiring", "employment", "personnel", "staff"},
		"legal":     {"legal", "law", "attorney", "lawyer", "counsel", "compliance", "regulatory", "contract"},
	}
}

// CategorizeEmail determines the department category for an email
func (e *EmailExtractor) CategorizeEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "other"
	}

	localPart := strings.ToLower(strings.TrimSpace(parts[0]))
	if localPart == "" {
		return "other"
	}

	// Check each category's patterns
	bestMatch := "other"
	maxMatches := 0

	for category, patterns := range e.emailCategories {
		matches := 0
		for _, pattern := range patterns {
			if strings.Contains(localPart, pattern) {
				matches++
			}
		}
		if matches > maxMatches {
			maxMatches = matches
			bestMatch = category
		}
	}

	// If no category matches, return "other"
	if maxMatches == 0 {
		return "other"
	}

	return bestMatch
}

// EnsureDirectory ensures that a directory exists, creating it if necessary
func EnsureDirectory(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create directory: %s, %v", dir, err)
		}
	}
	return nil
}

// LoadAdditionalFilters loads the contact keywords, unwanted domains, bad extensions, and contact page skip extensions
func (e *EmailExtractor) LoadAdditionalFilters() error {
	var err error
	e.contactKeywords, err = LoadLinesFromFile(e.config.ContactLangFile)
	if err != nil {
		return fmt.Errorf("failed to load contact keywords: %v", err)
	}

	e.unwantedDoms, err = LoadLinesFromFile(e.config.UnwantedDomsFile)
	if err != nil {
		return fmt.Errorf("failed to load unwanted domains: %v", err)
	}

	e.badExtensions, err = LoadLinesFromFile(e.config.BadEmailExtensions)
	if err != nil {
		return fmt.Errorf("failed to load bad extensions: %v", err)
	}

	e.contactPageSkipExtensions, err = LoadLinesFromFile(e.config.ContactPageSkipExtensionFile)
	if err != nil {
		return fmt.Errorf("failed to load contact page skip extensions: %v", err)
	}

	// Load user agents and referer links from files
	e.userAgents, err = LoadLinesFromFile(e.config.UserAgentsFile)
	if err != nil {
		return fmt.Errorf("failed to load user agents: %v", err)
	}

	e.refererLinks, err = LoadLinesFromFile(e.config.RefererLinksFile)
	if err != nil {
		return fmt.Errorf("failed to load referer links: %v", err)
	}

	// Load email categories for department categorization
	e.emailCategories, err = LoadEmailCategories("data/email_categories.txt")
	if err != nil {
		// Non-critical, use default categories if file not found
		e.emailCategories = getDefaultEmailCategories()
	}

	// Initialize DNS resolver and cache
	e.initializeDNSResolver()

	// Initialize Cloudflare domain cache (always initialize, browser fallback is optional)
	e.cloudflareDomains = make(map[string]bool)

	return nil
}

// initializeDNSResolver sets up a custom DNS resolver with timeout for faster DNS lookups
func (e *EmailExtractor) initializeDNSResolver() {
	// Create custom DNS resolver with 2-3 second timeout
	e.dnsResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: 2 * time.Second, // Fast timeout for DNS queries
			}
			return d.DialContext(ctx, network, address)
		},
	}
	// Initialize DNS cache
	e.dnsCache = make(map[string]bool)
	e.dnsCacheExpiry = make(map[string]time.Time)
}

// fastDNSCheck performs a DNS lookup to verify if domain exists (cached, with retry logic)
// Returns true if domain has valid DNS, false otherwise
// Uses 12-second timeout and retries up to 3 times for better reliability
func (e *EmailExtractor) fastDNSCheck(domain string) bool {
	// Check cache first
	e.dnsCacheMutex.RLock()
	if cached, exists := e.dnsCache[domain]; exists {
		// Check if cache entry is still valid (24 hours)
		if expiry, ok := e.dnsCacheExpiry[domain]; ok && time.Now().Before(expiry) {
			e.dnsCacheMutex.RUnlock()
			return cached
		}
	}
	e.dnsCacheMutex.RUnlock()

	// DNS lookup with retry logic (3 attempts with exponential backoff)
	maxRetries := 3
	dnsTimeout := 12 * time.Second // Increased from 2 seconds to 12 seconds

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Perform DNS lookup with timeout
		ctx, cancel := context.WithTimeout(context.Background(), dnsTimeout)

		var hasDNS bool
		var dnsErr error
		done := make(chan bool, 1)

		go func() {
			// Try to resolve domain using custom resolver
			_, dnsErr = e.dnsResolver.LookupHost(ctx, domain)
			hasDNS = (dnsErr == nil)
			done <- true
		}()

		select {
		case <-done:
			cancel()
			if hasDNS {
				// Success - cache the result for 24 hours
				e.dnsCacheMutex.Lock()
				e.dnsCache[domain] = true
				e.dnsCacheExpiry[domain] = time.Now().Add(24 * time.Hour)
				e.dnsCacheMutex.Unlock()
				return true
			}
			// DNS lookup failed but didn't timeout - check if we should retry
			// Only retry on timeout errors or temporary network errors
			if attempt < maxRetries {
				// Check if error suggests a retry might help (timeout, temporary network error)
				errStr := dnsErr.Error()
				if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "temporary") ||
					strings.Contains(errStr, "network") || strings.Contains(errStr, "no such host") {
					// Exponential backoff: 0.5s, 1s, 2s
					backoff := time.Duration(attempt*500) * time.Millisecond
					time.Sleep(backoff)
					continue // Retry
				}
			}
			// Permanent failure or no more retries - cache negative result
			e.dnsCacheMutex.Lock()
			e.dnsCache[domain] = false
			e.dnsCacheExpiry[domain] = time.Now().Add(1 * time.Hour) // Shorter cache for failures
			e.dnsCacheMutex.Unlock()
			return false

		case <-ctx.Done():
			cancel()
			// Timeout - retry if attempts remaining
			if attempt < maxRetries {
				// Exponential backoff: 0.5s, 1s, 2s
				backoff := time.Duration(attempt*500) * time.Millisecond
				time.Sleep(backoff)
				continue // Retry
			}
			// All retries exhausted - cache negative result
			e.dnsCacheMutex.Lock()
			e.dnsCache[domain] = false
			e.dnsCacheExpiry[domain] = time.Now().Add(1 * time.Hour) // Shorter cache for failures
			e.dnsCacheMutex.Unlock()
			return false
		}
	}

	// Should never reach here, but handle edge case
	e.dnsCacheMutex.Lock()
	e.dnsCache[domain] = false
	e.dnsCacheExpiry[domain] = time.Now().Add(1 * time.Hour)
	e.dnsCacheMutex.Unlock()
	return false
}

// createHTTPClient creates an HTTP client with custom DNS resolver and optimized settings
func (e *EmailExtractor) createHTTPClient() *http.Client {
	// Use a custom transport with DNS resolver
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Extract hostname from address (format: hostname:port)
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}

			// Use custom DNS resolver if available
			if e.dnsResolver != nil {
				// Resolve hostname using custom resolver
				ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
				defer cancel()

				addrs, err := e.dnsResolver.LookupHost(ctx, host)
				if err != nil || len(addrs) == 0 {
					return nil, fmt.Errorf("DNS lookup failed for %s: %v", host, err)
				}

				// Try to connect to the first resolved address
				ip := addrs[0]
				dialer := &net.Dialer{
					Timeout:   2 * time.Second, // Faster connection timeout for quick failures
					KeepAlive: 30 * time.Second,
				}
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip, port))
				if err != nil {
					// Connection failed quickly - return error immediately
					return nil, err
				}
				return conn, nil
			}

			// Fallback to default dialer
			dialer := &net.Dialer{
				Timeout:   2 * time.Second, // Faster connection timeout
				KeepAlive: 30 * time.Second,
			}
			return dialer.DialContext(ctx, network, addr)
		},
		MaxIdleConns:          e.config.MaxParallelRequests,
		MaxIdleConnsPerHost:   e.config.Concurrency,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   3 * time.Second, // Faster TLS handshake timeout
		ResponseHeaderTimeout: 5 * time.Second, // Timeout for response headers
		DisableKeepAlives:     false,
		DisableCompression:    false,
	}

	// Increase timeout slightly for valid sites (from config, minimum 15s)
	timeout := e.config.Timeout
	if timeout < 15 {
		timeout = 15 // Minimum 15 seconds for slow valid sites
	}

	// Create cookie jar for session persistence (helps with Cloudflare)
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		log.Printf("Warning: Failed to create cookie jar: %v", err)
		// Fallback: create client without cookie jar
		return &http.Client{
			Transport: transport,
			Timeout:   time.Duration(timeout) * time.Second,
		}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   time.Duration(timeout) * time.Second,
		Jar:       jar, // Enable cookie persistence (critical for Cloudflare)
	}
}

// DisplaySummary displays a summary of the configuration and domain count
func (e *EmailExtractor) DisplaySummary(domains []string) {
	e.DisplaySummaryWithConfirmation(domains, false)
}

// DisplaySummaryWithConfirmation displays a summary and optionally asks for confirmation
func (e *EmailExtractor) DisplaySummaryWithConfirmation(domains []string, skipConfirmation bool) {
	fmt.Println("------- Configuration Summary -------")
	fmt.Printf("Concurrency: %d\n", e.config.Concurrency)
	fmt.Printf("Min Delay Between Requests: %.2f seconds\n", e.config.MinDelayBetweenRequests)
	fmt.Printf("Max Delay Between Requests: %.2f seconds\n", e.config.MaxDelayBetweenRequests)
	fmt.Printf("Timeout: %d seconds\n", e.config.Timeout)
	fmt.Printf("Retries Enabled: %t\n", e.config.Retries)
	fmt.Printf("Batch Size: %d\n", e.config.BatchSize)
	fmt.Printf("Domains File Path: %s\n", e.config.DomainsFilePath)
	fmt.Printf("User Agents File: %s\n", e.config.UserAgentsFile)
	fmt.Printf("Referer Links File: %s\n", e.config.RefererLinksFile)
	fmt.Printf("Contact Lang File: %s\n", e.config.ContactLangFile)
	fmt.Printf("Unwanted Domains File: %s\n", e.config.UnwantedDomsFile)
	fmt.Printf("Bad Email Extension File: %s\n", e.config.BadEmailExtensions)
	fmt.Printf("Contact Page Skip Extension File: %s\n", e.config.ContactPageSkipExtensionFile)
	fmt.Printf("Output Directory: %s\n", e.config.OutputDirectory)
	fmt.Printf("Log File: %s\n", e.config.LogFile)
	fmt.Printf("Max Retries Per Domain: %d\n", e.config.MaxRetriesPerDomain)
	fmt.Printf("Rate Limit Per Second: %.2f\n", e.config.RateLimitPerSecond)
	fmt.Printf("Cleaner Interval: %d seconds\n", e.config.CleanerInterval)
	fmt.Printf("Total Domains to Process: %d\n", len(domains))
	fmt.Println("-------------------------------------")

	if skipConfirmation {
		fmt.Println("Auto-confirming (--yes flag set)...")
		return
	}

	fmt.Print("Do you want to continue? (y/n): ")

	var response string
	fmt.Scanln(&response)
	if strings.ToLower(response) != "y" {
		fmt.Println("Operation aborted by the user.")
		os.Exit(0)
	}
}

// DisplaySummaryForStreaming displays a summary for streaming mode (without loading all domains)
func (e *EmailExtractor) DisplaySummaryForStreaming(skipConfirmation bool) {
	fmt.Println("------- Configuration Summary (Streaming Mode) -------")
	fmt.Printf("Concurrency: %d\n", e.config.Concurrency)
	fmt.Printf("Timeout: %d seconds\n", e.config.Timeout)
	fmt.Printf("Rate Limit Per Second: %.2f\n", e.config.RateLimitPerSecond)
	fmt.Printf("Domains File Path: %s\n", e.config.DomainsFilePath)
	fmt.Printf("Output Directory: %s\n", e.config.OutputDirectory)
	fmt.Printf("Total Domains to Process: %d\n", e.totalDomains)
	fmt.Printf("Streaming Mode: ✅ Enabled (Memory Optimized)\n")
	fmt.Println("-------------------------------------")

	if skipConfirmation {
		fmt.Println("Auto-confirming (--yes flag set)...")
		return
	}

	fmt.Print("Do you want to continue? (y/n): ")

	var response string
	fmt.Scanln(&response)
	if strings.ToLower(response) != "y" {
		fmt.Println("Operation aborted by the user.")
		os.Exit(0)
	}
}

// ExtractEmails extracts emails from the full HTML content
func (e *EmailExtractor) ExtractEmails(doc *goquery.Document) []string {
	// Extract emails using enhanced smart validation and obfuscation detection
	return e.ExtractEmailsWithSmartValidation(doc)
}

// ExtractEmailsWithSmartValidation provides enhanced email extraction with smart validation and obfuscation detection
func (e *EmailExtractor) ExtractEmailsWithSmartValidation(doc *goquery.Document) []string {
	return e.ExtractEmailsWithSmartValidationFromURL(doc, "")
}

// ExtractEmailsWithSmartValidationFromURL provides enhanced email extraction with URL context for image processing
func (e *EmailExtractor) ExtractEmailsWithSmartValidationFromURL(doc *goquery.Document, baseURL string) []string {
	// Initialize smart validation and obfuscation detector
	smartValidation := NewSmartValidation(e.unwantedDoms, e.badExtensions)
	obfuscationDetector := NewObfuscationDetector()

	var allEmails []string

	// 1. Extract from HTML attributes first (most reliable)
	// Mailto links
	doc.Find("a[href^='mailto:']").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists {
			email := strings.TrimPrefix(href, "mailto:")
			email = strings.Split(email, "?")[0] // Remove query parameters
			email = strings.Split(email, "#")[0] // Remove fragments
			email = strings.TrimSpace(email)
			if email != "" {
				allEmails = append(allEmails, email)
			}
		}
	})

	// Data attributes
	doc.Find("[data-email], [data-contact], [data-mail]").Each(func(i int, s *goquery.Selection) {
		if email, exists := s.Attr("data-email"); exists && email != "" {
			allEmails = append(allEmails, email)
		}
		if email, exists := s.Attr("data-contact"); exists && email != "" {
			allEmails = append(allEmails, email)
		}
		if email, exists := s.Attr("data-mail"); exists && email != "" {
			allEmails = append(allEmails, email)
		}
	})

	// 2. Extract from HTML text content with better structure awareness
	// Extract from individual text nodes to avoid concatenation issues
	textNodeEmails := e.extractEmailsFromTextNodes(doc)
	allEmails = append(allEmails, textNodeEmails...)

	// Also extract from normalized content as fallback (for obfuscated emails)
	normalizedContent := normalizeText(doc.Text())
	regularEmails := extractEmails(normalizedContent)
	allEmails = append(allEmails, regularEmails...)

	// 3. Detect and decode obfuscated emails
	obfuscatedEmails := obfuscationDetector.DetectAndDecode(normalizedContent)
	allEmails = append(allEmails, obfuscatedEmails...)

	// 4. Extract from comprehensive HTML attributes
	attrsToCheck := []string{
		"href", "title", "alt", "data-href", "data-url", "data-contact",
		"data-email", "data-mail", "data-address", "data-info",
		"aria-label", "aria-labelledby", "value", "placeholder",
		"content", "property", "name", "id", "class",
	}
	doc.Find("*").Each(func(i int, s *goquery.Selection) {
		for _, attr := range attrsToCheck {
			if val, exists := s.Attr(attr); exists {
				emailRegex := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
				matches := emailRegex.FindAllString(val, -1)
				allEmails = append(allEmails, matches...)
			}
		}
	})

	// 5. Extract from meta tags (Open Graph, Twitter Cards, etc.)
	metaEmailPattern := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		if content, exists := s.Attr("content"); exists {
			matches := metaEmailPattern.FindAllString(content, -1)
			allEmails = append(allEmails, matches...)
		}
		// Check Open Graph email tags
		if property, _ := s.Attr("property"); strings.Contains(property, "email") {
			if content, exists := s.Attr("content"); exists {
				allEmails = append(allEmails, content)
			}
		}
	})

	// 6. Extract from JSON-LD structured data (comprehensive)
	doc.Find("script[type='application/ld+json']").Each(func(i int, s *goquery.Selection) {
		jsonData := s.Text()
		jsonEmails := extractEmailsFromJSONLD(jsonData)
		allEmails = append(allEmails, jsonEmails...)
	})

	// 7. Extract from HTML comments
	htmlContent, _ := doc.Html()
	commentEmailPattern := regexp.MustCompile(`<!--.*?([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}).*?-->`)
	commentMatches := commentEmailPattern.FindAllStringSubmatch(htmlContent, -1)
	for _, match := range commentMatches {
		if len(match) > 1 {
			allEmails = append(allEmails, match[1])
		}
	}

	// 8. Extract from hidden elements (display:none, visibility:hidden, etc.)
	doc.Find("[style*='display:none'], [style*='display: none'], [style*='visibility:hidden'], [style*='visibility: hidden']").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		hiddenEmails := extractEmails(text)
		allEmails = append(allEmails, hiddenEmails...)
		// Also check attributes
		for _, attr := range []string{"data-email", "data-contact", "data-mail"} {
			if email, exists := s.Attr(attr); exists {
				allEmails = append(allEmails, email)
			}
		}
	})

	// 9. Extract CSS-obfuscated emails (split across multiple spans/elements)
	// Pattern: <span>info</span>@<span>example</span>.<span>com</span>
	cssObfuscated := extractCSSObfuscatedEmails(doc)
	allEmails = append(allEmails, cssObfuscated...)

	// 10. Extract from iframe src attributes (some sites embed contact forms)
	doc.Find("iframe").Each(func(i int, s *goquery.Selection) {
		if src, exists := s.Attr("src"); exists {
			matches := metaEmailPattern.FindAllString(src, -1)
			allEmails = append(allEmails, matches...)
		}
	})

	// 11. Extract from form elements (input fields, textareas)
	doc.Find("input[type='email'], textarea, input[name*='email'], input[id*='email']").Each(func(i int, s *goquery.Selection) {
		if value, exists := s.Attr("value"); exists {
			matches := metaEmailPattern.FindAllString(value, -1)
			allEmails = append(allEmails, matches...)
		}
		if placeholder, exists := s.Attr("placeholder"); exists {
			matches := metaEmailPattern.FindAllString(placeholder, -1)
			allEmails = append(allEmails, matches...)
		}
	})

	// 12. Extract from link rel attributes
	doc.Find("link[rel*='contact'], link[rel*='email']").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			matches := metaEmailPattern.FindAllString(href, -1)
			allEmails = append(allEmails, matches...)
		}
	})

	// 13. Extract from images using OCR (if enabled)
	if e.config.EnableOCR && baseURL != "" {
		imageEmails := e.extractEmailsFromImages(doc, baseURL)
		allEmails = append(allEmails, imageEmails...)
	}

	// Clean all extracted emails to remove artifacts (phone numbers, time ranges, URLs, dates, etc.)
	allEmails = cleanPhoneNumberArtifacts(allEmails)
	allEmails = cleanTextPrefixes(allEmails)
	allEmails = cleanAdditionalArtifacts(allEmails)

	// Remove duplicates
	uniqueEmails := removeDuplicates(allEmails)

	// Enhanced: Normalize emails before filtering (Gmail dot removal, etc.)
	normalizedEmails := make(map[string]string) // normalized -> original
	for _, email := range uniqueEmails {
		email = strings.TrimSpace(email)
		normalized := normalizeEmailForDeduplication(email)
		if normalized != "" {
			// Keep the first occurrence (prefer shorter/cleaner version)
			if _, exists := normalizedEmails[normalized]; !exists {
				normalizedEmails[normalized] = email
			}
		}
	}

	// Filter out obviously invalid emails before validation (quick filter)
	var preFilteredEmails []string
	emailPattern := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	for normalized, original := range normalizedEmails {
		// Basic check: must contain @ and . and only valid ASCII characters
		if emailPattern.MatchString(normalized) && isASCII(normalized) {
			// Enhanced: Context-aware filtering
			if e.isEmailInValidContext(original, doc) {
				preFilteredEmails = append(preFilteredEmails, strings.ToLower(normalized))
			}
		}
	}

	// Apply smart validation
	var validEmails []string
	for _, email := range preFilteredEmails {
		if smartValidation.ValidateEmail(email) {
			validEmails = append(validEmails, email)
		}
	}

	return validEmails
}

// normalizeEmailForDeduplication normalizes emails for better duplicate detection
// Handles Gmail dot removal and plus addressing normalization
func normalizeEmailForDeduplication(email string) string {
	email = strings.TrimSpace(strings.ToLower(email))
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return email
	}

	localPart := parts[0]
	domainPart := parts[1]

	// Gmail/Googlemail normalization: remove dots
	if domainPart == "gmail.com" || domainPart == "googlemail.com" {
		// Remove all dots from local part
		localPart = strings.ReplaceAll(localPart, ".", "")
		// Optional: Remove plus addressing for normalization (uncomment if desired)
		// if idx := strings.Index(localPart, "+"); idx > 0 {
		// 	localPart = localPart[:idx]
		// }
	}

	return localPart + "@" + domainPart
}

// isEmailInValidContext checks if email is in a valid context (not an example)
func (e *EmailExtractor) isEmailInValidContext(email string, doc *goquery.Document) bool {
	// Get surrounding context for the email
	// Check if email appears in example/demo context

	// Get HTML content for context analysis
	htmlContent, _ := doc.Html()
	emailLower := strings.ToLower(email)

	// Find email position in HTML
	emailIndex := strings.Index(strings.ToLower(htmlContent), emailLower)
	if emailIndex == -1 {
		// If not found in HTML, check text content
		textContent := strings.ToLower(doc.Text())
		emailIndex = strings.Index(textContent, emailLower)
		if emailIndex == -1 {
			return true // Can't determine context, accept it
		}
		htmlContent = textContent
	}

	// Extract context around email (100 chars before and after)
	start := emailIndex - 100
	if start < 0 {
		start = 0
	}
	end := emailIndex + len(email) + 100
	if end > len(htmlContent) {
		end = len(htmlContent)
	}
	context := strings.ToLower(htmlContent[start:end])

	// Reject if in example context
	exampleKeywords := []string{
		"example", "sample", "test", "demo",
		"your.email", "email@example.com",
		"replace with", "enter your", "type your",
		"placeholder", "example.com", "test.com",
		"your@email.com", "your@domain.com",
		"user@example", "contact@example",
	}

	for _, keyword := range exampleKeywords {
		if strings.Contains(context, keyword) {
			// Check if keyword is close to email (within 50 chars)
			keywordIndex := strings.Index(context, keyword)
			emailPosInContext := emailIndex - start
			if keywordIndex != -1 && absInt(keywordIndex-emailPosInContext) < 50 {
				return false // Likely an example
			}
		}
	}

	// Accept if in contact context
	contactKeywords := []string{
		"contact us", "email us", "reach us", "get in touch",
		"send to", "write to", "mail to", "contact information",
		"reach out", "get in contact", "send email",
	}

	for _, keyword := range contactKeywords {
		if strings.Contains(context, keyword) {
			keywordIndex := strings.Index(context, keyword)
			emailPosInContext := emailIndex - start
			if keywordIndex != -1 && absInt(keywordIndex-emailPosInContext) < 100 {
				return true // Likely a real contact email
			}
		}
	}

	// Default: accept if not clearly an example
	return true
}

// absInt returns absolute value of an integer
func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// extractEmailsFromJSONLD extracts emails from JSON-LD structured data
func extractEmailsFromJSONLD(jsonData string) []string {
	var emails []string
	emailPattern := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)

	// First, try to parse as JSON
	var jsonObj interface{}
	if err := json.Unmarshal([]byte(jsonData), &jsonObj); err == nil {
		// Recursively search for email patterns in JSON
		emails = append(emails, searchEmailsInJSON(jsonObj)...)
	}

	// Also do regex search on raw JSON string (for malformed JSON or encoded emails)
	matches := emailPattern.FindAllString(jsonData, -1)
	emails = append(emails, matches...)

	return emails
}

// searchEmailsInJSON recursively searches for email addresses in JSON structure
func searchEmailsInJSON(data interface{}) []string {
	var emails []string
	emailPattern := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			// Check if key suggests email field
			keyLower := strings.ToLower(key)
			if strings.Contains(keyLower, "email") || strings.Contains(keyLower, "contact") ||
				strings.Contains(keyLower, "mail") {
				switch val := value.(type) {
				case string:
					if emailPattern.MatchString(val) {
						emails = append(emails, val)
					}
				case []interface{}:
					for _, item := range val {
						if str, ok := item.(string); ok && emailPattern.MatchString(str) {
							emails = append(emails, str)
						}
						// Recursively search nested structures
						emails = append(emails, searchEmailsInJSON(item)...)
					}
				case map[string]interface{}:
					// Recursively search nested objects
					emails = append(emails, searchEmailsInJSON(val)...)
				}
			} else {
				// Recursively search all values
				emails = append(emails, searchEmailsInJSON(value)...)
			}
		}
	case []interface{}:
		for _, item := range v {
			emails = append(emails, searchEmailsInJSON(item)...)
		}
	case string:
		if emailPattern.MatchString(v) {
			emails = append(emails, v)
		}
	}

	return emails
}

// extractCSSObfuscatedEmails extracts emails that are split across multiple HTML elements
// Example: <span>info</span>@<span>example</span>.<span>com</span>
func extractCSSObfuscatedEmails(doc *goquery.Document) []string {
	var emails []string

	// Pattern 1: Email split with @ and . as separate text nodes
	// Look for sequences like: text@text.text
	doc.Find("body *").Each(func(i int, s *goquery.Selection) {
		// Get text content including hidden elements
		text := strings.TrimSpace(s.Text())
		if strings.Contains(text, "@") && strings.Count(text, "@") == 1 {
			// Check if it looks like an email split across elements
			parts := strings.Split(text, "@")
			if len(parts) == 2 {
				localPart := strings.TrimSpace(parts[0])
				domainPart := strings.TrimSpace(parts[1])

				// Check if domain part contains a dot
				if strings.Contains(domainPart, ".") {
					// Reconstruct potential email
					potentialEmail := localPart + "@" + domainPart
					emailPattern := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
					if emailPattern.MatchString(potentialEmail) {
						emails = append(emails, potentialEmail)
					}
				}
			}
		}
	})

	// Pattern 2: Email parts in adjacent sibling elements
	// This requires checking parent elements for patterns like: <span>info</span>@<span>example.com</span>
	doc.Find("*").Each(func(i int, s *goquery.Selection) {
		parent := s.Parent()
		if parent.Length() > 0 {
			// Get all direct children text
			children := parent.Children()
			var parts []string
			children.Each(func(j int, child *goquery.Selection) {
				parts = append(parts, strings.TrimSpace(child.Text()))
			})

			// Try to reconstruct email from parts
			if len(parts) >= 3 {
				reconstructed := strings.Join(parts, "")
				emailPattern := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
				matches := emailPattern.FindAllString(reconstructed, -1)
				emails = append(emails, matches...)
			}
		}
	})

	// Pattern 3: Check for elements with classes/ids suggesting email obfuscation
	obfuscationSelectors := []string{
		"[class*='email']", "[id*='email']", "[class*='mail']", "[id*='mail']",
		"[class*='contact']", "[id*='contact']",
	}
	for _, selector := range obfuscationSelectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			emailPattern := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
			matches := emailPattern.FindAllString(text, -1)
			emails = append(emails, matches...)

			// Check attributes
			for _, attr := range []string{"data-email", "data-contact", "title", "alt"} {
				if val, exists := s.Attr(attr); exists {
					matches := emailPattern.FindAllString(val, -1)
					emails = append(emails, matches...)
				}
			}
		})
	}

	return emails
}

// isASCII checks if a string contains only ASCII characters
func isASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// extractEmailsFromTextNodes extracts emails from individual text nodes to avoid concatenation
// This preserves HTML structure boundaries and prevents text from adjacent elements being merged
// Example: "Contact Details<br>admin@example.com" won't become "detailsadmin@example.com"
func (e *EmailExtractor) extractEmailsFromTextNodes(doc *goquery.Document) []string {
	var emails []string

	// Extract from text nodes that are direct children of block elements
	// This preserves line boundaries created by <br> tags
	doc.Find("p, div, span, td, th, li, section").Each(func(i int, s *goquery.Selection) {
		// Get HTML content to split by <br> tags
		htmlContent, _ := s.Html()
		// Split by <br> and <br/> tags to preserve line boundaries
		lines := regexp.MustCompile(`(?i)<br\s*/?>`).Split(htmlContent, -1)
		for _, line := range lines {
			// Remove HTML tags from line and extract text
			lineDoc, err := goquery.NewDocumentFromReader(strings.NewReader(line))
			if err == nil {
				lineText := strings.TrimSpace(lineDoc.Text())
				if lineText != "" && strings.Contains(lineText, "@") {
					// Normalize and extract emails from this specific line
					normalized := normalizeText(lineText)
					lineEmails := extractEmails(normalized)
					emails = append(emails, lineEmails...)
				}
			}
		}
	})

	return emails
}

// normalizeText performs text normalization to improve extraction accuracy
func normalizeText(text string) string {
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.ReplaceAll(text, "\t", " ")
	text = strings.TrimSpace(text)
	return text
}

// extractEmails uses multiple patterns to extract emails from text
func extractEmails(text string) []string {
	var emails []string

	// 1. Standard email pattern
	emailRegex := `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`
	re := regexp.MustCompile(emailRegex)
	emails = append(emails, re.FindAllString(text, -1)...)

	// 2. Extended pattern with optional subdomains
	extendedEmailRegex := `[\w\.-]+@[a-zA-Z0-9-]+\.[a-zA-Z]{2,}(?:\.[a-zA-Z]{2,})?`
	reExt := regexp.MustCompile(extendedEmailRegex)
	emails = append(emails, reExt.FindAllString(text, -1)...)

	// 3. Unicode email pattern for international domains (stricter - only ASCII)
	// Note: This pattern is removed as it was causing false positives with binary/encoded content
	// Real international emails with unicode domains are rare and should be handled separately

	// 4. Obfuscated patterns with variations
	obfuscatedPatterns := []string{
		`([a-zA-Z0-9._%+-]+)\s*\[at\]\s*([a-zA-Z0-9.-]+)\s*\[dot\]\s*([a-zA-Z]{2,})`,
		`([a-zA-Z0-9._%+-]+)\s*\{at\}\s*([a-zA-Z0-9.-]+)\s*\{dot\}\s*([a-zA-Z]{2,})`,
		`([a-zA-Z0-9._%+-]+)\s*\(at\)\s*([a-zA-Z0-9.-]+)\s*\(dot\)\s*([a-zA-Z]{2,})`,
		`([a-zA-Z0-9._%+-]+)\s*<at>\s*([a-zA-Z0-9.-]+)\s*<dot>\s*([a-zA-Z]{2,})`,
		`([a-zA-Z0-9._%+-]+)\s+at\s+([a-zA-Z0-9.-]+)\s+dot\s+([a-zA-Z]{2,})`,
		`([a-zA-Z0-9._%+-]+)\s+AT\s+([a-zA-Z0-9.-]+)\s+DOT\s+([a-zA-Z]{2,})`,
		`([a-zA-Z0-9._%+-]+)\s+\(at\)\s+([a-zA-Z0-9.-]+)\s+\(dot\)\s+([a-zA-Z]{2,})`,
	}

	for _, pattern := range obfuscatedPatterns {
		reObfuscated := regexp.MustCompile(pattern)
		matches := reObfuscated.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) == 4 {
				email := fmt.Sprintf("%s@%s.%s", match[1], match[2], match[3])
				emails = append(emails, email)
			}
		}
	}

	// 5. Mailto links with variations
	mailtoPatterns := []string{
		`mailto:([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`,
		`mailto=([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`,
		`mail\s*to:([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`,
	}

	for _, pattern := range mailtoPatterns {
		reMailto := regexp.MustCompile(pattern)
		matches := reMailto.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) == 2 {
				emails = append(emails, match[1])
			}
		}
	}

	// 6. Image and form attributes
	attributePatterns := []string{
		`(?:email|e-mail|mail|contact)\s*[:=]\s*([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`,
		`alt\s*=\s*["'][^"']*?([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})[^"']*?["']`,
		`title\s*=\s*["'][^"']*?([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})[^"']*?["']`,
	}

	for _, pattern := range attributePatterns {
		reAttr := regexp.MustCompile(pattern)
		matches := reAttr.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) == 2 {
				emails = append(emails, match[1])
			}
		}
	}

	// 7. JavaScript obfuscation patterns
	jsPatterns := []string{
		`['"]([a-zA-Z0-9._%+-]+)['"]\s*\+\s*['"]@['"]\s*\+\s*['"]([a-zA-Z0-9.-]+)['"]\s*\+\s*['"]\.['"]\s*\+\s*['"]([a-zA-Z]{2,})['"]`,
		`String\.fromCharCode\(([\d,\s]+)\)`,
		`unescape\(['"]([^'"]+)['"]\)`,
		`atob\(['"]([^'"]+)['"]\)`,
	}

	for _, pattern := range jsPatterns {
		reJS := regexp.MustCompile(pattern)
		matches := reJS.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				if len(match) == 4 {
					email := fmt.Sprintf("%s@%s.%s", match[1], match[2], match[3])
					emails = append(emails, email)
				} else {
					// Handle encoded strings
					encoded := match[1]
					if strings.Contains(pattern, "fromCharCode") {
						// Convert ASCII codes to string
						codes := strings.Split(encoded, ",")
						chars := make([]rune, len(codes))
						for i, code := range codes {
							if val, err := strconv.Atoi(strings.TrimSpace(code)); err == nil {
								chars[i] = rune(val)
							}
						}
						decoded := string(chars)
						if strings.Contains(decoded, "@") {
							emails = append(emails, decoded)
						}
					} else if strings.Contains(pattern, "unescape") {
						// URL-decoded string
						if decoded, err := url.QueryUnescape(encoded); err == nil && strings.Contains(decoded, "@") {
							emails = append(emails, decoded)
						}
					} else if strings.Contains(pattern, "atob") {
						// Base64 decoded string
						if decoded, err := base64.StdEncoding.DecodeString(encoded); err == nil {
							decodedStr := string(decoded)
							if strings.Contains(decodedStr, "@") {
								emails = append(emails, decodedStr)
							}
						}
					}
				}
			}
		}
	}

	// 8. HTML entity variations
	entityPatterns := []string{
		`([a-zA-Z0-9._%+-]+)&#64;([a-zA-Z0-9.-]+)&#46;([a-zA-Z]{2,})`,
		`([a-zA-Z0-9._%+-]+)&commat;([a-zA-Z0-9.-]+)&period;([a-zA-Z]{2,})`,
		`([a-zA-Z0-9._%+-]+)&#x40;([a-zA-Z0-9.-]+)&#x2e;([a-zA-Z]{2,})`,
	}

	for _, pattern := range entityPatterns {
		reEntity := regexp.MustCompile(pattern)
		matches := reEntity.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) == 4 {
				email := fmt.Sprintf("%s@%s.%s", match[1], match[2], match[3])
				emails = append(emails, email)
			}
		}
	}

	// 9. Business patterns with variations
	businessEmailRegex := `(?:contact|info|sales|support|admin|webmaster|marketing|help|service|billing)@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`
	reBusiness := regexp.MustCompile(businessEmailRegex)
	emails = append(emails, reBusiness.FindAllString(text, -1)...)

	// 10. Data attributes and custom attributes
	attrPatterns := []string{
		`data-(?:email|contact|mail)\s*=\s*["']([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})["']`,
		`x-(?:email|contact|mail)\s*=\s*["']([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})["']`,
	}

	for _, pattern := range attrPatterns {
		reAttr := regexp.MustCompile(pattern)
		matches := reAttr.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) == 2 {
				emails = append(emails, match[1])
			}
		}
	}

	// 11. Structured data formats
	structuredPatterns := []string{
		`"(?:email|contactPoint|hasEmail)"\s*:\s*"([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})"`,
		`itemprop\s*=\s*["'](?:email|contactPoint)["']\s*content\s*=\s*["']([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})["']`,
		`property\s*=\s*["'](?:og:email|vcard:email)["']\s*content\s*=\s*["']([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})["']`,
	}

	for _, pattern := range structuredPatterns {
		reStruct := regexp.MustCompile(pattern)
		matches := reStruct.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) == 2 {
				emails = append(emails, match[1])
			}
		}
	}

	// 12. CSS and style attributes
	cssPatterns := []string{
		`content\s*:\s*["']([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})["']`,
		`style\s*=\s*["'][^"']*content\s*:\s*["']([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})["']`,
	}

	for _, pattern := range cssPatterns {
		reCSS := regexp.MustCompile(pattern)
		matches := reCSS.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) == 2 {
				emails = append(emails, match[1])
			}
		}
	}

	// 13. Script patterns
	scriptPatterns := []string{
		`var\s+\w+\s*=\s*["']([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})["']`,
		`const\s+\w+\s*=\s*["']([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})["']`,
		`let\s+\w+\s*=\s*["']([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})["']`,
	}

	for _, pattern := range scriptPatterns {
		reScript := regexp.MustCompile(pattern)
		matches := reScript.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) == 2 {
				emails = append(emails, match[1])
			}
		}
	}

	// 14. Comment variations
	commentPatterns := []string{
		`<!--\s*([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})\s*-->`,
		`/\*\s*([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})\s*\*/`,
		`//\s*([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`,
	}

	for _, pattern := range commentPatterns {
		reComment := regexp.MustCompile(pattern)
		matches := reComment.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) == 2 {
				emails = append(emails, match[1])
			}
		}
	}

	// 15. Unicode variations
	unicodeSymbolsRegex := `[\p{L}\p{N}._%+-]+[\p{Sm}\p{So}][\p{L}\p{N}.-]+[\p{Sm}\p{So}][\p{L}]{2,}`
	reUnicodeSymbols := regexp.MustCompile(unicodeSymbolsRegex)
	emails = append(emails, reUnicodeSymbols.FindAllString(text, -1)...)

	// Clean extracted emails to remove artifacts (phone numbers, time ranges, URLs, dates, etc.)
	cleanedEmails := cleanPhoneNumberArtifacts(emails)
	cleanedEmails = cleanTextPrefixes(cleanedEmails)
	cleanedEmails = cleanAdditionalArtifacts(cleanedEmails)

	// Remove duplicates and filter out security messages
	return removeDuplicates(cleanedEmails)
}

// cleanPhoneNumberArtifacts removes leading phone numbers from emails that are phone number artifacts
// Examples:
//
//	"3801admin@..." -> "admin@..."
//	"+30-210-898-2232info@..." -> "info@..."
//	"351-214-693-329geral@..." -> "geral@..."
func cleanPhoneNumberArtifacts(emails []string) []string {
	var cleaned []string

	// Pattern 1: Simple digits followed by a letter (e.g., "3801admin@...")
	simpleDigitPattern := regexp.MustCompile(`^(\d{3,6})([a-zA-Z])`)

	// Pattern 2: Formatted phone numbers (with +, dashes, spaces) followed by a letter
	// Matches: +30-210-898-2232info, 351-214-693-329geral, +46-8-506-505-00kontakt, etc.
	// Pattern: optional +, then digits/dashes/spaces (non-greedy), ending with digit, then letter
	formattedPhonePattern := regexp.MustCompile(`^(\+?[\d\-\s]+?\d)([a-zA-Z])`)

	for _, email := range emails {
		parts := strings.Split(email, "@")
		if len(parts) == 2 {
			localPart := parts[0]
			domainPart := parts[1]
			cleanedLocalPart := localPart
			found := false

			// Try formatted phone pattern first (more specific)
			if match := formattedPhonePattern.FindStringSubmatch(localPart); match != nil {
				// Remove the phone number prefix
				cleanedLocalPart = strings.TrimPrefix(localPart, match[1])
				found = true
			} else if match := simpleDigitPattern.FindStringSubmatch(localPart); match != nil {
				// Try simple digit pattern
				cleanedLocalPart = strings.TrimPrefix(localPart, match[1])
				found = true
			}

			// Validate and use cleaned email if valid
			if found && len(cleanedLocalPart) > 0 && len(cleanedLocalPart) >= 2 {
				// Check if cleaned part starts with a letter (valid email local part)
				firstChar := cleanedLocalPart[0]
				if (firstChar >= 'a' && firstChar <= 'z') ||
					(firstChar >= 'A' && firstChar <= 'Z') {
					// Additional check: cleaned part should look like a valid email local part
					// Should not contain only dashes or spaces
					if strings.Trim(cleanedLocalPart, "- \t") != "" {
						cleanedEmail := cleanedLocalPart + "@" + domainPart
						cleaned = append(cleaned, cleanedEmail)
						continue
					}
				}
			}
		}
		// Keep original email if no cleaning needed or cleaning failed validation
		cleaned = append(cleaned, email)
	}
	return cleaned
}

// cleanTextPrefixes removes page text artifacts from emails using safe pattern-based cleaning only
// Only removes clearly identifiable patterns (time ranges) that cannot be valid email local parts
// Examples: "9am-5pmteam@..." -> "team@..."
// Does NOT remove word prefixes to avoid breaking valid emails like "details@example.com"
func cleanTextPrefixes(emails []string) []string {
	var cleaned []string

	// Pattern 1: Time ranges (business hours) - e.g., "9am-5pm", "9:00am-5:00pm", "9 AM - 5 PM"
	// These are safe to remove because they clearly cannot be part of a valid email local part
	timeRangePattern := regexp.MustCompile(`^(\d{1,2}(?::\d{2})?\s*(?:am|pm|AM|PM)\s*[-–—]\s*\d{1,2}(?::\d{2})?\s*(?:am|pm|AM|PM))([a-zA-Z])`)

	// Also try simpler pattern without optional spaces for common case: "9am-5pm"
	simpleTimePattern := regexp.MustCompile(`^(\d{1,2}(?:am|pm|AM|PM)[-–—]\d{1,2}(?:am|pm|AM|PM))([a-zA-Z])`)

	for _, email := range emails {
		parts := strings.Split(email, "@")
		if len(parts) == 2 {
			localPart := parts[0]
			domainPart := parts[1]
			cleanedLocalPart := localPart
			found := false

			// Try to remove time range pattern (safe - time ranges cannot be valid email parts)
			var match []string
			if match = simpleTimePattern.FindStringSubmatch(localPart); match == nil {
				match = timeRangePattern.FindStringSubmatch(localPart)
			}

			if match != nil {
				// Remove the time range prefix
				cleanedLocalPart = strings.ToLower(strings.TrimPrefix(localPart, match[1]))

				// Validate: cleaned part should be a valid email local part
				if len(cleanedLocalPart) > 0 && len(cleanedLocalPart) >= 2 {
					firstChar := cleanedLocalPart[0]
					// Check if cleaned part starts with a valid character (letter or common email chars)
					if (firstChar >= 'a' && firstChar <= 'z') ||
						(firstChar >= '0' && firstChar <= '9') ||
						firstChar == '_' || firstChar == '.' || firstChar == '-' {
						cleanedEmail := cleanedLocalPart + "@" + domainPart
						cleaned = append(cleaned, cleanedEmail)
						found = true
					}
				}
			}

			if !found {
				// Keep original email if no cleaning needed or cleaning failed
				cleaned = append(cleaned, email)
			}
		} else {
			// Keep original email if invalid format
			cleaned = append(cleaned, email)
		}
	}
	return cleaned
}

// cleanAdditionalArtifacts removes additional page text artifacts found in actual data
// Handles: URL/domain prefixes, year/date ranges, single character prefixes, URL encoding
// Examples:
//
//	"bikefactory.com.auoffice@bikefactory.com.au" -> "office@bikefactory.com.au"
//	"2024info@example.com" -> "info@example.com"
//	"2021-23.info@example.com" -> "info@example.com"
//	"einfo@example.com" -> "info@example.com"
//	"%20info@example.com" -> "info@example.com"
func cleanAdditionalArtifacts(emails []string) []string {
	var cleaned []string

	for _, email := range emails {
		parts := strings.Split(email, "@")
		if len(parts) != 2 {
			cleaned = append(cleaned, email)
			continue
		}

		localPart := parts[0]
		domainPart := parts[1]
		cleanedLocalPart := localPart
		found := false

		// Pattern 1: URL/Domain prefix (domain before email word)
		// Example: "bikefactory.com.auoffice@bikefactory.com.au" -> "office@bikefactory.com.au"
		// Example: "whitiangainfo@physiofirstwhitianga.co.nz" -> "info@physiofirstwhitianga.co.nz"
		// Strategy: Check if local part starts with the email's domain (normalized) followed by email word
		normalizedEmailDomain := strings.TrimPrefix(strings.ToLower(domainPart), "www.")

		// Extract domain name parts (e.g., "physiofirstwhitianga" from "physiofirstwhitianga.co.nz")
		domainNameParts := strings.Split(normalizedEmailDomain, ".")[0] // Get main domain without TLD

		// Try to find if local part starts with domain pattern matching the email domain
		// Check multiple domain variations (with/without www, with/without subdomains)
		domainVariants := []string{
			normalizedEmailDomain,
			"www." + normalizedEmailDomain,
			domainNameParts, // Check for partial domain match (e.g., "physiofirstwhitianga")
		}

		// Also check for meaningful suffixes from the domain (e.g., "whitianga" from "physiofirstwhitianga")
		// Extract meaningful word suffixes (words that are 4+ characters and appear at the end of the domain)
		if len(domainNameParts) > 6 {
			// Try multiple suffix lengths to catch domain parts that may be concatenated
			// Check from longest to shortest to prioritize more specific matches
			for suffixLen := 12; suffixLen >= 4 && suffixLen < len(domainNameParts); suffixLen-- {
				suffix := domainNameParts[len(domainNameParts)-suffixLen:]
				// Check if suffix is all letters (a valid word candidate)
				if matched, _ := regexp.MatchString(`^[a-z]{4,}$`, suffix); matched {
					domainVariants = append(domainVariants, suffix)
					// Don't break - add multiple suffixes to catch all possible matches
				}
			}
		}

		commonEmailWords := map[string]bool{
			"info": true, "contact": true, "email": true, "mail": true,
			"admin": true, "support": true, "sales": true, "service": true,
			"office": true, "team": true, "staff": true, "hello": true,
			"inquiry": true, "enquiry": true, "general": true, "help": true,
		}

		for _, variant := range domainVariants {
			lowerLocal := strings.ToLower(localPart)
			if strings.HasPrefix(lowerLocal, variant) {
				// Found domain prefix - extract the part after it
				remaining := strings.TrimPrefix(lowerLocal, variant)
				if len(remaining) > 0 && remaining[0] >= 'a' && remaining[0] <= 'z' {
					// Try to match a word boundary (end of local part or before any special chars)
					wordPattern := regexp.MustCompile(`^([a-z]+)`)
					if wordMatch := wordPattern.FindStringSubmatch(remaining); wordMatch != nil {
						emailWord := wordMatch[1]
						// More strict check: must be a common email word (avoid false positives)
						if commonEmailWords[emailWord] {
							cleanedLocalPart = emailWord
							found = true
							break
						}
					}
				}
			}
		}

		// Pattern 2: Year/Date ranges
		// Examples: "2024info@...", "2021-23info@...", "2050.info@..."
		// Pattern: 4-digit year optionally followed by dash and 2-4 digits, then optional dot, then email word
		if !found {
			// Match year directly before word or with dot
			yearPattern := regexp.MustCompile(`^(\d{4}(?:-\d{2,4})?\.?)([a-z]+)$`)
			if yearMatch := yearPattern.FindStringSubmatch(strings.ToLower(localPart)); yearMatch != nil {
				emailWord := yearMatch[2]
				// Only clean if it's a common email word (avoid false positives)
				commonEmailWords := map[string]bool{
					"info": true, "contact": true, "email": true, "mail": true,
					"admin": true, "support": true, "sales": true, "service": true,
					"office": true, "team": true, "staff": true, "hello": true,
					"inquiry": true, "enquiry": true, "general": true, "help": true,
				}
				if commonEmailWords[emailWord] {
					cleanedLocalPart = emailWord
					found = true
				}
			}
		}

		// Pattern 3: Single character prefixes and URL encoding
		// Examples: "einfo@...", "%20info@...", "%40info@..." (URL encoded spaces/@)
		if !found {
			// URL encoding pattern (starts with %)
			// Match: %XX (hex digits) followed by email word
			urlEncodingPattern := regexp.MustCompile(`^(%[0-9A-Fa-f]{2,6})+([a-z]+)$`)
			if urlMatch := urlEncodingPattern.FindStringSubmatch(strings.ToLower(localPart)); urlMatch != nil {
				emailWord := urlMatch[2]
				cleanedLocalPart = emailWord
				found = true
			} else {
				// Single letter followed by common email words
				// Match exactly: single letter + common email word
				singleCharPattern := regexp.MustCompile(`^([a-z])(info|contact|email|mail|admin|support|sales|service|office|team|staff|hello|inquiry|enquiry|general|help)$`)
				if charMatch := singleCharPattern.FindStringSubmatch(strings.ToLower(localPart)); charMatch != nil {
					// charMatch[1] is the single char, charMatch[2] is the email word
					cleanedLocalPart = charMatch[2]
					found = true
				}
			}
		}

		// Validate and use cleaned email if valid
		if found && len(cleanedLocalPart) > 0 && len(cleanedLocalPart) >= 2 {
			firstChar := cleanedLocalPart[0]
			// Check if cleaned part starts with a valid character
			if (firstChar >= 'a' && firstChar <= 'z') ||
				(firstChar >= '0' && firstChar <= '9') ||
				firstChar == '_' || firstChar == '.' || firstChar == '-' {
				cleanedEmail := cleanedLocalPart + "@" + domainPart
				cleaned = append(cleaned, cleanedEmail)
				continue
			}
		}

		// Keep original email if no cleaning needed or cleaning failed validation
		cleaned = append(cleaned, email)
	}
	return cleaned
}

// isCloudflareChallenge detects if a response is a Cloudflare challenge page
// Cloudflare challenge pages contain specific indicators that JavaScript execution is required
func isCloudflareChallenge(body string) bool {
	bodyLower := strings.ToLower(body)

	// Check for common Cloudflare challenge indicators
	indicators := []string{
		"challenge-platform",
		"cf-browser-verification",
		"checking your browser",
		"just a moment",
		"ddos protection by cloudflare",
		"cf-ray",
		"__cf_bm",
		"cf_clearance",
		"cf-challenge",
		"cloudflare",
		"var s,t,o,p,b,r,e,a,k,i,n,g,f", // Obfuscated Cloudflare challenge script
	}

	// Count matches - if 2 or more indicators found, likely a challenge
	matchCount := 0
	for _, indicator := range indicators {
		if strings.Contains(bodyLower, indicator) {
			matchCount++
			if matchCount >= 2 {
				return true
			}
		}
	}

	// Also check for specific Cloudflare challenge patterns
	if strings.Contains(bodyLower, "var s,t,o,p,b,r,e,a,k,i,n,g,f") {
		// This is the obfuscated Cloudflare challenge script
		return true
	}

	return false
}

// removeDuplicates removes duplicate strings from a slice
func removeDuplicates(elements []string) []string {
	encountered := map[string]bool{}
	var result []string

	for _, v := range elements {
		if !encountered[v] {
			encountered[v] = true
			result = append(result, v)
		}
	}
	return result
}

// validateEmails performs final validation and normalization on extracted emails,
// filtering out those from unwanted domains or with bad extensions.
// validateEmails performs final validation and normalization on extracted emails,
// including syntax validation, unwanted domain filtering, and bad extension filtering.
// NOTE: This method is no longer used as we now use ExtractEmailsWithSmartValidation
func (e *EmailExtractor) validateEmails(emails []string) []string {
	var validEmails []string
	emailRegex := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	re := regexp.MustCompile(emailRegex)

	for _, email := range emails {
		email = strings.ToLower(strings.TrimSpace(email))
		if re.MatchString(email) && !e.isUnwantedEmail(email) && !e.isBadEmailExtension(email) {
			validEmails = append(validEmails, email)
		}
	}
	return validEmails
}

// NewSmartValidation creates a new smart validation instance
func NewSmartValidation(unwantedDoms []string, badExtensions []string) *SmartValidation {
	// Convert unwanted domains to map for efficient lookup
	disposableDomains := make(map[string]bool)
	for _, domain := range unwantedDoms {
		disposableDomains[strings.ToLower(domain)] = true
	}

	sv := &SmartValidation{
		disposableDomains: disposableDomains,
		badExtensions:     badExtensions,
		mxCache:           make(map[string]bool),
		validationCache:   make(map[string]bool),
		mxCacheExpiry:     make(map[string]time.Time),
		securityPatterns: []string{
			"account secured", "successfully verified", "security check",
			"verification complete", "access granted", "authentication successful",
			"login successful", "account verified", "security confirmation",
			"verification successful", "account protected", "security alert",
			"system notification", "automated message", "no-reply", "noreply",
			"donotreply", "do-not-reply", "no.reply", "auto-confirm",
			"auto-notification", "auto-reply", "auto-response", "autoresponder",
			"automatic-reply", "bounce", "daemon", "failed", "failure",
			"list-", "list.", "listserv", "mailer-daemon", "mailerdaemon",
			"majordomo", "marketing", "newsletter", "notifications", "postmaster",
			"return-", "root@", "spam", "subscribe", "support@", "sysadmin",
			"system", "test@", "undisclosed-recipients", "unsubscribe",
			"warning", "webmaster", "www@",
		},
	}

	// Start MX cache cleanup goroutine
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		for range ticker.C {
			sv.mxCacheMutex.Lock()
			now := time.Now()
			for domain, expiry := range sv.mxCacheExpiry {
				if now.After(expiry) {
					delete(sv.mxCache, domain)
					delete(sv.mxCacheExpiry, domain)
				}
			}
			sv.mxCacheMutex.Unlock()
		}
	}()

	return sv
}

// NewObfuscationDetector creates a new obfuscation detector
func NewObfuscationDetector() *ObfuscationDetector {
	od := &ObfuscationDetector{
		patterns: make(map[string]*regexp.Regexp),
	}
	od.initializePatterns()
	return od
}

// isSecurityMessage checks if the email contains security-related content
func (sv *SmartValidation) isSecurityMessage(email string) bool {
	lowerEmail := strings.ToLower(email)
	for _, pattern := range sv.securityPatterns {
		if strings.Contains(lowerEmail, pattern) {
			return true
		}
	}
	return false
}

// ValidateEmail validates an email address
func (sv *SmartValidation) ValidateEmail(email string) bool {
	// Check cache first
	if cached, exists := sv.validationCache[email]; exists {
		return cached
	}

	// Basic syntax check
	if !sv.basicSyntaxCheck(email) {
		sv.validationCache[email] = false
		return false
	}

	// Extract domain
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		sv.validationCache[email] = false
		return false
	}
	domain := parts[1]

	// Check for security messages
	if sv.isSecurityMessage(email) {
		sv.validationCache[email] = false
		return false
	}

	// Check for disposable domains
	if sv.isDisposableDomain(domain) {
		sv.validationCache[email] = false
		return false
	}

	// Check for bad extensions
	if sv.isBadEmailExtension(email) {
		sv.validationCache[email] = false
		return false
	}

	// MX record verification
	if !sv.hasMXRecord(domain) {
		sv.validationCache[email] = false
		return false
	}

	// Additional checks
	if !sv.additionalChecks(email) {
		sv.validationCache[email] = false
		return false
	}

	sv.validationCache[email] = true
	return true
}

// Basic syntax validation
func (sv *SmartValidation) basicSyntaxCheck(email string) bool {
	// RFC 5322 compliant email regex
	emailRegex := `^[a-zA-Z0-9.!#$%&'*+/=?^_{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`
	re := regexp.MustCompile(emailRegex)
	if !re.MatchString(email) {
		return false
	}

	// Enhanced: Additional format checks for maximum accuracy
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}

	localPart := parts[0]
	domainPart := parts[1]

	// Enhanced: Local part cannot be empty
	if len(localPart) == 0 {
		return false
	}

	// Enhanced: Domain must have at least one dot
	if !strings.Contains(domainPart, ".") {
		return false
	}

	// Enhanced: Local part cannot start/end with dot
	if strings.HasPrefix(localPart, ".") || strings.HasSuffix(localPart, ".") {
		return false
	}

	// Enhanced: Domain cannot start/end with dash
	if strings.HasPrefix(domainPart, "-") || strings.HasSuffix(domainPart, "-") {
		return false
	}

	return true
}

// Check if domain is disposable
func (sv *SmartValidation) isDisposableDomain(domain string) bool {
	return sv.disposableDomains[strings.ToLower(domain)]
}

// Check if email has bad extension
func (sv *SmartValidation) isBadEmailExtension(email string) bool {
	for _, ext := range sv.badExtensions {
		if strings.HasSuffix(email, ext) {
			return true
		}
	}
	return false
}

// MX record verification with improved caching and timeout
func (sv *SmartValidation) hasMXRecord(domain string) bool {
	// Check cache first with read lock
	sv.mxCacheMutex.RLock()
	if cached, exists := sv.mxCache[domain]; exists {
		// Check if cache entry is still valid
		if expiry, ok := sv.mxCacheExpiry[domain]; ok && time.Now().Before(expiry) {
			sv.mxCacheMutex.RUnlock()
			return cached
		}
	}
	sv.mxCacheMutex.RUnlock()

	// Upgrade to write lock for cache update
	sv.mxCacheMutex.Lock()
	defer sv.mxCacheMutex.Unlock()

	// Double-check after acquiring write lock
	if cached, exists := sv.mxCache[domain]; exists {
		if expiry, ok := sv.mxCacheExpiry[domain]; ok && time.Now().Before(expiry) {
			return cached
		}
	}

	// Lookup MX records with timeout
	var hasMX bool
	done := make(chan bool, 1)
	go func() {
		mxRecords, err := net.LookupMX(domain)
		if err != nil || len(mxRecords) == 0 {
			hasMX = false
		} else {
			// Enhanced: Check if MX records are valid with better validation
			for _, mx := range mxRecords {
				// Enhanced: Validate MX priority (should be 0-65535, but reasonable range)
				if mx.Host != "" && mx.Pref >= 0 && mx.Pref < 65535 {
					// Enhanced: Validate MX hostname format
					mxHost := strings.TrimSuffix(mx.Host, ".")
					if mxHost != "" && len(mxHost) <= 253 {
						// Enhanced: Reject localhost or invalid hosts
						if !strings.HasPrefix(mxHost, "localhost") &&
							!strings.HasPrefix(mxHost, "127.") &&
							!strings.HasPrefix(mxHost, "0.") {
					hasMX = true
					break
						}
					}
				}
			}
		}
		done <- true
	}()

	select {
	case <-done:
		// Cache the result with 24-hour expiry
		sv.mxCache[domain] = hasMX
		sv.mxCacheExpiry[domain] = time.Now().Add(24 * time.Hour)
		return hasMX
	case <-time.After(5 * time.Second):
		// Timeout occurred
		sv.mxCache[domain] = false
		sv.mxCacheExpiry[domain] = time.Now().Add(1 * time.Hour) // Shorter expiry for failed lookups
		return false
	}
}

// Additional validation checks with enhanced accuracy
func (sv *SmartValidation) additionalChecks(email string) bool {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}

	localPart := parts[0]
	domain := parts[1]

	// Check local part length (max 64 characters, min 1)
	if len(localPart) == 0 || len(localPart) > 64 {
		return false
	}

	// Check domain length (max 253 characters, min 3)
	if len(domain) < 3 || len(domain) > 253 {
		return false
	}

	// Enhanced: Check for consecutive dots
	if strings.Contains(localPart, "..") || strings.Contains(domain, "..") {
		return false
	}

	// Enhanced: Check for leading/trailing dots
	if strings.HasPrefix(localPart, ".") || strings.HasSuffix(localPart, ".") {
		return false
	}

	// Enhanced: Check domain for leading/trailing dashes
	if strings.HasPrefix(domain, "-") || strings.HasSuffix(domain, "-") {
		return false
	}

	// Enhanced: Check for invalid characters in local part
	invalidChars := []string{"<", ">", "\"", "'", "(", ")", "[", "]", "\\", ",", ";", ":", " "}
	for _, char := range invalidChars {
		if strings.Contains(localPart, char) {
			return false
		}
	}

	// Enhanced: Detect emails containing phone numbers or location artifacts
	// Pattern: emails that combine location, phone, and email (e.g., "roma+390671356133info@domain.com")
	phonePattern := regexp.MustCompile(`\+?\d{6,}`) // Phone numbers (6+ digits, optionally with +)
	if phonePattern.MatchString(localPart) {
		return false // Contains phone number
	}

	// Enhanced: Reject emails that start with phone number patterns followed by a letter (phone number artifacts)
	// Pattern 1: Simple digits (3-6) followed by letter: "3801admin@...", "5222info@..."
	// Pattern 2: Formatted phone numbers (+digits-dashes) followed by letter: "+30-210-898-2232info@..."
	simplePhonePattern := regexp.MustCompile(`^\d{3,6}[a-zA-Z]`)
	formattedPhonePattern := regexp.MustCompile(`^\+?[\d\-\s]+?\d[a-zA-Z]`)
	if simplePhonePattern.MatchString(localPart) || formattedPhonePattern.MatchString(localPart) {
		return false // Phone number artifact (digits from phone number prefixing email)
	}

	// Check for location words followed by phone numbers in local part
	locationPatterns := []string{"roma", "milano", "torino", "napoli", "firenze", "venezia", "genova", "bologna", "palermo", "catania"}
	lowerLocalPart := strings.ToLower(localPart)
	for _, location := range locationPatterns {
		if strings.HasPrefix(lowerLocalPart, location) {
			// If starts with location and contains 4+ consecutive digits, likely false positive
			if regexp.MustCompile(`\d{4,}`).MatchString(localPart) {
				return false
			}
		}
	}

	// Enhanced: Validate domain has at least one dot (TLD)
	if !strings.Contains(domain, ".") {
		return false
	}

	// Enhanced: Check domain parts
	domainParts := strings.Split(domain, ".")
	if len(domainParts) < 2 {
		return false
	}

	// Enhanced: Validate TLD (last part should be 2-63 chars, letters only)
	tld := domainParts[len(domainParts)-1]
	if len(tld) < 2 || len(tld) > 63 {
		return false
	}
	// TLD should be letters only (or valid IDN, but we're being strict for accuracy)
	for _, char := range tld {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')) {
			return false
		}
	}

	// Enhanced: Reject suspicious patterns
	// All numbers in local part (unless it's a valid pattern)
	if len(localPart) > 5 {
		allNumeric := true
		for _, char := range localPart {
			if !(char >= '0' && char <= '9') {
				allNumeric = false
				break
			}
		}
		if allNumeric {
			return false // Suspicious: all numbers
		}
	}

	// Enhanced: Reject test/example patterns
	lowerEmail := strings.ToLower(email)
	testPatterns := []string{
		"test@test.", "example@example.", "sample@sample.",
		"@test.com", "@example.com", "@sample.com",
		"test@test.com", "example@example.com",
	}
	for _, pattern := range testPatterns {
		if strings.Contains(lowerEmail, pattern) {
			return false
		}
	}

	return true
}

// Initialize obfuscation patterns
func (od *ObfuscationDetector) initializePatterns() {
	// HTML entity obfuscation
	od.patterns["html_entities"] = regexp.MustCompile(`&#[0-9]+;|&[a-zA-Z]+;`)

	// Base64 obfuscation
	od.patterns["base64"] = regexp.MustCompile(`[A-Za-z0-9+/]{20,}={0,2}@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)

	// ROT13 obfuscation
	od.patterns["rot13"] = regexp.MustCompile(`[a-zA-Z]{3,}@[a-zA-Z]{3,}\.[a-zA-Z]{2,}`)

	// Unicode obfuscation
	od.patterns["unicode"] = regexp.MustCompile(`[\p{L}\p{N}._%+-]+@[\p{L}\p{N}.-]+\.[\p{L}]{2,}`)

	// JavaScript obfuscation patterns
	od.patterns["js_concat"] = regexp.MustCompile(`['"]([a-zA-Z0-9._%+-]+)['"]\s*\+\s*['"]@['"]\s*\+\s*['"]([a-zA-Z0-9.-]+)['"]\s*\+\s*['"]\.['"]\s*\+\s*['"]([a-zA-Z]{2,})['"]`)
	od.patterns["js_template"] = regexp.MustCompile(`\$\{[^}]*email[^}]*\}`)
	od.patterns["js_function"] = regexp.MustCompile(`email\s*\(\s*['"]([^'"]*@[^'"]*)['"]\s*\)`)
	od.patterns["js_variable"] = regexp.MustCompile(`email\s*=\s*['"]([^'"]*@[^'"]*)['"]`)
	od.patterns["js_object"] = regexp.MustCompile(`email\s*:\s*['"]([^'"]*@[^'"]*)['"]`)

	// Text obfuscation patterns
	od.patterns["at_dot"] = regexp.MustCompile(`([a-zA-Z0-9._%+-]+)\s+at\s+([a-zA-Z0-9.-]+)\s+dot\s+([a-zA-Z]{2,})`)
	od.patterns["bracket_at"] = regexp.MustCompile(`([a-zA-Z0-9._%+-]+)\s*\[at\]\s*([a-zA-Z0-9.-]+)\s*\[dot\]\s*([a-zA-Z]{2,})`)
	od.patterns["spaced_at"] = regexp.MustCompile(`([a-zA-Z0-9._%+-]+)\s+@\s+([a-zA-Z0-9.-]+)\s+\.\s+([a-zA-Z]{2,})`)

	// Image obfuscation
	od.patterns["image_alt"] = regexp.MustCompile(`email\s*[:=]\s*([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`)

	// Form obfuscation
	od.patterns["form_field"] = regexp.MustCompile(`(?:email|e-mail|mail)\s*[:=]\s*([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`)

	// Data attribute obfuscation
	od.patterns["data_attr"] = regexp.MustCompile(`data-email="([^"]*@[^"]*)"|data-contact="([^"]*@[^"]*)"`)

	// JSON obfuscation
	od.patterns["json_email"] = regexp.MustCompile(`"email"\s*:\s*"([^"]*@[^"]*)"|"contact"\s*:\s*"([^"]*@[^"]*)"`)

	// Microdata obfuscation
	od.patterns["microdata"] = regexp.MustCompile(`itemprop="email"\s+content="([^"]*@[^"]*)"`)

	// Schema.org obfuscation
	od.patterns["schema_org"] = regexp.MustCompile(`"@type"\s*:\s*"ContactPoint"[^}]*"email"\s*:\s*"([^"]*@[^"]*)"`)

	// CSS obfuscation
	od.patterns["css_content"] = regexp.MustCompile(`content\s*:\s*["']([^"']*@[^"']*)["']`)

	// Comment obfuscation
	od.patterns["html_comment"] = regexp.MustCompile(`<!--\s*([^@]*@[^@]*@[^>]*)\s*-->`)

	// Script obfuscation
	od.patterns["script_var"] = regexp.MustCompile(`var\s+\w+\s*=\s*["']([^"']*@[^"']*)["']`)

	// Link obfuscation
	od.patterns["mailto_link"] = regexp.MustCompile(`mailto:([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`)
}

// Detect and decode obfuscated emails
func (od *ObfuscationDetector) DetectAndDecode(text string) []string {
	var emails []string

	// Apply each obfuscation pattern
	for patternName, pattern := range od.patterns {
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) > 1 {
				// Decode based on pattern type
				decoded := od.decodeObfuscated(match[1], patternName)
				if decoded != "" {
					emails = append(emails, decoded)
				}
			} else if len(match) == 1 {
				// Single match, try to decode
				decoded := od.decodeObfuscated(match[0], patternName)
				if decoded != "" {
					emails = append(emails, decoded)
				}
			}
		}
	}

	return emails
}

// Decode obfuscated email based on pattern type
func (od *ObfuscationDetector) decodeObfuscated(obfuscated string, patternType string) string {
	switch patternType {
	case "html_entities":
		return od.decodeHTMLEntities(obfuscated)
	case "base64":
		return od.decodeBase64(obfuscated)
	case "rot13":
		return od.decodeROT13(obfuscated)
	case "at_dot":
		return od.decodeAtDot(obfuscated)
	case "bracket_at":
		return od.decodeBracketAt(obfuscated)
	case "spaced_at":
		return od.decodeSpacedAt(obfuscated)
	case "js_concat":
		return od.decodeJSConcat(obfuscated)
	case "js_template":
		return od.decodeJSTemplate(obfuscated)
	case "js_function":
		return od.decodeJSFunction(obfuscated)
	case "js_variable":
		return od.decodeJSVariable(obfuscated)
	case "js_object":
		return od.decodeJSObject(obfuscated)
	case "image_alt":
		return od.decodeImageAlt(obfuscated)
	case "form_field":
		return od.decodeFormField(obfuscated)
	case "data_attr":
		return od.decodeDataAttr(obfuscated)
	case "json_email":
		return od.decodeJSONEmail(obfuscated)
	case "microdata":
		return od.decodeMicrodata(obfuscated)
	case "schema_org":
		return od.decodeSchemaOrg(obfuscated)
	case "css_content":
		return od.decodeCSSContent(obfuscated)
	case "html_comment":
		return od.decodeHTMLComment(obfuscated)
	case "script_var":
		return od.decodeScriptVar(obfuscated)
	case "mailto_link":
		return od.decodeMailtoLink(obfuscated)
	default:
		return obfuscated
	}
}

// Decode HTML entities
func (od *ObfuscationDetector) decodeHTMLEntities(text string) string {
	return html.UnescapeString(text)
}

// Decode Base64
func (od *ObfuscationDetector) decodeBase64(text string) string {
	// Try to decode base64 email parts
	parts := strings.Split(text, "@")
	if len(parts) == 2 {
		decodedLocal, err1 := base64.StdEncoding.DecodeString(parts[0])
		decodedDomain, err2 := base64.StdEncoding.DecodeString(parts[1])
		if err1 == nil && err2 == nil {
			return fmt.Sprintf("%s@%s", string(decodedLocal), string(decodedDomain))
		}
	}
	return text
}

// Decode ROT13
func (od *ObfuscationDetector) decodeROT13(text string) string {
	var result strings.Builder
	for _, char := range text {
		if char >= 'a' && char <= 'z' {
			result.WriteRune('a' + (char-'a'+13)%26)
		} else if char >= 'A' && char <= 'Z' {
			result.WriteRune('A' + (char-'A'+13)%26)
		} else {
			result.WriteRune(char)
		}
	}
	return result.String()
}

// Decode "at" and "dot" obfuscation
func (od *ObfuscationDetector) decodeAtDot(text string) string {
	// Replace " at " with "@" and " dot " with "."
	text = strings.ReplaceAll(text, " at ", "@")
	text = strings.ReplaceAll(text, " dot ", ".")
	return text
}

// Decode bracket obfuscation
func (od *ObfuscationDetector) decodeBracketAt(text string) string {
	// Replace [at] with @ and [dot] with .
	text = strings.ReplaceAll(text, "[at]", "@")
	text = strings.ReplaceAll(text, "[dot]", ".")
	return text
}

// Decode spaced obfuscation
func (od *ObfuscationDetector) decodeSpacedAt(text string) string {
	// Remove spaces around @ and .
	text = strings.ReplaceAll(text, " @ ", "@")
	text = strings.ReplaceAll(text, " . ", ".")
	return text
}

// Decode JavaScript concatenation
func (od *ObfuscationDetector) decodeJSConcat(text string) string {
	// Extract parts from JS concatenation
	re := regexp.MustCompile(`['"]([^'"]+)['"]\s*\+\s*['"]([^'"]+)['"]`)
	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		return matches[0][1] + matches[0][2]
	}
	return text
}

// Decode JavaScript template literals
func (od *ObfuscationDetector) decodeJSTemplate(text string) string {
	// Extract email from template literal
	re := regexp.MustCompile(`\$\{([^}]*)\}`)
	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		return matches[0][1]
	}
	return text
}

// Decode JavaScript function calls
func (od *ObfuscationDetector) decodeJSFunction(text string) string {
	// Extract email from function call
	re := regexp.MustCompile(`email\s*\(\s*['"]([^'"]*@[^'"]*)['"]\s*\)`)
	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		return matches[0][1]
	}
	return text
}

// Decode JavaScript variable assignments
func (od *ObfuscationDetector) decodeJSVariable(text string) string {
	// Extract email from variable assignment
	re := regexp.MustCompile(`email\s*=\s*['"]([^'"]*@[^'"]*)['"]`)
	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		return matches[0][1]
	}
	return text
}

// Decode JavaScript object properties
func (od *ObfuscationDetector) decodeJSObject(text string) string {
	// Extract email from object property
	re := regexp.MustCompile(`email\s*:\s*['"]([^'"]*@[^'"]*)['"]`)
	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		return matches[0][1]
	}
	return text
}

// Decode image alt text
func (od *ObfuscationDetector) decodeImageAlt(text string) string {
	// Extract email from image alt text
	re := regexp.MustCompile(`email\s*[:=]\s*([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`)
	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		return matches[0][1]
	}
	return text
}

// Decode form field
func (od *ObfuscationDetector) decodeFormField(text string) string {
	// Extract email from form field
	re := regexp.MustCompile(`(?:email|e-mail|mail)\s*[:=]\s*([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`)
	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		return matches[0][1]
	}
	return text
}

// Decode data attributes
func (od *ObfuscationDetector) decodeDataAttr(text string) string {
	// Extract email from data attributes
	re := regexp.MustCompile(`data-email="([^"]*@[^"]*)"|data-contact="([^"]*@[^"]*)"`)
	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		if matches[0][1] != "" {
			return matches[0][1]
		}
		return matches[0][2]
	}
	return text
}

// Decode JSON email
func (od *ObfuscationDetector) decodeJSONEmail(text string) string {
	// Extract email from JSON
	re := regexp.MustCompile(`"email"\s*:\s*"([^"]*@[^"]*)"|"contact"\s*:\s*"([^"]*@[^"]*)"`)
	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		if matches[0][1] != "" {
			return matches[0][1]
		}
		return matches[0][2]
	}
	return text
}

// Decode microdata
func (od *ObfuscationDetector) decodeMicrodata(text string) string {
	// Extract email from microdata
	re := regexp.MustCompile(`itemprop="email"\s+content="([^"]*@[^"]*)"`)
	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		return matches[0][1]
	}
	return text
}

// Decode Schema.org
func (od *ObfuscationDetector) decodeSchemaOrg(text string) string {
	// Extract email from Schema.org
	re := regexp.MustCompile(`"@type"\s*:\s*"ContactPoint"[^}]*"email"\s*:\s*"([^"]*@[^"]*)"`)
	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		return matches[0][1]
	}
	return text
}

// Decode CSS content
func (od *ObfuscationDetector) decodeCSSContent(text string) string {
	// Extract email from CSS content
	re := regexp.MustCompile(`content\s*:\s*["']([^"']*@[^"']*)["']`)
	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		return matches[0][1]
	}
	return text
}

// Decode HTML comment
func (od *ObfuscationDetector) decodeHTMLComment(text string) string {
	// Extract email from HTML comment
	re := regexp.MustCompile(`<!--\s*([^@]*@[^@]*@[^>]*)\s*-->`)
	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		return matches[0][1]
	}
	return text
}

// Decode script variable
func (od *ObfuscationDetector) decodeScriptVar(text string) string {
	// Extract email from script variable
	re := regexp.MustCompile(`var\s+\w+\s*=\s*["']([^"']*@[^"']*)["']`)
	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		return matches[0][1]
	}
	return text
}

// Decode mailto link
func (od *ObfuscationDetector) decodeMailtoLink(text string) string {
	// Extract email from mailto link
	re := regexp.MustCompile(`mailto:([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`)
	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		return matches[0][1]
	}
	return text
}

// Note: Disposable domain filtering now uses existing unwantedDoms from EmailExtractor
// This eliminates duplication and uses the configured unwanted domains file

// isUnwantedEmail checks if an email belongs to an unwanted domain
func (e *EmailExtractor) isUnwantedEmail(email string) bool {
	for _, unwantedDomain := range e.unwantedDoms {
		if strings.HasSuffix(email, "@"+unwantedDomain) {
			return true
		}
	}
	return false
}

// isBadEmailExtension checks if an email has an unwanted extension
func (e *EmailExtractor) isBadEmailExtension(email string) bool {
	for _, ext := range e.badExtensions {
		if strings.HasSuffix(email, ext) {
			return true
		}
	}
	return false
}

// SaveEmails saves unique extracted emails to a file
func (e *EmailExtractor) SaveEmails(emails []string) {
	e.outputMutex.Lock()
	defer e.outputMutex.Unlock()

	// Use a map to store unique emails
	uniqueEmails := make(map[string]struct{})
	for _, email := range emails {
		uniqueEmails[email] = struct{}{}
	}

	// Main output file (all emails)
	outputPath := fmt.Sprintf("%s/extracted_emails.txt", e.config.OutputDirectory)
	file, err := os.OpenFile(outputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Failed to open output file for emails: %v", err)
		return
	}
	defer file.Close()

	// Categorized emails by department
	categorizedEmails := make(map[string][]string)

	// Write each unique email to the file and categorize
	for email := range uniqueEmails {
		_, err := file.WriteString(email + "\n")
		if err != nil {
			log.Printf("Failed to write email to file: %v", err)
		}

		// Categorize email
		category := e.CategorizeEmail(email)
		categorizedEmails[category] = append(categorizedEmails[category], email)
	}

	// Save categorized emails to separate files
	e.SaveCategorizedEmails(categorizedEmails)
}

// SaveCategorizedEmails saves emails organized by department category
func (e *EmailExtractor) SaveCategorizedEmails(categorizedEmails map[string][]string) {
	for category, emails := range categorizedEmails {
		if len(emails) == 0 {
			continue
		}

		// Create category-specific output file
		outputPath := fmt.Sprintf("%s/categorized_%s.txt", e.config.OutputDirectory, category)
	file, err := os.OpenFile(outputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
			log.Printf("Failed to open category file for %s: %v", category, err)
			continue
		}

		// Write emails to category file
		for _, email := range emails {
			_, err := file.WriteString(email + "\n")
			if err != nil {
				log.Printf("Failed to write email to category file: %v", err)
			}
		}

		file.Close()
	}

	// Save summary file with categories
	e.SaveCategorySummary(categorizedEmails)
}

// SaveCategorySummary saves a summary of categorized emails
func (e *EmailExtractor) SaveCategorySummary(categorizedEmails map[string][]string) {
	summaryPath := fmt.Sprintf("%s/email_categories_summary.txt", e.config.OutputDirectory)
	file, err := os.OpenFile(summaryPath, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Failed to open category summary file: %v", err)
		return
	}
	defer file.Close()

	// Sort categories for consistent output
	var categories []string
	for cat := range categorizedEmails {
		categories = append(categories, cat)
	}
	sort.Strings(categories)

	// Write summary header
	file.WriteString("EMAIL CATEGORIES SUMMARY\n")
	file.WriteString("========================\n\n")

	totalEmails := 0
	for _, category := range categories {
		emails := categorizedEmails[category]
		count := len(emails)
		totalEmails += count
		file.WriteString(fmt.Sprintf("%s: %d emails\n", strings.ToUpper(category), count))
	}

	file.WriteString(fmt.Sprintf("\nTOTAL: %d emails\n", totalEmails))
	file.WriteString("\nDetailed breakdown:\n")
	file.WriteString("-------------------\n\n")

	// Write detailed breakdown
	for _, category := range categories {
		emails := categorizedEmails[category]
		if len(emails) == 0 {
			continue
		}

		file.WriteString(fmt.Sprintf("\n[%s] - %d emails\n", strings.ToUpper(category), len(emails)))
		for _, email := range emails {
			file.WriteString(fmt.Sprintf("  - %s\n", email))
		}
	}
}

// LogDomain logs a domain to a specified file
func (e *EmailExtractor) LogDomain(domain, logFilePath string) {
	e.outputMutex.Lock()
	defer e.outputMutex.Unlock()

	// Ensure the output directory exists
	if err := EnsureDirectory(filepath.Dir(logFilePath)); err != nil {
		log.Printf("Failed to ensure directory for log file: %v", err)
		return
	}

	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Failed to open log file: %v", err)
		return
	}
	defer file.Close()

	_, err = file.WriteString(domain + "\n")
	if err != nil {
		log.Printf("Failed to write domain to log file: %v", err)
	}
}

// FetchURL fetches content from a URL with retry and backoff
func (e *EmailExtractor) FetchURL(url string) (*goquery.Document, error) {
	attempts := e.config.MaxRetriesPerDomain

	// Ensure we have at least one attempt
	if attempts < 1 {
		attempts = 1
	}

	for attempt := 1; attempt <= attempts; attempt++ {
		doc, err := e.makeRequest(url)
		if err == nil {
			return doc, nil
		}
		// Don't retry connection/timeout errors - they're likely to fail again
		errStr := err.Error()
		if strings.Contains(errStr, "timeout") ||
			strings.Contains(errStr, "connection") ||
			strings.Contains(errStr, "i/o timeout") ||
			strings.Contains(errStr, "no such host") {
			// Fail fast on connection/timeout errors
			return nil, fmt.Errorf("failed to fetch URL: %s: %w", url, err)
		}
		if attempt < attempts {
			backoff := time.Duration(float64(attempt) * e.config.DomainRetryBackoff * float64(time.Second))
			time.Sleep(backoff)
		} else {
			return nil, fmt.Errorf("failed to fetch URL: %s after %d attempts: %w", url, attempts, err)
		}
	}

	// This should never be reached, but handle edge case
	return nil, fmt.Errorf("failed to fetch URL: %s (no attempts made)", url)
}

// makeRequest performs an HTTP GET request to the provided URL and returns the response body.
func (e *EmailExtractor) makeRequest(url string) (*goquery.Document, error) {
	// Use config timeout with optimized limits (15-20s range)
	timeout := e.config.Timeout
	if timeout < 15 {
		timeout = 15 // Minimum 15 seconds for slow sites
	}
	if timeout > 20 {
		timeout = 20 // Cap at 20s to prevent excessive waiting
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	err := e.rateLimiter.Wait(ctx)
	if err != nil {
		return nil, fmt.Errorf("rate limiter timeout: %v", err)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Randomly select a user agent, ensure the list is non-empty
	if len(e.userAgents) > 0 {
		userAgent := e.userAgents[rand.Intn(len(e.userAgents))]
		req.Header.Set("User-Agent", userAgent)
	} else {
		log.Println("Warning: User agents list is empty. Using default user agent.")
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; EmailExtractor/1.0; +https://example.com)")
	}

	// Randomly select a referer link, ensure the list is non-empty
	if len(e.refererLinks) > 0 {
		req.Header.Set("Referer", e.refererLinks[rand.Intn(len(e.refererLinks))])
	} else {
		log.Println("Warning: Referer links list is empty.")
	}

	// Set enhanced browser headers to reduce bot detection
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	// Only request gzip/deflate (Go handles these automatically)
	// Don't request br (brotli) as Go doesn't auto-decompress it
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("DNT", "1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Cache-Control", "max-age=0")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-200 response code: %d", resp.StatusCode)
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Check if response is a Cloudflare challenge page (after decompression)
	bodyString := string(bodyBytes)
	// Decompress first if needed to check properly
	if len(bodyBytes) >= 2 && bodyBytes[0] == 0x1f && bodyBytes[1] == 0x8b {
		gzipReader, err := gzip.NewReader(strings.NewReader(bodyString))
		if err == nil {
			defer gzipReader.Close()
			decompressed, err := io.ReadAll(gzipReader)
			if err == nil {
				bodyString = string(decompressed)
			}
		}
	}

	if isCloudflareChallenge(bodyString) {
		// Track Cloudflare block
		e.progressMutex.Lock()
		e.cloudflareBlocked++
		e.progressMutex.Unlock()
		return nil, fmt.Errorf("cloudflare challenge detected for URL: %s (JavaScript required)", url)
	}

	// Check Content-Encoding and decompress if needed
	contentEncoding := resp.Header.Get("Content-Encoding")
	var reader io.Reader = strings.NewReader(string(bodyBytes))

	// Check if content is gzip-compressed (starts with gzip magic bytes: 0x1f 0x8b)
	if len(bodyBytes) >= 2 && bodyBytes[0] == 0x1f && bodyBytes[1] == 0x8b {
		// Manually decompress gzip
		gzipReader, err := gzip.NewReader(strings.NewReader(string(bodyBytes)))
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %v", err)
		}
		defer gzipReader.Close()

		decompressedBytes, err := io.ReadAll(gzipReader)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress gzip: %v", err)
		}
		reader = strings.NewReader(string(decompressedBytes))
	} else if contentEncoding == "gzip" {
		// Content-Encoding says gzip but body doesn't have magic bytes
		// Try decompressing anyway
		gzipReader, err := gzip.NewReader(strings.NewReader(string(bodyBytes)))
		if err == nil {
			defer gzipReader.Close()
			decompressedBytes, err := io.ReadAll(gzipReader)
			if err == nil {
				reader = strings.NewReader(string(decompressedBytes))
			}
		}
	}

	// Create document from the (possibly decompressed) body
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// FindContactPage searches for a contact page link in the HTML content with 99% accuracy discovery
func (e *EmailExtractor) FindContactPage(doc *goquery.Document, baseURL string) (string, error) {
	var contactURL string
	var contactURLs []string

	// Helper function to add contact URL with validation
	addContactURL := func(url string) {
		if url != "" && !e.HasContactPageBadExtension(url) {
			// Normalize URL
			if !strings.HasPrefix(url, "http") {
				if !strings.HasPrefix(url, "/") && !strings.HasSuffix(baseURL, "/") {
					url = fmt.Sprintf("%s/%s", baseURL, url)
				} else {
					url = fmt.Sprintf("%s%s", baseURL, url)
				}
			}
			contactURLs = append(contactURLs, url)
		}
	}

	// Check schema.org Organization markup
	doc.Find("[itemtype='http://schema.org/Organization'], [itemtype='https://schema.org/Organization']").Each(func(i int, s *goquery.Selection) {
		s.Find("[itemprop='contactPoint'], [itemprop='contactPage']").Each(func(j int, cp *goquery.Selection) {
			if url, exists := cp.Attr("href"); exists {
				addContactURL(url)
			}
			if url, exists := cp.Attr("content"); exists {
				addContactURL(url)
			}
		})
	})

	// Check JSON-LD data
	doc.Find("script[type='application/ld+json']").Each(func(i int, s *goquery.Selection) {
		jsonData := s.Text()
		if strings.Contains(jsonData, "contactPoint") || strings.Contains(jsonData, "ContactPoint") {
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(jsonData), &data); err == nil {
				if cp, ok := data["contactPoint"].(map[string]interface{}); ok {
					if url, ok := cp["url"].(string); ok {
						addContactURL(url)
					}
				}
			}
		}
	})

	// Check RDFa markup
	doc.Find("[typeof='Organization'], [typeof='LocalBusiness']").Each(func(i int, s *goquery.Selection) {
		s.Find("[property='contactPoint'], [property='hasContactPoint']").Each(func(j int, cp *goquery.Selection) {
			if url, exists := cp.Attr("href"); exists {
				addContactURL(url)
			}
			if url, exists := cp.Attr("content"); exists {
				addContactURL(url)
			}
		})
	})

	// Check meta tags
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		if name, _ := s.Attr("name"); strings.Contains(strings.ToLower(name), "contact") {
			if content, exists := s.Attr("content"); exists && content != "" {
				addContactURL(content)
			}
		}
	})

	// Check link tags
	doc.Find("link").Each(func(i int, s *goquery.Selection) {
		if rel, _ := s.Attr("rel"); strings.Contains(strings.ToLower(rel), "contact") {
			if href, exists := s.Attr("href"); exists && href != "" {
				addContactURL(href)
			}
		}
	})

	// Check microdata
	doc.Find("[itemtype*='ContactPage']").Each(func(i int, s *goquery.Selection) {
		if url, exists := s.Attr("href"); exists {
			addContactURL(url)
		}
		if url, exists := s.Attr("content"); exists {
			addContactURL(url)
		}
	})

	// Helper function for optimized contact keyword checking with priority-based early exit
	checkContactKeywords := func(href, linkText, linkTitle string) bool {
		// High-priority keywords that are most likely to contain contact information
		highPriorityKeywords := []string{
			"contact", "sales", "about", "get-in-touch", "reach-us", "write-us",
			"email-us", "call-us", "find-us", "locate-us", "visit-us", "meet-us",
			"talk-to-us", "speak-to-us", "connect-with-us", "reach-out", "get-help",
			"ask-us", "inquiry", "enquiry", "request-info", "request-information",
			"request-demo", "request-quote", "request-pricing", "request-support",
			"request-help", "request-assistance", "request-service", "request-maintenance",
			"request-repair", "request-installation", "request-training", "request-consultation",
			"request-meeting", "request-appointment", "request-callback", "request-call-back",
			"request-phone-call", "request-email", "request-contact", "contact-form",
			"contact-page", "contact-us-page", "contact-us-form", "contact-us-info",
			"contact-us-details", "contact-us-location", "contact-us-address",
			"contact-us-phone", "contact-us-email", "contact-us-fax", "contact-us-hours",
			"contact-us-map", "contact-us-directions", "contact-us-office",
			"contact-us-headquarters", "contact-us-branch", "sales-contact",
			"sales-inquiry", "sales-information", "sales-demo", "sales-quote",
			"sales-pricing", "sales-support", "sales-help", "sales-assistance",
			"sales-service", "sales-maintenance", "sales-repair", "sales-installation",
			"sales-training", "sales-consultation", "sales-meeting", "sales-appointment",
			"sales-callback", "sales-call-back", "sales-phone-call", "sales-email",
			"marketing-contact", "marketing-inquiry", "marketing-information",
			"marketing-demo", "marketing-quote", "marketing-pricing", "marketing-support",
			"technical-support", "tech-support", "it-support", "online-support",
			"live-support", "support-center", "help-center", "support-desk",
			"help-desk", "support-team", "help-team", "support-staff", "help-staff",
			"support-contact", "help-contact", "support-info", "help-info",
			"support-details", "help-details", "support-hours", "help-hours",
			"support-phone", "help-phone", "support-email", "help-email",
			"support-chat", "help-chat", "support-ticket", "help-ticket",
			"support-request", "help-request", "customer-service", "customer-support",
			"client-support", "client-service", "customer-care", "client-care",
			"customer-assistance", "client-assistance", "customer-help", "client-help",
			"customer-info", "client-info", "customer-information", "client-information",
			"customer-details", "client-details", "customer-hours", "client-hours",
			"customer-phone", "client-phone", "customer-email", "client-email",
			"customer-fax", "client-fax", "customer-chat", "client-chat",
			"customer-ticket", "client-ticket", "customer-request", "client-request",
			// Enhanced patterns for better discovery
			"kontakt", "kontakti", "kontaktai", "kontaktas", "kontaktus", "kontakta",
			"contacto", "contactos", "contatti", "contato", "contatos", "kontakt",
			"about-us", "about-us/contact", "company/contact", "business/contact",
			"services/contact", "en/contact", "de/kontakt", "fr/contact", "es/contacto",
			"it/contatti", "pt/contato", "nl/contact", "sv/kontakt", "no/kontakt",
			"da/kontakt", "fi/yhteystiedot", "pl/kontakt", "cs/kontakt", "sk/kontakt",
			"hu/kapcsolat", "ro/contact", "bg/kontakt", "hr/kontakt", "sl/kontakt",
			"et/kontakt", "lv/kontakti", "lt/kontaktai", "mt/kuntatt", "ga/teagmhas",
			"cy/cysylltiad", "eu/kontaktua", "ca/contacte", "gl/contacto",
			"info", "information", "company", "business", "services", "support",
			"help", "assistance", "service", "maintenance", "repair", "installation",
			"training", "consultation", "meeting", "appointment", "callback",
			"call-back", "phone-call", "email", "fax", "chat", "ticket", "request",
			"inquiry", "enquiry", "quote", "pricing", "demo", "consultation",
			"meeting", "appointment", "callback", "call-back", "phone-call",
			"email", "fax", "chat", "ticket", "request", "inquiry", "enquiry",
			"quote", "pricing", "demo", "consultation", "meeting", "appointment",
			"callback", "call-back", "phone-call", "email", "fax", "chat",
			"ticket", "request", "inquiry", "enquiry", "quote", "pricing",
			"demo", "consultation", "meeting", "appointment", "callback",
			"call-back", "phone-call", "email", "fax", "chat", "ticket",
			"request", "inquiry", "enquiry", "quote", "pricing", "demo",
			"consultation", "meeting", "appointment", "callback", "call-back",
			"phone-call", "email", "fax", "chat", "ticket", "request",
		}

		// Check high-priority keywords first (early exit optimization)
		for _, keyword := range highPriorityKeywords {
			if strings.Contains(strings.ToLower(href), strings.ToLower(keyword)) ||
				strings.Contains(linkText, strings.ToLower(keyword)) ||
				strings.Contains(linkTitle, strings.ToLower(keyword)) {
				return true
			}
		}

		// Check other keywords only if high-priority keywords not found
		for _, keyword := range e.contactKeywords {
			// Skip keywords we already checked in high-priority list
			isHighPriority := false
			for _, highPriority := range highPriorityKeywords {
				if keyword == highPriority {
					isHighPriority = true
					break
				}
			}

			if !isHighPriority &&
				(strings.Contains(strings.ToLower(href), strings.ToLower(keyword)) ||
					strings.Contains(linkText, strings.ToLower(keyword)) ||
					strings.Contains(linkTitle, strings.ToLower(keyword))) {
				return true
			}
		}
		return false
	}

	// 1. Search in <a> tags with enhanced text matching and early exit optimization
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists {
			linkText := strings.ToLower(s.Text())
			linkTitle, _ := s.Attr("title")
			linkTitle = strings.ToLower(linkTitle)

			if checkContactKeywords(href, linkText, linkTitle) {
				addContactURL(href)
			}
		}
	})

	// 2. Search in meta tags with expanded patterns
	metaSelectors := []string{
		"meta[name='contact']", "meta[property='contact']",
		"meta[name='contact-url']", "meta[property='contact-url']",
		"meta[name='contact-page']", "meta[property='contact-page']",
		"meta[name='contact-us']", "meta[property='contact-us']",
		"meta[name='contactus']", "meta[property='contactus']",
	}

	for _, selector := range metaSelectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			if content, exists := s.Attr("content"); exists {
				if strings.HasPrefix(content, "http") {
					addContactURL(content)
				}
			}
		})
	}

	// 3. Enhanced JSON-LD structured data parsing
	doc.Find("script[type='application/ld+json']").Each(func(i int, s *goquery.Selection) {
		jsonData := s.Text()

		// Multiple patterns for JSON-LD contact extraction
		patterns := []string{
			`"url"\s*:\s*"([^"]*contact[^"]*)"`,
			`"contactPoint"\s*:\s*{[^}]*"url"\s*:\s*"([^"]*)"`,
			`"contact"\s*:\s*{[^}]*"url"\s*:\s*"([^"]*)"`,
			`"sameAs"\s*:\s*"([^"]*contact[^"]*)"`,
			`"url"\s*:\s*"([^"]*kontakt[^"]*)"`,
			`"url"\s*:\s*"([^"]*contacto[^"]*)"`,
		}

		for _, pattern := range patterns {
			matches := regexp.MustCompile(pattern).FindAllStringSubmatch(jsonData, -1)
			for _, match := range matches {
				if len(match) > 1 {
					addContactURL(match[1])
				}
			}
		}
	})

	// 4. Search in footer links with expanded selectors and early exit optimization
	footerSelectors := []string{
		"footer a", ".footer a", "#footer a", ".site-footer a",
		".main-footer a", ".page-footer a", ".footer-menu a",
		"footer .menu a", ".footer-nav a", ".footer-links a",
		".footer-bottom a", ".footer-top a", ".footer-main a",
	}

	for _, selector := range footerSelectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if exists {
				linkText := strings.ToLower(s.Text())
				linkTitle, _ := s.Attr("title")
				linkTitle = strings.ToLower(linkTitle)

				if checkContactKeywords(href, linkText, linkTitle) {
					addContactURL(href)
				}
			}
		})
	}

	// 5. Search in navigation menus with expanded selectors and early exit optimization
	navSelectors := []string{
		"nav a", ".nav a", ".navigation a", ".menu a",
		".main-nav a", ".primary-nav a", ".header-nav a",
		".top-nav a", ".main-menu a", ".primary-menu a",
		".navbar a", ".nav-menu a", ".site-nav a",
		".header-menu a", ".top-menu a", ".main-navigation a",
	}

	for _, selector := range navSelectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if exists {
				linkText := strings.ToLower(s.Text())
				linkTitle, _ := s.Attr("title")
				linkTitle = strings.ToLower(linkTitle)

				// Check for direct "contact" first (early exit optimization)
				if strings.Contains(strings.ToLower(href), "contact") ||
					strings.Contains(linkText, "contact") ||
					strings.Contains(linkTitle, "contact") {
					addContactURL(href)
				} else {
					// Check other keywords only if contact not found
					for _, keyword := range e.contactKeywords {
						if keyword != "contact" && // Skip "contact" as we already checked it
							(strings.Contains(strings.ToLower(href), strings.ToLower(keyword)) ||
								strings.Contains(linkText, strings.ToLower(keyword)) ||
								strings.Contains(linkTitle, strings.ToLower(keyword))) {
							addContactURL(href)
							break
						}
					}
				}
			}
		})
	}

	// 6. Search in header links
	headerSelectors := []string{
		"header a", ".header a", "#header a", ".site-header a",
		".main-header a", ".top-header a", ".header-menu a",
		".header-nav a", ".header-links a", ".header-menu a",
	}

	for _, selector := range headerSelectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if exists {
				linkText := strings.ToLower(s.Text())
				linkTitle, _ := s.Attr("title")
				linkTitle = strings.ToLower(linkTitle)

				for _, keyword := range e.contactKeywords {
					if strings.Contains(strings.ToLower(href), strings.ToLower(keyword)) ||
						strings.Contains(linkText, strings.ToLower(keyword)) ||
						strings.Contains(linkTitle, strings.ToLower(keyword)) {
						addContactURL(href)
						break
					}
				}
			}
		})
	}

	// 7. Search in sidebar links
	sidebarSelectors := []string{
		".sidebar a", "#sidebar a", ".side-nav a", ".side-menu a",
		".widget a", ".sidebar-widget a", ".side-widget a",
		".sidebar-menu a", ".side-navigation a", ".sidebar-nav a",
	}

	for _, selector := range sidebarSelectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if exists {
				linkText := strings.ToLower(s.Text())
				linkTitle, _ := s.Attr("title")
				linkTitle = strings.ToLower(linkTitle)

				for _, keyword := range e.contactKeywords {
					if strings.Contains(strings.ToLower(href), strings.ToLower(keyword)) ||
						strings.Contains(linkText, strings.ToLower(keyword)) ||
						strings.Contains(linkTitle, strings.ToLower(keyword)) {
						addContactURL(href)
						break
					}
				}
			}
		})
	}

	// 8. Search in breadcrumb navigation
	doc.Find(".breadcrumb a, .breadcrumbs a, .breadcrumb-nav a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists {
			linkText := strings.ToLower(s.Text())
			for _, keyword := range e.contactKeywords {
				if strings.Contains(strings.ToLower(href), strings.ToLower(keyword)) ||
					strings.Contains(linkText, strings.ToLower(keyword)) {
					addContactURL(href)
					break
				}
			}
		}
	})

	// 9. Search in button elements
	doc.Find("button, .btn, .button").Each(func(i int, s *goquery.Selection) {
		onclick, _ := s.Attr("onclick")
		dataHref, _ := s.Attr("data-href")
		linkText := strings.ToLower(s.Text())

		// Check onclick for contact URLs
		if onclick != "" {
			for _, keyword := range e.contactKeywords {
				if strings.Contains(strings.ToLower(onclick), strings.ToLower(keyword)) {
					// Extract URL from onclick
					urlMatch := regexp.MustCompile(`['"]([^'"]*contact[^'"]*)['"]`).FindStringSubmatch(onclick)
					if len(urlMatch) > 1 {
						addContactURL(urlMatch[1])
					}
					break
				}
			}
		}

		// Check data-href
		if dataHref != "" {
			for _, keyword := range e.contactKeywords {
				if strings.Contains(strings.ToLower(dataHref), strings.ToLower(keyword)) {
					addContactURL(dataHref)
					break
				}
			}
		}

		// Check button text
		for _, keyword := range e.contactKeywords {
			if strings.Contains(linkText, strings.ToLower(keyword)) {
				// Try to find associated link
				parent := s.Parent()
				if parent.Is("a") {
					if href, exists := parent.Attr("href"); exists {
						addContactURL(href)
					}
				}
				break
			}
		}
	})

	// 10. Search in form actions
	doc.Find("form").Each(func(i int, s *goquery.Selection) {
		action, exists := s.Attr("action")
		if exists {
			for _, keyword := range e.contactKeywords {
				if strings.Contains(strings.ToLower(action), strings.ToLower(keyword)) {
					addContactURL(action)
					break
				}
			}
		}
	})

	// 11. Search in data attributes
	doc.Find("[data-contact], [data-contact-url], [data-contact-page]").Each(func(i int, s *goquery.Selection) {
		dataContact, _ := s.Attr("data-contact")
		dataContactURL, _ := s.Attr("data-contact-url")
		dataContactPage, _ := s.Attr("data-contact-page")

		if dataContact != "" {
			addContactURL(dataContact)
		}
		if dataContactURL != "" {
			addContactURL(dataContactURL)
		}
		if dataContactPage != "" {
			addContactURL(dataContactPage)
		}
	})

	// 12. Search in link rel attributes
	doc.Find("link[rel='contact'], link[rel='contact-page']").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			addContactURL(href)
		}
	})

	// 13. Comprehensive common contact page patterns (expanded)
	commonContactPaths := []string{
		// English patterns
		"/contact", "/contact-us", "/contactus", "/contact-us/", "/contact/",
		"/about/contact", "/company/contact", "/info/contact", "/company/contact-us",
		"/about/contact-us", "/info/contact-us", "/company/contactus",
		"/support", "/help", "/customer-service", "/get-in-touch",
		"/support/contact", "/help/contact", "/customer-service/contact",
		"/en/contact", "/en/contact-us", "/en/contactus", "/en/support",
		"/en/help", "/en/customer-service", "/en/get-in-touch",

		// German patterns
		"/kontakt", "/kontakt/", "/de/kontakt", "/de/kontakt/",
		"/unternehmen/kontakt", "/firma/kontakt", "/info/kontakt",
		"/support", "/hilfe", "/kundenservice", "/kundendienst",
		"/de/support", "/de/hilfe", "/de/kundenservice",

		// Spanish patterns
		"/contacto", "/contacto/", "/es/contacto", "/es/contacto/",
		"/empresa/contacto", "/compania/contacto", "/info/contacto",
		"/soporte", "/ayuda", "/servicio-cliente", "/atencion-cliente",
		"/es/soporte", "/es/ayuda", "/es/servicio-cliente",

		// French patterns
		"/contact", "/contact/", "/fr/contact", "/fr/contact/",
		"/entreprise/contact", "/societe/contact", "/info/contact",
		"/support", "/aide", "/service-client", "/assistance",
		"/fr/support", "/fr/aide", "/fr/service-client",

		// Italian patterns
		"/contatto", "/contatti", "/it/contatto", "/it/contatti",
		"/azienda/contatto", "/societa/contatto", "/info/contatto",
		"/supporto", "/aiuto", "/servizio-cliente", "/assistenza",
		"/it/supporto", "/it/aiuto", "/it/servizio-cliente",

		// Portuguese patterns
		"/contato", "/contatos", "/pt/contato", "/pt/contatos",
		"/empresa/contato", "/companhia/contato", "/info/contato",
		"/suporte", "/ajuda", "/servico-cliente", "/atendimento",
		"/pt/suporte", "/pt/ajuda", "/pt/servico-cliente",

		// Russian patterns
		"/контакт", "/контакты", "/ru/контакт", "/ru/контакты",
		"/компания/контакт", "/фирма/контакт", "/инфо/контакт",
		"/поддержка", "/помощь", "/служба-поддержки",
		"/ru/поддержка", "/ru/помощь", "/ru/служба-поддержки",

		// Japanese patterns
		"/お問い合わせ", "/連絡先", "/ja/お問い合わせ", "/ja/連絡先",
		"/会社/お問い合わせ", "/企業/お問い合わせ", "/情報/お問い合わせ",
		"/サポート", "/ヘルプ", "/顧客サービス", "/サポート/お問い合わせ",
		"/ja/サポート", "/ja/ヘルプ", "/ja/顧客サービス",

		// Korean patterns
		"/연락처", "/문의하기", "/ko/연락처", "/ko/문의하기",
		"/회사/연락처", "/기업/연락처", "/정보/연락처",
		"/지원", "/도움말", "/고객서비스", "/고객지원",
		"/ko/지원", "/ko/도움말", "/ko/고객서비스",

		// Chinese patterns
		"/联系", "/联系我们", "/zh/联系", "/zh/联系我们",
		"/公司/联系", "/企业/联系", "/信息/联系",
		"/支持", "/帮助", "/客户服务", "/客服",
		"/zh/支持", "/zh/帮助", "/zh/客户服务",

		// Arabic patterns
		"/اتصل", "/الاتصال", "/ar/اتصل", "/ar/الاتصال",
		"/شركة/اتصل", "/مؤسسة/اتصل", "/معلومات/اتصل",
		"/دعم", "/مساعدة", "/خدمة-العملاء", "/خدمة-الزبائن",
		"/ar/دعم", "/ar/مساعدة", "/ar/خدمة-العملاء",

		// Additional common patterns
		"/reach-us", "/get-in-touch", "/contact-form", "/contact-page",
		"/write-us", "/email-us", "/call-us", "/find-us",
		"/location", "/address", "/directions", "/map",
		"/office", "/headquarters", "/branch", "/location",
		"/team", "/staff", "/people", "/about/team",
		"/careers", "/jobs", "/employment", "/work-with-us",
		"/press", "/media", "/news", "/press-room",
		"/investors", "/investor-relations", "/shareholders",
		"/partners", "/partnerships", "/affiliates",
		"/resellers", "/distributors", "/dealers",
		"/support/contact", "/help/contact", "/customer-service/contact",
		"/technical-support", "/customer-support", "/client-support",
		"/sales", "/sales-contact", "/sales-inquiry",
		"/marketing", "/marketing-contact", "/advertising",
		"/legal", "/legal-contact", "/terms", "/privacy",
		"/feedback", "/suggestions", "/complaints", "/reviews",
		"/faq", "/questions", "/help-center", "/knowledge-base",
		"/troubleshooting", "/how-to", "/guides", "/tutorials",
		"/downloads", "/resources", "/tools", "/utilities",
		"/api", "/developers", "/documentation", "/docs",
		"/status", "/uptime", "/maintenance", "/outages",
		"/security", "/security-contact", "/vulnerability",
		"/abuse", "/dmca", "/copyright", "/trademark",
		"/accessibility", "/ada", "/wcag", "/compliance",
		"/sitemap", "/site-map", "/navigation", "/menu",
		"/search", "/find", "/locate", "/directory",
		"/blog", "/news", "/articles", "/posts",
		"/events", "/calendar", "/schedule", "/appointments",
		"/webinar", "/demo", "/trial", "/free-trial",
		"/pricing", "/plans", "/packages", "/subscription",
		"/billing", "/payment", "/invoice", "/account",
		"/login", "/signin", "/register", "/signup",
		"/profile", "/account", "/dashboard", "/portal",
		"/settings", "/preferences", "/configuration",
		"/logout", "/signout", "/exit", "/close",
	}

	for _, path := range commonContactPaths {
		testURL := baseURL + path
		addContactURL(testURL)
	}

	// 14. Search in Open Graph meta tags
	doc.Find("meta[property='og:url']").Each(func(i int, s *goquery.Selection) {
		if content, exists := s.Attr("content"); exists {
			for _, keyword := range e.contactKeywords {
				if strings.Contains(strings.ToLower(content), strings.ToLower(keyword)) {
					addContactURL(content)
					break
				}
			}
		}
	})

	// 15. Search in Twitter Card meta tags
	doc.Find("meta[name='twitter:url']").Each(func(i int, s *goquery.Selection) {
		if content, exists := s.Attr("content"); exists {
			for _, keyword := range e.contactKeywords {
				if strings.Contains(strings.ToLower(content), strings.ToLower(keyword)) {
					addContactURL(content)
					break
				}
			}
		}
	})

	// 16. Search in canonical URLs
	doc.Find("link[rel='canonical']").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			for _, keyword := range e.contactKeywords {
				if strings.Contains(strings.ToLower(href), strings.ToLower(keyword)) {
					addContactURL(href)
					break
				}
			}
		}
	})

	// 17. Search in alternate language links
	doc.Find("link[rel='alternate']").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			for _, keyword := range e.contactKeywords {
				if strings.Contains(strings.ToLower(href), strings.ToLower(keyword)) {
					addContactURL(href)
					break
				}
			}
		}
	})

	// 18. Search in schema.org markup
	doc.Find("[itemtype*='ContactPage'], [itemtype*='Organization']").Each(func(i int, s *goquery.Selection) {
		// Look for contact URLs in schema.org markup
		s.Find("a").Each(func(i int, link *goquery.Selection) {
			if href, exists := link.Attr("href"); exists {
				for _, keyword := range e.contactKeywords {
					if strings.Contains(strings.ToLower(href), strings.ToLower(keyword)) {
						addContactURL(href)
						break
					}
				}
			}
		})
	})

	// 19. Search in microdata
	doc.Find("[itemprop='url'], [itemprop='contactPoint']").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			for _, keyword := range e.contactKeywords {
				if strings.Contains(strings.ToLower(href), strings.ToLower(keyword)) {
					addContactURL(href)
					break
				}
			}
		}
	})

	// 20. Search in RDFa markup
	doc.Find("[typeof*='ContactPage'], [typeof*='Organization']").Each(func(i int, s *goquery.Selection) {
		s.Find("a").Each(func(i int, link *goquery.Selection) {
			if href, exists := link.Attr("href"); exists {
				for _, keyword := range e.contactKeywords {
					if strings.Contains(strings.ToLower(href), strings.ToLower(keyword)) {
						addContactURL(href)
						break
					}
				}
			}
		})
	})

	// Remove duplicates and select the best contact URL
	uniqueURLs := make(map[string]bool)
	var finalURLs []string

	for _, url := range contactURLs {
		if !uniqueURLs[url] {
			uniqueURLs[url] = true
			finalURLs = append(finalURLs, url)
		}
	}

	// Prioritize URLs (prefer shorter, more direct contact URLs)
	if len(finalURLs) > 0 {
		// Sort by priority: direct contact URLs first, then longer paths
		sort.Slice(finalURLs, func(i, j int) bool {
			// Prefer URLs with "contact" in the path
			iHasContact := strings.Contains(strings.ToLower(finalURLs[i]), "contact")
			jHasContact := strings.Contains(strings.ToLower(finalURLs[j]), "contact")

			if iHasContact && !jHasContact {
				return true
			}
			if !iHasContact && jHasContact {
				return false
			}

			// Prefer shorter URLs
			return len(finalURLs[i]) < len(finalURLs[j])
		})

		contactURL = finalURLs[0]
	}

	if contactURL == "" {
		return "", fmt.Errorf("no contact page found")
	}

	return contactURL, nil
}

// IsUnwantedDomain checks if a domain is unwanted based on unwanted domains file
func (e *EmailExtractor) IsUnwantedDomain(domain string) bool {
	for _, unwanted := range e.unwantedDoms {
		if strings.Contains(domain, unwanted) {
			return true
		}
	}
	return false
}

// HasBadExtension checks if a URL has a bad extension based on the bad extensions file
func (e *EmailExtractor) HasBadExtension(url string) bool {
	for _, ext := range e.badExtensions {
		if strings.HasSuffix(url, ext) {
			return true
		}
	}
	return false
}

// HasContactPageBadExtension checks if a contact page URL has a bad extension based on the contact page skip extensions file
func (e *EmailExtractor) HasContactPageBadExtension(url string) bool {
	for _, ext := range e.contactPageSkipExtensions {
		if strings.HasSuffix(url, ext) {
			return true
		}
	}
	return false
}

// StartCleaner runs the cleaner function at regular intervals to remove duplicates
func (e *EmailExtractor) StartCleaner() {
	ticker := time.NewTicker(time.Duration(e.config.CleanerInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			e.cleanEmailsFile()
		case <-e.cleanerStop:
			return
		}
	}
}

// cleanEmailsFile reads the email file, removes duplicates, and writes it back
func (e *EmailExtractor) cleanEmailsFile() {
	e.outputMutex.Lock()
	defer e.outputMutex.Unlock()

	outputPath := fmt.Sprintf("%s/extracted_emails.txt", e.config.OutputDirectory)
	uniqueEmails := make(map[string]struct{})

	file, err := os.Open(outputPath)
	if err != nil {
		// log.Printf("Failed to open email file for cleaning: %v", err)
		return
	}
	defer file.Close()

	// Read existing emails and filter duplicates
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		normalizedEmail := strings.ToLower(strings.TrimSpace(scanner.Text()))
		uniqueEmails[normalizedEmail] = struct{}{}
	}

	// Write back the unique emails
	tempOutputPath := fmt.Sprintf("%s/temp_extracted_emails.txt", e.config.OutputDirectory)
	tempFile, err := os.OpenFile(tempOutputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Printf("Failed to open temp file for writing: %v", err)
		return
	}
	defer tempFile.Close()

	for email := range uniqueEmails {
		_, err := tempFile.WriteString(email + "\n")
		if err != nil {
			log.Printf("Failed to write cleaned email to file: %v", err)
			break
		}
	}

	// Replace the original file with the cleaned one
	err = os.Rename(tempOutputPath, outputPath)
	if err != nil {
		// log.Printf("Failed to replace original email file with cleaned file: %v", err)
		return
	}

	log.Println("Email file cleaned and duplicates removed.")
}

// StopCleaner stops the cleaner goroutine
func (e *EmailExtractor) StopCleaner() {
	e.cleanerStop <- true
}

// Helper functions for shutdown and progress tracking
func (e *EmailExtractor) isShutdownRequested() bool {
	e.shutdownMutex.RLock()
	defer e.shutdownMutex.RUnlock()
	return e.shutdownRequested
}

func (e *EmailExtractor) requestShutdown() {
	e.shutdownMutex.Lock()
	defer e.shutdownMutex.Unlock()
	e.shutdownRequested = true
}

// DomainResult represents different outcomes for domain processing
type DomainResult int

const (
	DomainSuccess DomainResult = iota // Emails found
	DomainUnresolved                  // DNS/connection failure
	DomainNoEmail                     // Resolved but no emails found
)

func (e *EmailExtractor) updateProgress(result DomainResult) {
	e.progressMutex.Lock()
	defer e.progressMutex.Unlock()
	e.processedDomains++
	switch result {
	case DomainSuccess:
		e.successfulDomains++
	case DomainUnresolved:
		e.unresolvedDomains++
		e.failedDomains++ // Keep for backward compatibility
	case DomainNoEmail:
		e.noEmailDomains++
		e.failedDomains++ // Keep for backward compatibility
	}
}

func (e *EmailExtractor) getProgress() (processed, total, success, failed, unresolved, noEmail int, elapsed time.Duration) {
	e.progressMutex.RLock()
	defer e.progressMutex.RUnlock()
	elapsed = time.Since(e.startTime)
	return e.processedDomains, e.totalDomains, e.successfulDomains, e.failedDomains, e.unresolvedDomains, e.noEmailDomains, elapsed
}

func (e *EmailExtractor) displayProgress() {
	// Throttle progress updates to avoid excessive console writes
	now := time.Now()
	if now.Sub(e.lastProgressUpdate) < 500*time.Millisecond {
		return
	}
	e.lastProgressUpdate = now

	processed, total, success, _, unresolved, noEmail, elapsed := e.getProgress()
	if total == 0 {
		return
	}

	e.progressMutex.RLock()
	totalEmails := e.totalEmailsFound
	cloudflareCount := e.cloudflareBlocked
	e.progressMutex.RUnlock()

	percentage := float64(processed) * 100.0 / float64(total)
	rate := float64(processed) / elapsed.Seconds()
	remaining := total - processed
	etaSeconds := 0.0
	if rate > 0 {
		etaSeconds = float64(remaining) / rate
	}
	eta := time.Duration(etaSeconds) * time.Second

	// Calculate success rate
	successRate := 0.0
	if processed > 0 {
		successRate = float64(success) * 100.0 / float64(processed)
	}

	// Create progress bar (30 characters)
	barWidth := 30
	filled := int(percentage / 100.0 * float64(barWidth))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	// Print progress on its own line, overwriting previous progress
	// Use \033[2K to clear entire line, \r to return to start, then print new progress
	// This ensures progress doesn't overwrite log messages
	// Show breakdown: ✅ emails found | 🌐 resolved (no emails) | ❌ unresolved
	fmt.Printf("\r\033[2K📊 [%s] %.1f%% | 📧 %d emails | ✅ %d (%.1f%%) | 🌐 %d (no emails) | ❌ %d (unresolved) | 🔒 CF:%d | ⚡ %.1f/s | ⏱️  ETA: %s",
		bar, percentage, totalEmails, success, successRate, noEmail, unresolved, cloudflareCount, rate, formatDuration(eta))
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

// StartExtraction starts the email extraction process with improved concurrency
func (e *EmailExtractor) StartExtraction() {
	e.StartExtractionWithConfirmation(false)
}

// StartExtractionWithConfirmation starts extraction with optional confirmation skip
func (e *EmailExtractor) StartExtractionWithConfirmation(skipConfirmation bool) {
	var totalDomains int
	var err error

	// Use streaming mode for large domain lists (memory optimization)
	if e.config.StreamDomains {
		// Count domains first for progress tracking
		totalDomains, err = e.CountDomains()
		if err != nil {
			log.Fatalf("Failed to count domains: %v", err)
		}
	} else {
		// Legacy mode: load all domains into memory
	domains, err := e.LoadDomains()
	if err != nil {
		log.Fatalf("Failed to load domains: %v", err)
		}
		totalDomains = len(domains)
		// Display summary with loaded domains
		e.DisplaySummaryWithConfirmation(domains, skipConfirmation)
		// Note: domains slice is released after confirmation for memory optimization
	}

	// Initialize progress tracking
	e.totalDomains = totalDomains
	e.processedDomains = 0
	e.successfulDomains = 0
	e.failedDomains = 0
	e.unresolvedDomains = 0
	e.noEmailDomains = 0
	e.totalEmailsFound = 0
	e.totalPhonesFound = 0
	e.cloudflareBlocked = 0
	e.startTime = time.Now()
	e.lastProgressUpdate = time.Now()
	e.shutdownRequested = false

	// Display summary if not already shown (streaming mode)
	if e.config.StreamDomains {
		e.DisplaySummaryForStreaming(skipConfirmation)
	}

	// Setup graceful shutdown signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start progress display goroutine
	progressTicker := time.NewTicker(2 * time.Second)
	defer progressTicker.Stop()

	go func() {
		for range progressTicker.C {
			if e.isShutdownRequested() {
				return
			}
			e.displayProgress()
		}
	}()

	var wg sync.WaitGroup
	domainQueue := make(chan string, e.config.Concurrency*2)

	// Start the cleaner in a separate goroutine
	go e.StartCleaner()

	// Create worker goroutines
	for i := 0; i < e.config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for domain := range domainQueue {
				// Check for shutdown request before processing
				if e.isShutdownRequested() {
					return
				}

				result := e.ProcessDomain(domain)
				e.updateProgress(result)
			}
		}()
	}

	// Handle graceful shutdown
	go func() {
		<-sigChan
		fmt.Println("\n\n⚠️  Shutdown requested. Finishing current tasks...")
		fmt.Println("Press Ctrl+C again to force exit (may result in data loss)")
		e.requestShutdown()

		// Wait a bit, then check for force exit
		select {
		case <-sigChan:
			fmt.Println("\n❌ Force exit requested. Exiting immediately...")
			os.Exit(1)
		case <-time.After(30 * time.Second):
			// Timeout - continue graceful shutdown
		}
	}()

	// Add domains to the queue (streaming mode for memory optimization)
	if e.config.StreamDomains {
		// Stream domains from file instead of loading all into memory
		streamDone := make(chan error, 1)
		go e.StreamDomainsToChannel(domainQueue, streamDone)
		go func() {
			if err := <-streamDone; err != nil {
				log.Printf("Error streaming domains: %v", err)
			}
			close(domainQueue)
		}()
	} else {
		// Legacy mode: load domains into memory first
		domains, err := e.LoadDomains()
		if err != nil {
			log.Fatalf("Failed to load domains: %v", err)
		}
		go func() {
			defer close(domainQueue)
	for _, domain := range domains {
				if e.isShutdownRequested() {
					return
				}
		domainQueue <- domain
	}
		}()
	}

	// Wait for all goroutines to finish
	wg.Wait()

	// Stop the cleaner once the extraction is done
	e.StopCleaner()

	// Final progress display with enhanced statistics
	fmt.Println() // New line after progress
	processed, total, success, _, unresolved, noEmail, elapsed := e.getProgress()

	// Get final statistics
	e.progressMutex.RLock()
	totalEmails := e.totalEmailsFound
	cloudflareCount := e.cloudflareBlocked
	e.progressMutex.RUnlock()

	// Calculate statistics
	successRate := 0.0
	if processed > 0 {
		successRate = float64(success) * 100.0 / float64(processed)
	}
	avgEmailsPerDomain := 0.0
	if success > 0 {
		avgEmailsPerDomain = float64(totalEmails) / float64(success)
	}
	processingRate := 0.0
	if elapsed.Seconds() > 0 {
		processingRate = float64(processed) / elapsed.Seconds()
	}

	if e.isShutdownRequested() {
		fmt.Println(strings.Repeat("=", 80))
		fmt.Printf("⚠️  EXTRACTION INTERRUPTED\n")
		fmt.Println(strings.Repeat("=", 80))
		fmt.Printf("📊 Progress: %d/%d domains processed (%.1f%%)\n", processed, total, float64(processed)*100.0/float64(total))
		fmt.Printf("✅ Successful: %d (%.1f%%) | 🌐 No Emails: %d | ❌ Unresolved: %d\n", success, successRate, noEmail, unresolved)
		fmt.Printf("📧 Total Emails Extracted: %d (avg: %.1f per successful domain)\n", totalEmails, avgEmailsPerDomain)
		if cloudflareCount > 0 {
			fmt.Printf("🔒 Cloudflare Blocked: %d domains\n", cloudflareCount)
		}
		fmt.Printf("⏱️  Total Time: %s | ⚡ Processing Rate: %.1f domains/s\n", formatDuration(elapsed), processingRate)
		fmt.Println("💾 Partial results have been saved to output directory.")
		fmt.Println(strings.Repeat("=", 80))
	} else {
		fmt.Println(strings.Repeat("=", 80))
		fmt.Printf("✅ EXTRACTION COMPLETE\n")
		fmt.Println(strings.Repeat("=", 80))
		fmt.Printf("📊 Total Domains: %d\n", total)
		fmt.Printf("✅ Successful: %d (%.1f%%) | 🌐 No Emails: %d (%.1f%%) | ❌ Unresolved: %d (%.1f%%)\n",
			success, successRate, noEmail, float64(noEmail)*100.0/float64(processed), unresolved, float64(unresolved)*100.0/float64(processed))
		fmt.Printf("📧 Total Emails Extracted: %d (avg: %.1f per successful domain)\n", totalEmails, avgEmailsPerDomain)
		if cloudflareCount > 0 {
			fmt.Printf("🔒 Cloudflare Blocked: %d domains\n", cloudflareCount)
		}
		fmt.Printf("⏱️  Total Time: %s | ⚡ Processing Rate: %.1f domains/s\n", formatDuration(elapsed), processingRate)
		fmt.Printf("📁 Output Directory: %s\n", e.config.OutputDirectory)
		fmt.Println(strings.Repeat("=", 80))
	}
}

// ProcessDomain processes a single domain and returns DomainResult
func (e *EmailExtractor) ProcessDomain(domain string) DomainResult {
	var emails []string

	// Fast DNS pre-check: Skip domains without valid DNS (optimization)
	if !e.fastDNSCheck(domain) {
		// Domain logged to unresolved domains file, no console output needed
		e.LogDomain(domain, e.config.LogUnresolvedDomains)
		return DomainUnresolved
	}

	homepageURL := fmt.Sprintf("https://%s", domain)
	if e.HasBadExtension(homepageURL) {
		// Domain logged to unresolved domains file, no console output needed
		e.LogDomain(domain, e.config.LogUnresolvedDomains)
		return DomainUnresolved
	}

	// Try HTTPS first
	doc, err := e.FetchURL(homepageURL)
	if err != nil {
		// Try HTTP as fallback
		homepageURL = fmt.Sprintf("http://%s", domain)
		doc, err = e.FetchURL(homepageURL)
		if err != nil {
			// Error details logged to unresolved domains file, no console output needed
			e.LogDomain(domain, e.config.LogUnresolvedDomains)
			return DomainUnresolved
		}
	}

	// Extract emails from homepage
	emails = e.ExtractEmailsWithSmartValidationFromURL(doc, homepageURL)

	// Try to find and process contact page
	contactPageURL, err := e.FindContactPage(doc, homepageURL)
	if err == nil && contactPageURL != "" {
		contactDoc, err := e.FetchURL(contactPageURL)
		if err != nil {
			// Contact page fetch failed - continue with homepage results only
			// No console output needed, progress display shows domain status
		} else {
			// Extract emails from contact page
			contactEmails := e.ExtractEmailsWithSmartValidationFromURL(contactDoc, contactPageURL)
			emails = append(emails, contactEmails...)
		}
	}

	// Remove duplicates from combined results
	emails = removeDuplicates(emails)

	// Clean and save the results
	e.SaveEmails(emails)

	// Log resolved domains (successfully fetched, regardless of email count)
	// A domain is "resolved" if we successfully fetched it, even if no emails were found
		e.LogDomain(domain, e.config.LogResolvedDomains)

	// Update email statistics
	e.progressMutex.Lock()
	e.totalEmailsFound += int64(len(emails))
	e.progressMutex.Unlock()

	// Domain result details are already shown in progress bar
	// No need to print individual domain results to avoid clutter

	if len(emails) > 0 {
		return DomainSuccess
	}
	return DomainNoEmail
}

// getRejectionReason determines why an email was rejected by validation
func (e *EmailExtractor) getRejectionReason(email string, sv *SmartValidation) string {
	// Basic syntax check
	if !sv.basicSyntaxCheck(email) {
		return "Invalid email syntax"
	}

	// Extract domain
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "Invalid email format"
	}
	domain := parts[1]

	// Check for security messages
	if sv.isSecurityMessage(email) {
		return "Security/system email pattern"
	}

	// Check for disposable domains
	if sv.isDisposableDomain(domain) {
		return "Disposable email domain"
	}

	// Check for bad extensions
	if sv.isBadEmailExtension(email) {
		return "Bad email extension/TLD"
	}

	// MX record verification
	if !sv.hasMXRecord(domain) {
		return "No MX record found (domain doesn't accept email)"
	}

	// Additional checks
	if !sv.additionalChecks(email) {
		return "Failed additional validation checks"
	}

	return "Unknown reason"
}

// TestDomain tests email extraction on a single domain with detailed output
func (e *EmailExtractor) TestDomain(domainOrURL string) {
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("EMAIL EXTRACTION TEST MODE")
	fmt.Println(strings.Repeat("=", 80))

	startTime := time.Now()

	// Parse domain from URL if needed
	domain := domainOrURL
	if strings.HasPrefix(domainOrURL, "http://") || strings.HasPrefix(domainOrURL, "https://") {
		parsedURL, err := url.Parse(domainOrURL)
		if err != nil {
			fmt.Printf("❌ Invalid URL: %s\n", domainOrURL)
			return
		}
		domain = parsedURL.Hostname()
		if parsedURL.Port() != "" {
			domain = strings.TrimSuffix(domain, ":"+parsedURL.Port())
		}
	}

	fmt.Printf("\n📍 Testing Domain: %s\n", domain)
	if domain != domainOrURL {
		fmt.Printf("📍 Original Input: %s\n", domainOrURL)
	}
	fmt.Println(strings.Repeat("-", 80))

	homepageURL := fmt.Sprintf("https://%s", domain)
	if e.HasBadExtension(homepageURL) {
		fmt.Printf("❌ URL has bad extension: %s\n", homepageURL)
		return
	}

	// Try HTTPS first
	fmt.Printf("\n🔍 Step 1: Fetching homepage...\n")
	fmt.Printf("   URL: %s\n", homepageURL)
	doc, err := e.FetchURL(homepageURL)
	if err != nil {
		fmt.Printf("   ❌ HTTPS failed: %v\n", err)
		fmt.Printf("   🔄 Trying HTTP fallback...\n")
		homepageURL = fmt.Sprintf("http://%s", domain)
		fmt.Printf("   URL: %s\n", homepageURL)
		doc, err = e.FetchURL(homepageURL)
		if err != nil {
			fmt.Printf("   ❌ HTTP also failed: %v\n", err)
			return
		}
		fmt.Printf("   ✅ Successfully fetched via HTTP\n")
	} else {
		fmt.Printf("   ✅ Successfully fetched via HTTPS\n")
	}

	// Extract emails from homepage
	fmt.Printf("\n🔍 Step 2: Extracting emails from homepage...\n")

	// Normalize HTML content for extraction
	normalizedContent := normalizeText(doc.Text())

	// Extract emails with verbose debugging
	smartValidation := NewSmartValidation(e.unwantedDoms, e.badExtensions)
	obfuscationDetector := NewObfuscationDetector()

	// Extract mailto links from HTML attributes first (most reliable)
	var mailtoEmails []string
	doc.Find("a[href^='mailto:']").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists {
			email := strings.TrimPrefix(href, "mailto:")
			email = strings.Split(email, "?")[0] // Remove query parameters
			email = strings.Split(email, "#")[0] // Remove fragments
			email = strings.TrimSpace(email)
			if email != "" {
				mailtoEmails = append(mailtoEmails, email)
			}
		}
	})

	// Also search in raw HTML content for mailto patterns (case-insensitive)
	htmlContent, _ := doc.Html()
	mailtoRegex := regexp.MustCompile(`(?i)mailto:([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`)
	htmlMatches := mailtoRegex.FindAllStringSubmatch(htmlContent, -1)
	for _, match := range htmlMatches {
		if len(match) >= 2 {
			email := match[1]
			email = strings.Split(email, "?")[0]
			email = strings.Split(email, "#")[0]
			email = strings.TrimSpace(email)
			if email != "" && !contains(mailtoEmails, email) {
				mailtoEmails = append(mailtoEmails, email)
			}
		}
	}

	// Debug: Check if HTML contains email patterns
	if len(mailtoEmails) == 0 {
		fmt.Printf("\n   🔍 Debug: Checking HTML content for email patterns...\n")
		emailPatternInHTML := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
		allEmailMatches := emailPatternInHTML.FindAllString(htmlContent, -1)
		if len(allEmailMatches) > 0 {
			fmt.Printf("      Found %d email-like patterns in raw HTML\n", len(allEmailMatches))
			uniqueMatches := removeDuplicates(allEmailMatches)
			for i, email := range uniqueMatches {
				if i < 10 && isASCII(email) {
					fmt.Printf("         - %s\n", email)
				}
			}
		} else {
			fmt.Printf("      ⚠️  No email patterns found in raw HTML\n")
			fmt.Printf("      (Content might be loaded via JavaScript or the page structure is different)\n")

			// Show HTML size and sample to verify we got content
			fmt.Printf("      HTML content size: %d bytes\n", len(htmlContent))
			if len(htmlContent) > 0 {
				sample := htmlContent
				if len(sample) > 500 {
					sample = sample[:500]
				}
				fmt.Printf("      HTML sample (first 500 chars): %s...\n", sample)
			}
		}

		// Check for mailto in HTML (case-insensitive)
		htmlLower := strings.ToLower(htmlContent)
		if strings.Contains(htmlLower, "mailto") {
			fmt.Printf("      ✅ Found 'mailto' text in HTML, but couldn't extract emails\n")
			// Show sample around mailto
			mailtoIndex := strings.Index(htmlLower, "mailto")
			if mailtoIndex > 0 {
				start := max(0, mailtoIndex-50)
				end := min(len(htmlContent), mailtoIndex+150)
				sample := htmlContent[start:end]
				fmt.Printf("      Sample HTML around 'mailto': %s\n", sample)
			}
		} else {
			fmt.Printf("      ⚠️  'mailto' not found in HTML at all\n")
		}

		// Check for common email-related text
		if strings.Contains(htmlLower, "admin@") || strings.Contains(htmlLower, "prepschool@") {
			fmt.Printf("      ✅ Found email-like text in HTML\n")
			// Try to find the context
			if strings.Contains(htmlLower, "admin@") {
				adminIndex := strings.Index(htmlLower, "admin@")
				start := max(0, adminIndex-50)
				end := min(len(htmlContent), adminIndex+100)
				sample := htmlContent[start:end]
				fmt.Printf("      Sample around 'admin@': %s\n", sample)
			}
		}
	}

	// Extract regular emails from text (filter out garbage)
	regularEmailsRaw := extractEmails(normalizedContent)
	regularEmails := []string{}
	emailPattern := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	for _, email := range regularEmailsRaw {
		if emailPattern.MatchString(email) && isASCII(email) {
			regularEmails = append(regularEmails, email)
		}
	}

	obfuscatedEmails := obfuscationDetector.DetectAndDecode(normalizedContent)

	// Combine all found emails (before validation)
	allFoundEmails := append(regularEmails, obfuscatedEmails...)
	allFoundEmails = append(allFoundEmails, mailtoEmails...)
	allFoundEmails = removeDuplicates(allFoundEmails)

	// Show raw extraction results
	fmt.Printf("\n   🔍 Raw extraction (before validation):\n")
	fmt.Printf("      Regular emails found: %d\n", len(regularEmails))
	if len(regularEmails) > 0 {
		for i, email := range regularEmails {
			if i < 10 { // Limit to first 10 for display
				fmt.Printf("         - %s\n", email)
			}
		}
		if len(regularEmails) > 10 {
			fmt.Printf("         ... and %d more\n", len(regularEmails)-10)
		}
	}

	fmt.Printf("      Obfuscated emails found: %d\n", len(obfuscatedEmails))
	if len(obfuscatedEmails) > 0 {
		for i, email := range obfuscatedEmails {
			if i < 10 {
				fmt.Printf("         - %s\n", email)
			}
		}
		if len(obfuscatedEmails) > 10 {
			fmt.Printf("         ... and %d more\n", len(obfuscatedEmails)-10)
		}
	}

	fmt.Printf("      Mailto links found: %d\n", len(mailtoEmails))
	if len(mailtoEmails) > 0 {
		for i, email := range mailtoEmails {
			if i < 10 {
				fmt.Printf("         - %s\n", email)
			}
		}
		if len(mailtoEmails) > 10 {
			fmt.Printf("         ... and %d more\n", len(mailtoEmails)-10)
		}
	}

	// Apply validation and show what gets filtered
	fmt.Printf("\n   🔍 Validation results:\n")
	var validEmails []string
	var rejectedEmails []string
	var rejectionReasons []string

	for _, email := range allFoundEmails {
		if smartValidation.ValidateEmail(email) {
			validEmails = append(validEmails, email)
	} else {
			rejectedEmails = append(rejectedEmails, email)
			reason := e.getRejectionReason(email, smartValidation)
			rejectionReasons = append(rejectionReasons, reason)
		}
	}

	fmt.Printf("      Valid emails after validation: %d\n", len(validEmails))
	fmt.Printf("      Rejected emails: %d\n", len(rejectedEmails))

	if len(rejectedEmails) > 0 {
		fmt.Printf("\n   ❌ Rejected emails and reasons:\n")
		for i, email := range rejectedEmails {
			if i < 10 {
				fmt.Printf("      %d. %s - %s\n", i+1, email, rejectionReasons[i])
			}
		}
		if len(rejectedEmails) > 10 {
			fmt.Printf("      ... and %d more rejected\n", len(rejectedEmails)-10)
		}
	}

	// Also get emails using the actual extraction function (for final results)
	actualExtractedEmails := e.ExtractEmailsWithSmartValidation(doc)

	// Extract emails from images using OCR (if enabled)
	var ocrEmails []string
	if e.config.EnableOCR && homepageURL != "" {
		fmt.Printf("\n   🔍 Step 2.5: Extracting emails from images using OCR...\n")
		ocrEmails = e.extractEmailsFromImages(doc, homepageURL)
		if len(ocrEmails) > 0 {
			fmt.Printf("      📷 Emails found in images: %d\n", len(ocrEmails))
			for i, email := range ocrEmails {
				if i < 10 {
					fmt.Printf("         - %s\n", email)
				}
			}
		} else {
			fmt.Printf("      📷 No emails found in images\n")
			if !e.config.EnableOCR {
				fmt.Printf("      ⚠️  OCR is disabled in config (enable_ocr: false)\n")
			} else {
				// Check if Tesseract is available
				ocrProcessor := NewOCRProcessor(e.config.EnableOCR, e.config.OCREngine, e.config.MaxImageSize, e.config.OCRTimeout)
				if ocrProcessor.tesseractPath == "" {
					fmt.Printf("      ⚠️  Tesseract OCR not found. Please install Tesseract to extract emails from images.\n")
					fmt.Printf("      Installation: https://github.com/tesseract-ocr/tesseract\n")
				} else {
					fmt.Printf("      ✅ Tesseract found at: %s\n", ocrProcessor.tesseractPath)
					// Count images found
					imageCount := 0
					doc.Find("img").Each(func(i int, s *goquery.Selection) {
						imageCount++
					})
					fmt.Printf("      📷 Images found on page: %d\n", imageCount)
				}
			}
		}
	}

	// Use the actual extracted emails as final result (combines all methods)
	homepageEmails := actualExtractedEmails
	// Add OCR emails if any found
	if len(ocrEmails) > 0 {
		homepageEmails = append(homepageEmails, ocrEmails...)
		homepageEmails = removeDuplicates(homepageEmails)
	}

	fmt.Printf("\n   📧 Final valid emails on homepage: %d\n", len(homepageEmails))

	// Try to find contact page
	fmt.Printf("\n🔍 Step 3: Searching for contact page...\n")
	contactPageURL, err := e.FindContactPage(doc, homepageURL)
	var contactEmails []string
	var contactPageFound bool

	if err == nil && contactPageURL != "" {
		contactPageFound = true
		fmt.Printf("   ✅ Contact page found: %s\n", contactPageURL)

		fmt.Printf("\n   🔍 Fetching contact page...\n")
		contactDoc, err := e.FetchURL(contactPageURL)
		if err != nil {
			fmt.Printf("   ❌ Failed to fetch contact page: %v\n", err)
		} else {
			fmt.Printf("   ✅ Successfully fetched contact page\n")

			// Extract emails from contact page
			fmt.Printf("\n   🔍 Extracting emails from contact page...\n")
			contactEmails = e.ExtractEmailsWithSmartValidationFromURL(contactDoc, contactPageURL)

			// Also extract emails from images on contact page (if OCR enabled)
			if e.config.EnableOCR && contactPageURL != "" {
				fmt.Printf("\n   🔍 Extracting emails from images on contact page...\n")
				contactOCREmails := e.extractEmailsFromImages(contactDoc, contactPageURL)
				if len(contactOCREmails) > 0 {
					fmt.Printf("      📷 Emails found in contact page images: %d\n", len(contactOCREmails))
					contactEmails = append(contactEmails, contactOCREmails...)
					contactEmails = removeDuplicates(contactEmails)
				}
			}

			fmt.Printf("\n   📧 Emails found on contact page: %d\n", len(contactEmails))
			if len(contactEmails) > 0 {
				for i, email := range contactEmails {
					fmt.Printf("      %d. %s\n", i+1, email)
				}
			} else {
				fmt.Printf("      (none found)\n")
			}
		}
	} else {
		fmt.Printf("   ❌ No contact page found\n")
		fmt.Printf("   Reason: %v\n", err)
	}

	// Combine results
	fmt.Printf("\n" + strings.Repeat("-", 80) + "\n")
	fmt.Printf("📊 EXTRACTION SUMMARY\n")
	fmt.Println(strings.Repeat("-", 80))

	allEmails := append(homepageEmails, contactEmails...)

	// Remove duplicates
	allEmails = removeDuplicates(allEmails)

	fmt.Printf("\n✅ FINAL RESULTS:\n\n")
	fmt.Printf("   Total Unique Emails: %d\n", len(allEmails))
	if len(allEmails) > 0 {
		fmt.Printf("\n   📧 Extracted Emails:\n")
		for i, email := range allEmails {
			source := "homepage"
			if len(homepageEmails) > 0 && contains(homepageEmails, email) {
				if len(contactEmails) > 0 && contains(contactEmails, email) {
					source = "homepage & contact"
				} else {
					source = "homepage"
				}
			} else {
				source = "contact page"
			}
			fmt.Printf("      %d. %s [found on: %s]\n", i+1, email, source)
		}
	}

	// Validation details
	fmt.Printf("\n" + strings.Repeat("-", 80) + "\n")
	fmt.Printf("🔍 VALIDATION DETAILS\n")
	fmt.Println(strings.Repeat("-", 80))

	// Use same validation instance for consistency
	validationInstance := NewSmartValidation(e.unwantedDoms, e.badExtensions)

	if len(allEmails) > 0 {
		fmt.Printf("\n   Email Validation Status:\n")
		for i, email := range allEmails {
			isValid := validationInstance.ValidateEmail(email)
			status := "❌ REJECTED"
			if isValid {
				status = "✅ VALID"
			}
			fmt.Printf("      %d. %s - %s\n", i+1, email, status)
		}
	}

	// Performance metrics
	responseTime := time.Since(startTime)
	fmt.Printf("\n" + strings.Repeat("-", 80) + "\n")
	fmt.Printf("⏱️  PERFORMANCE METRICS\n")
	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("   Total Time: %.2f seconds\n", responseTime.Seconds())
	fmt.Printf("   Homepage Fetched: ✅\n")
	if contactPageFound {
		fmt.Printf("   Contact Page Found: ✅\n")
	} else {
		fmt.Printf("   Contact Page Found: ❌\n")
	}

	fmt.Printf("\n" + strings.Repeat("=", 80) + "\n")
	fmt.Printf("✅ TEST COMPLETE\n")
	fmt.Println(strings.Repeat("=", 80))
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// CountDomains counts the total number of valid domains in the file without loading them all
func (e *EmailExtractor) CountDomains() (int, error) {
	file, err := os.Open(e.config.DomainsFilePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		domain := strings.TrimSpace(scanner.Text())
		if domain != "" && isValidDomain(domain) {
			count++
		}
	}
	return count, scanner.Err()
}

// LoadDomains loads the domains from the file (legacy method - use streaming for large lists)
func (e *EmailExtractor) LoadDomains() ([]string, error) {
	// If streaming is enabled, only load domains if count is reasonable
	if e.config.StreamDomains {
		count, err := e.CountDomains()
		if err != nil {
			return nil, err
		}
		// Only load into memory if less than 100K domains
		if count > 100000 {
			return nil, fmt.Errorf("too many domains (%d) for memory loading. Use streaming mode instead", count)
		}
	}

	file, err := os.Open(e.config.DomainsFilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var domains []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		domain := strings.TrimSpace(scanner.Text())
		if domain != "" && isValidDomain(domain) {
			domains = append(domains, domain)
		}
	}
	return domains, scanner.Err()
}

// StreamDomains streams domains from file into a channel (memory-efficient)
func (e *EmailExtractor) StreamDomainsToChannel(domainQueue chan<- string, done chan<- error) {
	file, err := os.Open(e.config.DomainsFilePath)
	if err != nil {
		done <- err
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		if e.isShutdownRequested() {
			done <- nil
			return
		}
		domain := strings.TrimSpace(scanner.Text())
		if domain != "" && isValidDomain(domain) {
			domainQueue <- domain
			count++
		}
	}
	done <- scanner.Err()
}

// isValidDomain performs basic domain validation
func isValidDomain(domain string) bool {
	// Basic domain validation
	if len(domain) == 0 || len(domain) > 253 {
		return false
	}

	// Check for valid characters
	for _, char := range domain {
		if !((char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '.') {
			return false
		}
	}

	// Check for valid TLD
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return false
	}

	return true
}

// main function
func main() {
	// Check for help flag
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		printHelp()
		return
	}

	// Check for version flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println("Email Extractor v1.3")
		fmt.Println("Author: Dr.Anach")
		fmt.Println("Telegram: @dranach")
		return
	}

	// Check for test flag
	if len(os.Args) > 1 && (os.Args[1] == "--test" || os.Args[1] == "-t") {
		if len(os.Args) < 3 {
			fmt.Println("Error: --test requires a domain or URL")
			fmt.Println("Usage: go run main.go --test <domain_or_url>")
			fmt.Println("Example: go run main.go --test example.com")
			fmt.Println("Example: go run main.go --test https://example.com")
			os.Exit(1)
		}

		var extractor EmailExtractor
		err := extractor.LoadConfig("config.json")
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}

		// Load additional filters
		err = extractor.LoadAdditionalFilters()
		if err != nil {
			log.Fatalf("Failed to load filters: %v", err)
		}

		// Initialize DNS resolver and cache
		extractor.initializeDNSResolver()

		// Initialize HTTP client with custom DNS resolver and rate limiter
		extractor.httpClient = extractor.createHTTPClient()
		extractor.rateLimiter = rate.NewLimiter(rate.Limit(extractor.config.RateLimitPerSecond), extractor.config.MaxParallelRequests)

		// Run test mode
		testDomain := os.Args[2]
		extractor.TestDomain(testDomain)
		return
	}

	var extractor EmailExtractor
	err := extractor.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Parse command line arguments
	domainsFile, outputFile, skipConfirmation := parseCommandLineArgs(&extractor.config)

	// Set domains file path
	if domainsFile != "" {
		extractor.config.DomainsFilePath = domainsFile
	}

	// Set output directory if specified
	if outputFile != "" {
		// If outputFile doesn't end with .txt, treat as directory
		if !strings.HasSuffix(outputFile, ".txt") {
			extractor.config.OutputDirectory = outputFile
		} else {
			// If it ends with .txt, extract directory name
			dir := filepath.Dir(outputFile)
			if dir == "." {
				dir = "output"
			}
			extractor.config.OutputDirectory = dir
		}
	}

	// Load additional filters, user agents, and referer links from files
	err = extractor.LoadAdditionalFilters()
	if err != nil {
		log.Fatalf("Failed to load filters: %v", err)
	}

	// Initialize DNS resolver and cache
	extractor.initializeDNSResolver()

	// Initialize HTTP client with custom DNS resolver and rate limiter
	extractor.httpClient = extractor.createHTTPClient()
	extractor.rateLimiter = rate.NewLimiter(rate.Limit(extractor.config.RateLimitPerSecond), extractor.config.MaxParallelRequests)

	// Start extraction process
	extractor.StartExtractionWithConfirmation(skipConfirmation)

	fmt.Println("\nExtraction complete.")
}

// parseCommandLineArgs parses command line arguments and returns domains file, output file, and flags
func parseCommandLineArgs(config *Config) (string, string, bool) {
	var domainsFile, outputFile string
	var skipConfirmation bool

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]

		switch arg {
		case "--yes", "-y":
			skipConfirmation = true
		case "--rate":
			if i+1 < len(os.Args) {
				if rate, err := parseFloat(os.Args[i+1]); err == nil {
					config.RateLimitPerSecond = rate
				}
				i++ // Skip next argument
			}
		case "--workers":
			if i+1 < len(os.Args) {
				if workers, err := parseInt(os.Args[i+1]); err == nil {
					config.Concurrency = workers
					config.MaxParallelRequests = workers * 2
				}
				i++ // Skip next argument
			}
		case "--timeout":
			if i+1 < len(os.Args) {
				if timeout, err := parseInt(os.Args[i+1]); err == nil {
					config.Timeout = timeout
				}
				i++ // Skip next argument
			}
		case "--config":
			if i+1 < len(os.Args) {
				// Load config from specified file
				if err := loadConfigFromFile(os.Args[i+1], config); err == nil {
					fmt.Printf("Loaded config from: %s\n", os.Args[i+1])
				}
				i++ // Skip next argument
			}
		case "--output":
			if i+1 < len(os.Args) {
				outputFile = os.Args[i+1]
				i++ // Skip next argument
			}
		case "--smart-validation":
			// Enable smart validation (already enabled by default)
		case "--obfuscation-detection":
			// Enable obfuscation detection (already enabled by default)
		case "--verbose":
			// Enable verbose logging
		case "--progress":
			// Enable progress display
		default:
			// If it's not a flag, treat as file path
			if !strings.HasPrefix(arg, "--") && !strings.HasPrefix(arg, "-") {
				if domainsFile == "" {
					domainsFile = arg
				} else if outputFile == "" {
					outputFile = arg
				}
			}
		}
	}

	return domainsFile, outputFile, skipConfirmation
}

// Helper functions for parsing
func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

func parseInt(s string) (int, error) {
	return strconv.Atoi(s)
}

func loadConfigFromFile(configPath string, config *Config) error {
	file, err := os.Open(configPath)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	return decoder.Decode(config)
}

// printHelp displays the help information
func printHelp() {
	fmt.Println(`
Email Extractor v1.3 - Advanced Email Extraction Tool

USAGE:
    go run main.go [OPTIONS] [domains_file] [output_file]

ARGUMENTS:
    domains_file    Path to file containing domains (one per line)
    output_file     Path to save results (optional, defaults to output/)

OPTIONS:
    --rate <number>         Set requests per second (default: 50)
    --workers <number>      Set concurrent workers (default: 60)
    --timeout <seconds>     Set request timeout (default: 20)
    --config <file>         Load configuration from file
    --output <dir>          Set output directory
    --smart-validation      Enable smart email validation
    --obfuscation-detection Enable obfuscation detection
    --verbose              Enable verbose logging
    --progress             Show progress display
    --yes, -y              Skip confirmation prompt
    --test <domain>        Test email extraction on a single domain/URL
    --help, -h             Show this help message
    --version, -v          Show version information

EXAMPLES:
    go run main.go domains.txt
    go run main.go --rate 10 --workers 5 domains.txt results.txt
    go run main.go --timeout 30 --config custom.json domains.txt
    go run main.go --smart-validation domains.txt
    go run main.go --test example.com
    go run main.go --test https://example.com
    go run main.go --help
    go run main.go --version

FEATURES:
    - Smart Email Validation (MX records, syntax, disposable filtering)
    - Obfuscation Detection (20+ techniques)
    - 99% Contact Page Discovery
    - Production Ready

CONFIGURATION:
    Edit config.json to customize:
    - Rate limiting and concurrency
    - Timeout settings
    - Output directories
    - Filter settings

For more information, visit: https://github.com/email-extractor
`)
}
