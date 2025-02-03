# DomainSurvivor

**DomainSurvivor** is a fast, multithreaded domain scanner that identifies live domains. It checks for alive domains, matches specific HTTP status codes, and supports proxy rotation for improved anonymity.

Pair it with the **[RoundRobinizer](https://github.com/Victor-Security/RoundRobinizer)** script for balanced domain inputs and optimal scanning results.

---

## Features

- **Fast and Efficient**: Uses multithreading for concurrent scanning.
- **Customizable**: Adjust the number of workers, timeout, and response evaluation criteria via command-line options.
- **Proxy Support**: Supports rotating proxies configured via a `.env` file.
- **Baseline Comparison**: Optionally compare responses to a baseline to filter false positives.
- **Flexible**: Supports detection of live domains or domains matching specific HTTP status codes.
- **Detailed Output**: Logs domains that meet the specified criteria to an output file.
- **Drop Redirects**: Optionally prevent following redirects for more precise results.

---

## Installation

### Prerequisites

- **Go (1.19 or higher)**: Install Go from the [official site](https://go.dev/dl/).

### Building DomainSurvivor

1. Clone the repository:
   ```bash
   git clone https://github.com/Victor-Security/DomainSurvivor.git
   cd DomainSurvivor
   ```

2. Build the binary:
   ```bash
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
- `-o <file>`: Output file for domains matching criteria.
- `-t <number>`: Number of concurrent workers (default: 100).
- `-timeout <number>`: Timeout in seconds for each HTTP request (default: 5).
- `-status <number>`: HTTP status code to match (default: 200).
- `-alive`: Check for alive domains (any successful response).
- `-drop-redirects`: Drop redirected responses.
- `-new_connection`: Create a new HTTP connection for each request (useful for proxy rotation).
- `-log_fetch_ip`: Log the IP used for each request to verify IP rotation.
- `-h, --help`: Show the help message and exit.

### Proxy Configuration

Proxies can be set up using a `.env` file with the following format:

```
PROXY_ADDRESSES=dc.oxylabs.io:8001,dc.oxylabs.io:8002
PROXY_USERNAME=username
PROXY_PASSWORD=password
```

If no proxies are configured, DomainSurvivor will make direct connections.

### Examples

1. **Check for Specific Status Code**
   ```bash
   ./DomainSurvivor -l domainlist.txt -o results.txt -t 200 -timeout 10 -status 404
   ```

2. **Check for Alive Domains**
   ```bash
   ./DomainSurvivor -l domainlist.txt -o alivelist.txt -alive
   ```

3. **Use Proxy Rotation**
   ```bash
   ./DomainSurvivor -l domainlist.txt -o results.txt -new_connection -log_fetch_ip
   ```

---

## Example Output

### Input File (`domainlist.txt`):
```
example.com
test.com
google.com
nonexistent.example
```

### Command:
```bash
./DomainSurvivor -l domainlist.txt -o alive_domains.txt -t 200 -timeout 10
```

### Console Output:
```
DomainSurvivor: Find the Domains that Survive the Test of Time!
Effortlessly detect live domains with speed and precision.
Created by Victor Security (https://victorsecurity.com.br)

Scanning started...
Using 200 workers with a timeout of 10 seconds per request.
Match: example.com
Match: google.com
Scanning completed. Results saved to alive_domains.txt
```

### Output File (`alive_domains.txt`):
```
example.com
google.com
```

---

## Recommendations

1. **Preprocess with RoundRobinizer**:  
   Balance the input domain list using the **[RoundRobinizer](https://github.com/Victor-Security/RoundRobinizer)** script to ensure a fair distribution of domains across workers.

2. **Optimize Worker Count**:  
   Adjust the `-t` flag (number of workers) based on your system’s capabilities for optimal performance.

3. **Leverage Proxy Rotation**:  
   Use the `.env` proxy configuration along with `-new_connection` for rotating IPs dynamically.

4. **Adjust Timeout**:  
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

