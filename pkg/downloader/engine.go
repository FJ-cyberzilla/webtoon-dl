package downloader

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/FJ-cyberzilla/webtoon-dl/pkg/httpclient"
	"golang.org/x/sync/singleflight"
)

type DownloadTask struct {
	ChapterID    string
	ChapterTitle string
	PageNum      int
	ImageURL     string
	Referer      string
	OutputDir    string
}

type Engine struct {
	mu        sync.RWMutex
	Workers   int
	Client    *http.Client
	CreateCBZ bool // Config flag: compile to .cbz
	KeepRaw   bool // Config flag: keep uncompressed images after .cbz creation
	Fetcher   Fetcher
	Archiver  Archiver
	sf        singleflight.Group
}

func NewEngine(workers int, createCBZ bool, keepRaw bool, fetcher Fetcher, archiver Archiver) *Engine {
	client := httpclient.NewClient("", 45*time.Second, 10)
	if fetcher == nil {
		fetcher = &WebFetcher{Client: client}
	}
	if archiver == nil {
		archiver = &CBZArchiver{}
	}
	return &Engine{
		Workers:   workers,
		Client:    client,
		CreateCBZ: createCBZ,
		KeepRaw:   keepRaw,
		Fetcher:   fetcher,
		Archiver:  archiver,
	}
}

// DownloadChapterPages extracts image URLs for a chapter, downloads them, and packs into CBZ.
func (e *Engine) DownloadChapterPages(ctx context.Context, chapterURL, chapterTitle, baseOutDir string) error {
	if err := ValidatePath(baseOutDir); err != nil {
		return fmt.Errorf("invalid output directory: %w", err)
	}
	if err := ValidateTitle(chapterTitle); err != nil {
		return fmt.Errorf("invalid chapter title: %w", err)
	}

	baseOutDir = SanitizePath(baseOutDir)
	cleanTitle := sanitizeFilename(chapterTitle)
	cbzPath := filepath.Join(baseOutDir, fmt.Sprintf("%s.cbz", cleanTitle))

	if e.isSkippable(cbzPath, cleanTitle) {
		return nil
	}

	chapterDir := filepath.Join(baseOutDir, cleanTitle)

	tasks, err := e.Fetcher.ExtractImageTasks(ctx, chapterURL, chapterTitle, chapterDir)
	if err != nil {
		return err
	}

	if err := e.processTasks(ctx, tasks); err != nil {
		return fmt.Errorf("failed downloading chapter images: %w", err)
	}

	return e.finalizeChapter(chapterDir, cbzPath)
}

func (e *Engine) isSkippable(cbzPath, title string) bool {
	e.mu.RLock()
	createCBZ := e.CreateCBZ
	e.mu.RUnlock()

	if createCBZ {
		if _, err := os.Stat(cbzPath); err == nil {
			log.Printf(" Skipping %s (CBZ archive already exists)\n", title)
			return true
		}
	}
	return false
}

// Remaining methods...

func (e *Engine) finalizeChapter(chapterDir, cbzPath string) error {
	if e.CreateCBZ {
		if err := e.Archiver.Archive(chapterDir, cbzPath); err != nil {
			return fmt.Errorf("failed creating CBZ archive: %w", err)
		}

		if !e.KeepRaw {
			if err := os.RemoveAll(chapterDir); err != nil {
				log.Printf("failed to remove raw chapter directory: %v", err)
			}
		}

	}
	return nil
}

func (e *Engine) processTasks(ctx context.Context, tasks []DownloadTask) error {
	taskChan := make(chan DownloadTask, len(tasks))
	var wg sync.WaitGroup

	var errMu sync.Mutex
	var firstErr error

	for i := 0; i < e.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Recovered from panic in worker: %v", r)
				}
			}()
			for task := range taskChan {
				select {
				case <-ctx.Done():
					return
				default:
				}

				if err := e.downloadImage(ctx, task); err != nil {
					errMu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					errMu.Unlock()
				}
			}
		}()
	}

	for _, task := range tasks {
		taskChan <- task
	}
	close(taskChan)

	wg.Wait()
	return firstErr
}

func (e *Engine) downloadImage(ctx context.Context, task DownloadTask) error {
	filePath := filepath.Join(task.OutputDir, fmt.Sprintf("%03d.jpg", task.PageNum))
	if _, err := os.Stat(filePath); err == nil {
		return nil
	}

	_, err, _ := e.sf.Do(task.ImageURL, func() (interface{}, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", task.ImageURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed creating image request: %w", err)
		}
		req.Header.Set("Referer", task.Referer)

		resp, err := e.Client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed fetching image: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}

		return nil, AtomicWriteFile(filePath, resp.Body)
	})

	if err != nil {
		return fmt.Errorf("singleflight download failed: %w", err)
	}
	return nil
}
