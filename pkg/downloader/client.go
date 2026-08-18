package downloader

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter wraps golang.org/x/time/rate
type RateLimiter struct {
	*rate.Limiter
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	return &RateLimiter{
		Limiter: rate.NewLimiter(rate.Limit(rps), burst),
	}
}

// RetryConfig configures exponential backoff behavior.
type RetryConfig struct {
	MaxAttempts int           // Maximum number of attempts (e.g., 5)
	InitialWait time.Duration // Initial delay before the first retry (e.g., 500ms)
	MaxWait     time.Duration // Maximum cap for delay (e.g., 10s)
	Multiplier  float64       // Backoff factor (e.g., 2.0)
	Jitter      bool          // Add randomness to prevent synchronized retries
}

// DefaultRetryConfig provides sensible defaults for web scrapers/downloaders.
var DefaultRetryConfig = RetryConfig{
	MaxAttempts: 5,
	InitialWait: 500 * time.Millisecond,
	MaxWait:     10 * time.Second,
	Multiplier:  2.0,
	Jitter:      true,
}

// DownloadWithRetry downloads a file to destPath with exponential backoff retries.
func DownloadWithRetry(ctx context.Context, client *http.Client, url, destPath string, headers map[string]string, config RetryConfig) error {
	var lastErr error

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		if attempt > 0 {
			backoff := calculateBackoff(attempt, config)

			select {
			case <-ctx.Done():
				return fmt.Errorf("download canceled during backoff: %w", ctx.Err())
			case <-time.After(backoff):
			}
		}

		err := attemptDownload(ctx, client, url, destPath, headers, attempt)
		if err == nil {
			return nil
		}
		lastErr = err

		// Check if error is retryable
		if !isRetryableError(err) {
			return err
		}
	}

	return fmt.Errorf("exceeded max retries (%d attempts): %w", config.MaxAttempts, lastErr)
}

func attemptDownload(ctx context.Context, client *http.Client, url, destPath string, headers map[string]string, attempt int) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("network error on attempt %d: %w", attempt+1, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		err = AtomicWriteFile(destPath, resp.Body)
		if err != nil {
			return fmt.Errorf("failed to save file on attempt %d: %w", attempt+1, err)
		}
		return nil
	}

	if isRetryableStatus(resp.StatusCode) {
		return fmt.Errorf("HTTP %d on attempt %d", resp.StatusCode, attempt+1)
	}

	return fmt.Errorf("permanent HTTP error %d for URL: %s", resp.StatusCode, url)
}

func isRetryableError(err error) bool {
	// Check if it's a permanent error
	if strings.Contains(err.Error(), "permanent HTTP error") {
		return false
	}
	return true
}

// calculateBackoff computes duration = initialWait * (multiplier ^ (attempt - 1)) + jitter
func calculateBackoff(attempt int, config RetryConfig) time.Duration {
	multiplier := math.Pow(config.Multiplier, float64(attempt-1))
	wait := float64(config.InitialWait) * multiplier

	if wait > float64(config.MaxWait) {
		wait = float64(config.MaxWait)
	}

import (
	"crypto/rand"
	"math"
	"math/big"
	"time"
)

// ...

	if config.Jitter {
		// Use crypto/rand for jitter to avoid weak PRNG usage
		n, err := rand.Int(rand.Reader, big.NewInt(1000000))
		if err == nil {
			jitter := float64(n.Int64()) / 1000000.0
			wait = jitter * wait
		}
	}

	return time.Duration(wait)
}

// isRetryableStatus returns true for 429 (Rate Limit), 408 (Request Timeout), and 5xx (Server Error) codes
func isRetryableStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests ||
		statusCode == http.StatusRequestTimeout ||
		(statusCode >= 500 && statusCode <= 599)
}
