package gonpi

import (
	"encoding/json"
	"strconv"
	"time"
)

// FlexInt is a custom type that handles flexible JSON unmarshaling for numeric values.
// The NPI Registry API inconsistently returns epoch timestamps as either strings ("1234567890")
// or integers (1234567890). FlexInt automatically handles both formats during JSON unmarshaling.
//
// Example usage:
//
//	type MyStruct struct {
//	    Timestamp FlexInt `json:"timestamp"`
//	}
//
//	// Works with both: {"timestamp": "1234567890"} and {"timestamp": 1234567890}
//	var s MyStruct
//	json.Unmarshal(data, &s)
//	fmt.Println(s.Timestamp.Int64()) // 1234567890
type FlexInt int64

// UnmarshalJSON implements custom unmarshaling to handle both string and integer values.
// It first attempts to unmarshal as an integer. If that fails, it tries to unmarshal
// as a string and convert it to an integer. Empty strings are treated as 0.
//
// Supported input formats:
//   - Integer: 1234567890
//   - String number: "1234567890"
//   - Empty string: "" (returns 0)
//
// Returns an error if the value cannot be parsed as an integer.
func (f *FlexInt) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as int first
	var i int64
	if err := json.Unmarshal(data, &i); err == nil {
		*f = FlexInt(i)
		return nil
	}

	// Try to unmarshal as string
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	// Convert string to int
	if s == "" {
		*f = 0
		return nil
	}

	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}

	*f = FlexInt(i)
	return nil
}

// Int64 returns the underlying int64 value of the FlexInt.
// This is the primary method for accessing the numeric value after unmarshaling.
//
// Example:
//
//	var provider Provider
//	json.Unmarshal(data, &provider)
//	timestamp := provider.CreatedEpoch.Int64() // Get the epoch as int64
func (f FlexInt) Int64() int64 {
	return int64(f)
}

// Provider represents a healthcare provider from the NPI Registry.
// This struct contains comprehensive information about individual healthcare providers
// (NPI-1) and organizational healthcare providers (NPI-2) including their basic
// information, addresses, specialties (taxonomies), identifiers, and endpoints.
//
// Key fields:
//   - Number: The unique 10-digit NPI number
//   - EnumerationType: "NPI-1" for individuals, "NPI-2" for organizations
//   - Basic: Core information (name, credentials, dates)
//   - Addresses: Practice and mailing addresses
//   - Taxonomies: Specialties and healthcare classifications
//   - Identifiers: Additional IDs like state licenses
//   - Endpoints: Electronic communication endpoints
type Provider struct {
	Number            string             `json:"number"`
	EnumerationType   string             `json:"enumeration_type"`
	Basic             BasicInfo          `json:"basic"`
	Addresses         []Address          `json:"addresses"`
	Taxonomies        []Taxonomy         `json:"taxonomies"`
	Identifiers       []Identifier       `json:"identifiers"`
	Endpoints         []Endpoint         `json:"endpoints"`
	PracticeLocations []PracticeLocation `json:"practice_locations"`
	OtherNames        []OtherName        `json:"other_names"`
	CreatedEpoch      FlexInt            `json:"created_epoch"`
	LastUpdated       string             `json:"last_updated"`
	LastUpdatedEpoch  FlexInt            `json:"last_updated_epoch"`
}

// BasicInfo contains basic information about the provider.
type BasicInfo struct {
	FirstName                         string `json:"first_name"`
	LastName                          string `json:"last_name"`
	MiddleName                        string `json:"middle_name"`
	Credential                        string `json:"credential"`
	SoleProprietor                    string `json:"sole_proprietor"`
	Gender                            string `json:"gender"`
	EnumerationDate                   string `json:"enumeration_date"`
	LastUpdated                       string `json:"last_updated"`
	Status                            string `json:"status"`
	Name                              string `json:"name"`
	NamePrefix                        string `json:"name_prefix"`
	NameSuffix                        string `json:"name_suffix"`
	OrganizationName                  string `json:"organization_name"`
	OrganizationalSubpart             string `json:"organizational_subpart"`
	AuthorizedOfficialFirstName       string `json:"authorized_official_first_name"`
	AuthorizedOfficialLastName        string `json:"authorized_official_last_name"`
	AuthorizedOfficialMiddleName      string `json:"authorized_official_middle_name"`
	AuthorizedOfficialTelephoneNumber string `json:"authorized_official_telephone_number"`
	AuthorizedOfficialTitleOrPosition string `json:"authorized_official_title_or_position"`
	AuthorizedOfficialCredential      string `json:"authorized_official_credential"`
	CertificationDate                 string `json:"certification_date"`
}

