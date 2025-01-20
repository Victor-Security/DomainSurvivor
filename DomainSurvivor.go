package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/xrash/smetrics"
)

// Worker function to process hosts
func worker(jobs <-chan string, results chan<- string, wg *sync.WaitGroup, timeout time.Duration, targetStatusCode int, checkAlive, useBaseline bool, baselineThreshold float64) {
	defer wg.Done()
	client := &http.Client{
		Timeout: timeout,
	}

	for host := range jobs {
		if checkDomain(client, host, targetStatusCode, checkAlive, useBaseline, baselineThreshold) {
			results <- host
		}
	}
}

// Check if a domain meets the criteria (alive, specific status code, or baseline similarity)
func checkDomain(client *http.Client, host string, targetStatusCode int, checkAlive, useBaseline bool, baselineThreshold float64) bool {
	baseline := ""
	if useBaseline {
		baseline = getBaseline(client, host)
		if baseline == "" {
			return false
		}
	}

	for _, protocol := range []string{"http", "https"} {
		resp, err := client.Get(fmt.Sprintf("%s://%s", protocol, host))
		if err != nil {
			fmt.Printf("Error fetching %s: %v\n", host, err)
			continue
		}
		defer resp.Body.Close()

		if evaluateResponse(resp, baseline, targetStatusCode, checkAlive, useBaseline, baselineThreshold) {
			return true
		}
	}

	return false
}

// Get the baseline content by appending a random string to the host
func getBaseline(client *http.Client, host string) string {
	randomString := randomString(12)
	url := fmt.Sprintf("http://%s/%s", host, randomString)
	resp, err := client.Get(url)
	if err != nil {
		fmt.Printf("Error fetching baseline for %s: %v\n", host, err)
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body) // Updated from ioutil.ReadAll to io.ReadAll
	if err != nil {
		fmt.Printf("Error reading baseline response for %s: %v\n", host, err)
		return ""
	}
	return string(body)
}

// Evaluate the response against the criteria
func evaluateResponse(resp *http.Response, baseline string, targetStatusCode int, checkAlive, useBaseline bool, baselineThreshold float64) bool {
	if checkAlive {
		return true
	}
	if resp.StatusCode == targetStatusCode {
		if useBaseline {
			body, err := io.ReadAll(resp.Body) // Updated from ioutil.ReadAll to io.ReadAll
			if err != nil {
				fmt.Printf("Error reading response body: %v\n", err)
				return false
			}
			similarity := smetrics.JaroWinkler(baseline, string(body), 0.7, 4)
			fmt.Printf("Similarity: %f\n", similarity)
			return similarity < baselineThreshold
		}
		return true
	}
	return false
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

// Print a cool banner with tool details
func printBanner() {
	fmt.Println(`
DomainSurvivor: Find the Domains that Survive the Test of Time!
Effortlessly detect live domains with speed and precision.
Created by Victor Security (https://victorsecurity.com.br)
`)
}

// Print detailed usage information
func printUsage() {
	fmt.Println("Usage: DomainSurvivor [options]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -l <file>         Input file containing a list of domains (one per line)")
	fmt.Println("  -o <file>         Output file for domains matching criteria")
	fmt.Println("  -t <number>       Number of concurrent workers (default: 100)")
	fmt.Println("  -timeout <number> Timeout in seconds for each HTTP request (default: 5)")
	fmt.Println("  -status <number>  HTTP status code to match (default: 200)")
	fmt.Println("  -alive            Check for alive domains (any successful response)")
	fmt.Println("  -baseline         Enable baseline comparison (default: false)")
	fmt.Println("  -threshold <num>  Baseline similarity threshold (default: 0.9)")
	fmt.Println("  -h, --help        Show this help message")
	fmt.Println()
	fmt.Println("Example:")
	fmt.Println("  ./domainsurvivor -l domainlist.txt -o results.txt -t 200 -timeout 10 -status 404")
	fmt.Println("  ./domainsurvivor -l domainlist.txt -o alivelist.txt -alive")
	fmt.Println("  ./domainsurvivor -l domainlist.txt -o filtered.txt -baseline -threshold 0.85")
}

func main() {
	// Command-line flags
	inputFile := flag.String("l", "", "Input file containing a list of domains")
	outputFile := flag.String("o", "", "Output file for domains matching criteria")
	numWorkers := flag.Int("t", 100, "Number of concurrent workers")
	timeoutSeconds := flag.Int("timeout", 5, "Timeout in seconds for each HTTP request")
	targetStatusCode := flag.Int("status", 200, "HTTP status code to match")
	checkAlive := flag.Bool("alive", false, "Check for alive domains (any successful response)")
	useBaseline := flag.Bool("baseline", false, "Enable baseline comparison")
	baselineThreshold := flag.Float64("threshold", 0.9, "Baseline similarity threshold")
	showHelp := flag.Bool("h", false, "Show help message")
	flag.Parse()

	// If help is requested, print the banner and usage, then exit
	if *showHelp || flag.NFlag() == 0 {
		printBanner()
		printUsage()
		os.Exit(0)
	}

	// Validate flags
	if *inputFile == "" || *outputFile == "" {
		fmt.Println("Error: Both input file (-l) and output file (-o) are required.")
		fmt.Println("Use -h or --help for more information.")
		os.Exit(1)
	}

	// Parse timeout as a duration
	timeout := time.Duration(*timeoutSeconds) * time.Second

	// Open the input file
	file, err := os.Open(*inputFile)
	if err != nil {
		fmt.Printf("Error opening input file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	// Create the output file
	output, err := os.Create(*outputFile)
	if err != nil {
		fmt.Printf("Error creating output file: %v\n", err)
		os.Exit(1)
	}
	defer output.Close()

	// Prepare channels
	jobs := make(chan string)
	results := make(chan string)

	// Create worker pool
	var wg sync.WaitGroup
	for i := 0; i < *numWorkers; i++ {
		wg.Add(1)
		go worker(jobs, results, &wg, timeout, *targetStatusCode, *checkAlive, *useBaseline, *baselineThreshold)
	}

	// Start a goroutine to close the results channel after all workers are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Feed jobs to workers directly from the file
	go func() {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			jobs <- scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			fmt.Printf("Error reading input file: %v\n", err)
		}
		close(jobs)
	}()

	// Collect results
	printBanner()
	fmt.Println("Scanning started...")
	fmt.Printf("Using %d workers with a timeout of %d seconds per request.\n", *numWorkers, *timeoutSeconds)

	for result := range results {
		fmt.Printf("Match: %s\n", result)
		if _, err := output.WriteString(result + "\n"); err != nil {
			fmt.Printf("Error writing to output file: %v\n", err)
		}
	}

	fmt.Println("Scanning completed. Results saved to", *outputFile)
}
