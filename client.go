package gonpi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

const (
	// DefaultBaseURL is the base URL for the NPI Registry API.
	DefaultBaseURL = "https://npiregistry.cms.hhs.gov/api"
	
	// DefaultTimeout is the default HTTP client timeout.
	DefaultTimeout = 30 * time.Second
	
	// DefaultMaxRetries is the default number of retry attempts.
	DefaultMaxRetries = 3
	
	// DefaultLimit is the default result limit per request.
	DefaultLimit = 10
	
	// MaxLimit is the maximum allowed result limit.
	MaxLimit = 200
)

// Client is the NPI Registry API client.
type Client struct {
	baseURL    string
	httpClient *http.Client
	retry      RetryConfig
	cache      *cacheStore
	mu         sync.RWMutex
}

// cacheStore provides simple in-memory caching for NPI lookups.
type cacheStore struct {
	enabled bool
	data    map[string]*cacheEntry
	mu      sync.RWMutex
}

type cacheEntry struct {
	provider  *Provider
	expiresAt time.Time
}

// NewClient creates a new NPI Registry API client with optional configuration.
func NewClient(opts ...ClientOption) *Client {
	client := &Client{
		baseURL: DefaultBaseURL,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		retry: RetryConfig{
			MaxRetries:        DefaultMaxRetries,
			InitialDelay:      100 * time.Millisecond,
			MaxDelay:          5 * time.Second,
			BackoffMultiplier: 2.0,
		},
		cache: &cacheStore{
			enabled: false,
			data:    make(map[string]*cacheEntry),
		},
	}
	
	for _, opt := range opts {
		opt(client)
	}
	
	return client
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithBaseURL sets a custom base URL.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

// WithRetry configures retry behavior.
func WithRetry(config RetryConfig) ClientOption {
	return func(c *Client) {
		c.retry = config
	}
}

// WithCache enables in-memory caching with the specified TTL.
func WithCache(ttl time.Duration) ClientOption {
	return func(c *Client) {
		c.cache.enabled = true
		// Start background cleanup goroutine
		go c.cleanupCache(ttl)
	}
}

// GetProviderByNPI fetches a single provider by NPI number.
func (c *Client) GetProviderByNPI(ctx context.Context, npi string) (*Provider, error) {
	if npi == "" {
		return nil, fmt.Errorf("npi cannot be empty")
	}
	
	// Check cache first
	if c.cache.enabled {
		if provider := c.getCached(npi); provider != nil {
			return provider, nil
		}
	}
	
	opts := SearchOptions{
		Number: npi,
		Limit:  1,
	}
	
	providers, err := c.SearchProviders(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider by NPI %s: %w", npi, err)
	}
	
	if len(providers) == 0 {
		return nil, fmt.Errorf("no provider found with NPI %s", npi)
	}
	
	provider := &providers[0]
	
	// Cache the result
	if c.cache.enabled {
		c.setCached(npi, provider)
	}
	
	return provider, nil
}

// SearchProviders searches for providers using the specified filters.
func (c *Client) SearchProviders(ctx context.Context, opts SearchOptions) ([]Provider, error) {
	// Build query parameters
	params := c.buildQueryParams(opts)
	
	// Construct URL
	apiURL := fmt.Sprintf("%s/?%s", c.baseURL, params.Encode())
	
	// Make request with retry logic
	var response APIResponse
	err := c.doRequestWithRetry(ctx, apiURL, &response)
	if err != nil {
		return nil, fmt.Errorf("search providers failed: %w", err)
	}
	
	return response.Results, nil
}

// buildQueryParams converts SearchOptions to URL query parameters.
func (c *Client) buildQueryParams(opts SearchOptions) url.Values {
	params := url.Values{}
	
	params.Set("version", "2.1")
	
	if opts.Number != "" {
		params.Set("number", opts.Number)
	}
	
	if opts.EnumerationType != "" {
		params.Set("enumeration_type", opts.EnumerationType)
	}
	
	if opts.FirstName != "" {
		params.Set("first_name", opts.FirstName)
	}
	
	if opts.LastName != "" {
		params.Set("last_name", opts.LastName)
	}
	
	if opts.OrganizationName != "" {
		params.Set("organization_name", opts.OrganizationName)
	}
	
	if opts.TaxonomyDescription != "" {
		params.Set("taxonomy_description", opts.TaxonomyDescription)
	}
	
	if opts.AddressPurpose != "" {
		params.Set("address_purpose", opts.AddressPurpose)
	}
	
	if opts.City != "" {
		params.Set("city", opts.City)
	}
	
	if opts.State != "" {
		params.Set("state", opts.State)
	}
	
	if opts.PostalCode != "" {
		params.Set("postal_code", opts.PostalCode)
	}
	
	if opts.CountryCode != "" {
		params.Set("country_code", opts.CountryCode)
	}
	
	// Set limit with validation
	limit := opts.Limit
	if limit == 0 {
		limit = DefaultLimit
	} else if limit > MaxLimit {
		limit = MaxLimit
	}
	params.Set("limit", strconv.Itoa(limit))
	
	if opts.Skip > 0 {
		params.Set("skip", strconv.Itoa(opts.Skip))
	}
	
	if opts.Pretty {
		params.Set("pretty", "true")
	}
	
	return params
}

// doRequestWithRetry performs an HTTP request with exponential backoff retry.
func (c *Client) doRequestWithRetry(ctx context.Context, url string, result interface{}) error {
	var lastErr error
	
	for attempt := 0; attempt <= c.retry.MaxRetries; attempt++ {
		if attempt > 0 {
			// Calculate delay with exponential backoff
			delay := time.Duration(float64(c.retry.InitialDelay) * math.Pow(c.retry.BackoffMultiplier, float64(attempt-1)))
			if delay > c.retry.MaxDelay {
				delay = c.retry.MaxDelay
			}
			
			// Wait before retry, respecting context cancellation
			select {
			case <-ctx.Done():
				return fmt.Errorf("request cancelled: %w", ctx.Err())
			case <-time.After(delay):
			}
		}
		
		err := c.doRequest(ctx, url, result)
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		// Don't retry on client errors (4xx)
		if !c.shouldRetry(err) {
			return err
		}
	}
	
	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// doRequest performs a single HTTP GET request.
func (c *Client) doRequest(ctx context.Context, url string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "gonpi/1.0")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("API returned status %d: %s", resp.StatusCode, string(body)),
		}
	}
	
	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	
	return nil
}