// Address represents a mailing or practice address for a healthcare provider.
// Each provider can have multiple addresses with different purposes (LOCATION, MAILING).
// The AddressPurpose field indicates whether this is a practice location or mailing address.
type Address struct {
	CountryCode     string `json:"country_code"`
	CountryName     string `json:"country_name"`
	AddressPurpose  string `json:"address_purpose"`
	AddressType     string `json:"address_type"`
	Address1        string `json:"address_1"`
	Address2        string `json:"address_2"`
	City            string `json:"city"`
	State           string `json:"state"`
	PostalCode      string `json:"postal_code"`
	TelephoneNumber string `json:"telephone_number"`
	FaxNumber       string `json:"fax_number"`
}

// Taxonomy represents a provider's specialty or healthcare classification.
// Each provider can have multiple taxonomies, with one marked as primary.
// The taxonomy code follows the Healthcare Provider Taxonomy Code Set.
//
// Example taxonomies:
//   - 207Q00000X: Family Medicine
//   - 208D00000X: General Practice
//   - 2084P0800X: Psychiatry & Neurology - Psychiatry
type Taxonomy struct {
	Code          string `json:"code"`
	TaxonomyGroup string `json:"taxonomy_group"`
	Desc          string `json:"desc"`
	State         string `json:"state"`
	License       string `json:"license"`
	Primary       bool   `json:"primary"`
}

// Identifier represents an additional identifier for the provider beyond the NPI.
// Common identifier types include state licenses, DEA numbers, and other
// professional credentials issued by various authorities.
type Identifier struct {
	Code       string `json:"code"`
	Desc       string `json:"desc"`
	Identifier string `json:"identifier"`
	State      string `json:"state"`
	Issuer     string `json:"issuer"`
}

// Endpoint represents an electronic service endpoint for the provider.
// This includes Direct addresses (secure email), FHIR endpoints, and other
// methods of electronic health information exchange.
type Endpoint struct {
	EndpointType            string `json:"endpointType"`
	EndpointTypeDescription string `json:"endpointTypeDescription"`
	Endpoint                string `json:"endpoint"`
	Affiliation             string `json:"affiliation"`
	UseDescription          string `json:"useDescription"`
	ContentType             string `json:"contentType"`
	ContentTypeDescription  string `json:"contentTypeDescription"`
	Country                 string `json:"country"`
	CountryName             string `json:"countryName"`
	Address                 string `json:"address"`
	City                    string `json:"city"`
	State                   string `json:"state"`
	Zip                     string `json:"zip"`
}

// PracticeLocation represents a location where the provider practices.
type PracticeLocation struct {
	Address1        string `json:"address_1"`
	Address2        string `json:"address_2"`
	City            string `json:"city"`
	State           string `json:"state"`
	PostalCode      string `json:"postal_code"`
	CountryCode     string `json:"country_code"`
	CountryName     string `json:"country_name"`
	TelephoneNumber string `json:"telephone_number"`
	FaxNumber       string `json:"fax_number"`
}

// OtherName represents alternative names for the provider.
type OtherName struct {
	Type             string `json:"type"`
	Code             string `json:"code"`
	Credential       string `json:"credential"`
	FirstName        string `json:"first_name"`
	LastName         string `json:"last_name"`
	MiddleName       string `json:"middle_name"`
	Prefix           string `json:"prefix"`
	Suffix           string `json:"suffix"`
	OrganizationName string `json:"organization_name"`
}

// APIResponse represents the full response from the NPI Registry API.
type APIResponse struct {
	ResultCount int        `json:"result_count"`
	Results     []Provider `json:"results"`
}

