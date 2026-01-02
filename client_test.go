package npiregistry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// mockProvider returns a sample provider for testing.
func mockProvider() Provider {
	return Provider{
		Number:          "1234567890",
		EnumerationType: "NPI-1",
		Basic: BasicInfo{
			FirstName:       "John",
			LastName:        "Doe",
			Credential:      "MD",
			Gender:          "M",
			EnumerationDate: "2010-05-15",
			Status:          "A",
		},
		Addresses: []Address{
			{
				CountryCode:     "US",
				CountryName:     "United States",
				AddressPurpose:  "LOCATION",
				AddressType:     "DOM",
				Address1:        "123 MAIN ST",
				City:            "ANYTOWN",
				State:           "CA",
				PostalCode:      "12345",
				TelephoneNumber: "555-1234",
			},
		},
		Taxonomies: []Taxonomy{
			{
				Code:    "207Q00000X",
				Desc:    "Family Medicine",
				Primary: true,
			},
		},
		CreatedEpoch:     FlexInt(1273881600),
		LastUpdated:      "2023-01-01",
		LastUpdatedEpoch: FlexInt(1672531200),
	}
}

// mockAPIResponse creates a mock API response.
func mockAPIResponse(providers []Provider) APIResponse {
	return APIResponse{
		ResultCount: len(providers),
		Results:     providers,
	}
}

// TestNewClient verifies that a new client is created with defaults.
func TestNewClient(t *testing.T) {
	client := NewClient()

	if client.baseURL != DefaultBaseURL {
		t.Errorf("expected baseURL %s, got %s", DefaultBaseURL, client.baseURL)
	}

	if client.httpClient.Timeout != DefaultTimeout {
		t.Errorf("expected timeout %v, got %v", DefaultTimeout, client.httpClient.Timeout)
	}

	if client.retry.MaxRetries != DefaultMaxRetries {
		t.Errorf("expected max retries %d, got %d", DefaultMaxRetries, client.retry.MaxRetries)
	}
}

// TestNewClientWithOptions verifies custom options work.
func TestNewClientWithOptions(t *testing.T) {
	customHTTP := &http.Client{Timeout: 10 * time.Second}
	customBaseURL := "https://custom.api.com"

	client := NewClient(
		WithHTTPClient(customHTTP),
		WithBaseURL(customBaseURL),
	)

	if client.httpClient != customHTTP {
		t.Error("custom HTTP client not set")
	}

	if client.baseURL != customBaseURL {
		t.Errorf("expected baseURL %s, got %s", customBaseURL, client.baseURL)
	}
}

