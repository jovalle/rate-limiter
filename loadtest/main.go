package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type LoadTestConfig struct {
	TargetURL       string
	Duration        time.Duration
	Concurrency     int
	RequestsPerSec  int
	MetricsURL      string
	ValidateMetrics bool
}

type LoadTestResults struct {
	TotalRequests   int64
	SuccessRequests int64
	FailedRequests  int64
	RateLimited     int64
	Duration        time.Duration
	RequestsPerSec  float64
}

type PrometheusMetrics struct {
	RateLimited    float64
	DroppedPackets float64
	AvailableTokens float64
}

func main() {
	config := LoadTestConfig{}

	flag.StringVar(&config.TargetURL, "target", "http://localhost:8000/hello", "Target URL to load test")
	flag.DurationVar(&config.Duration, "duration", 60*time.Second, "Test duration")
	flag.IntVar(&config.Concurrency, "concurrency", 100, "Number of concurrent connections")
	flag.IntVar(&config.RequestsPerSec, "rps", 1000, "Target requests per second")
	flag.StringVar(&config.MetricsURL, "metrics", "http://localhost:8080/metrics", "Metrics endpoint URL")
	flag.BoolVar(&config.ValidateMetrics, "validate", true, "Validate metrics after test")
	flag.Parse()

	fmt.Printf("Starting load test:\n")
	fmt.Printf("  Target: %s\n", config.TargetURL)
	fmt.Printf("  Duration: %s\n", config.Duration)
	fmt.Printf("  Concurrency: %d\n", config.Concurrency)
	fmt.Printf("  Target RPS: %d\n", config.RequestsPerSec)
	fmt.Printf("  Metrics URL: %s\n", config.MetricsURL)
	fmt.Println()

	// Validate configuration
	if config.RequestsPerSec <= 0 {
		fmt.Fprintf(os.Stderr, "Error: RequestsPerSec must be greater than 0\n")
		os.Exit(1)
	}

	results := runLoadTest(config)
	printResults(results)

	if config.ValidateMetrics {
		fmt.Println("\nValidating metrics...")
		validateMetrics(config.MetricsURL)
	}
}

func runLoadTest(config LoadTestConfig) LoadTestResults {
	var results LoadTestResults
	var wg sync.WaitGroup

	// Calculate delay between requests for rate limiting
	delayBetweenRequests := time.Second / time.Duration(config.RequestsPerSec)

	startTime := time.Now()
	deadline := startTime.Add(config.Duration)

	// Create worker pool
	requestChan := make(chan struct{}, config.RequestsPerSec*2)
	
	// Start workers
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := &http.Client{
				Timeout: 5 * time.Second,
			}

			for range requestChan {
				atomic.AddInt64(&results.TotalRequests, 1)

				resp, err := client.Get(config.TargetURL)
				if err != nil {
					atomic.AddInt64(&results.FailedRequests, 1)
					continue
				}

				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()

				if resp.StatusCode == http.StatusOK {
					atomic.AddInt64(&results.SuccessRequests, 1)
				} else if resp.StatusCode == http.StatusTooManyRequests {
					atomic.AddInt64(&results.RateLimited, 1)
				} else {
					atomic.AddInt64(&results.FailedRequests, 1)
				}
			}
		}()
	}

	// Request generator
	ticker := time.NewTicker(delayBetweenRequests)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				if time.Now().After(deadline) {
					close(requestChan)
					return
				}
				requestChan <- struct{}{}
			}
		}
	}()

	wg.Wait()

	results.Duration = time.Since(startTime)
	if results.Duration.Seconds() > 0 {
		results.RequestsPerSec = float64(results.TotalRequests) / results.Duration.Seconds()
	}

	return results
}

func printResults(results LoadTestResults) {
	fmt.Println("Load Test Results:")
	fmt.Println("==================")
	fmt.Printf("Total Requests:     %d\n", results.TotalRequests)
	
	if results.TotalRequests > 0 {
		fmt.Printf("Success:            %d (%.2f%%)\n", results.SuccessRequests, 
			float64(results.SuccessRequests)/float64(results.TotalRequests)*100)
		fmt.Printf("Failed:             %d (%.2f%%)\n", results.FailedRequests,
			float64(results.FailedRequests)/float64(results.TotalRequests)*100)
		fmt.Printf("Rate Limited:       %d (%.2f%%)\n", results.RateLimited,
			float64(results.RateLimited)/float64(results.TotalRequests)*100)
	} else {
		fmt.Printf("Success:            %d\n", results.SuccessRequests)
		fmt.Printf("Failed:             %d\n", results.FailedRequests)
		fmt.Printf("Rate Limited:       %d\n", results.RateLimited)
	}
	
	fmt.Printf("Duration:           %s\n", results.Duration)
	fmt.Printf("Requests/sec:       %.2f\n", results.RequestsPerSec)
}

func validateMetrics(metricsURL string) {
	resp, err := http.Get(metricsURL)
	if err != nil {
		fmt.Printf("Failed to fetch metrics: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Metrics endpoint returned status: %d\n", resp.StatusCode)
		os.Exit(1)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Failed to read metrics: %v\n", err)
		os.Exit(1)
	}

	// Basic validation - check if expected metrics exist
	metricsBody := string(body)
	expectedMetrics := []string{
		"rate_limited",
		"rate_limited_drops",
		"rate_limited_tokens",
	}

	allFound := true
	for _, metric := range expectedMetrics {
		if !strings.Contains(metricsBody, metric) {
			fmt.Printf("Missing expected metric: %s\n", metric)
			allFound = false
		}
	}

	if allFound {
		fmt.Println("✓ All expected metrics are present")
		fmt.Println("✓ Metrics validation passed")
	} else {
		fmt.Println("✗ Metrics validation failed")
		os.Exit(1)
	}
}
