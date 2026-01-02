// Package npiregistry provides a comprehensive Go client for the CMS NPI Registry API.
//
// The National Provider Identifier (NPI) is a unique identification number for covered
// health care providers. This library allows you to search and retrieve provider information
// from the NPI Registry maintained by the Centers for Medicare & Medicaid Services (CMS).
//
// # Quick Start
//
// Create a new client and look up a provider by NPI:
//
//	client := npiregistry.NewClient()
//	provider, err := client.GetProviderByNPI(context.Background(), "1043218118")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Provider: %s %s\n", provider.Basic.FirstName, provider.Basic.LastName)
//
// # Searching Providers
//
// Search for providers using various filters:
//
//	opts := npiregistry.SearchOptions{
//	    FirstName: "John",
//	    LastName:  "Smith",
//	    State:     "CA",
//	    Limit:     10,
//	}
//	providers, err := client.SearchProviders(context.Background(), opts)
//
// # Configuration
//
// Configure the client with custom options:
//
//	client := npiregistry.NewClient(
//	    npiregistry.WithCache(5 * time.Minute),
//	    npiregistry.WithRetry(npiregistry.RetryConfig{
//	        MaxRetries: 3,
//	        InitialDelay: 100 * time.Millisecond,
//	    }),
//	)
//
// # Batch Operations
//
// Fetch multiple providers concurrently:
//
//	npis := []string{"1043218118", "1003000126"}
//	results, err := client.GetProvidersByNPIs(context.Background(), npis)
//
// For more information, see the examples directory and README.md.
package npiregistry
