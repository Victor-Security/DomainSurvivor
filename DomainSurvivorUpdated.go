package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/xrash/smetrics"
)

// Global variables.
var (
	dropRedirects bool
	httpClient    *http.Client
	// When enabled, log the IP used for each fetchURL request.
	logFetchIP bool

	// Proxy configuration loaded from .env
	proxies                      []string
	proxyIndex                   int
	proxyMu                      sync.Mutex
	proxyUsername, proxyPassword string
)

// loadProxyConfig loads proxy settings from the .env file.
func loadProxyConfig() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("No .env file found or error reading .env, proceeding without .env proxies")
	}
	proxiesEnv := os.Getenv("PROXY_ADDRESSES")
	if proxiesEnv != "" {
		// Expect a comma-separated list of proxy addresses.
		// e.g. PROXY_ADDRESSES=proxy1.example.com:8080,proxy2.example.com:8080
		proxies = strings.Split(proxiesEnv, ",")
	}
	proxyUsername = os.Getenv("PROXY_USERNAME")
	proxyPassword = os.Getenv("PROXY_PASSWORD")
}

// getNextProxyURL returns the next proxy URL in a round-robin fashion.
func getNextProxyURL() (*url.URL, error) {
	proxyMu.Lock()
	defer proxyMu.Unlock()

	if len(proxies) == 0 {
		// No proxies configured: return nil so that no proxy is used.
		return nil, nil
	}

	// Trim spaces in case there are extra spaces around commas.
	proxyAddr := strings.TrimSpace(proxies[proxyIndex])
	proxyIndex = (proxyIndex + 1) % len(proxies)

	// Build the URL.
	proxyURL := &url.URL{
		Scheme: "http", // Adjust the scheme if needed.
		Host:   proxyAddr,
	}
	if proxyUsername != "" && proxyPassword != "" {
		proxyURL.User = url.UserPassword(proxyUsername, proxyPassword)
	}
	return proxyURL, nil
}

// getHTTPClient returns an HTTP client. If proxy settings are available,
// it configures the transport to use a round-robin proxy function.
func getHTTPClient(timeout time.Duration, newConnection bool) *http.Client {
	transport := &http.Transport{
		// The Proxy field is set to a function that picks the next proxy.
		Proxy: func(req *http.Request) (*url.URL, error) {
			return getNextProxyURL()
		},
		MaxIdleConns:    100,
		MaxConnsPerHost: 100,
		IdleConnTimeout: 5 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: 5 * time.Second,
		}).DialContext,
		// When newConnection is true, disable keep-alives so each request uses a fresh connection.
		DisableKeepAlives: newConnection,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if dropRedirects {
				return http.ErrUseLastResponse // Prevent following redirects.
			}
			return nil
		},
	}
	return client
}

// getCurrentIP retrieves the current IP address by querying the IP service.
func getCurrentIP(client *http.Client) (string, error) {
	fmt.Println("Requesting current IP from https://ip.oxylabs.io/location")
	resp, err := client.Get("https://ip.oxylabs.io/location")
	if err != nil {
		return "", fmt.Errorf("failed to get current IP: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read IP response: %v", err)
	}

	ipResponse := string(body)
	fmt.Printf("Received IP response: %s\n", ipResponse)
	return ipResponse, nil
}

// fetchURL fetches and evaluates a URL.
func fetchURL(urlStr string, results chan<- string, wg *sync.WaitGroup, semaphore chan struct{},
	targetStatusCode int, checkAlive, useBaseline bool, baselineThreshold float64) {
	defer wg.Done()
	defer func() { <-semaphore }()

	baseline := ""
	if useBaseline {
		baseline = getBaseline(urlStr)
		if baseline == "" {
			return
		}
	}

	// Try both http and https.
	for _, protocol := range []string{"http", "https"} {
		targetURL := fmt.Sprintf("%s://%s", protocol, urlStr)
		resp, err := httpClient.Get(targetURL)
		if err != nil {
			fmt.Printf("Error fetching %s: %v\n", targetURL, err)
			continue
		}

		// Log the IP used for this request if enabled.
		if logFetchIP {
			ip, err := getCurrentIP(httpClient)
			if err != nil {
				fmt.Printf("Error getting fetch IP for %s: %v\n", targetURL, err)
			} else {
				fmt.Printf("Fetched %s using IP: %s\n", targetURL, ip)
			}
		}
		defer resp.Body.Close()

		if dropRedirects && (resp.StatusCode >= 300 && resp.StatusCode < 400) {
			fmt.Printf("Skipping redirect %s (%d)\n", targetURL, resp.StatusCode)
			return
		}

		if evaluateResponse(resp, baseline, targetStatusCode, checkAlive, useBaseline, baselineThreshold) {
			results <- urlStr
			return
		}
	}
}

