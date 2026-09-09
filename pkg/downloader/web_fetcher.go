package downloader

import (
	"context"
	"fmt"
	"net/http"

	"github.com/PuerkitoBio/goquery"
)

type WebFetcher struct {
	Client *http.Client
}

func (f *WebFetcher) FetchChapterDoc(ctx context.Context, chapterURL string) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", chapterURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed creating request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed fetching chapter page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d fetching chapter page", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to create goquery document: %w", err)
	}
	return doc, nil
}

func (f *WebFetcher) ExtractImageTasks(ctx context.Context, chapterURL, chapterTitle, chapterDir string) ([]DownloadTask, error) {
	doc, err := f.FetchChapterDoc(ctx, chapterURL)
	if err != nil {
		return nil, err
	}

	var tasks []DownloadTask
	doc.Find("#_imageList img").Each(func(i int, sel *goquery.Selection) {
		imgURL, exists := sel.Attr("data-url")
		if !exists {
			imgURL, _ = sel.Attr("src")
		}

		if imgURL != "" {
			tasks = append(tasks, DownloadTask{
				ChapterTitle: chapterTitle,
				PageNum:      i + 1,
				ImageURL:     imgURL,
				Referer:      chapterURL,
				OutputDir:    chapterDir,
			})
		}
	})

	if len(tasks) == 0 {
		return nil, fmt.Errorf("no images found for chapter: %s", chapterTitle)
	}
	return tasks, nil
}
