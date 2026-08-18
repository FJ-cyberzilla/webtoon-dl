package downloader

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/FJ-cyberzilla/webtoon-dl/pkg/pdf"
	"github.com/FJ-cyberzilla/webtoon-dl/pkg/scraper"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
	"golang.org/x/sync/errgroup"
)

// DownloadBatchConcurrently processes multiple chapters in parallel using errgroup and mpb.
func DownloadBatchConcurrently(
	ctx context.Context,
	chapters []scraper.Chapter,
	scr *scraper.Scraper,
	outputDir string,
	maxConcurrentChapters int,
	workersPerChapter int,
	pdfOpts ...pdf.Option,
) error {
	p := mpb.NewWithContext(ctx, mpb.WithWidth(64))

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrentChapters)

	var (
		completedMu sync.Mutex
		completed   int
	)

	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Check for at least 500MB of free space before starting
	if err := CheckDiskSpace(outputDir, 500*1024*1024); err != nil {
		return fmt.Errorf("disk space check failed: %w", err)
	}

	for idx, ch := range chapters {
		chapterIdx := idx + 1
		chapter := ch

		g.Go(func() error {
			tmpDir, err := os.MkdirTemp("", fmt.Sprintf("webtoon-dl-temp-%d-*", chapterIdx))
			if err != nil {
				return fmt.Errorf("failed to create temp dir for %s: %w", chapter.Title, err)
			}
			defer os.RemoveAll(tmpDir)

			// 1. Fetch Chapter Image Metadata
			images, err := scr.GetChapterImages(gCtx, chapter.URL)
			if err != nil {
				return fmt.Errorf("failed to fetch images for %s: %w", chapter.Title, err)
			}

			// 2. Add MPB Progress Bar
			barName := fmt.Sprintf("[%d/%d] %s", chapterIdx, len(chapters), truncateTitle(chapter.Title, 20))
			bar := p.AddBar(int64(len(images)),
				mpb.PrependDecorators(
					decor.Name(barName, decor.WCSyncWidth),
					decor.CountersNoUnit("%d / %d", decor.WCSyncSpace),
				),
				mpb.AppendDecorators(
					decor.Percentage(decor.WCSyncWidth),
					decor.OnComplete(
						decor.EwmaETA(decor.ET_STYLE_GO, 60),
						"DONE",
					),
				),
			)

			// 3. Download Image Slices
			localImagePaths, err := downloadChapterImagesConcurrently(gCtx, scr, images, tmpDir, workersPerChapter, bar)
			if err != nil {
				return fmt.Errorf("download error in %s: %w", chapter.Title, err)
			}

			// 4. Generate Output PDF
			pdfPath := filepath.Join(outputDir, fmt.Sprintf("%s.pdf", sanitizeFilename(chapter.Title)))
			if err := pdf.CreatePDF(localImagePaths, pdfPath, pdfOpts...); err != nil {
				return fmt.Errorf("pdf creation error in %s: %w", chapter.Title, err)
			}

			completedMu.Lock()
			completed++
			completedMu.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		p.Wait()
		return fmt.Errorf("batch download failed: %w", err)
	}

	p.Wait()
	return nil
}

func downloadChapterImagesConcurrently(
	ctx context.Context,
	scr *scraper.Scraper,
	images []scraper.ChapterImage,
	outputDir string,
	workers int,
	bar *mpb.Bar,
) ([]string, error) {
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(workers)

	paths := make([]string, len(images))

	for i, img := range images {
		index := i
		imgURL := img.URL

		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
			}

			filePath := filepath.Join(outputDir, fmt.Sprintf("img_%03d.jpg", index))
			if err := downloadSingleFile(gCtx, scr.Client, imgURL, filePath); err != nil {
				return fmt.Errorf("failed to download image %s: %w", imgURL, err)
			}

			paths[index] = filePath
			bar.Increment()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("concurrent download failed: %w", err)
	}

	return paths, nil
}

func downloadSingleFile(ctx context.Context, client *http.Client, url, destination string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Webtoon servers require referrer header to avoid 403 Forbidden
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://www.webtoons.com/")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download %s: HTTP %d", url, resp.StatusCode)
	}

	if err := AtomicWriteFile(destination, resp.Body); err != nil {
		return fmt.Errorf("failed to save image content: %w", err)
	}

	return nil
}

func sanitizeFilename(name string) string {
	forbidden := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, f := range forbidden {
		name = strings.ReplaceAll(name, f, "_")
	}
	return strings.TrimSpace(name)
}

func truncateTitle(title string, maxLen int) string {
	if len(title) <= maxLen {
		return title
	}
	return title[:maxLen-3] + "..."
}