// TestGetProviderByNPI_Success tests successful NPI lookup.
func TestGetProviderByNPI_Success(t *testing.T) {
	provider := mockProvider()
	response := mockAPIResponse([]Provider{provider})

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify query parameters
		if r.URL.Query().Get("number") != "1234567890" {
			t.Errorf("expected NPI 1234567890, got %s", r.URL.Query().Get("number"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))

	result, err := client.GetProviderByNPI(context.Background(), "1234567890")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Number != provider.Number {
		t.Errorf("expected NPI %s, got %s", provider.Number, result.Number)
	}

	if result.Basic.FirstName != provider.Basic.FirstName {
		t.Errorf("expected first name %s, got %s", provider.Basic.FirstName, result.Basic.FirstName)
	}
}

// TestGetProviderByNPI_NotFound tests NPI not found case.
func TestGetProviderByNPI_NotFound(t *testing.T) {
	response := mockAPIResponse([]Provider{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))

	_, err := client.GetProviderByNPI(context.Background(), "9999999999")
	if err == nil {
		t.Fatal("expected error for non-existent NPI")
	}

	if err.Error() != "no provider found with NPI 9999999999" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestGetProviderByNPI_EmptyNPI tests empty NPI validation.
func TestGetProviderByNPI_EmptyNPI(t *testing.T) {
	client := NewClient()

	_, err := client.GetProviderByNPI(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty NPI")
	}
}

// TestSearchProviders_Success tests successful provider search.
func TestSearchProviders_Success(t *testing.T) {
	providers := []Provider{
		mockProvider(),
		{
			Number:          "0987654321",
			EnumerationType: "NPI-1",
			Basic: BasicInfo{
				FirstName: "Jane",
				LastName:  "Smith",
			},
		},
	}
	response := mockAPIResponse(providers)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))

	opts := SearchOptions{
		LastName: "Doe",
		State:    "CA",
		Limit:    20,
	}

	results, err := client.SearchProviders(context.Background(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

// TestSearchProviders_Pagination tests limit and skip parameters.
func TestSearchProviders_Pagination(t *testing.T) {
	response := mockAPIResponse([]Provider{mockProvider()})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		if query.Get("limit") != "50" {
			t.Errorf("expected limit 50, got %s", query.Get("limit"))
		}

		if query.Get("skip") != "100" {
			t.Errorf("expected skip 100, got %s", query.Get("skip"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))

	opts := SearchOptions{
		LastName: "Smith",
		Limit:    50,
		Skip:     100,
	}

	_, err := client.SearchProviders(context.Background(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestAPIError_ServerError tests handling of server errors.
func TestAPIError_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "Internal Server Error")
	}))
	defer server.Close()

	client := NewClient(
		WithBaseURL(server.URL),
		WithRetry(RetryConfig{MaxRetries: 0}), // Disable retries for this test
	)

	_, err := client.GetProviderByNPI(context.Background(), "1234567890")
	if err == nil {
		t.Fatal("expected error for server error")
	}

	// Check that the error message contains status 500
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

// TestRetryLogic tests exponential backoff retry.
func TestRetryLogic(t *testing.T) {
	attempts := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		response := mockAPIResponse([]Provider{mockProvider()})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(
		WithBaseURL(server.URL),
		WithRetry(RetryConfig{
			MaxRetries:        3,
			InitialDelay:      10 * time.Millisecond,
			MaxDelay:          100 * time.Millisecond,
			BackoffMultiplier: 2.0,
		}),
	)

	_, err := client.GetProviderByNPI(context.Background(), "1234567890")
	if err != nil {
		t.Fatalf("unexpected error after retries: %v", err)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

// TestContextCancellation tests request cancellation.
func TestContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		response := mockAPIResponse([]Provider{mockProvider()})
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := client.GetProviderByNPI(ctx, "1234567890")
	if err == nil {
		t.Fatal("expected error due to context cancellation")
	}
}

// TestGetProvidersByNPIs_Success tests batch NPI lookup.
func TestGetProvidersByNPIs_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		npi := r.URL.Query().Get("number")
		provider := mockProvider()
		provider.Number = npi

		response := mockAPIResponse([]Provider{provider})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))

	npis := []string{"1234567890", "0987654321", "1111111111"}
	results, err := client.GetProvidersByNPIs(context.Background(), npis)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != len(npis) {
		t.Errorf("expected %d results, got %d", len(npis), len(results))
	}

	for _, npi := range npis {
		if _, exists := results[npi]; !exists {
			t.Errorf("missing result for NPI %s", npi)
		}
	}
}

// ============================================================================
// FlexInt Tests
// ============================================================================

// TestFlexInt_UnmarshalJSON tests the FlexInt type's ability to unmarshal
// various JSON input formats including integers, string numbers, and edge cases.
func TestFlexInt_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    FlexInt
		wantErr bool
	}{
		{
			name:    "unmarshal positive integer",
			input:   `{"value": 1234567890}`,
			want:    FlexInt(1234567890),
			wantErr: false,
		},
		{
			name:    "unmarshal positive string number",
			input:   `{"value": "1234567890"}`,
			want:    FlexInt(1234567890),
			wantErr: false,
		},
		{
			name:    "unmarshal zero as integer",
			input:   `{"value": 0}`,
			want:    FlexInt(0),
			wantErr: false,
		},
		{
			name:    "unmarshal zero as string",
			input:   `{"value": "0"}`,
			want:    FlexInt(0),
			wantErr: false,
		},
		{
			name:    "unmarshal empty string",
			input:   `{"value": ""}`,
			want:    FlexInt(0),
			wantErr: false,
		},
		{
			name:    "unmarshal negative integer",
			input:   `{"value": -1234567890}`,
			want:    FlexInt(-1234567890),
			wantErr: false,
		},
		{
			name:    "unmarshal negative string number",
			input:   `{"value": "-1234567890"}`,
			want:    FlexInt(-1234567890),
			wantErr: false,
		},
		{
			name:    "unmarshal large epoch timestamp",
			input:   `{"value": "1672531200"}`,
			want:    FlexInt(1672531200),
			wantErr: false,
		},
		{
			name:    "unmarshal invalid string",
			input:   `{"value": "not-a-number"}`,
			want:    FlexInt(0),
			wantErr: true,
		},
		{
			name:    "unmarshal float as string - should fail",
			input:   `{"value": "123.45"}`,
			want:    FlexInt(0),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result struct {
				Value FlexInt `json:"value"`
			}

			err := json.Unmarshal([]byte(tt.input), &result)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result.Value != tt.want {
				t.Errorf("UnmarshalJSON() got = %v, want %v", result.Value, tt.want)
			}
		})
	}
}

