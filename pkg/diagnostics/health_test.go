package diagnostics

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ============ NETWORK PROBE TESTS ============

func TestNetworkProbe_Success(t *testing.T) {
	t.Run("reachable endpoint returns OK", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		}))
		defer server.Close()

		probe := NewNetworkProbe(server.Client())
		probe.resolver = server.URL
		probe.fallbackDNS = nil // Disable fallback for test

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result := probe.Execute(ctx)

		if result.Status != StatusOK {
			t.Errorf("expected StatusOK, got %s", result.Status)
		}
		if result.Severity != SeverityInfo {
			t.Errorf("expected SeverityInfo, got %s", result.Severity)
		}
		if !strings.Contains(result.Summary, "verified") {
			t.Errorf("expected success summary, got: %s", result.Summary)
		}

		// Verify latency was recorded
		if latency, ok := result.Details["latency_ms"].(int64); !ok || latency < 0 {
			t.Errorf("expected valid latency_ms, got: %v", result.Details["latency_ms"])
		}
	})
}

func TestNetworkProbe_ConnectionFailure(t *testing.T) {
	t.Run("unreachable endpoint returns Failed", func(t *testing.T) {
		client := &http.Client{
			Timeout: 500 * time.Millisecond,
			Transport: &http.Transport{
				DisableKeepAlives: true,
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return nil, fmt.Errorf("connection refused")
				},
			},
		}

		probe := NewNetworkProbe(client)
		probe.resolver = "http://192.0.2.1:81" // TEST-NET-1, guaranteed unreachable
		probe.fallbackDNS = []string{"192.0.2.1:53"}
		probe.lookupDomain = "non-existent.test.invalid"

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		result := probe.Execute(ctx)

		if result.Status != StatusFailed {
			t.Errorf("expected StatusFailed, got %s", result.Status)
		}
		if result.Severity != SeverityCritical {
			t.Errorf("expected SeverityCritical, got %s", result.Severity)
		}

		// Verify error details are present
		if _, ok := result.Details["http_error"]; !ok {
			t.Error("expected http_error in details")
		}
	})
}

func TestNetworkProbe_DNSFallback(t *testing.T) {
	t.Run("DNS fallback when HTTP fails", func(t *testing.T) {
		// Create a client that fails HTTP but can do DNS
		client := &http.Client{
			Timeout: 100 * time.Millisecond,
			Transport: &http.Transport{
				DisableKeepAlives: true,
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return nil, fmt.Errorf("dial tcp: connection refused")
				},
			},
		}

		probe := NewNetworkProbe(client)
		probe.resolver = "http://10.255.255.1:81" // Unreachable HTTP
		// Use real DNS servers for fallback test
		probe.fallbackDNS = []string{"8.8.8.8:53", "1.1.1.1:53"}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result := probe.Execute(ctx)

		// Status is Degraded if DNS works, Failed if not
		if result.Status != StatusDegraded && result.Status != StatusFailed {
			t.Errorf("expected StatusDegraded or StatusFailed, got %s", result.Status)
		}
	})
}

func TestNetworkProbe_InvalidRequestConstruction(t *testing.T) {
	t.Run("invalid URL fails at construction", func(t *testing.T) {
		probe := NewNetworkProbe(&http.Client{})
		probe.resolver = "://invalid-url"
		probe.fallbackDNS = nil

		result := probe.Execute(context.Background())

		if result.Status != StatusFailed {
			t.Errorf("expected StatusFailed, got %s", result.Status)
		}
		if result.Severity != SeverityCritical {
			t.Errorf("expected SeverityCritical, got %s", result.Severity)
		}
	})
}

// ============ TARGET PROBE TESTS ============

func TestTargetProbe_Success(t *testing.T) {
	t.Run("accessible endpoint returns OK", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Server", "nginx/1.18.0")
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html><body>Webtoon Content</body></html>"))
		}))
		defer server.Close()

		probe := NewTargetProbe(server.Client(), server.URL)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result := probe.Execute(ctx)

		if result.Status != StatusOK {
			t.Errorf("expected StatusOK, got %s", result.Status)
		}
		if result.Severity != SeverityInfo {
			t.Errorf("expected SeverityInfo, got %s", result.Severity)
		}

		// Verify details
		details := result.Details
		if statusCode, ok := details["status_code"].(int); !ok || statusCode != 200 {
			t.Errorf("expected status_code 200, got: %v", details["status_code"])
		}
		if server, ok := details["server"].(string); !ok || server != "nginx/1.18.0" {
			t.Errorf("expected server nginx/1.18.0, got: %v", details["server"])
		}
	})
}

