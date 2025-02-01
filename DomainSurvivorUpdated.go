package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/xrash/smetrics"
)

// Optimized HTTP client with connection pooling
var httpClient = &http.Client{
	Timeout: 5 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:    500,
		MaxConnsPerHost: 5,
		IdleConnTimeout: 5 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 5 * time.Second,
		}).DialContext,
		DisableKeepAlives: false,
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if dropRedirects {
			return http.ErrUseLastResponse // Prevents following redirects
		}
		return nil
	},
}

var dropRedirects bool

// Fetch and evaluate a URL
// Fetch and evaluate a URL
func fetchURL(url string, results chan<- string, wg *sync.WaitGroup, semaphore chan struct{}, targetStatusCode int, checkAlive, useBaseline bool, baselineThreshold float64) {
	defer wg.Done()
	defer func() { <-semaphore }()

	baseline := ""
	if useBaseline {
		baseline = getBaseline(url)
		if baseline == "" {
			return
		}
	}

	for _, protocol := range []string{"http", "https"} {
		resp, err := httpClient.Get(fmt.Sprintf("%s://%s", protocol, url))
		if err != nil {
			fmt.Printf("Error fetching %s: %v\n", url, err)
			continue
		}
		defer resp.Body.Close()

		if dropRedirects && (resp.StatusCode >= 300 && resp.StatusCode < 400) {
			fmt.Printf("Skipping redirect %s (%d)\n", url, resp.StatusCode)
			return
		}

		if evaluateResponse(resp, baseline, targetStatusCode, checkAlive, useBaseline, baselineThreshold) {
			results <- url
			return
		}
	}
}

// Generate baseline content for comparison
func getBaseline(url string) string {
	randomPath := randomString(12)
	randomParam := randomString(6)
	fullURL := fmt.Sprintf("http://%s?%s=%s", url, randomParam, randomPath)

	resp, err := httpClient.Get(fullURL)
	if err != nil {
		fmt.Printf("Error fetching baseline for %s: %v\n", url, err)
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading baseline response for %s: %v\n", url, err)
		return ""
	}
	return string(body)
}

// Evaluate the response based on criteria
func evaluateResponse(resp *http.Response, baseline string, targetStatusCode int, checkAlive, useBaseline bool, baselineThreshold float64) bool {
	if checkAlive {
		return true // Return true for any valid response if checkAlive is enabled
	}

	if resp.StatusCode == targetStatusCode {
		if useBaseline {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				fmt.Printf("Error reading response body: %v\n", err)
				return false
			}
			// Compare response content with the baseline
			similarity := smetrics.JaroWinkler(baseline, string(body), 0.7, 4)
			fmt.Printf("Similarity: %f for %s\n", similarity, resp.Request.URL)
			return similarity < baselineThreshold
		}
		return true // If baseline isn't used, just check the status code
	}

	return false
}

// Utility function for absolute difference
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// Generate a random alphanumeric string
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func processBatch(batch []string, results chan<- string, wg *sync.WaitGroup, semaphore chan struct{}, targetStatusCode int, checkAlive, useBaseline bool, baselineThreshold float64) {
	for _, url := range batch {
		wg.Add(1)
		semaphore <- struct{}{}
		go fetchURL(url, results, wg, semaphore, targetStatusCode, checkAlive, useBaseline, baselineThreshold)
	}
}

func main() {
	// Command-line flags
	// Command-line flags
	inputFile := flag.String("l", "", "Input file containing a list of domains")
	outputFile := flag.String("o", "", "Output file for domains matching criteria")
	numWorkers := flag.Int("t", 100, "Number of concurrent workers")
	timeoutSeconds := flag.Int("timeout", 5, "Timeout in seconds for each HTTP request")
	targetStatusCode := flag.Int("status", 200, "HTTP status code to match")
	checkAlive := flag.Bool("alive", false, "Check for alive domains (any successful response)")
	useBaseline := flag.Bool("baseline", false, "Enable baseline comparison")
	baselineThreshold := flag.Float64("threshold", 0.9, "Baseline similarity threshold")
	dropRedirectsFlag := flag.Bool("drop-redirects", false, "Drop redirected responses")
	showHelp := flag.Bool("h", false, "Show help message")
	flag.Parse()

	if *showHelp || flag.NFlag() == 0 {
		fmt.Println("Usage: [options]")
		flag.PrintDefaults()
		os.Exit(0)
	}

	dropRedirects = *dropRedirectsFlag
	httpClient.Timeout = time.Duration(*timeoutSeconds) * time.Second

	if *inputFile == "" || *outputFile == "" {
		fmt.Println("Error: Both input file (-l) and output file (-o) are required.")
		os.Exit(1)
	}

	httpClient.Timeout = time.Duration(*timeoutSeconds) * time.Second

	// Open input and output files
	file, err := os.Open(*inputFile)
	if err != nil {
		fmt.Printf("Error opening input file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	output, err := os.Create(*outputFile)
	if err != nil {
		fmt.Printf("Error creating output file: %v\n", err)
		os.Exit(1)
	}
	defer output.Close()

	results := make(chan string)
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, *numWorkers)

	// Start result writer
	go func() {
		for result := range results {
			_, err := output.WriteString(result + "\n")
			if err != nil {
				fmt.Printf("Error writing to output file: %v\n", err)
			}
		}
	}()

	batchSize := 1000 // Adjust as needed
	var batch []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		batch = append(batch, scanner.Text())

		if len(batch) >= batchSize {
			processBatch(batch, results, &wg, semaphore, *targetStatusCode, *checkAlive, *useBaseline, *baselineThreshold)
			batch = nil // Free memory after processing
		}
	}

	// Process remaining batch if not empty
	if len(batch) > 0 {
		processBatch(batch, results, &wg, semaphore, *targetStatusCode, *checkAlive, *useBaseline, *baselineThreshold)
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading input file: %v\n", err)
		os.Exit(1)
	}

	// Wait for all goroutines to finish
	wg.Wait()
	close(results)

	fmt.Printf("Scanning completed. Results saved to %s\n", *outputFile)
}
