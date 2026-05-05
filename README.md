# URL Checker (Go)

A lightweight Go tool that crawls a website starting from a base URL and validates all discovered links (`href`s).

It recursively scrapes pages and checks links for errors, performing both operations concurrently for improved performance.

## Requirements

- Minimum **1.26.2** (or higher)

## Usage

Run the tool from your terminal:

```bash
go run main.go <your_url>
```

### Example

```bash
go run main.go https://example.com
```

(If the https:// prefix is not added the tool will add it automatically, so example.com works too)

## Features

- Recursive crawling starting from a base URL
- Concurrent scraping and link validation
- Detection of broken links
- Optional verbose output
- Optional JSON export

## Flags

### `--verbose`

By default, the tool only outputs broken links.
Use this flag to display **all checked URLs**, including valid ones:

```bash
go run main.go <your_url> --verbose
```

### `--json`

Outputs the results in JSON format:

```bash
go run main.go <your_url> --json
```

You can redirect the output into a file:

```bash
go run main.go <your_url> --json > results.json
```

## Notes

- The crawler follows links recursively starting from the provided base URL.
- Scraping and validation are executed concurrently to improve speed.
- Large websites may take longer to process depending on their size and structure.