func TestTargetProbe_StatusCodes(t *testing.T) {
	tests := []struct {
		name             string
		statusCode       int
		expectedStatus   Status
		expectedSeverity Severity
	}{
		{
			name:             "200 OK is successful",
			statusCode:       http.StatusOK,
			expectedStatus:   StatusOK,
			expectedSeverity: SeverityInfo,
		},
		{
			name:             "404 is degraded",
			statusCode:       http.StatusNotFound,
			expectedStatus:   StatusDegraded,
			expectedSeverity: SeverityWarning,
		},
		{
			name:             "503 is degraded",
			statusCode:       http.StatusServiceUnavailable,
			expectedStatus:   StatusDegraded,
			expectedSeverity: SeverityWarning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			probe := NewTargetProbe(server.Client(), server.URL)
			result := probe.Execute(context.Background())

			if result.Status != tt.expectedStatus {
				t.Errorf("expected status %s, got %s", tt.expectedStatus, result.Status)
			}
			if result.Severity != tt.expectedSeverity {
				t.Errorf("expected severity %s, got %s", tt.expectedSeverity, result.Severity)
			}
		})
	}
}

func TestTargetProbe_AntiBotDetection(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		headers        map[string]string
		expectedCheck  string
		expectedStatus Status
		minIndicators  int
	}{
		{
			name:       "cloudflare challenge",
			statusCode: http.StatusForbidden,
			headers: map[string]string{
				"Server":       "cloudflare",
				"CF-Ray":       "8a1f2b3c4d5e6f7",
				"cf-mitigated": "challenge",
			},
			expectedCheck:  "anti_bot_detection",
			expectedStatus: StatusDegraded,
			minIndicators:  2,
		},
		{
			name:       "rate limited",
			statusCode: http.StatusTooManyRequests,
			headers: map[string]string{
				"Retry-After": "30",
			},
			expectedCheck:  "anti_bot_detection",
			expectedStatus: StatusDegraded,
			minIndicators:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				for key, value := range tt.headers {
					w.Header().Set(key, value)
				}
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			probe := NewTargetProbe(server.Client(), server.URL)
			result := probe.Execute(context.Background())

			if result.Check != tt.expectedCheck {
				t.Errorf("expected check %s, got %s", tt.expectedCheck, result.Check)
			}
			if result.Status != tt.expectedStatus {
				t.Errorf("expected status %s, got %s", tt.expectedStatus, result.Status)
			}

			// Verify indicators
			indicators, ok := result.Details["indicators"].([]string)
			if !ok {
				t.Fatalf("expected indicators in details, got: %v", result.Details)
			}
			if len(indicators) < tt.minIndicators {
				t.Errorf("expected at least %d indicators, got %d: %v",
					tt.minIndicators, len(indicators), indicators)
			}
		})
	}
}

func TestTargetProbe_InvalidURL(t *testing.T) {
	t.Run("malformed URL returns Failed", func(t *testing.T) {
		client := &http.Client{Timeout: 5 * time.Second}
		probe := NewTargetProbe(client, "://invalid-url")

		result := probe.Execute(context.Background())

		if result.Status != StatusFailed {
			t.Errorf("expected StatusFailed, got %s", result.Status)
		}
		if result.Severity != SeverityCritical {
			t.Errorf("expected SeverityCritical, got %s", result.Severity)
		}
		if result.Summary != "Malformed target URL" {
			t.Errorf("expected 'Malformed target URL', got: %s", result.Summary)
		}
	})
}

func TestTargetProbe_Timeout(t *testing.T) {
	t.Run("slow endpoint times out", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := &http.Client{Timeout: 100 * time.Millisecond}
		probe := NewTargetProbe(client, server.URL)

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		result := probe.Execute(ctx)

		if result.Status != StatusFailed {
			t.Errorf("expected StatusFailed, got %s", result.Status)
		}
		if result.Severity != SeverityCritical {
			t.Errorf("expected SeverityCritical, got %s", result.Severity)
		}
	})
}

// ============ FILESYSTEM PROBE TESTS ============

func TestFilesystemProbe_Success(t *testing.T) {
	t.Run("writable directory", func(t *testing.T) {
		tempDir := t.TempDir()
		probe := NewFilesystemProbe(tempDir)

		result := probe.Execute(context.Background())

		if result.Status != StatusOK {
			t.Errorf("expected StatusOK, got %s", result.Status)
		}
		if result.Severity != SeverityInfo {
			t.Errorf("expected SeverityInfo, got %s", result.Severity)
		}

		// Verify no temporary files left behind
		files, err := os.ReadDir(tempDir)
		if err != nil {
			t.Fatalf("failed to read temp dir: %v", err)
		}
		if len(files) != 0 {
			t.Errorf("expected no leftover files, found %d", len(files))
		}
	})
}

