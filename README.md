# CTF-dlers

A high-performance CLI tool for downloading challenges from CTFd platforms with concurrent processing.

## Features

- **Concurrent Downloads**: Uses goroutines and worker pools for parallel processing
- **Bearer Token Authentication**: Access using CTFd access tokens
- **Directory Mapping**: Creates organized folder structure: `./challenges/[category]/[challenge_name]/`
- **Metadata Extraction**: Generates `challenge.yml` and `README.md` for each challenge
- **Robust Error Handling**: Handles authentication failures, rate limiting, and network errors
- **Rate Limiting**: Configurable request rate limiting to respect server limits
- **Resume Support**: Skip existing challenges to resume interrupted downloads

## Installation

```bash
git clone https://github.com/UmmItKin/CTF-dlers
cd CTF-dlers
just install
```

Or:

```bash
./install.sh
```

## Usage

```bash
# Download all challenges
./ctfd-downloader -url https://ctf.example.com -token ctfd_abc123def456

# Test connection
./ctfd-downloader -url https://ctf.example.com -token ctfd_abc123def456 -test

# Dry run to see what would be downloaded
./ctfd-downloader -url https://ctf.example.com -token ctfd_abc123def456 -dry-run
```

### Advanced Usage

```bash
# Use configuration file
./ctfd-downloader -config config.yml

# Customize workers and rate limiting
./ctfd-downloader -url https://ctf.example.com -token $CTFD_TOKEN -workers 10 -rate-limit 15

# Include hints and solves
./ctfd-downloader -url https://ctf.example.com -token $CTFD_TOKEN -hints -solves
```

### Command Line Options

| Flag | Description | Default |
|------|-------------|---------|
| `-url` | CTFd base URL (required) | - |
| `-token` | CTFd access token (required) | - |
| `-output` | Output directory | `./challenges` |
| `-config` | Configuration file path | - |
| `-workers` | Number of concurrent workers | `5` |
| `-rate-limit` | Rate limit (requests per second) | `10` |
| `-retry` | Number of retry attempts | `3` |
| `-retry-delay` | Delay between retries | `1s` |
| `-hints` | Include challenge hints | `false` |
| `-solves` | Include challenge solves | `false` |
| `-skip-existing` | Skip existing challenges | `true` |
| `-overwrite` | Overwrite existing files | `false` |
| `-verbose` | Enable verbose logging | `false` |
| `-test` | Test connection and exit | `false` |
| `-dry-run` | Show what would be downloaded | `false` |
| `-version` | Show version information | `false` |

### Environment Variables

- `CTFD_URL`: CTFd base URL
- `CTFD_TOKEN`: CTFd access token

## Configuration File

Create a YAML configuration file to avoid passing parameters on command line:

```yaml
base_url: "https://ctf.example.com"
token: "ctfd_abc123def456"
output_dir: "./challenges"
max_workers: 5
rate_limit: 10
retry_count: 3
retry_delay: "1s"
include_hints: false
include_solves: false
```

## Output Structure

The tool creates the following directory structure:

```
challenges/
├── category1/
│   ├── challenge1/
│   │   ├── challenge.yml      # Challenge metadata
│   │   ├── README.md         # Human-readable description
│   │   ├── file1.zip         # Challenge files
│   │   └── file2.txt
│   └── challenge2/
│       ├── challenge.yml
│       ├── README.md
│       └── exploit.py
└── category2/
    └── challenge3/
        ├── challenge.yml
        ├── README.md
        ├── binary
        └── source.c
```

### Metadata Format

Each challenge includes a `challenge.yml` file with comprehensive metadata:

```yaml
id: 123
name: "Buffer Overflow 1"
description: "Find the vulnerability in this program..."
category: "pwn"
value: 100
tags: ["binary", "stack"]
type: "standard"
state: "visible"
author: "challenge_author"
connection_info: "nc pwn.example.com 1337"
max_attempts: 0
files:
  - name: "vuln.c"
    url: "https://ctf.example.com/files/abc123.c"
    path: "vuln.c"
    size: 1024
    sha1: "da39a3ee5e6b4b0d3255bfef95601890afd80709"
downloaded_at: "2024-01-15T10:30:00Z"
```

## Authentication

The tool uses Bearer token authentication. Generate your token from:
1. Log into your CTFd instance
2. Go to Settings → Access Tokens
3. Generate a new token
4. Use the token with the `-token` flag or `CTFD_TOKEN` environment variable

## Error Handling

The tool handles various error conditions:

- **401 Unauthorized**: Invalid or expired token
- **403 Forbidden**: Insufficient permissions or CTF not started
- **429 Rate Limited**: Automatic retry with backoff
- **5xx Server Errors**: Automatic retry with exponential backoff
- **Network Errors**: Configurable retry with delay

## Performance Tuning

Adjust these parameters based on your server and network:

- **Workers**: Number of concurrent challenge processors (`-workers`)
- **Rate Limit**: Requests per second to avoid overwhelming the server (`-rate-limit`)
- **File Workers**: Concurrent file downloads per challenge (hardcoded to 3)
- **Retry Settings**: Number and delay for failed requests (`-retry`, `-retry-delay`)

## Examples

### Download from a public CTF
```bash
export CTFD_TOKEN="ctfd_abc123def456"
./ctfd-downloader -url https://demo.ctfd.io -test
./ctfd-downloader -url https://demo.ctfd.io -dry-run
./ctfd-downloader -url https://demo.ctfd.io
```

### High-performance download
```bash
./ctfd-downloader \
  -url https://ctf.example.com \
  -token $CTFD_TOKEN \
  -workers 15 \
  -rate-limit 25 \
  -output /opt/challenges
```

### Complete challenge archive
```bash
./ctfd-downloader \
  -url https://ctf.example.com \
  -token $CTFD_TOKEN \
  -hints \
  -solves \
  -verbose
```

## License

This project is licensed under the MIT License.
