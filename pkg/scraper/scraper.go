package scraper

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/FJ-cyberzilla/webtoon-dl/pkg/httpclient"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/time/rate"
)

// Chapter represents a single webtoon chapter.
type Chapter struct {
	Title string
	URL   string
}

// ChapterImage represents a parsed image with metadata.
type ChapterImage struct {
	URL    string
	Width  int
	Height int
	Format string // "jpg", "png", "gif", "webp", etc.
}

// Scraper handles the fetching of chapter data.
type Scraper struct {
	Client  *http.Client
	limiter *rate.Limiter
}

// NewScraper creates a new Scraper instance with the provided rate limit parameters.
func NewScraper(rps float64, burst int) *Scraper {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	return &Scraper{
		Client:  httpclient.DecorateClient(client, "", 30*time.Second, 10),
		limiter: rate.NewLimiter(rate.Limit(rps), burst),
	}
}

// doRequest handles rate limiting, custom headers, and standard context execution.
func (s *Scraper) doRequest(ctx context.Context, targetURL string, referer string) (*http.Response, error) {
	if s.limiter != nil {
		if err := s.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limit wait cancelled: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Referer header is still needed for some sites
	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	return resp, nil
}

// FetchChapters fetches all available chapters for the given URL.
func (s *Scraper) FetchChapters(ctx context.Context, targetURL string) ([]Chapter, error) {
	if strings.Contains(targetURL, "webtoons.com") {
		return s.scrapeWebtoons(ctx, targetURL)
	} else if strings.Contains(targetURL, "comicland.org") {
		return s.scrapeComicland(ctx, targetURL)
	}
	return nil, fmt.Errorf("unsupported site: %s", targetURL)
}

func (s *Scraper) scrapeWebtoons(ctx context.Context, targetURL string) ([]Chapter, error) {
	resp, err := s.doRequest(ctx, targetURL, "")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var chapters []Chapter
	doc.Find("#_episodeList li a").Each(func(_ int, sel *goquery.Selection) {
		chapterURL, exists := sel.Attr("href")
		title := strings.TrimSpace(sel.Find(".subj span").Text())
		if exists && title != "" {
			chapters = append(chapters, Chapter{
				Title: title,
				URL:   chapterURL,
			})
		}
	})
	return chapters, nil
}

func (s *Scraper) scrapeComicland(ctx context.Context, targetURL string) ([]Chapter, error) {
	resp, err := s.doRequest(ctx, targetURL, "")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var chapters []Chapter
	doc.Find("div.chapter-list a").Each(func(_ int, sel *goquery.Selection) {
		chapterURL, exists := sel.Attr("href")
		title := strings.TrimSpace(sel.Text())
		if exists && title != "" {
			chapters = append(chapters, Chapter{
				Title: title,
				URL:   chapterURL,
			})
		}
	})
	return chapters, nil
}

// GetChapterImages fetches all image URLs for a specific chapter with retry logic.
func (s *Scraper) GetChapterImages(ctx context.Context, chapterURL string) ([]ChapterImage, error) {
	log.Printf("[INFO] Scraping chapter: %s", chapterURL)

	doc, err := s.fetchChapterDocument(ctx, chapterURL)
	if err != nil {
		return nil, err
	}

	baseURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, fmt.Errorf("invalid chapter URL: %w", err)
	}

	rawImages := extractImages(doc, baseURL)
	if len(rawImages) == 0 {
		return nil, fmt.Errorf("no valid chapter images found for URL: %s", chapterURL)
	}

	result := deduplicateImages(rawImages)
	log.Printf("[INFO] Successfully scraped %d unique images", len(result))
	return result, nil
}

func (s *Scraper) fetchChapterDocument(ctx context.Context, chapterURL string) (*goquery.Document, error) {
	resp, err := s.executeRetryLoop(ctx, chapterURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("HTML parsing failed: %w", err)
	}
	return doc, nil
}

func (s *Scraper) executeRetryLoop(ctx context.Context, chapterURL string) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt < 3; attempt++ {
		resp, err = s.doRequest(ctx, chapterURL, chapterURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			return resp, nil
		}

		if resp != nil {
			if err := resp.Body.Close(); err != nil {
				log.Printf("[WARN] Failed to close response body: %v", err)
			}
		}

		log.Printf("[WARN] Attempt %d failed (err: %v)", attempt+1, err)
		if attempt < 2 {
			if waitErr := waitBackoff(ctx, attempt); waitErr != nil {
				return nil, waitErr
			}
		}
	}

	if err != nil {
		return nil, fmt.Errorf("request failed after retries: %w", err)
	}
	return nil, fmt.Errorf("HTTP status %d for URL: %s", resp.StatusCode, chapterURL)
}

func waitBackoff(ctx context.Context, attempt int) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("scraping cancelled: %w", ctx.Err())
	case <-time.After(time.Duration(1<<attempt) * time.Second):
		return nil
	}
}

func extractImages(doc *goquery.Document, baseURL *url.URL) []string {
	// Selectors targeting main viewer containers first to avoid UI image leakage
	selectors := []string{
		"#_imageList img",         // Webtoons primary
		"#viewer_img img",         // Generic viewer
		"div.chapter-content img", // Generic comic reader
		"img[data-url]",           // Lazy-loaded attribute
	}

	var rawImages []string
	for _, selector := range selectors {
		doc.Find(selector).Each(func(_ int, sel *goquery.Selection) {
			src := getImgSrc(sel)
			if src != "" {
				resolvedURL := resolveURL(baseURL, src)
				if resolvedURL != "" {
					rawImages = append(rawImages, resolvedURL)
				}
			}
		})

		if len(rawImages) > 0 {
			log.Printf("[INFO] Found %d images with selector: %s", len(rawImages), selector)
			break
		}
	}
	return rawImages
}

func getImgSrc(sel *goquery.Selection) string {
	if attr, exists := sel.Attr("data-url"); exists && attr != "" {
		return attr
	}
	if attr, exists := sel.Attr("data-src"); exists && attr != "" {
		return attr
	}
	if attr, exists := sel.Attr("src"); exists && attr != "" {
		return attr
	}
	return ""
}

func deduplicateImages(rawImages []string) []ChapterImage {
	seen := make(map[string]struct{}, len(rawImages))
	var result []ChapterImage

	for _, imgURL := range rawImages {
		if _, exists := seen[imgURL]; !exists {
			seen[imgURL] = struct{}{}
			result = append(result, ChapterImage{
				URL:    imgURL,
				Format: detectImageFormat(imgURL),
			})
		}
	}
	return result
}

// Helper: Resolve relative or protocol-relative URLs
func resolveURL(base *url.URL, src string) string {
	src = strings.TrimSpace(src)
	if strings.HasPrefix(src, "//") {
		return "https:" + src
	}

	rel, err := url.Parse(src)
	if err != nil {
		return ""
	}

	return base.ResolveReference(rel).String()
}

// Helper: Detect image format using path extension parsing
func detectImageFormat(imgURL string) string {
	u, err := url.Parse(imgURL)
	if err != nil {
		return "unknown"
	}

	ext := strings.ToLower(path.Ext(u.Path))
	switch ext {
	case ".jpg", ".jpeg":
		return "jpg"
	case ".png":
		return "png"
	case ".gif":
		return "gif"
	case ".webp":
		return "webp"
	default:
		return "unknown"
	}
}
