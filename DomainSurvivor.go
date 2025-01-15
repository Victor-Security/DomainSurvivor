package main

import (
	"bufio"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

// Worker function to process hosts
func worker(_ int, jobs <-chan string, results chan<- string, wg *sync.WaitGroup, timeout time.Duration) {
	defer wg.Done()
	client := &http.Client{
		Timeout: timeout,
	}
	for host := range jobs {
		if isAlive(client, host) {
			results <- host
		}
	}
}

// Check if a domain is alive by making an HTTP GET request
func isAlive(client *http.Client, host string) bool {
	// Try HTTP
	_, err := client.Get(fmt.Sprintf("http://%s", host))
	if err == nil {
		return true
	}

	// Try HTTPS if HTTP fails
	_, err = client.Get(fmt.Sprintf("https://%s", host))
	return err == nil
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
	fmt.Println("  -o <file>         Output file for alive domains")
	fmt.Println("  -t <number>       Number of concurrent workers (default: 100)")
	fmt.Println("  -timeout <number> Timeout in seconds for each HTTP request (default: 5)")
	fmt.Println("  -h, --help        Show this help message")
	fmt.Println()
	fmt.Println("Example:")
	fmt.Println("  ./domainsurvivor -l domainlist.txt -o alivelist.txt -t 200 -timeout 10")
}

func main() {
	// Command-line flags
	inputFile := flag.String("l", "", "Input file containing a list of domains")
	outputFile := flag.String("o", "", "Output file for alive domains")
	numWorkers := flag.Int("t", 100, "Number of concurrent workers")
	timeoutSeconds := flag.Int("timeout", 5, "Timeout in seconds for each HTTP request")
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
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Printf("Error closing input file: %v\n", err)
		}
	}()

	// Create the output file
	output, err := os.Create(*outputFile)
	if err != nil {
		fmt.Printf("Error creating output file: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := output.Close(); err != nil {
			fmt.Printf("Error closing output file: %v\n", err)
		}
	}()

	// Prepare channels
	jobs := make(chan string, *numWorkers)
	results := make(chan string, *numWorkers)

	// Read hosts into a slice
	var hosts []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		hosts = append(hosts, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading input file: %v\n", err)
		os.Exit(1)
	}

	// Create worker pool
	var wg sync.WaitGroup
	for i := 0; i < *numWorkers; i++ {
		wg.Add(1)
		go worker(i, jobs, results, &wg, timeout)
	}

	// Start a goroutine to close the results channel after all workers are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Feed jobs to workers
	go func() {
		for _, host := range hosts {
			jobs <- host
		}
		close(jobs)
	}()

	// Collect results
	printBanner()
	fmt.Println("Scanning started...")
	fmt.Printf("Using %d workers with a timeout of %d seconds per request.\n", *numWorkers, *timeoutSeconds)

	for result := range results {
		fmt.Printf("Alive: %s\n", result)
		if _, err := output.WriteString(result + "\n"); err != nil {
			fmt.Printf("Error writing to output file: %v\n", err)
		}
	}

	fmt.Println("Scanning completed. Results saved to", *outputFile)
}
