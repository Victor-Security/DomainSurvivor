# DomainSurvivor

**DomainSurvivor** is a fast, multithreaded domain scanner that identifies live domains. It checks if domains respond to HTTP or HTTPS requests, regardless of the response code, and logs the active ones.

Pair it with the **[RoundRobinizer](https://github.com/Victor-Security/RoundRobinizer)** script for balanced domain inputs and optimal scanning results.

---

## Features

- **Fast and Efficient**: Uses multithreading for concurrent scanning.
- **Customizable**: Adjust the number of workers and timeout via command-line options.
- **Simple Output**: Logs live domains to a specified output file.
- **Flexible**: Detects live domains regardless of the HTTP response code.

---

## Installation

### Prerequisites

- **Go (1.19 or higher)**: Install Go from the [official site](https://go.dev/dl/).

### Building DomainSurvivor

1. Clone the repository:
   ```
   git clone https://github.com/Victor-Security/DomainSurvivor.git
   cd DomainSurvivor
   ```

2. Build the binary:
   ```
   go build -o DomainSurvivor
   ```

3. Verify the build:
   ```bash
   ./DomainSurvivor -h
   ```

---

## Usage

### Command-Line Options

- `-l <file>`: Input file containing a list of domains (one per line).
- `-o <file>`: Output file for alive domains.
- `-t <number>`: Number of concurrent workers (default: 100).
- `-timeout <number>`: Timeout in seconds for each HTTP request (default: 5).
- `-h, --help`: Show the help message and exit.

### Example

1. **Preprocess the Input File**  
   Use the **[RoundRobinizer](https://github.com/Victor-Security/RoundRobinizer)** script to balance domain inputs:
   ```
   python RoundRobinizer.py -i domainlist.txt -o balanced_domains.txt
   ```

2. **Run DomainSurvivor**  
   Scan the balanced domain list:
   ```
   ./DomainSurvivor -l balanced_domains.txt -o alive_domains.txt -t 200 -timeout 10
   ```

3. **Results**  
   The output file (`alive_domains.txt`) will contain all detected live domains.

---

## Example Output

### Input File (`balanced_domains.txt`):
```
example.com
test.com
google.com
nonexistent.example
```

### Command:
```
./DomainSurvivor -l balanced_domains.txt -o alive_domains.txt -t 200 -timeout 10
```

### Console Output:
```
Scanning started...
Using 200 workers with a timeout of 10 seconds per request.
Alive: example.com
Alive: google.com
Scanning completed. Results saved to alive_domains.txt
```

### Output File (`alive_domains.txt`):
```
example.com
google.com
```

---

## Recommendations

For best results:
1. **Preprocess with RoundRobinizer**:
   Balance the input domain list using the **[RoundRobinizer](https://github.com/Victor-Security/RoundRobinizer)** script to ensure a fair distribution of domains across workers.
2. **Optimize Worker Count**:
   Adjust the `-t` flag (number of workers) based on your system’s capabilities for optimal performance.
3. **Adjust Timeout**:
   Use the `-timeout` flag to handle slow or distant servers.

---

## Contributing

Contributions are welcome! Feel free to:
- Fork the repository.
- Submit pull requests with features or fixes.
- Open issues to suggest improvements.

---

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

---

## About

**DomainSurvivor** was created by [Victor Security](https://victorsecurity.com.br) to simplify live domain detection for web explorers and security professionals.