func TestFilesystemProbe_ReadOnly(t *testing.T) {
	t.Run("read-only directory fails", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("test requires non-root user")
		}

		tempDir := t.TempDir()
		readOnlyDir := filepath.Join(tempDir, "readonly")
		if err := os.MkdirAll(readOnlyDir, 0555); err != nil {
			t.Fatalf("failed to create read-only dir: %v", err)
		}
		defer func() { _ = os.Chmod(readOnlyDir, 0755) }() // Cleanup

		probe := NewFilesystemProbe(readOnlyDir)
		result := probe.Execute(context.Background())

		if result.Status != StatusFailed {
			t.Errorf("expected StatusFailed, got %s", result.Status)
		}
		if result.Severity != SeverityCritical {
			t.Errorf("expected SeverityCritical, got %s", result.Severity)
		}
	})
}

func TestFilesystemProbe_CreatesNestedDirectories(t *testing.T) {
	t.Run("creates nested directories", func(t *testing.T) {
		baseDir := t.TempDir()
		nestedDir := filepath.Join(baseDir, "nested", "path", "logs")

		probe := NewFilesystemProbe(nestedDir)
		result := probe.Execute(context.Background())

		if result.Status != StatusOK {
			t.Errorf("expected StatusOK, got %s", result.Status)
		}

		// Verify directory was created
		if _, err := os.Stat(nestedDir); os.IsNotExist(err) {
			t.Error("expected nested directory to be created")
		}
	})
}

func TestFilesystemProbe_LowDiskSpace(t *testing.T) {
	t.Run("warns on low disk space", func(t *testing.T) {
		tempDir := t.TempDir()
		probe := NewFilesystemProbe(tempDir)
		probe.minFreeMB = uint64(^uint64(0) >> 1) // Set impossibly high

		result := probe.Execute(context.Background())

		if result.Status != StatusDegraded {
			t.Errorf("expected StatusDegraded, got %s", result.Status)
		}
		if result.Severity != SeverityWarning {
			t.Errorf("expected SeverityWarning, got %s", result.Severity)
		}

		// Verify details
		if _, ok := result.Details["free_mb"]; !ok {
			t.Error("expected free_mb in details")
		}
		if _, ok := result.Details["min_recommended_mb"]; !ok {
			t.Error("expected min_recommended_mb in details")
		}
	})
}

// ============ RUNNER INTEGRATION TESTS ============

func TestRunner_AllProbesSucceed(t *testing.T) {
	t.Run("all probes pass", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		tempDir := t.TempDir()

		// Real probes
		networkProbe := NewNetworkProbe(server.Client())
		networkProbe.resolver = server.URL
		networkProbe.fallbackDNS = nil

		targetProbe := NewTargetProbe(server.Client(), server.URL)
		fsProbe := NewFilesystemProbe(tempDir)

		runner := NewRunner(
			WithHTTPClient(server.Client()),
			WithMaxConcurrency(3),
		)
		runner.Register(networkProbe, targetProbe, fsProbe)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		start := time.Now()
		results := runner.Execute(ctx)
		elapsed := time.Since(start)

		if len(results) != 3 {
			t.Fatalf("expected 3 results, got %d", len(results))
		}

		for _, result := range results {
			if result.Status != StatusOK {
				t.Errorf("probe %s: expected StatusOK, got %s (summary: %s)",
					result.Check, result.Status, result.Summary)
			}
		}

		// Completes quickly due to concurrency
		if elapsed > 5*time.Second {
			t.Errorf("execution took %v, expected under 5s", elapsed)
		}

		// Verify aggregation
		summary := AggregateResults(results)
		if summary[StatusOK] != 3 {
			t.Errorf("expected 3 OK results, got %d", summary[StatusOK])
		}
	})
}

func TestRunner_PartialFailure(t *testing.T) {
	t.Run("handles mixed results", func(t *testing.T) {
		tempDir := t.TempDir()

		// Good probe
		fsProbe := NewFilesystemProbe(tempDir)

		// Bad probe
		targetProbe := NewTargetProbe(&http.Client{}, "invalid-url")

		runner := NewRunner()
		runner.Register(fsProbe, targetProbe)

		results := runner.Execute(context.Background())

		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}

		var okCount, failCount int
		for _, result := range results {
			switch result.Status {
			case StatusOK:
				okCount++
			case StatusFailed:
				failCount++
			case StatusDegraded:
				// Degraded counted separately or ignored if desired, but case must exist
			}
		}

		if okCount != 1 || failCount != 1 {
			t.Errorf("expected 1 OK and 1 Failed, got %d OK and %d Failed",
				okCount, failCount)
		}
	})
}

