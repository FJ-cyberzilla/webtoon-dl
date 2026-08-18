package downloader

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
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
	sf        singleflight.Group
}

func NewEngine(workers int, createCBZ bool, keepRaw bool) *Engine {
	return &Engine{
		Workers:   workers,
		Client:    &http.Client{},
		CreateCBZ: createCBZ,
		KeepRaw:   keepRaw,
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

	tasks, err := e.extractImageTasks(ctx, chapterURL, chapterTitle, chapterDir)
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

func (e *Engine) extractImageTasks(ctx context.Context, chapterURL, chapterTitle, chapterDir string) ([]DownloadTask, error) {
	doc, err := e.fetchChapterDoc(ctx, chapterURL)
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

func (e *Engine) fetchChapterDoc(ctx context.Context, chapterURL string) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", chapterURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed creating request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := e.Client.Do(req)
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

func (e *Engine) finalizeChapter(chapterDir, cbzPath string) error {
	if e.CreateCBZ {
		if err := PackCBZ(chapterDir, cbzPath); err != nil {
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
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
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

// PackCBZ bundles all images in srcDir into a .cbz archive at dstCBZ.
func PackCBZ(srcDir, dstCBZ string) error {
	// Create temp file in same directory
	tempFile, err := os.CreateTemp(filepath.Dir(dstCBZ), ".tmp-cbz-*")
	if err != nil {
		return fmt.Errorf("failed creating temp CBZ file: %w", err)
	}
	tempPath := tempFile.Name()

	// Ensure cleanup if rename fails
	defer func() {
		if err := tempFile.Close(); err != nil {
			log.Printf("failed to close temp CBZ file: %v", err)
		}
		if _, err := os.Stat(tempPath); err == nil {
			if err := os.Remove(tempPath); err != nil {
				log.Printf("failed to remove temp CBZ file: %v", err)
			}
		}
	}()

	zipWriter := zip.NewWriter(tempFile)

	files, err := getImageFiles(srcDir)
	if err != nil {
		return err
	}

	for _, fileName := range files {
		if err := addFileToZip(zipWriter, srcDir, fileName); err != nil {
			if err := zipWriter.Close(); err != nil {
				log.Printf("failed closing zip writer: %v", err)
			}
			return fmt.Errorf("failed adding file to zip: %w", err)
		}
	}

	if err := zipWriter.Close(); err != nil {
		return fmt.Errorf("failed closing zip writer: %w", err)
	}

	// Sync to disk
	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("failed syncing CBZ file: %w", err)
	}
	tempFile.Close()

	// Rename atomically
	if err := os.Rename(tempPath, dstCBZ); err != nil {
		return fmt.Errorf("failed renaming temp CBZ file: %w", err)
	}
	return nil
}

func getImageFiles(srcDir string) ([]string, error) {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, fmt.Errorf("failed reading source directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".jpg") || strings.HasSuffix(entry.Name(), ".png")) {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	return files, nil
}

func addFileToZip(zw *zip.Writer, srcDir, fileName string) error {
	filePath := filepath.Join(srcDir, fileName)
	/* #nosec G304 */
	fileToZip, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed opening image file: %w", err)
	}
	defer fileToZip.Close()

	writer, err := zw.Create(fileName)
	if err != nil {
		return fmt.Errorf("failed creating zip entry: %w", err)
	}

	_, err = io.Copy(writer, fileToZip)
	if err != nil {
		return fmt.Errorf("failed copying image to zip: %w", err)
	}
	return nil
}
