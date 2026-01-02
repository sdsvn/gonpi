# gonpi

[![Go Reference](https://pkg.go.dev/badge/github.com/sdsvn/gonpi.svg)](https://pkg.go.dev/github.com/sdsvn/gonpi)
[![Go Report Card](https://goreportcard.com/badge/github.com/sdsvn/gonpi)](https://goreportcard.com/report/github.com/sdsvn/gonpi)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go](https://github.com/sdsvn/gonpi/actions/workflows/go.yml/badge.svg)](https://github.com/sdsvn/gonpi/actions/workflows/go.yml)

**Unofficial**, production-ready Go client for the [CMS NPI Registry API v2.1](https://npiregistry.cms.hhs.gov/api-page) with retry logic, caching, and batch operations.

## Installation

```bash
go get github.com/sdsvn/gonpi
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/sdsvn/gonpi"
)

func main() {
    client := gonpi.NewClient()
    
    // Get provider by NPI
    provider, err := client.GetProviderByNPI(context.Background(), "1043218118")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Provider: %s %s\n", provider.Basic.FirstName, provider.Basic.LastName)
    
    // Search providers
    results, err := client.SearchProviders(context.Background(), gonpi.SearchOptions{
        LastName: "Smith",
        State:    "CA",
        Limit:    10,
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Found %d providers\n", len(results))
}
```

## Features

- **Simple API**: Clean, idiomatic Go interface
- **Production-ready**: Exponential backoff retry, configurable timeouts
- **Caching**: Optional in-memory cache with TTL
- **Batch operations**: Concurrent NPI lookups
- **Full API coverage**: All NPI Registry v2.1 search filters
- **OpenTelemetry tracing**: Built-in distributed tracing support
- **Well-tested**: 86.7% test coverage

## Configuration

```go
client := gonpi.NewClient(
    gonpi.WithCache(5 * time.Minute),
    gonpi.WithRetry(gonpi.RetryConfig{
        MaxRetries:   3,
        InitialDelay: 100 * time.Millisecond,
    }),
)
```

### OpenTelemetry Tracing

Tracing is automatically enabled using the global OpenTelemetry tracer. Configure your tracer provider at the application level:

```go
// All client operations are automatically traced
client := gonpi.NewClient()

// Operations include detailed spans
provider, err := client.GetProviderByNPI(ctx, "1043218118")
```

Traces include spans for:
- Provider lookups and searches
- Cache hits/misses
- Retry attempts with errors
- HTTP requests with status codes
- Batch operations

## Documentation

- **[API Reference](https://pkg.go.dev/github.com/sdsvn/gonpi)** - Complete package documentation
- **[Quick Start Guide](QUICKSTART.md)** - Detailed examples and usage patterns
- **[Examples](examples/)** - Working code samples
- **[Search Options](types.go)** - Available filters (name, location, specialty, etc.)

## Testing

```bash
go test -v -cover
```

## Resources

- [NPI Registry API Documentation](https://npiregistry.cms.hhs.gov/api-page)
- [CMS NPI Registry](https://npiregistry.cms.hhs.gov/)

## License

MIT License - see [LICENSE](LICENSE) file for details.

---

*Not affiliated with CMS or the U.S. Department of Health and Human Services.*
