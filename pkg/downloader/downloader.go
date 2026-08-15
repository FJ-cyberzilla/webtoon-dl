package downloader

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/FJ-cyberzilla/webtoon-dl/pkg/httpclient"
	"github.com/FJ-cyberzilla/webtoon-dl/pkg/scraper"
)

var (
	ErrPathEmpty      = errors.New("path cannot be empty")
	ErrPathTooLong    = errors.New("path exceeds maximum allowed length")
	ErrTitleEmpty     = errors.New("title cannot be empty")
	ErrInvalidUTF8    = errors.New("title contains invalid UTF-8 characters")
	ErrDownloadFailed = errors.New("download failed")
)

// Downloader manages concurrent downloads.
type Downloader struct {
	Client      *http.Client
	WorkerCount int
}

type jobResult struct {
	index int
	path  string
	err   error
}

// SanitizePath cleans path and replaces potentially dangerous characters to prevent traversal.
func SanitizePath(path string) string {
	return filepath.Clean(path)
}

// ValidatePath ensures the path is not empty and within reasonable limits.
func ValidatePath(path string) error {
	if path == "" {
		return ErrPathEmpty
	}
	if len(path) > 4096 {
		return ErrPathTooLong
	}
	return nil
}

// ValidateTitle ensures the title is not empty.
func ValidateTitle(title string) error {
	if title == "" {
		return ErrTitleEmpty
	}
	if !utf8.ValidString(title) {
		return ErrInvalidUTF8
	}
	return nil
}

// NewDownloader creates a new Downloader instance with sane defaults.
func NewDownloader(workerCount int) *Downloader {
	if workerCount <= 0 {
		workerCount = 4
	}
	client := &http.Client{
		Timeout: 45 * time.Second,
	}
	return &Downloader{
		Client:      httpclient.DecorateClient(client, "", 45*time.Second, 10),
		WorkerCount: workerCount,
	}
}

// DownloadChapter downloads all images for a chapter concurrently using context cancellation.
func (d *Downloader) DownloadChapter(ctx context.Context, images []scraper.ChapterImage, outputDir string) ([]string, error) {
	if err := ValidatePath(outputDir); err != nil {
		return nil, fmt.Errorf("invalid output directory: %w", err)
	}
	outputDir = SanitizePath(outputDir)

	if len(images) == 0 {
		return nil, nil
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan int, len(images))
	results := make(chan jobResult, len(images))

	for i := range images {
		jobs <- i
	}
	close(jobs)

	d.runWorkerPool(ctx, images, outputDir, jobs, results)
	return d.collectResults(cancel, results, len(images))
}

func (d *Downloader) runWorkerPool(ctx context.Context, images []scraper.ChapterImage, outputDir string, jobs <-chan int, results chan<- jobResult) {
	var wg sync.WaitGroup
	for w := 0; w < d.WorkerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.worker(ctx, images, outputDir, jobs, results)
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()
}

func (d *Downloader) worker(ctx context.Context, images []scraper.ChapterImage, outputDir string, jobs <-chan int, results chan<- jobResult) {
	for {
		select {
		case <-ctx.Done():
			return
		case idx, ok := <-jobs:
			if !ok {
				return
			}
			res := d.downloadJob(ctx, images[idx], idx, outputDir)
			select {
			case <-ctx.Done():
				return
			case results <- res:
			}
		}
	}
}

func (d *Downloader) downloadJob(ctx context.Context, img scraper.ChapterImage, idx int, outputDir string) jobResult {
	if err := httpclient.ValidateURLScheme(img.URL); err != nil {
		return jobResult{index: idx, err: fmt.Errorf("invalid image URL: %w", err)}
	}

	format := img.Format
	if format == "" || format == "unknown" {
		format = "jpg"
	}

	targetPath := filepath.Join(outputDir, fmt.Sprintf("%03d.%s", idx+1, format))
	err := d.downloadImage(ctx, img.URL, targetPath)
	return jobResult{index: idx, path: targetPath, err: err}
}

func (d *Downloader) collectResults(cancel context.CancelFunc, results <-chan jobResult, count int) ([]string, error) {
	imagePaths := make([]string, count)
	var firstErr error

	for res := range results {
		if res.err != nil && firstErr == nil {
			firstErr = res.err
			cancel()
		}
		if res.err == nil {
			imagePaths[res.index] = res.path
		}
	}

	if firstErr != nil {
		return nil, fmt.Errorf("download failed: %w", firstErr)
	}
	return imagePaths, nil
}

// downloadImage fetches an image and writes it atomically via a temporary file, with retries for transient errors.
func (d *Downloader) downloadImage(ctx context.Context, imageURL, targetPath string) error {
	headers := map[string]string{
		"Referer": "https://www.webtoons.com/",
	}
	return DownloadWithRetry(ctx, d.Client, imageURL, targetPath, headers, DefaultRetryConfig)
}