// getBaseline generates baseline content for comparison.
func getBaseline(urlStr string) string {
	randomPath := randomString(12)
	randomParam := randomString(6)
	fullURL := fmt.Sprintf("http://%s?%s=%s", urlStr, randomParam, randomPath)

	resp, err := httpClient.Get(fullURL)
	if err != nil {
		fmt.Printf("Error fetching baseline for %s: %v\n", urlStr, err)
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading baseline response for %s: %v\n", urlStr, err)
		return ""
	}
	return string(body)
}

// evaluateResponse checks if the HTTP response meets the desired criteria.
func evaluateResponse(resp *http.Response, baseline string, targetStatusCode int,
	checkAlive, useBaseline bool, baselineThreshold float64) bool {
	if checkAlive {
		return true // Any valid response counts when checkAlive is enabled.
	}

	if resp.StatusCode == targetStatusCode {
		if useBaseline {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				fmt.Printf("Error reading response body: %v\n", err)
				return false
			}
			// Compare response content with the baseline.
			similarity := smetrics.JaroWinkler(baseline, string(body), 0.7, 4)
			fmt.Printf("Similarity: %f for %s\n", similarity, resp.Request.URL)
			return similarity < baselineThreshold
		}
		return true
	}

	return false
}

// randomString generates a random alphanumeric string.
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

// processBatch processes a batch of URLs concurrently.
func processBatch(batch []string, results chan<- string, wg *sync.WaitGroup, semaphore chan struct{},
	targetStatusCode int, checkAlive, useBaseline bool, baselineThreshold float64) {
	for _, urlStr := range batch {
		wg.Add(1)
		semaphore <- struct{}{}
		go fetchURL(urlStr, results, wg, semaphore, targetStatusCode, checkAlive, useBaseline, baselineThreshold)
	}
}

func main() {
	// Command-line flags.
	inputFile := flag.String("l", "", "Input file containing a list of domains")
	outputFile := flag.String("o", "", "Output file for domains matching criteria")
	numWorkers := flag.Int("t", 100, "Number of concurrent workers")
	timeoutSeconds := flag.Int("timeout", 5, "Timeout in seconds for each HTTP request")
	targetStatusCode := flag.Int("status", 200, "HTTP status code to match")
	checkAlive := flag.Bool("alive", false, "Check for alive domains (any successful response)")
	useBaseline := flag.Bool("baseline", false, "Enable baseline comparison")
	baselineThreshold := flag.Float64("threshold", 0.9, "Baseline similarity threshold")
	dropRedirectsFlag := flag.Bool("drop-redirects", false, "Drop redirected responses")
	newConnectionFlag := flag.Bool("new_connection", false, "Create a new HTTP connection for each host to allow IP rotation")
	logFetchIPFlag := flag.Bool("log_fetch_ip", false, "Log the IP used for each fetchURL request to verify IP rotation")
	showHelp := flag.Bool("h", false, "Show help message")
	flag.Parse()

	if *showHelp || flag.NFlag() == 0 {
		fmt.Println("Usage: [options]")
		flag.PrintDefaults()
		os.Exit(0)
	}
	dropRedirects = *dropRedirectsFlag
	logFetchIP = *logFetchIPFlag
	timeoutDuration := time.Duration(*timeoutSeconds) * time.Second

	// Load proxy configuration from .env (if available).
	loadProxyConfig()

	// Initialize the httpClient.
	// If proxies are configured via .env, our transport will use the round-robin proxy function.
	httpClient = getHTTPClient(timeoutDuration, *newConnectionFlag)

	// Validate required file flags.
	if *inputFile == "" || *outputFile == "" {
		fmt.Println("Error: Both input file (-l) and output file (-o) are required.")
		os.Exit(1)
	}

	// Open input and output files.
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

	// Start result writer goroutine.
	go func() {
		for result := range results {
			_, err := output.WriteString(result + "\n")
			if err != nil {
				fmt.Printf("Error writing to output file: %v\n", err)
			}
		}
	}()

	batchSize := 1000 // Adjust as needed.
	var batch []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		batch = append(batch, scanner.Text())
		if len(batch) >= batchSize {
			processBatch(batch, results, &wg, semaphore, *targetStatusCode, *checkAlive, *useBaseline, *baselineThreshold)
			batch = nil // free memory after processing
		}
	}
	if len(batch) > 0 {
		processBatch(batch, results, &wg, semaphore, *targetStatusCode, *checkAlive, *useBaseline, *baselineThreshold)
	}
	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading input file: %v\n", err)
		os.Exit(1)
	}

	// Wait for all goroutines to finish.
	wg.Wait()
	close(results)

	fmt.Printf("Scanning completed. Results saved to %s\n", *outputFile)
}