// TestFlexInt_Int64 tests the Int64 method for retrieving the underlying value.
func TestFlexInt_Int64(t *testing.T) {
	tests := []struct {
		name  string
		value FlexInt
		want  int64
	}{
		{
			name:  "positive value",
			value: FlexInt(1234567890),
			want:  1234567890,
		},
		{
			name:  "zero value",
			value: FlexInt(0),
			want:  0,
		},
		{
			name:  "negative value",
			value: FlexInt(-1234567890),
			want:  -1234567890,
		},
		{
			name:  "max int64",
			value: FlexInt(9223372036854775807),
			want:  9223372036854775807,
		},
		{
			name:  "min int64",
			value: FlexInt(-9223372036854775808),
			want:  -9223372036854775808,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.value.Int64(); got != tt.want {
				t.Errorf("Int64() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestProvider_UnmarshalWithFlexInt tests that Provider correctly unmarshals
// epoch fields as both strings and integers, matching real API behavior.
func TestProvider_UnmarshalWithFlexInt(t *testing.T) {
	tests := []struct {
		name            string
		jsonData        string
		wantCreated     int64
		wantLastUpdated int64
		wantErr         bool
	}{
		{
			name: "string epoch values (real API format)",
			jsonData: `{
				"number": "1234567890",
				"enumeration_type": "NPI-1",
				"basic": {"first_name": "John", "last_name": "Doe"},
				"addresses": [],
				"taxonomies": [],
				"identifiers": [],
				"endpoints": [],
				"practice_locations": [],
				"other_names": [],
				"created_epoch": "1273881600",
				"last_updated": "2023-01-01",
				"last_updated_epoch": "1672531200"
			}`,
			wantCreated:     1273881600,
			wantLastUpdated: 1672531200,
			wantErr:         false,
		},
		{
			name: "integer epoch values",
			jsonData: `{
				"number": "1234567890",
				"enumeration_type": "NPI-1",
				"basic": {"first_name": "Jane", "last_name": "Smith"},
				"addresses": [],
				"taxonomies": [],
				"identifiers": [],
				"endpoints": [],
				"practice_locations": [],
				"other_names": [],
				"created_epoch": 1273881600,
				"last_updated": "2023-01-01",
				"last_updated_epoch": 1672531200
			}`,
			wantCreated:     1273881600,
			wantLastUpdated: 1672531200,
			wantErr:         false,
		},
		{
			name: "mixed format epochs",
			jsonData: `{
				"number": "1234567890",
				"enumeration_type": "NPI-2",
				"basic": {"organization_name": "Test Hospital"},
				"addresses": [],
				"taxonomies": [],
				"identifiers": [],
				"endpoints": [],
				"practice_locations": [],
				"other_names": [],
				"created_epoch": "1273881600",
				"last_updated": "2023-01-01",
				"last_updated_epoch": 1672531200
			}`,
			wantCreated:     1273881600,
			wantLastUpdated: 1672531200,
			wantErr:         false,
		},
		{
			name: "zero epoch values",
			jsonData: `{
				"number": "1234567890",
				"enumeration_type": "NPI-1",
				"basic": {"first_name": "Test", "last_name": "User"},
				"addresses": [],
				"taxonomies": [],
				"identifiers": [],
				"endpoints": [],
				"practice_locations": [],
				"other_names": [],
				"created_epoch": "0",
				"last_updated": "",
				"last_updated_epoch": 0
			}`,
			wantCreated:     0,
			wantLastUpdated: 0,
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var provider Provider
			err := json.Unmarshal([]byte(tt.jsonData), &provider)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if got := provider.CreatedEpoch.Int64(); got != tt.wantCreated {
					t.Errorf("CreatedEpoch = %v, want %v", got, tt.wantCreated)
				}

				if got := provider.LastUpdatedEpoch.Int64(); got != tt.wantLastUpdated {
					t.Errorf("LastUpdatedEpoch = %v, want %v", got, tt.wantLastUpdated)
				}
			}
		})
	}
}

// TestProvider_CompleteUnmarshal tests unmarshaling a complete provider
// with all fields populated to ensure no data loss.
func TestProvider_CompleteUnmarshal(t *testing.T) {
	jsonData := `{
		"number": "1043218118",
		"enumeration_type": "NPI-1",
		"basic": {
			"first_name": "AHAD",
			"last_name": "MAHOOTCHI",
			"credential": "MD",
			"gender": "M",
			"enumeration_date": "2005-07-12",
			"status": "A"
		},
		"addresses": [{
			"country_code": "US",
			"country_name": "United States",
			"address_purpose": "LOCATION",
			"address_type": "DOM",
			"address_1": "6739 GALL BLVD",
			"city": "ZEPHYRHILLS",
			"state": "FL",
			"postal_code": "335422522",
			"telephone_number": "813-779-3338"
		}],
		"taxonomies": [{
			"code": "207W00000X",
			"desc": "Ophthalmology",
			"primary": true,
			"state": "FL",
			"license": "ME12345"
		}],
		"identifiers": [],
		"endpoints": [],
		"practice_locations": [],
		"other_names": [],
		"created_epoch": "1121126400",
		"last_updated": "2023-01-15",
		"last_updated_epoch": "1673740800"
	}`

	var provider Provider
	err := json.Unmarshal([]byte(jsonData), &provider)
	if err != nil {
		t.Fatalf("Failed to unmarshal complete provider: %v", err)
	}

	// Verify basic fields
	if provider.Number != "1043218118" {
		t.Errorf("Number = %v, want 1043218118", provider.Number)
	}

	if provider.Basic.FirstName != "AHAD" {
		t.Errorf("FirstName = %v, want AHAD", provider.Basic.FirstName)
	}

	if provider.Basic.Credential != "MD" {
		t.Errorf("Credential = %v, want MD", provider.Basic.Credential)
	}

	// Verify addresses
	if len(provider.Addresses) != 1 {
		t.Fatalf("Expected 1 address, got %d", len(provider.Addresses))
	}

	if provider.Addresses[0].City != "ZEPHYRHILLS" {
		t.Errorf("City = %v, want ZEPHYRHILLS", provider.Addresses[0].City)
	}

	// Verify taxonomies
	if len(provider.Taxonomies) != 1 {
		t.Fatalf("Expected 1 taxonomy, got %d", len(provider.Taxonomies))
	}

	if provider.Taxonomies[0].Desc != "Ophthalmology" {
		t.Errorf("Taxonomy = %v, want Ophthalmology", provider.Taxonomies[0].Desc)
	}

	if !provider.Taxonomies[0].Primary {
		t.Error("Expected primary taxonomy to be true")
	}

	// Verify epoch fields
	if provider.CreatedEpoch.Int64() != 1121126400 {
		t.Errorf("CreatedEpoch = %v, want 1121126400", provider.CreatedEpoch.Int64())
	}
}

// TestAPIResponse_Unmarshal tests unmarshaling the full API response structure.
func TestAPIResponse_Unmarshal(t *testing.T) {
	jsonData := `{
		"result_count": 2,
		"results": [
			{
				"number": "1234567890",
				"enumeration_type": "NPI-1",
				"basic": {"first_name": "John", "last_name": "Doe"},
				"addresses": [],
				"taxonomies": [],
				"identifiers": [],
				"endpoints": [],
				"practice_locations": [],
				"other_names": [],
				"created_epoch": "1273881600",
				"last_updated": "2023-01-01",
				"last_updated_epoch": "1672531200"
			},
			{
				"number": "0987654321",
				"enumeration_type": "NPI-2",
				"basic": {"organization_name": "Test Clinic"},
				"addresses": [],
				"taxonomies": [],
				"identifiers": [],
				"endpoints": [],
				"practice_locations": [],
				"other_names": [],
				"created_epoch": "1273881600",
				"last_updated": "2023-01-01",
				"last_updated_epoch": "1672531200"
			}
		]
	}`

	var response APIResponse
	err := json.Unmarshal([]byte(jsonData), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal API response: %v", err)
	}

	if response.ResultCount != 2 {
		t.Errorf("ResultCount = %v, want 2", response.ResultCount)
	}

	if len(response.Results) != 2 {
		t.Errorf("Results length = %v, want 2", len(response.Results))
	}

	// Verify first result
	if response.Results[0].Number != "1234567890" {
		t.Errorf("First result NPI = %v, want 1234567890", response.Results[0].Number)
	}

	// Verify second result
	if response.Results[1].Basic.OrganizationName != "Test Clinic" {
		t.Errorf("Second result org name = %v, want Test Clinic", response.Results[1].Basic.OrganizationName)
	}
}

// ============================================================================
// Extended Client Tests
// ============================================================================

// TestBuildQueryParams tests the URL parameter building logic.
func TestBuildQueryParams(t *testing.T) {
	tests := []struct {
		name string
		opts SearchOptions
		want map[string]string
	}{
		{
			name: "all fields populated",
			opts: SearchOptions{
				Number:              "1234567890",
				EnumerationType:     "NPI-1",
				FirstName:           "John",
				LastName:            "Smith",
				OrganizationName:    "Test Clinic",
				TaxonomyDescription: "Family Medicine",
				AddressPurpose:      "LOCATION",
				City:                "Boston",
				State:               "MA",
				PostalCode:          "02115",
				CountryCode:         "US",
				Limit:               50,
				Skip:                100,
				Pretty:              true,
			},
			want: map[string]string{
				"version":              "2.1",
				"number":               "1234567890",
				"enumeration_type":     "NPI-1",
				"first_name":           "John",
				"last_name":            "Smith",
				"organization_name":    "Test Clinic",
				"taxonomy_description": "Family Medicine",
				"address_purpose":      "LOCATION",
				"city":                 "Boston",
				"state":                "MA",
				"postal_code":          "02115",
				"country_code":         "US",
				"limit":                "50",
				"skip":                 "100",
				"pretty":               "true",
			},
		},
		{
			name: "minimal fields",
			opts: SearchOptions{
				LastName: "Johnson",
				State:    "FL",
			},
			want: map[string]string{
				"version":   "2.1",
				"last_name": "Johnson",
				"state":     "FL",
				"limit":     "10",
			},
		},
		{
			name: "limit above max",
			opts: SearchOptions{
				LastName: "Smith",
				Limit:    500,
			},
			want: map[string]string{
				"version":   "2.1",
				"last_name": "Smith",
				"limit":     "200",
			},
		},
		{
			name: "zero limit uses default",
			opts: SearchOptions{
				LastName: "Brown",
				Limit:    0,
			},
			want: map[string]string{
				"version":   "2.1",
				"last_name": "Brown",
				"limit":     "10",
			},
		},
		{
			name: "skip zero not included",
			opts: SearchOptions{
				LastName: "Davis",
				Skip:     0,
			},
			want: map[string]string{
				"version":   "2.1",
				"last_name": "Davis",
				"limit":     "10",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient()
			params := client.buildQueryParams(tt.opts)

			for key, expectedValue := range tt.want {
				if got := params.Get(key); got != expectedValue {
					t.Errorf("param %s = %v, want %v", key, got, expectedValue)
				}
			}

			// Verify no extra parameters
			for key := range params {
				if _, expected := tt.want[key]; !expected {
					t.Errorf("unexpected parameter %s = %v", key, params.Get(key))
				}
			}
		})
	}
}

// TestSearchProviders_AllFilters tests various filter combinations.
func TestSearchProviders_AllFilters(t *testing.T) {
	tests := []struct {
		name           string
		opts           SearchOptions
		validateParams func(*testing.T, url.Values)
	}{
		{
			name: "individual provider search",
			opts: SearchOptions{
				FirstName:           "John",
				LastName:            "Smith",
				EnumerationType:     "NPI-1",
				TaxonomyDescription: "Family Medicine",
				State:               "CA",
			},
			validateParams: func(t *testing.T, params url.Values) {
				if params.Get("first_name") != "John" {
					t.Errorf("first_name not set correctly")
				}
				if params.Get("enumeration_type") != "NPI-1" {
					t.Errorf("enumeration_type not set correctly")
				}
			},
		},
		{
			name: "organization search",
			opts: SearchOptions{
				OrganizationName: "General Hospital",
				EnumerationType:  "NPI-2",
				City:             "New York",
				State:            "NY",
			},
			validateParams: func(t *testing.T, params url.Values) {
				if params.Get("organization_name") != "General Hospital" {
					t.Errorf("organization_name not set correctly")
				}
				if params.Get("enumeration_type") != "NPI-2" {
					t.Errorf("enumeration_type not set correctly")
				}
			},
		},
		{
			name: "address purpose filter",
			opts: SearchOptions{
				LastName:       "Johnson",
				AddressPurpose: "MAILING",
				PostalCode:     "90210",
			},
			validateParams: func(t *testing.T, params url.Values) {
				if params.Get("address_purpose") != "MAILING" {
					t.Errorf("address_purpose not set correctly")
				}
				if params.Get("postal_code") != "90210" {
					t.Errorf("postal_code not set correctly")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := mockAPIResponse([]Provider{mockProvider()})

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tt.validateParams(t, r.URL.Query())
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()

			client := NewClient(WithBaseURL(server.URL))
			_, err := client.SearchProviders(context.Background(), tt.opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestGetProviderByNPI_WithCache tests caching behavior.
func TestGetProviderByNPI_WithCache(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		response := mockAPIResponse([]Provider{mockProvider()})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(
		WithBaseURL(server.URL),
		WithCache(10*time.Second),
	)

	ctx := context.Background()

	// First call - should hit API
	_, err := client.GetProviderByNPI(ctx, "1234567890")
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	// Second call - should use cache
	_, err = client.GetProviderByNPI(ctx, "1234567890")
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	// Third call with different NPI - should hit API
	_, err = client.GetProviderByNPI(ctx, "0987654321")
	if err != nil {
		t.Fatalf("third call failed: %v", err)
	}

	// Should have made 2 API calls (first and third)
	if callCount != 2 {
		t.Errorf("expected 2 API calls, got %d", callCount)
	}
}

// TestDoRequest_InvalidJSON tests handling of malformed JSON responses.
func TestDoRequest_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json{{{"))
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))

	_, err := client.GetProviderByNPI(context.Background(), "1234567890")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}

	if !strings.Contains(err.Error(), "failed to decode response") {
		t.Errorf("expected decode error, got: %v", err)
	}
}

// TestDoRequest_HTTPErrors tests various HTTP error responses.
func TestDoRequest_HTTPErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantRetry  bool
	}{
		{
			name:       "400 bad request - no retry",
			statusCode: http.StatusBadRequest,
			wantRetry:  false,
		},
		{
			name:       "404 not found - no retry",
			statusCode: http.StatusNotFound,
			wantRetry:  false,
		},
		{
			name:       "429 rate limit - should retry",
			statusCode: http.StatusTooManyRequests,
			wantRetry:  true,
		},
		{
			name:       "500 server error - should retry",
			statusCode: http.StatusInternalServerError,
			wantRetry:  true,
		},
		{
			name:       "502 bad gateway - should retry",
			statusCode: http.StatusBadGateway,
			wantRetry:  true,
		},
		{
			name:       "503 service unavailable - should retry",
			statusCode: http.StatusServiceUnavailable,
			wantRetry:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attemptCount := 0

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attemptCount++
				w.WriteHeader(tt.statusCode)
				w.Write([]byte("Error"))
			}))
			defer server.Close()

			client := NewClient(
				WithBaseURL(server.URL),
				WithRetry(RetryConfig{
					MaxRetries:        2,
					InitialDelay:      10 * time.Millisecond,
					MaxDelay:          50 * time.Millisecond,
					BackoffMultiplier: 2.0,
				}),
			)

			_, err := client.GetProviderByNPI(context.Background(), "1234567890")
			if err == nil {
				t.Fatal("expected error")
			}

			// Check if retries happened as expected
			if tt.wantRetry {
				// Should have retried (1 initial + 2 retries = 3 attempts)
				if attemptCount != 3 {
					t.Errorf("expected 3 attempts for retryable error, got %d", attemptCount)
				}
			} else {
				// Should not have retried (1 attempt only)
				if attemptCount != 1 {
					t.Errorf("expected 1 attempt for non-retryable error, got %d", attemptCount)
				}
			}
		})
	}
}

// TestGetProvidersByNPIs_EmptyList tests batch lookup with empty input.
func TestGetProvidersByNPIs_EmptyList(t *testing.T) {
	client := NewClient()

	_, err := client.GetProvidersByNPIs(context.Background(), []string{})
	if err == nil {
		t.Fatal("expected error for empty NPI list")
	}

	if !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestGetProvidersByNPIs_PartialFailure tests batch lookup with some failures.
func TestGetProvidersByNPIs_PartialFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		npi := r.URL.Query().Get("number")

		// Fail for specific NPI
		if npi == "9999999999" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		provider := mockProvider()
		provider.Number = npi
		response := mockAPIResponse([]Provider{provider})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))

	npis := []string{"1234567890", "9999999999", "1111111111"}
	results, err := client.GetProvidersByNPIs(context.Background(), npis)

	// Should have partial results and an error
	if err == nil {
		t.Error("expected error for partial failure")
	}

	// Should have successfully fetched 2 out of 3
	if len(results) != 2 {
		t.Errorf("expected 2 successful results, got %d", len(results))
	}
}