// shouldRetry determines if an error is retryable.
func (c *Client) shouldRetry(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		// Retry on 5xx server errors and 429 rate limit
		return apiErr.StatusCode >= 500 || apiErr.StatusCode == 429
	}
	// Retry on network errors
	return true
}

// getCached retrieves a provider from cache if available and not expired.
func (c *Client) getCached(npi string) *Provider {
	c.cache.mu.RLock()
	defer c.cache.mu.RUnlock()
	
	entry, exists := c.cache.data[npi]
	if !exists || time.Now().After(entry.expiresAt) {
		return nil
	}
	
	return entry.provider
}

// setCached stores a provider in cache.
func (c *Client) setCached(npi string, provider *Provider) {
	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()
	
	c.cache.data[npi] = &cacheEntry{
		provider:  provider,
		expiresAt: time.Now().Add(5 * time.Minute), // Default 5 minute TTL
	}
}

// cleanupCache periodically removes expired cache entries.
func (c *Client) cleanupCache(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for range ticker.C {
		c.cache.mu.Lock()
		now := time.Now()
		for key, entry := range c.cache.data {
			if now.After(entry.expiresAt) {
				delete(c.cache.data, key)
			}
		}
		c.cache.mu.Unlock()
	}
}

// APIError represents an error from the NPI Registry API.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return e.Message
}

// GetProvidersByNPIs fetches multiple providers by NPI numbers in batch.
func (c *Client) GetProvidersByNPIs(ctx context.Context, npis []string) (map[string]*Provider, error) {
	if len(npis) == 0 {
		return nil, fmt.Errorf("npi list cannot be empty")
	}
	
	results := make(map[string]*Provider)
	var mu sync.Mutex
	var wg sync.WaitGroup
	
	// Limit concurrent requests to avoid overwhelming the API
	semaphore := make(chan struct{}, 5)
	errChan := make(chan error, len(npis))
	
	for _, npi := range npis {
		wg.Add(1)
		go func(npi string) {
			defer wg.Done()
			
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			provider, err := c.GetProviderByNPI(ctx, npi)
			if err != nil {
				errChan <- fmt.Errorf("failed to fetch NPI %s: %w", npi, err)
				return
			}
			
			mu.Lock()
			results[npi] = provider
			mu.Unlock()
		}(npi)
	}
	
	wg.Wait()
	close(errChan)
	
	// Collect any errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	
	if len(errs) > 0 {
		return results, fmt.Errorf("batch fetch completed with %d errors: %v", len(errs), errs[0])
	}
	
	return results, nil
}
