package diagnostics

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/FJ-cyberzilla/webtoon-dl/pkg/httpclient"
)

// Result encapsulates the outcome of a diagnostic probe with actionable intelligence.
type Result struct {
	Check    string         `json:"check"`
	Status   Status         `json:"status"`
	Severity Severity       `json:"severity"`
	Summary  string         `json:"summary"`
	Details  map[string]any `json:"details,omitempty"`
}

type Status string

const (
	StatusOK       Status = "ok"
	StatusDegraded Status = "degraded"
	StatusFailed   Status = "failed"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Probe defines the contract for executing a single diagnostic check.
type Probe interface {
	Name() string
	Execute(ctx context.Context) Result
}

// Runner orchestrates concurrent execution of diagnostic probes.
type Runner struct {
	client   *http.Client
	probes   []Probe
	maxConns int
}

// RunnerOption configures diagnostic runner behavior.
type RunnerOption func(*Runner)

// WithHTTPClient injects a custom HTTP client for network probes.
func WithHTTPClient(client *http.Client) RunnerOption {
	return func(r *Runner) {
		if client != nil {
			r.client = client
		}
	}
}

// WithMaxConcurrency limits parallel probe execution.
func WithMaxConcurrency(limit int) RunnerOption {
	return func(r *Runner) {
		if limit > 0 {
			r.maxConns = limit
		}
	}
}

// NewRunner constructs a diagnostic runner with sensible defaults.
func NewRunner(opts ...RunnerOption) *Runner {
	client := &http.Client{
		Timeout: 12 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
			DisableCompression:  false,
			TLSHandshakeTimeout: 5 * time.Second,
		},
	}

	r := &Runner{
		client:   httpclient.DecorateClient(client, "", 12*time.Second, 5),
		maxConns: 4,
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Register adds probes to the diagnostic pipeline.
func (r *Runner) Register(probes ...Probe) {
	r.probes = append(r.probes, probes...)
}

// Execute runs all registered probes concurrently and aggregates results.
func (r *Runner) Execute(ctx context.Context) []Result {
	if len(r.probes) == 0 {
		return nil
	}

	sem := make(chan struct{}, r.maxConns)
	results := make([]Result, len(r.probes))
	var wg sync.WaitGroup

	for i, probe := range r.probes {
		wg.Add(1)
		go func(idx int, p Probe) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[idx] = p.Execute(ctx)
		}(i, probe)
	}

	wg.Wait()
	return results
}

// NetworkProbe validates outbound connectivity and DNS resolution.
type NetworkProbe struct {
	client       *http.Client
	resolver     string
	fallbackDNS  []string
	lookupDomain string
}

// NewNetworkProbe creates a network diagnostic probe.
func NewNetworkProbe(client *http.Client) *NetworkProbe {
	return &NetworkProbe{
		client:       client,
		resolver:     "https://1.1.1.1",
		fallbackDNS:  []string{"8.8.8.8:53", "9.9.9.9:53"},
		lookupDomain: "cloudflare.com",
	}
}

func (p *NetworkProbe) Name() string { return "connectivity" }

func (p *NetworkProbe) Execute(ctx context.Context) Result {
	start := time.Now()

	// HTTP connectivity check
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.resolver, nil)
	if err != nil {
		return Result{
			Check:    p.Name(),
			Status:   StatusFailed,
			Severity: SeverityCritical,
			Summary:  "Failed to construct connectivity probe",
			Details:  map[string]any{"error": err.Error()},
		}
	}

	resp, err := p.client.Do(req)
	if err != nil {
		// Fallback: raw DNS lookup
		if dnsErr := p.checkDNS(ctx); dnsErr != nil {
			return Result{
				Check:    p.Name(),
				Status:   StatusFailed,
				Severity: SeverityCritical,
				Summary:  "No outbound connectivity or DNS resolution",
				Details: map[string]any{
					"http_error": err.Error(),
					"dns_error":  dnsErr.Error(),
				},
			}
		}

		return Result{
			Check:    p.Name(),
			Status:   StatusDegraded,
			Severity: SeverityWarning,
			Summary:  "DNS resolves but HTTP connectivity is impaired",
			Details:  map[string]any{"http_error": err.Error()},
		}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))

	return Result{
		Check:    p.Name(),
		Status:   StatusOK,
		Severity: SeverityInfo,
		Summary:  "Outbound connectivity verified",
		Details: map[string]any{
			"latency_ms":  time.Since(start).Milliseconds(),
			"status_code": resp.StatusCode,
		},
	}
}

func (p *NetworkProbe) checkDNS(ctx context.Context) error {
	for _, server := range p.fallbackDNS {
		resolver := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
				d := net.Dialer{Timeout: 3 * time.Second}
				return d.DialContext(ctx, "udp", server)
			},
		}

		if _, err := resolver.LookupHost(ctx, p.lookupDomain); err == nil {
			return nil
		}
	}
	return fmt.Errorf("all DNS servers unreachable")
}

// TargetProbe validates specific endpoint accessibility and anti-bot measures.
type TargetProbe struct {
	client       *http.Client
	targetURL    string
	allowedCodes map[int]bool
}

// NewTargetProbe creates an endpoint diagnostic probe.
func NewTargetProbe(client *http.Client, targetURL string) *TargetProbe {
	return &TargetProbe{
		client:    client,
		targetURL: targetURL,
		allowedCodes: map[int]bool{
			http.StatusOK:          true,
			http.StatusFound:       true,
			http.StatusNotModified: true,
		},
	}
}

func (p *TargetProbe) Name() string { return "target_endpoint" }

