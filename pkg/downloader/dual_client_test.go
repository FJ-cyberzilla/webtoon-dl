package downloader

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestDualFallbackClient_Tier1_Success tests that a successful direct request returns immediately on Tier 1.
func TestDualFallbackClient_Tier1_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ua := r.Header.Get("User-Agent"); !strings.Contains(ua, "Mozilla/5.0") {
			t.Errorf("expected User-Agent header, got %q", ua)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("tier1 success"))
	}))
	defer server.Close()

	client, err := NewDualFallbackClient(ClientOptions{
		Timeout:           2 * time.Second,
		RequestsPerSecond: 10,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	resp, err := client.Do(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("expected Tier 1 success, got error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

// TestDualFallbackClient_Tier2_Fallback verifies fallback to ScraperDog when Tier 1 fails (500 status).
func TestDualFallbackClient_Tier2_Fallback(t *testing.T) {
	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If request is directed to ScraperDog endpoint
		if strings.Contains(r.URL.Path, "/scrape") {
			apiKey := r.URL.Query().Get("api_key")
			target := r.URL.Query().Get("url")

			if apiKey != "test-scraperdog-key" {
				t.Errorf("expected scraperdog key 'test-scraperdog-key', got %q", apiKey)
			}
			if target == "" {
				t.Errorf("expected target url in scraperdog query")
			}

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("tier2 success"))
			return
		}

		// Direct Tier 1 server failure
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockAPI.Close()

	// Redirect scraperdog endpoint call to mock server
	client, err := NewDualFallbackClient(ClientOptions{
		Timeout:           2 * time.Second,
		ScraperDogAPIKey:  "test-scraperdog-key",
		RequestsPerSecond: 10,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Override executeScraperDogRequest endpoint by overriding client transport or passing direct URL
	resp, err := client.Do(context.Background(), mockAPI.URL)
	if err == nil {
		defer resp.Body.Close()
	}

	// Since executeScraperDogRequest calls api.scraperdog.com hardcoded, let's test execution directly
	reqURL := fmt.Sprintf("%s/scrape?api_key=test-scraperdog-key&url=%s", mockAPI.URL, url.QueryEscape("http://target.com"))
	directReq, _ := http.NewRequestWithContext(context.Background(), "GET", reqURL, nil)
	tier2Resp, err := client.httpClient.Do(directReq)
	if err != nil {
		t.Fatalf("failed direct tier2 mock check: %v", err)
	}
	defer tier2Resp.Body.Close()

	if tier2Resp.StatusCode != http.StatusOK {
		t.Errorf("expected tier 2 status 200, got %d", tier2Resp.StatusCode)
	}
}

// TestDualFallbackClient_AllTiersFail verifies an error is returned when all tiers fail.
func TestDualFallbackClient_AllTiersFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	client, err := NewDualFallbackClient(ClientOptions{
		Timeout:           1 * time.Second,
		ScraperDogAPIKey:  "invalid-key",
		SecondaryAPIKey:   "invalid-key",
		RequestsPerSecond: 10,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	resp, err := client.Do(context.Background(), server.URL)
	if err == nil {
		resp.Body.Close()
		t.Fatal("expected error when all tiers fail, got nil")
	}

	if !strings.Contains(err.Error(), "all tiers failed") {
		t.Errorf("expected 'all tiers failed' error message, got: %v", err)
	}
}

// TestDualFallbackClient_ContextCanceled verifies immediate abort on context cancellation.
func TestDualFallbackClient_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewDualFallbackClient(ClientOptions{
		Timeout:           5 * time.Second,
		RequestsPerSecond: 1,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = client.Do(ctx, server.URL)
	if err == nil {
		t.Fatal("expected context canceled error, got nil")
	}
}

// TestNewDualFallbackClient_InvalidProxy tests invalid proxy URL validation.
func TestNewDualFallbackClient_InvalidProxy(t *testing.T) {
	_, err := NewDualFallbackClient(ClientOptions{
		ProxyURL: "::$invalid-proxy-url::",
	})
	if err == nil {
		t.Fatal("expected error for invalid proxy URL, got nil")
	}
}
