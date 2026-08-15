package httpclient_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/FJ-cyberzilla/webtoon-dl/pkg/httpclient"
)

func TestUserAgentTransport_Default(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		if ua != httpclient.DefaultUserAgent {
			t.Errorf("expected default User-Agent %q, got %q", httpclient.DefaultUserAgent, ua)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := httpclient.NewClient("", 30*time.Second, 5)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", resp.Status)
	}
}

func TestUserAgentTransport_Custom(t *testing.T) {
	customUA := "CustomAgent/1.0"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		if ua != customUA {
			t.Errorf("expected custom User-Agent %q, got %q", customUA, ua)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := httpclient.NewClient(customUA, 30*time.Second, 5)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", resp.Status)
	}
}

func TestDecorateClient(t *testing.T) {
	customUA := "DecoratedAgent/1.0"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		if ua != customUA {
			t.Errorf("expected custom User-Agent %q, got %q", customUA, ua)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	baseClient := &http.Client{}
	client := httpclient.DecorateClient(baseClient, customUA, 30*time.Second, 5)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", resp.Status)
	}
}

func TestValidateURLScheme(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{"http://example.com", false},
		{"https://example.com", false},
		{"ftp://example.com", true},
		{"invalid url", true},
	}

	for _, tc := range tests {
		err := httpclient.ValidateURLScheme(tc.url)
		if (err != nil) != tc.wantErr {
			t.Errorf("ValidateURLScheme(%s) error = %v, wantErr %v", tc.url, err, tc.wantErr)
		}
	}
}

func TestRedirectLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "/redirect")
		w.WriteHeader(http.StatusMovedPermanently)
	}))
	defer server.Close()

	client := httpclient.NewClient("", time.Second, 1) // limit 1
	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)

	_, err := client.Do(req)
	if err == nil {
		t.Error("Expected error due to redirect limit, got nil")
	}
}

func TestDecorateClient_NilClient(t *testing.T) {
	client := httpclient.DecorateClient(nil, "TestAgent", 10*time.Second, 5)
	if client == nil {
		t.Fatal("DecorateClient with nil client returned nil")
	}
	if client.Timeout != 10*time.Second {
		t.Errorf("Expected timeout 10s, got %v", client.Timeout)
	}
}