// TestContextDeadline tests various context deadline scenarios.
func TestContextDeadline(t *testing.T) {
	tests := []struct {
		name           string
		serverDelay    time.Duration
		contextTimeout time.Duration
		expectError    bool
	}{
		{
			name:           "request completes before deadline",
			serverDelay:    10 * time.Millisecond,
			contextTimeout: 100 * time.Millisecond,
			expectError:    false,
		},
		{
			name:           "request exceeds deadline",
			serverDelay:    100 * time.Millisecond,
			contextTimeout: 10 * time.Millisecond,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(tt.serverDelay)
				response := mockAPIResponse([]Provider{mockProvider()})
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()

			client := NewClient(WithBaseURL(server.URL))

			ctx, cancel := context.WithTimeout(context.Background(), tt.contextTimeout)
			defer cancel()

			_, err := client.GetProviderByNPI(ctx, "1234567890")

			if tt.expectError && err == nil {
				t.Error("expected error due to context deadline")
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestShouldRetry tests the retry decision logic.
func TestShouldRetry(t *testing.T) {
	client := NewClient()

	tests := []struct {
		name      string
		err       error
		wantRetry bool
	}{
		{
			name: "API error 500",
			err: &APIError{
				StatusCode: 500,
				Message:    "Internal Server Error",
			},
			wantRetry: true,
		},
		{
			name: "API error 429",
			err: &APIError{
				StatusCode: 429,
				Message:    "Rate Limited",
			},
			wantRetry: true,
		},
		{
			name: "API error 400",
			err: &APIError{
				StatusCode: 400,
				Message:    "Bad Request",
			},
			wantRetry: false,
		},
		{
			name: "API error 404",
			err: &APIError{
				StatusCode: 404,
				Message:    "Not Found",
			},
			wantRetry: false,
		},
		{
			name:      "network error",
			err:       errors.New("network error"),
			wantRetry: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.shouldRetry(tt.err)
			if got != tt.wantRetry {
				t.Errorf("shouldRetry() = %v, want %v", got, tt.wantRetry)
			}
		})
	}
}

// TestAPIError_Error tests the APIError error message.
func TestAPIError_Error(t *testing.T) {
	err := &APIError{
		StatusCode: 500,
		Message:    "Internal Server Error",
	}

	if err.Error() != "Internal Server Error" {
		t.Errorf("Error() = %v, want %v", err.Error(), "Internal Server Error")
	}
}

// TestClientOptions tests all client configuration options.
func TestClientOptions(t *testing.T) {
	t.Run("WithHTTPClient", func(t *testing.T) {
		customClient := &http.Client{Timeout: 60 * time.Second}
		client := NewClient(WithHTTPClient(customClient))

		if client.httpClient != customClient {
			t.Error("custom HTTP client not set")
		}
	})

	t.Run("WithBaseURL", func(t *testing.T) {
		customURL := "https://test.example.com"
		client := NewClient(WithBaseURL(customURL))

		if client.baseURL != customURL {
			t.Errorf("baseURL = %v, want %v", client.baseURL, customURL)
		}
	})

	t.Run("WithRetry", func(t *testing.T) {
		retryConfig := RetryConfig{
			MaxRetries:        5,
			InitialDelay:      200 * time.Millisecond,
			MaxDelay:          10 * time.Second,
			BackoffMultiplier: 3.0,
		}
		client := NewClient(WithRetry(retryConfig))

		if client.retry.MaxRetries != 5 {
			t.Errorf("MaxRetries = %v, want 5", client.retry.MaxRetries)
		}
		if client.retry.InitialDelay != 200*time.Millisecond {
			t.Errorf("InitialDelay = %v, want 200ms", client.retry.InitialDelay)
		}
		if client.retry.BackoffMultiplier != 3.0 {
			t.Errorf("BackoffMultiplier = %v, want 3.0", client.retry.BackoffMultiplier)
		}
	})

	t.Run("WithCache", func(t *testing.T) {
		client := NewClient(WithCache(5 * time.Minute))

		if !client.cache.enabled {
			t.Error("cache not enabled")
		}
	})

	t.Run("multiple options", func(t *testing.T) {
		customClient := &http.Client{Timeout: 45 * time.Second}
		customURL := "https://test.api.com"

		client := NewClient(
			WithHTTPClient(customClient),
			WithBaseURL(customURL),
			WithCache(10*time.Minute),
		)

		if client.httpClient != customClient {
			t.Error("custom HTTP client not set")
		}
		if client.baseURL != customURL {
			t.Error("custom base URL not set")
		}
		if !client.cache.enabled {
			t.Error("cache not enabled")
		}
	})
}

// TestSearchProviders_EmptyResults tests handling of empty search results.
func TestSearchProviders_EmptyResults(t *testing.T) {
	response := mockAPIResponse([]Provider{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))

	opts := SearchOptions{
		LastName: "NonExistentName",
		State:    "XX",
	}

	results, err := client.SearchProviders(context.Background(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// TestConcurrentRequests tests thread safety of concurrent requests.
func TestConcurrentRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate some processing time
		time.Sleep(10 * time.Millisecond)

		npi := r.URL.Query().Get("number")
		provider := mockProvider()
		provider.Number = npi

		response := mockAPIResponse([]Provider{provider})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))

	// Make 10 concurrent requests
	const numRequests = 10
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			npi := fmt.Sprintf("123456789%d", id)
			_, err := client.GetProviderByNPI(context.Background(), npi)
			results <- err
		}(i)
	}

	// Collect results
	for i := 0; i < numRequests; i++ {
		if err := <-results; err != nil {
			t.Errorf("concurrent request %d failed: %v", i, err)
		}
	}
}

