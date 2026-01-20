package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/sdsvn/gonpi"
)

func main() {
	// accept NPI as command line argument
	npi := os.Args[1]
	// Create a new NPI Registry client with caching enabled
	client := gonpi.NewClient(
		gonpi.WithCache(5 * 60 * 1000000000), // 5 minutes in nanoseconds
	)
	ctx := context.Background()
	exampleLookupByNPI(ctx, client, npi)
}

// exampleLookupByNPI demonstrates looking up a provider by NPI number.
func exampleLookupByNPI(ctx context.Context, client *gonpi.Client, npi string) {
	provider, err := client.GetProviderByNPI(ctx, npi)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}
	fmt.Println(provider.FullName())
}
