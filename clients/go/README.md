# Kodit Go Client

Go client library for the Kodit API, auto-generated from the OpenAPI specification.

## Installation

```bash
go get github.com/helixml/kodit/clients/go
```

## Usage

```go
import (
    kodit "github.com/helixml/kodit/clients/go"
)

// Create a new client
client, err := kodit.NewClient("https://your-kodit-instance.com")
if err != nil {
    log.Fatal(err)
}

// Use the client
// Example code here
```

## Generation

This client is auto-generated from the OpenAPI specification. To regenerate:

```bash
make generate-go-client
```

Or run the script directly:

```bash
./scripts/generate-go-client.sh
```
