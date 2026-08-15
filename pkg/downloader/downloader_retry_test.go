package downloader

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// TestDownloadWithRetry_SuccessOnFirstAttempt tests immediate success without retries.
func TestDownloadWithRetry_SuccessOnFirstAttempt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("file-content"))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	dest := filepath.Join(tmpDir, "out.txt")

	cfg := RetryConfig{
		MaxAttempts: 3,
		InitialWait: 10 * time.Millisecond,
		MaxWait:     50 * time.Millisecond,
		Multiplier:  2.0,
		Jitter:      false,
	}

	err := DownloadWithRetry(context.Background(), server.Client(), server.URL, dest, nil, cfg)
	if err != nil {
		t.Fatalf("expected download to succeed, got: %v", err)
	}

	content, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}

	if string(content) != "file-content" {
		t.Errorf("expected 'file-content', got '%s'", string(content))
	}
}

// TestDownloadWithRetry_RetryThenSucceed verifies recovery after HTTP 500 & 429.
func TestDownloadWithRetry_RetryThenSucceed(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		current := atomic.AddInt32(&attempts, 1)
		switch current {
		case 1:
			w.WriteHeader(http.StatusInternalServerError)
		case 2:
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("recovered"))
		}
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	dest := filepath.Join(tmpDir, "recovered.txt")

	cfg := RetryConfig{
		MaxAttempts: 4,
		InitialWait: 5 * time.Millisecond,
		MaxWait:     20 * time.Millisecond,
		Multiplier:  1.5,
		Jitter:      false,
	}

	err := DownloadWithRetry(context.Background(), server.Client(), server.URL, dest, nil, cfg)
	if err != nil {
		t.Fatalf("expected download to succeed on 3rd attempt, got: %v", err)
	}

	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 total attempts, got %d", atomic.LoadInt32(&attempts))
	}
}

// TestDownloadWithRetry_PermanentErrorFailFast ensures non-retryable 404 errors fail immediately without retrying.
func TestDownloadWithRetry_PermanentErrorFailFast(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	dest := filepath.Join(tmpDir, "missing.txt")

	cfg := RetryConfig{
		MaxAttempts: 5,
		InitialWait: 10 * time.Millisecond,
		MaxWait:     50 * time.Millisecond,
		Multiplier:  2.0,
		Jitter:      false,
	}

	err := DownloadWithRetry(context.Background(), server.Client(), server.URL, dest, nil, cfg)
	if err == nil {
		t.Fatal("expected error on permanent 404 response, got nil")
	}

	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("expected exactly 1 attempt for permanent error, got %d", atomic.LoadInt32(&attempts))
	}
}

// TestDownloadWithRetry_ContextCanceledDuringBackoff ensures cancellation terminates backoff immediately.
func TestDownloadWithRetry_ContextCanceledDuringBackoff(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	dest := filepath.Join(tmpDir, "canceled.txt")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	cfg := RetryConfig{
		MaxAttempts: 5,
		InitialWait: 200 * time.Millisecond, // Longer than context timeout
		MaxWait:     500 * time.Millisecond,
		Multiplier:  2.0,
		Jitter:      false,
	}

	err := DownloadWithRetry(ctx, server.Client(), server.URL, dest, nil, cfg)
	if err == nil {
		t.Fatal("expected context canceled error, got nil")
	}
}

// TestIsRetryableStatus tests status code classification.
func TestIsRetryableStatus(t *testing.T) {
	tests := []struct {
		code     int
		expected bool
	}{
		{http.StatusOK, false},
		{http.StatusNotFound, false},
		{http.StatusForbidden, false},
		{http.StatusBadRequest, false},
		{http.StatusTooManyRequests, true},
		{http.StatusRequestTimeout, true},
		{http.StatusInternalServerError, true},
		{http.StatusBadGateway, true},
		{http.StatusServiceUnavailable, true},
	}

	for _, tt := range tests {
		result := isRetryableStatus(tt.code)
		if result != tt.expected {
			t.Errorf("isRetryableStatus(%d) = %v; want %v", tt.code, result, tt.expected)
		}
	}
}
