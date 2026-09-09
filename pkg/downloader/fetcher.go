package downloader

import (
	"context"
	"github.com/PuerkitoBio/goquery"
)

// Fetcher defines the interface for scraping chapter content.
type Fetcher interface {
	FetchChapterDoc(ctx context.Context, chapterURL string) (*goquery.Document, error)
	ExtractImageTasks(ctx context.Context, chapterURL, chapterTitle, chapterDir string) ([]DownloadTask, error)
}
