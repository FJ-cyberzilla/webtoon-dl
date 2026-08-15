package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestDetectImageFormat(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://example.com/image.jpg", "jpg"},
		{"https://example.com/image.JPEG", "jpg"},
		{"https://example.com/image.png?v=123", "png"},
		{"https://example.com/image.webp", "webp"},
		{"https://example.com/image.gif", "gif"},
		{"https://example.com/image.unknown", "unknown"},
		{"invalid-url", "unknown"},
	}

	for _, tt := range tests {
		got := detectImageFormat(tt.url)
		if got != tt.expected {
			t.Errorf("detectImageFormat(%q) = %q; want %q", tt.url, got, tt.expected)
		}
	}
}

func TestResolveURL(t *testing.T) {
	base, _ := url.Parse("https://example.com/chapter/1")

	tests := []struct {
		src      string
		expected string
	}{
		{"//cdn.example.com/img.jpg", "https://cdn.example.com/img.jpg"},
		{"/images/001.jpg", "https://example.com/images/001.jpg"},
		{"relative/002.png", "https://example.com/chapter/relative/002.png"},
		{"https://absolute.com/003.webp", "https://absolute.com/003.webp"},
	}

	for _, tt := range tests {
		got := resolveURL(base, tt.src)
		if got != tt.expected {
			t.Errorf("resolveURL(base, %q) = %q; want %q", tt.src, got, tt.expected)
		}
	}
}

func TestFetchChapters_Webtoons(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`
			<ul id="_episodeList">
				<li><a href="/ep1"><span class="subj"><span>Episode 1</span></span></a></li>
				<li><a href="/ep2"><span class="subj"><span>Episode 2</span></span></a></li>
			</ul>
		`))
	}))
	defer server.Close()

	scraper := NewScraper(10, 10)
	scraper.Client = server.Client()

	// Append webtoons.com query param to route through scrapeWebtoons handler
	targetURL := server.URL + "?site=webtoons.com"
	chapters, err := scraper.FetchChapters(context.Background(), targetURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(chapters) != 2 {
		t.Fatalf("expected 2 chapters, got %d", len(chapters))
	}

	if chapters[0].Title != "Episode 1" || chapters[0].URL != "/ep1" {
		t.Errorf("unexpected chapter 0: %+v", chapters[0])
	}
}

func TestFetchChapters_UnsupportedSite(t *testing.T) {
	scraper := NewScraper(10, 10)
	_, err := scraper.FetchChapters(context.Background(), "https://unsupported-domain.com")
	if err == nil {
		t.Fatal("expected error for unsupported site, got nil")
	}
}

func TestGetChapterImages_SuccessAndDeduplication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`
			<div id="_imageList">
				<img data-url="/img1.jpg" />
				<img data-url="/img2.png" />
				<img data-url="/img1.jpg" /> <!-- Duplicate -->
			</div>
		`))
	}))
	defer server.Close()

	scraper := NewScraper(10, 10)
	scraper.Client = server.Client()

	images, err := scraper.GetChapterImages(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(images) != 2 {
		t.Fatalf("expected 2 unique images, got %d", len(images))
	}

	if images[0].Format != "jpg" || images[1].Format != "png" {
		t.Errorf("unexpected image formats: %s, %s", images[0].Format, images[1].Format)
	}
}
