package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/xrash/smetrics"
)

const defaultMaxConcurrent = 100 // Default number of concurrent requests

// Optimized HTTP client with connection pooling
var httpClient = &http.Client{
	Timeout: 5 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:    1000,
		MaxConnsPerHost: 10,
		IdleConnTimeout: 10 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 10 * time.Second,
		}).DialContext,
	},
}

// Fetch a URL and evaluate the response
func fetchURL(url string, results chan<- string, wg *sync.WaitGroup, semaphore chan struct{}, targetStatusCode int, checkAlive, useBaseline bool, baseline string, baselineThreshold float64) {
	defer wg.Done()
	defer func() { <-semaphore }()

	// Make the request
	resp, err := httpClient.Get(url)
	if err != nil {
		fmt.Printf("Error fetching %s: %v\n", url, err)
		return
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response body for %s: %v\n", url, err)
		return
	}

	// Evaluate the response
	if checkAlive || (resp.StatusCode == targetStatusCode) {
		if useBaseline {
			similarity := smetrics.JaroWinkler(baseline, string(body), 0.7, 4)
			if similarity < baselineThreshold {
				results <- url
			}
		} else {
			results <- url
		}
	}
}

func monitorMemory() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc / 1024 / 1024
}

func main() {
	// Command-line flags
	inputFile := flag.String("l", "", "Input file containing a list of domains")
	outputFile := flag.String("o", "", "Output file for domains matching criteria")
	numWorkers := flag.Int("t", defaultMaxConcurrent, "Number of concurrent workers")
	timeoutSeconds := flag.Int("timeout", 5, "Timeout in seconds for each HTTP request")
	targetStatusCode := flag.Int("status", 200, "HTTP status code to match")
	checkAlive := flag.Bool("alive", false, "Check for alive domains (any successful response)")
	useBaseline := flag.Bool("baseline", false, "Enable baseline comparison")
	baselineThreshold := flag.Float64("threshold", 0.9, "Baseline similarity threshold")
	showHelp := flag.Bool("h", false, "Show help message")
	flag.Parse()

	// Show help and exit if no flags or help flag is used
	if *showHelp || flag.NFlag() == 0 {
		fmt.Println("Usage: [options]")
		fmt.Println("Options:")
		flag.PrintDefaults()
		os.Exit(0)
	}

	// Validate input and output file flags
	if *inputFile == "" || *outputFile == "" {
		fmt.Println("Error: Both input file (-l) and output file (-o) are required.")
		os.Exit(1)
	}

	// Configure HTTP client timeout
	httpClient.Timeout = time.Duration(*timeoutSeconds) * time.Second

	// Open input file
	file, err := os.Open(*inputFile)
	if err != nil {
		fmt.Printf("Error opening input file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	// Open output file
	output, err := os.Create(*outputFile)
	if err != nil {
		fmt.Printf("Error creating output file: %v\n", err)
		os.Exit(1)
	}
	defer output.Close()

	// Read URLs from input file
	urls := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		url := scanner.Text()
		if url != "" {
			urls = append(urls, url)
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading input file: %v\n", err)
		os.Exit(1)
	}

	// If baseline is enabled, fetch baseline content
	baseline := ""
	if *useBaseline {
		fmt.Println("Baseline comparison enabled.")
		baseline = "example baseline content" // Replace with actual logic if needed
	}

	// Prepare concurrency controls
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, *numWorkers)
	results := make(chan string)

	// Start result writer
	go func() {
		for result := range results {
			_, err := output.WriteString(result + "\n")
			if err != nil {
				fmt.Printf("Error writing to output file: %v\n", err)
			}
		}
	}()

	// Track memory usage and start scanning
	fmt.Printf("Total URLs: %d\n", len(urls))
	fmt.Printf("Initial Memory Usage: %d MB\n", monitorMemory())

	start := time.Now()

	// Launch goroutines for each URL
	for _, url := range urls {
		wg.Add(1)
		semaphore <- struct{}{}
		go fetchURL(url, results, &wg, semaphore, *targetStatusCode, *checkAlive, *useBaseline, baseline, *baselineThreshold)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(results)

	// Report statistics
	duration := time.Since(start)
	fmt.Printf("Final Memory Usage: %d MB\n", monitorMemory())
	fmt.Printf("Execution Time: %v\n", duration)
	fmt.Printf("Results saved to: %s\n", *outputFile)
}
