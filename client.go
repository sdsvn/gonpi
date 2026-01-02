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

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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

	// TracerName is the name used for OpenTelemetry tracer.
	TracerName = "github.com/sdsvn/gonpi"
)

// Client is the NPI Registry API client.
type Client struct {
	baseURL    string
	httpClient *http.Client
	retry      RetryConfig
	cache      *cacheStore
	tracer     trace.Tracer
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
		tracer: otel.Tracer(TracerName),
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

// WithTracer sets a custom OpenTelemetry tracer.
func WithTracer(tracer trace.Tracer) ClientOption {
	return func(c *Client) {
		c.tracer = tracer
	}
}

// GetProviderByNPI fetches a single provider by NPI number.
func (c *Client) GetProviderByNPI(ctx context.Context, npi string) (*Provider, error) {
	ctx, span := c.tracer.Start(ctx, "GetProviderByNPI",
		trace.WithAttributes(
			attribute.String("npi", npi),
		),
	)
	defer span.End()

	if npi == "" {
		err := fmt.Errorf("npi cannot be empty")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	// Check cache first
	if c.cache.enabled {
		if provider := c.getCached(npi); provider != nil {
			span.SetAttributes(attribute.Bool("cache_hit", true))
			return provider, nil
		}
		span.SetAttributes(attribute.Bool("cache_hit", false))
	}

	opts := SearchOptions{
		Number: npi,
		Limit:  1,
	}

	providers, err := c.SearchProviders(ctx, opts)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to search providers")
		return nil, fmt.Errorf("failed to get provider by NPI %s: %w", npi, err)
	}

	if len(providers) == 0 {
		err := fmt.Errorf("no provider found with NPI %s", npi)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
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
	ctx, span := c.tracer.Start(ctx, "SearchProviders",
		trace.WithAttributes(
			attribute.String("last_name", opts.LastName),
			attribute.String("first_name", opts.FirstName),
			attribute.String("organization_name", opts.OrganizationName),
			attribute.String("state", opts.State),
			attribute.String("city", opts.City),
			attribute.Int("limit", opts.Limit),
			attribute.Int("skip", opts.Skip),
		),
	)
	defer span.End()

	// Build query parameters
	params := c.buildQueryParams(opts)

	// Construct URL
	apiURL := fmt.Sprintf("%s/?%s", c.baseURL, params.Encode())
	span.SetAttributes(attribute.String("url", apiURL))

	// Make request with retry logic
	var response APIResponse
	err := c.doRequestWithRetry(ctx, apiURL, &response)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "search request failed")
		return nil, fmt.Errorf("search providers failed: %w", err)
	}

	span.SetAttributes(attribute.Int("result_count", len(response.Results)))
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
	ctx, span := c.tracer.Start(ctx, "doRequestWithRetry",
		trace.WithAttributes(
			attribute.String("url", url),
			attribute.Int("max_retries", c.retry.MaxRetries),
		),
	)
	defer span.End()

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
			span.SetAttributes(attribute.Int("attempts", attempt+1))
			return nil
		}

		lastErr = err
		span.AddEvent("retry_attempt",
			trace.WithAttributes(
				attribute.Int("attempt", attempt+1),
				attribute.String("error", err.Error()),
			),
		)

		// Don't retry on client errors (4xx)
		if !c.shouldRetry(err) {
			span.RecordError(err)
			span.SetStatus(codes.Error, "non-retryable error")
			return err
		}
	}

	span.RecordError(lastErr)
	span.SetStatus(codes.Error, "max retries exceeded")
	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// doRequest performs a single HTTP GET request.
func (c *Client) doRequest(ctx context.Context, url string, result interface{}) error {
	ctx, span := c.tracer.Start(ctx, "doRequest",
		trace.WithAttributes(
			attribute.String("http.method", "GET"),
			attribute.String("http.url", url),
		),
	)
	defer span.End()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create request")
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "gonpi/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "http request failed")
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		apiErr := &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("API returned status %d: %s", resp.StatusCode, string(body)),
		}
		span.RecordError(apiErr)
		span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", resp.StatusCode))
		return apiErr
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to decode response")
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
	ctx, span := c.tracer.Start(ctx, "GetProvidersByNPIs",
		trace.WithAttributes(
			attribute.Int("npi_count", len(npis)),
		),
	)
	defer span.End()

	if len(npis) == 0 {
		err := fmt.Errorf("npi list cannot be empty")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
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

	span.SetAttributes(
		attribute.Int("successful_fetches", len(results)),
		attribute.Int("failed_fetches", len(errs)),
	)

	if len(errs) > 0 {
		err := fmt.Errorf("batch fetch completed with %d errors: %v", len(errs), errs[0])
		span.RecordError(err)
		span.SetStatus(codes.Error, "partial batch failure")
		return results, err
	}

	return results, nil
}
