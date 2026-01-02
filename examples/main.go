package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	npiregistry "github.com/sdsvn/gonpi"
)

func main() {
	// Create a new NPI Registry client with caching enabled
	client := npiregistry.NewClient(
		npiregistry.WithCache(5 * 60 * 1000000000), // 5 minutes in nanoseconds
	)

	ctx := context.Background()

	fmt.Println("=== NPI Registry Go Library Demo ===")

	// Example 1: Lookup by NPI number
	fmt.Println("1. Lookup Provider by NPI Number")
	fmt.Println("----------------------------------")
	exampleLookupByNPI(ctx, client)

	// Example 2: Search by name and state
	fmt.Println("2. Search Providers by Name and State")
	fmt.Println("--------------------------------------")
	exampleSearchByName(ctx, client)
}

// exampleLookupByNPI demonstrates looking up a provider by NPI number.
func exampleLookupByNPI(ctx context.Context, client *npiregistry.Client) {
	// Example NPI
	npi := "1043218118"

	provider, err := client.GetProviderByNPI(ctx, npi)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	printProvider(provider)
}

// exampleSearchByName demonstrates searching by provider name.
func exampleSearchByName(ctx context.Context, client *npiregistry.Client) {
	opts := npiregistry.SearchOptions{
		FirstName: "John",
		LastName:  "Smith",
		State:     "CA",
		Limit:     5,
	}

	providers, err := client.SearchProviders(ctx, opts)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Found %d providers matching 'John Smith' in California\n\n", len(providers))
	for i, provider := range providers {
		fmt.Printf("[%d] %s %s (NPI: %s)\n", i+1,
			provider.Basic.FirstName,
			provider.Basic.LastName,
			provider.Number)
		if len(provider.Addresses) > 0 {
			addr := provider.Addresses[0]
			fmt.Printf("    %s, %s %s\n", addr.City, addr.State, addr.PostalCode)
		}
	}
}

// printProvider prints detailed information about a provider.
func printProvider(provider *npiregistry.Provider) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	fmt.Fprintf(w, "NPI Number:\t%s\n", provider.Number)
	fmt.Fprintf(w, "Type:\t%s\n", provider.EnumerationType)

	// Basic info
	if provider.Basic.FirstName != "" {
		fmt.Fprintf(w, "Name:\t%s %s %s\n",
			provider.Basic.FirstName,
			provider.Basic.MiddleName,
			provider.Basic.LastName)
		if provider.Basic.Credential != "" {
			fmt.Fprintf(w, "Credential:\t%s\n", provider.Basic.Credential)
		}
		if provider.Basic.Gender != "" {
			fmt.Fprintf(w, "Gender:\t%s\n", provider.Basic.Gender)
		}
	} else if provider.Basic.OrganizationName != "" {
		fmt.Fprintf(w, "Organization:\t%s\n", provider.Basic.OrganizationName)
	}

	fmt.Fprintf(w, "Status:\t%s\n", provider.Basic.Status)
	fmt.Fprintf(w, "Enumeration Date:\t%s\n", provider.Basic.EnumerationDate)

	// Addresses
	if len(provider.Addresses) > 0 {
		fmt.Fprintf(w, "\nAddresses:\n")
		for i, addr := range provider.Addresses {
			fmt.Fprintf(w, "  [%d] %s:\t%s\n", i+1, addr.AddressPurpose, addr.Address1)
			if addr.Address2 != "" {
				fmt.Fprintf(w, "      \t%s\n", addr.Address2)
			}
			fmt.Fprintf(w, "      \t%s, %s %s\n", addr.City, addr.State, addr.PostalCode)
			if addr.TelephoneNumber != "" {
				fmt.Fprintf(w, "      Phone:\t%s\n", addr.TelephoneNumber)
			}
		}
	}

	// Taxonomies
	if len(provider.Taxonomies) > 0 {
		fmt.Fprintf(w, "\nSpecialties:\n")
		for i, tax := range provider.Taxonomies {
			primary := ""
			if tax.Primary {
				primary = " (Primary)"
			}
			fmt.Fprintf(w, "  [%d]\t%s%s\n", i+1, tax.Desc, primary)
			fmt.Fprintf(w, "      Code:\t%s\n", tax.Code)
			if tax.License != "" {
				fmt.Fprintf(w, "      License:\t%s (%s)\n", tax.License, tax.State)
			}
		}
	}

	// Identifiers
	if len(provider.Identifiers) > 0 {
		fmt.Fprintf(w, "\nIdentifiers:\n")
		for i, id := range provider.Identifiers {
			fmt.Fprintf(w, "  [%d] %s:\t%s\n", i+1, id.Desc, id.Identifier)
			if id.State != "" {
				fmt.Fprintf(w, "      State:\t%s\n", id.State)
			}
		}
	}

	// Endpoints
	if len(provider.Endpoints) > 0 {
		fmt.Fprintf(w, "\nEndpoints:\n")
		for i, ep := range provider.Endpoints {
			fmt.Fprintf(w, "  [%d] %s:\t%s\n", i+1, ep.EndpointTypeDescription, ep.Endpoint)
		}
	}

	w.Flush()
}
