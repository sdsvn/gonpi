# gonpi

[![Go Reference](https://pkg.go.dev/badge/github.com/sarathsadasivanpillai/gonpi.svg)](https://pkg.go.dev/github.com/sarathsadasivanpillai/gonpi)
[![Go Report Card](https://goreportcard.com/badge/github.com/sarathsadasivanpillai/gonpi)](https://goreportcard.com/report/github.com/sarathsadasivanpillai/gonpi)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Unofficial**, production-ready Go client for the [CMS NPI Registry API v2.1](https://npiregistry.cms.hhs.gov/api-page) with retry logic, caching, and batch operations.

## Installation

```bash
go get github.com/sarathsadasivanpillai/gonpi
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/sarathsadasivanpillai/gonpi"
)

func main() {
    client := npiregistry.NewClient()
    
    // Get provider by NPI
    provider, err := client.GetProviderByNPI(context.Background(), "1043218118")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Provider: %s %s\n", provider.Basic.FirstName, provider.Basic.LastName)
    
    // Search providers
    results, err := client.SearchProviders(context.Background(), npiregistry.SearchOptions{
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
- **Well-tested**: 94.6% test coverage

## Configuration

```go
client := npiregistry.NewClient(
    npiregistry.WithCache(5 * time.Minute),
    npiregistry.WithRetry(npiregistry.RetryConfig{
        MaxRetries:   3,
        InitialDelay: 100 * time.Millisecond,
    }),
)
```

## Documentation

- **[API Reference](https://pkg.go.dev/github.com/sarathsadasivanpillai/gonpi)** - Complete package documentation
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
