package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/FJ-cyberzilla/webtoon-dl/pkg/httpclient"

	"golang.org/x/time/rate"
)

// ClientOptions configures the dual fallback client.
type ClientOptions struct {
	Timeout           time.Duration
	RequestsPerSecond float64
	ScraperDogAPIKey  string
	SecondaryAPIKey   string
	ProxyURL          string
	MaxRetries        int
}

// DualClient implements a multi-tier fallback HTTP client.
type DualClient struct {
	httpClient      *http.Client
	rateLimiter     *rate.Limiter
	options         ClientOptions
	scraperDogURL   string
	secondaryAPIURL string
	mu              sync.RWMutex
}

// NewDualFallbackClient creates a client with primary and fallback mechanisms.
func NewDualFallbackClient(opts ClientOptions) (*DualClient, error) {
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.RequestsPerSecond == 0 {
		opts.RequestsPerSecond = 5
	}
	if opts.MaxRetries == 0 {
		opts.MaxRetries = 2
	}

	if opts.ProxyURL != "" {
		if _, err := url.Parse(opts.ProxyURL); err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}
	}

	transport := &http.Transport{
		MaxConnsPerHost:     10,
		MaxIdleConns:        20,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		DisableCompression:  false,
	}

	if opts.ProxyURL != "" {
		proxyURL, _ := url.Parse(opts.ProxyURL)
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	client := &DualClient{
		httpClient: httpclient.DecorateClient(&http.Client{
			Timeout:   opts.Timeout,
			Transport: transport,
		}, "", opts.Timeout, 10),
		rateLimiter:     rate.NewLimiter(rate.Limit(opts.RequestsPerSecond), 1),
		options:         opts,
		scraperDogURL:   "https://api.scraperdog.com/scrape",
		secondaryAPIURL: "https://api.scrapingbee.com/scrape",
	}

	return client, nil
}

// Do performs an HTTP request with fallback logic.
func (d *DualClient) Do(ctx context.Context, targetURL string) (*http.Response, error) {
	resp, err := d.tryTier1(ctx, targetURL)
	if err == nil {
		return resp, nil
	}
	lastErr := err

	resp, err = d.tryTier2(ctx, targetURL)
	if err == nil {
		return resp, nil
	}
	if err != errTierDisabled {
		lastErr = err
	}

	resp, err = d.tryTier3(ctx, targetURL)
	if err == nil {
		return resp, nil
	}
	if err != errTierDisabled {
		lastErr = err
	}

	return nil, fmt.Errorf("all tiers failed for %s: %w", targetURL, lastErr)
}

var errTierDisabled = fmt.Errorf("tier disabled")

func (d *DualClient) tryTier1(ctx context.Context, targetURL string) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= d.options.MaxRetries; attempt++ {
		resp, err := d.directRequest(ctx, targetURL)
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		lastErr = formatError(err, resp, "direct")
	}
	return nil, lastErr
}

func (d *DualClient) tryTier2(ctx context.Context, targetURL string) (*http.Response, error) {
	if d.options.ScraperDogAPIKey == "" {
		return nil, errTierDisabled
	}
	resp, err := d.scraperDogRequest(ctx, targetURL)
	if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}
	if resp != nil {
		resp.Body.Close()
	}
	return nil, formatError(err, resp, "scraperdog")
}

func (d *DualClient) tryTier3(ctx context.Context, targetURL string) (*http.Response, error) {
	if d.options.SecondaryAPIKey == "" || d.options.SecondaryAPIKey == d.options.ScraperDogAPIKey {
		return nil, errTierDisabled
	}
	resp, err := d.secondaryAPIRequest(ctx, targetURL)
	if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}
	if resp != nil {
		resp.Body.Close()
	}
	return nil, formatError(err, resp, "secondary API")
}

func formatError(err error, resp *http.Response, label string) error {
	if err != nil {
		return fmt.Errorf("%s failed: %w", label, err)
	}
	if resp != nil {
		return fmt.Errorf("%s HTTP %d", label, resp.StatusCode)
	}
	return fmt.Errorf("%s request failed", label)
}

// directRequest performs a direct HTTP request (Tier 1).
func (d *DualClient) directRequest(ctx context.Context, targetURL string) (*http.Response, error) {
	if err := d.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter wait failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("direct request failed: %w", err)
	}
	return resp, nil
}

// scraperDogRequest uses the ScraperDog API as fallback (Tier 2).
func (d *DualClient) scraperDogRequest(ctx context.Context, targetURL string) (*http.Response, error) {
	apiURL := fmt.Sprintf("%s?api_key=%s&url=%s",
		d.scraperDogURL,
		d.options.ScraperDogAPIKey,
		url.QueryEscape(targetURL))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create ScraperDog request: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ScraperDog request failed: %w", err)
	}
	return resp, nil
}

// secondaryAPIRequest uses a secondary API as final fallback (Tier 3).
func (d *DualClient) secondaryAPIRequest(ctx context.Context, targetURL string) (*http.Response, error) {
	apiURL := fmt.Sprintf("%s?api_key=%s&url=%s",
		d.secondaryAPIURL,
		d.options.SecondaryAPIKey,
		url.QueryEscape(targetURL))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create secondary API request: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("secondary API request failed: %w", err)
	}
	return resp, nil
}

// Get performs a GET request and returns the body as bytes.
func (d *DualClient) Get(ctx context.Context, targetURL string) ([]byte, error) {
	resp, err := d.Do(ctx, targetURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	return body, nil
}

// Close releases any resources held by the client.
func (d *DualClient) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if transport, ok := d.httpClient.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
}