func (p *TargetProbe) Execute(ctx context.Context) Result {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.targetURL, nil)
	if err != nil {
		return Result{
			Check:    p.Name(),
			Status:   StatusFailed,
			Severity: SeverityCritical,
			Summary:  "Malformed target URL",
			Details:  map[string]any{"error": err.Error()},
		}
	}

	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := p.client.Do(req)
	latency := time.Since(start)

	if err != nil {
		return Result{
			Check:    p.Name(),
			Status:   StatusFailed,
			Severity: SeverityCritical,
			Summary:  "Endpoint unreachable",
			Details: map[string]any{
				"error":      err.Error(),
				"latency_ms": latency.Milliseconds(),
			},
		}
	}
	defer resp.Body.Close()

	// Analyze anti-bot indicators
	indicators := detectAntiBot(resp)

	if len(indicators) > 0 {
		return Result{
			Check:    "anti_bot_detection",
			Status:   StatusDegraded,
			Severity: SeverityWarning,
			Summary:  "Anti-bot protection detected",
			Details: map[string]any{
				"indicators":     indicators,
				"status_code":    resp.StatusCode,
				"latency_ms":     latency.Milliseconds(),
				"recommendation": "Reduce request frequency or implement session persistence",
			},
		}
	}

	if !p.allowedCodes[resp.StatusCode] {
		return Result{
			Check:    p.Name(),
			Status:   StatusDegraded,
			Severity: SeverityWarning,
			Summary:  fmt.Sprintf("Unexpected HTTP status: %d", resp.StatusCode),
			Details: map[string]any{
				"status_code": resp.StatusCode,
				"headers":     sanitizeHeaders(resp.Header),
				"latency_ms":  latency.Milliseconds(),
			},
		}
	}

	return Result{
		Check:    p.Name(),
		Status:   StatusOK,
		Severity: SeverityInfo,
		Summary:  "Endpoint accessible",
		Details: map[string]any{
			"status_code": resp.StatusCode,
			"latency_ms":  latency.Milliseconds(),
			"server":      resp.Header.Get("Server"),
		},
	}
}

func detectAntiBot(resp *http.Response) []string {
	var indicators []string

	if resp.Header.Get("CF-Ray") != "" {
		indicators = append(indicators, "cloudflare_ray")
	}

	if resp.Header.Get("cf-mitigated") != "" {
		indicators = append(indicators, "cloudflare_mitigation")
	}

	if resp.Header.Get("X-Cache") == "Error from cloudfront" {
		indicators = append(indicators, "cloudfront_error")
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		indicators = append(indicators, "rate_limited")
	}

	if resp.StatusCode == http.StatusForbidden &&
		strings.Contains(resp.Header.Get("Server"), "cloudflare") {
		indicators = append(indicators, "cloudflare_block")
	}

	return indicators
}

func sanitizeHeaders(headers http.Header) map[string]string {
	safe := make(map[string]string)
	allowed := map[string]bool{
		"Server": true, "Content-Type": true, "Cache-Control": true,
		"Retry-After": true, "X-RateLimit-Remaining": true,
	}

	for key := range headers {
		if allowed[key] {
			safe[key] = headers.Get(key)
		}
	}
	return safe
}

// FilesystemProbe validates write permissions and disk health.
type FilesystemProbe struct {
	path      string
	writeTest bool
	minFreeMB uint64
}

// NewFilesystemProbe creates a filesystem diagnostic probe.
func NewFilesystemProbe(path string) *FilesystemProbe {
	return &FilesystemProbe{
		path:      path,
		writeTest: true,
		minFreeMB: 100,
	}
}

func (p *FilesystemProbe) Name() string { return "filesystem" }

func (p *FilesystemProbe) Execute(_ context.Context) Result {
	if err := os.MkdirAll(p.path, 0750); err != nil {
		return Result{
			Check:    p.Name(),
			Status:   StatusFailed,
			Severity: SeverityCritical,
			Summary:  "Cannot create target directory",
			Details:  map[string]any{"path": p.path, "error": err.Error()},
		}
	}

	if p.writeTest {
		testFile := filepath.Join(p.path, fmt.Sprintf(".health_%d.tmp", time.Now().UnixNano()))
		if err := os.WriteFile(testFile, []byte("probe"), 0600); err != nil {
			return Result{
				Check:    p.Name(),
				Status:   StatusFailed,
				Severity: SeverityCritical,
				Summary:  "Filesystem is read-only or permissions insufficient",
				Details:  map[string]any{"path": p.path, "error": err.Error()},
			}
		}
		defer os.Remove(testFile)
	}

	// Check available space
	var stat syscall.Statfs_t
	if err := syscall.Statfs(p.path, &stat); err == nil {
		freeMB := uint64(stat.Bavail) * uint64(stat.Bsize) / 1024 / 1024
		if freeMB < p.minFreeMB {
			return Result{
				Check:    p.Name(),
				Status:   StatusDegraded,
				Severity: SeverityWarning,
				Summary:  fmt.Sprintf("Low disk space: %d MB free", freeMB),
				Details:  map[string]any{"free_mb": freeMB, "min_recommended_mb": p.minFreeMB},
			}
		}
	}

	return Result{
		Check:    p.Name(),
		Status:   StatusOK,
		Severity: SeverityInfo,
		Summary:  "Filesystem accessible and writable",
		Details:  map[string]any{"path": p.path},
	}
}

// AggregateResults summarizes diagnostic outcomes for quick assessment.
func AggregateResults(results []Result) map[Status]int {
	summary := make(map[Status]int)
	var criticalCount int

	for _, result := range results {
		summary[result.Status]++
		if result.Severity == SeverityCritical {
			criticalCount++
		}
	}

	summary["critical_total"] = criticalCount
	return summary
}
