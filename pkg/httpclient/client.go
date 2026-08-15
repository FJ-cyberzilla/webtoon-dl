package httpclient

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

const (
	// DefaultUserAgent is the standard User-Agent used across the application to bypass basic anti-scraping measures.
	DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

// UserAgentTransport is an http.RoundTripper that adds a User-Agent header to every request.
type UserAgentTransport struct {
	UserAgent string
	Base      http.RoundTripper
}

// RoundTrip implements the http.RoundTripper interface.
func (t *UserAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone request to avoid mutating original state in concurrent calls
	reqCopy := req.Clone(req.Context())

	ua := t.UserAgent
	if ua == "" {
		ua = DefaultUserAgent
	}

	reqCopy.Header.Set("User-Agent", ua)

	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(reqCopy)
	if err != nil {
		return nil, fmt.Errorf("transport roundtrip failed: %w", err)
	}
	return resp, nil
}

// NewClient returns an http.Client pre-configured with the User-Agent transport,
// specific timeouts, and a redirect policy.
func NewClient(userAgent string, timeout time.Duration, maxRedirects int) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &UserAgentTransport{
			UserAgent: userAgent,
			Base: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		},
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			return nil
		},
	}
}

// ValidateURLScheme ensures the URL has a valid HTTP/HTTPS scheme.
func ValidateURLScheme(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
	return nil
}

// DecorateClient adds the UserAgentTransport to an existing http.Client and configures timeouts/redirects.
func DecorateClient(client *http.Client, userAgent string, timeout time.Duration, maxRedirects int) *http.Client {
	if client == nil {
		return NewClient(userAgent, timeout, maxRedirects)
	}

	client.Timeout = timeout
	client.Transport = &UserAgentTransport{
		UserAgent: userAgent,
		Base:      client.Transport,
	}
	client.CheckRedirect = func(_ *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return fmt.Errorf("stopped after %d redirects", maxRedirects)
		}
		return nil
	}
	return client
}