// SearchOptions defines all available filters for searching providers in the NPI Registry.
// All fields are optional and can be combined to narrow search results.
// At least one search criterion must be provided.
//
// Example usage:
//
//	opts := SearchOptions{
//	    FirstName: "John",
//	    LastName:  "Smith",
//	    State:     "CA",
//	    TaxonomyDescription: "Family Medicine",
//	    Limit:     20,
//	}
//	providers, err := client.SearchProviders(ctx, opts)
type SearchOptions struct {
	// Number searches for a specific NPI number (10 digits).
	// When provided, this takes precedence and returns at most one result.
	Number string

	// EnumerationType filters by provider type:
	//   - "NPI-1" or "ind" for individual providers
	//   - "NPI-2" or "org" for organizational providers
	EnumerationType string

	// FirstName searches for individual provider's first name.
	// Supports partial matching (e.g., "John" matches "Johnny").
	// Only applicable for individual providers (NPI-1).
	FirstName string

	// LastName searches for individual provider's last name.
	// Supports partial matching and is case-insensitive.
	// Only applicable for individual providers (NPI-1).
	LastName string

	// OrganizationName searches for organization name.
	// Supports partial matching and is case-insensitive.
	// Only applicable for organizational providers (NPI-2).
	OrganizationName string

	// TaxonomyDescription searches by specialty or healthcare classification.
	// Examples: "Family Medicine", "Cardiology", "Hospital"
	// Supports partial matching (e.g., "Medicine" matches "Family Medicine").
	TaxonomyDescription string

	// AddressPurpose filters by address type:
	//   - "LOCATION" for practice addresses
	//   - "MAILING" for mailing addresses
	// Leave empty to search both address types.
	AddressPurpose string

	// City filters by city name (case-insensitive).
	City string

	// State filters by two-letter state code (e.g., "CA", "NY", "TX").
	// Must be uppercase.
	State string

	// PostalCode filters by ZIP code.
	// Supports 5-digit (e.g., "90210") or 9-digit (e.g., "90210-1234") formats.
	PostalCode string

	// CountryCode filters by two-letter country code (default: "US").
	// Examples: "US", "CA", "MX"
	CountryCode string

	// Limit specifies the maximum number of results to return per request.
	// Valid range: 1-200. Default: 10 if not specified or 0.
	// Values exceeding 200 are automatically capped at 200.
	Limit int

	// Skip specifies the number of results to skip for pagination.
	// Use this with Limit to implement pagination:
	//   - Page 1: Skip=0, Limit=10
	//   - Page 2: Skip=10, Limit=10
	//   - Page 3: Skip=20, Limit=10
	Skip int

	// Pretty formats the JSON response for human readability.
	// Only affects the raw API response; has no effect on returned Go structs.
	Pretty bool
}

// ClientOption allows configuration of the Client.
type ClientOption func(*Client)

// RetryConfig defines the exponential backoff retry behavior for transient HTTP errors.
// The client will retry failed requests with increasing delays between attempts.
//
// Example usage:
//
//	client := NewClient(
//	    WithRetry(RetryConfig{
//	        MaxRetries:        5,
//	        InitialDelay:      100 * time.Millisecond,
//	        MaxDelay:          10 * time.Second,
//	        BackoffMultiplier: 2.0,
//	    }),
//	)
//
// Retry logic:
//   - Retry #1: waits InitialDelay (100ms)
//   - Retry #2: waits InitialDelay * BackoffMultiplier (200ms)
//   - Retry #3: waits 400ms
//   - Continues up to MaxRetries, capped at MaxDelay
//
// Only retries server errors (5xx) and rate limits (429).
// Client errors (4xx) are not retried.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts.
	// 0 means no retries. Default: 3.
	MaxRetries int

	// InitialDelay is the delay before the first retry.
	// Default: 100ms.
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries.
	// Prevents exponential backoff from growing indefinitely.
	// Default: 5 seconds.
	MaxDelay time.Duration

	// BackoffMultiplier is the factor by which the delay increases after each retry.
	// For example, with a multiplier of 2.0:
	//   Delay = InitialDelay * (BackoffMultiplier ^ retryNumber)
	// Default: 2.0 (exponential backoff).
	BackoffMultiplier float64
}