func TestRunner_ContextCancellation(t *testing.T) {
	t.Run("cancellation propagates to probes", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-time.After(5 * time.Second):
				w.WriteHeader(http.StatusOK)
			case <-r.Context().Done():
				return
			}
		}))
		defer server.Close()

		probe := NewTargetProbe(server.Client(), server.URL)
		runner := NewRunner(WithMaxConcurrency(1))
		runner.Register(probe)

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		results := runner.Execute(ctx)

		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}

		if results[0].Status != StatusFailed {
			t.Errorf("expected StatusFailed due to cancellation, got %s", results[0].Status)
		}
	})
}

func TestRunner_NoProbes(t *testing.T) {
	t.Run("nil result for empty runner", func(t *testing.T) {
		runner := NewRunner()
		results := runner.Execute(context.Background())

		if results != nil {
			t.Errorf("expected nil for no probes, got %v", results)
		}
	})
}

// ============ UTILITY FUNCTION TESTS ============

func TestSanitizeHeaders(t *testing.T) {
	t.Run("removes sensitive headers", func(t *testing.T) {
		headers := http.Header{
			"Server":        []string{"nginx/1.18.0"},
			"Content-Type":  []string{"text/html"},
			"CF-Ray":        []string{"sensitive-data"},
			"X-Internal-IP": []string{"10.0.0.1"},
			"Authorization": []string{"Bearer secret"},
			"Retry-After":   []string{"30"},
		}

		sanitized := sanitizeHeaders(headers)

		// Contains safe headers
		if sanitized["Server"] != "nginx/1.18.0" {
			t.Error("expected Server header to be preserved")
		}
		if sanitized["Content-Type"] != "text/html" {
			t.Error("expected Content-Type header to be preserved")
		}
		if sanitized["Retry-After"] != "30" {
			t.Error("expected Retry-After header to be preserved")
		}

		// Does not contain sensitive headers
		sensitiveHeaders := []string{"CF-Ray", "X-Internal-IP", "Authorization"}
		for _, header := range sensitiveHeaders {
			if _, exists := sanitized[header]; exists {
				t.Errorf("%s is not sanitized", header)
			}
		}
	})
}

func TestDetectAntiBot_CloudFront(t *testing.T) {
	t.Run("detects cloudfront error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rec.Header().Set("X-Cache", "Error from cloudfront")
		rec.WriteHeader(http.StatusOK)

		indicators := detectAntiBot(rec.Result())

		found := false
		for _, ind := range indicators {
			if ind == "cloudfront_error" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected cloudfront_error indicator")
		}
	})
}

func TestDetectAntiBot_CloudflareBlock(t *testing.T) {
	t.Run("detects cloudflare block", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rec.Header().Set("Server", "cloudflare")
		rec.WriteHeader(http.StatusForbidden)

		indicators := detectAntiBot(rec.Result())

		found := false
		for _, ind := range indicators {
			if ind == "cloudflare_block" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected cloudflare_block indicator")
		}
	})
}

func TestAggregateResults(t *testing.T) {
	t.Run("counts results by status", func(t *testing.T) {
		results := []Result{
			{Check: "test1", Status: StatusOK, Severity: SeverityInfo},
			{Check: "test2", Status: StatusOK, Severity: SeverityInfo},
			{Check: "test3", Status: StatusDegraded, Severity: SeverityWarning},
			{Check: "test4", Status: StatusFailed, Severity: SeverityCritical},
		}

		summary := AggregateResults(results)

		if summary[StatusOK] != 2 {
			t.Errorf("expected 2 OK, got %d", summary[StatusOK])
		}
		if summary[StatusDegraded] != 1 {
			t.Errorf("expected 1 degraded, got %d", summary[StatusDegraded])
		}
		if summary[StatusFailed] != 1 {
			t.Errorf("expected 1 failed, got %d", summary[StatusFailed])
		}
	})
}

// ============ BENCHMARKS ============

func BenchmarkNetworkProbe(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	probe := NewNetworkProbe(server.Client())
	probe.resolver = server.URL
	probe.fallbackDNS = nil

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		probe.Execute(ctx)
	}
}

func BenchmarkRunnerConcurrent(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tempDir := b.TempDir()

	runner := NewRunner(WithMaxConcurrency(4))
	runner.Register(
		NewNetworkProbe(server.Client()),
		NewTargetProbe(server.Client(), server.URL),
		NewFilesystemProbe(tempDir),
	)

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		runner.Execute(ctx)
	}
}

func TestFilesystemProbe_PathIsFile(t *testing.T) {
	t.Run("fails when path is a file", func(t *testing.T) {
		tempFile := filepath.Join(t.TempDir(), "file.txt")
		if err := os.WriteFile(tempFile, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		probe := NewFilesystemProbe(tempFile)
		result := probe.Execute(context.Background())

		// MkdirAll will fail because a file exists at that path
		if result.Status != StatusFailed {
			t.Errorf("expected StatusFailed, got %s", result.Status)
		}
	})
}
