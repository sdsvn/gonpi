package gonpi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// BenchmarkFlexIntUnmarshal_Integer benchmarks unmarshaling integer values.
func BenchmarkFlexIntUnmarshal_Integer(b *testing.B) {
	jsonData := []byte(`{"epoch": 1234567890}`)
	var result struct {
		Epoch FlexInt `json:"epoch"`
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Unmarshal(jsonData, &result)
	}
}

// BenchmarkFlexIntUnmarshal_String benchmarks unmarshaling string values.
func BenchmarkFlexIntUnmarshal_String(b *testing.B) {
	jsonData := []byte(`{"epoch": "1234567890"}`)
	var result struct {
		Epoch FlexInt `json:"epoch"`
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Unmarshal(jsonData, &result)
	}
}

// BenchmarkCacheWithTTL benchmarks cache operations with proper TTL.
func BenchmarkCacheWithTTL(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := mockAPIResponse([]Provider{mockProvider()})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(
		WithBaseURL(server.URL),
		WithCache(10*time.Minute),
	)
	defer client.Close()

	ctx := context.Background()

	// Warm up cache
	client.GetProviderByNPI(ctx, "1234567890")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.GetProviderByNPI(ctx, "1234567890")
	}
}

// BenchmarkBatchOperationsWithSyncMap benchmarks batch operations using sync.Map.
func BenchmarkBatchOperationsWithSyncMap(b *testing.B) {
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

	npis := []string{"1234567890", "0987654321", "1111111111", "2222222222", "3333333333"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.GetProvidersByNPIs(context.Background(), npis)
	}
}

// BenchmarkProviderUnmarshal benchmarks full provider unmarshaling with FlexInt fields.
func BenchmarkProviderUnmarshal(b *testing.B) {
	jsonData := []byte(`{
		"number": "1234567890",
		"enumeration_type": "NPI-1",
		"basic": {
			"first_name": "John",
			"last_name": "Doe"
		},
		"created_epoch": "1234567890",
		"last_updated_epoch": 9876543210
	}`)

	var provider Provider

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Unmarshal(jsonData, &provider)
	}
}
