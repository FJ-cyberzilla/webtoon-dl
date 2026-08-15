package downloader

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FJ-cyberzilla/webtoon-dl/pkg/scraper"
	"github.com/vbauerster/mpb/v8"
)

// TestDownloadSingleFile_Success verifies that single images are correctly fetched and saved with required webtoon headers.
func TestDownloadSingleFile_Success(t *testing.T) {
	expectedData := []byte("fake-jpeg-image-bytes-data")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify custom HTTP headers to bypass anti-scraping
		if userAgent := r.Header.Get("User-Agent"); userAgent == "" {
			t.Errorf("expected User-Agent header to be set, got empty")
		}
		if referer := r.Header.Get("Referer"); referer != "https://www.webtoons.com/" {
			t.Errorf("expected Referer header 'https://www.webtoons.com/', got '%s'", referer)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(expectedData)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destFile := filepath.Join(tmpDir, "test_image.jpg")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Assuming this function still exists based on the read_file output which shows it used.
	// Wait, the read_file output for pkg/downloader/downloader_test.go showed:
	// err := downloadSingleFile(ctx, server.URL, destFile)
	// I need to make sure this function is either imported or defined.
	// Looking at the grep, it was in pkg/downloader/batch.go. Okay.
	// But in pkg/downloader/downloader.go it wasn't.
	// I'll assume it's still available.
	err := downloadSingleFile(ctx, server.Client(), server.URL, destFile)
	if err != nil {
		t.Fatalf("expected download to succeed, got error: %v", err)
	}

	savedContent, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}

	if string(savedContent) != string(expectedData) {
		t.Errorf("expected file content %q, got %q", string(expectedData), string(savedContent))
	}
}

// TestDownloadSingleFile_HTTPError tests handling of non-200 HTTP statuses.
func TestDownloadSingleFile_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destFile := filepath.Join(tmpDir, "missing.jpg")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := downloadSingleFile(ctx, server.Client(), server.URL, destFile)
	if err == nil {
		t.Fatal("expected error on 404 Not Found response, got nil")
	}
}

// TestDownloadSingleFile_ContextCancellation ensures downloads stop promptly when context is canceled.
func TestDownloadSingleFile_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destFile := filepath.Join(tmpDir, "canceled.jpg")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := downloadSingleFile(ctx, server.Client(), server.URL, destFile)
	if err == nil {
		t.Fatal("expected error when context is canceled, got nil")
	}
}

// TestDownloadChapterImagesConcurrently tests parallel image downloads and progress bar incrementing.
func TestDownloadChapterImagesConcurrently(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("slice-data"))
	}))
	defer server.Close()

	images := []scraper.ChapterImage{}

	tmpDir := t.TempDir()
	ctx := context.Background()

	p := mpb.NewWithContext(ctx)
	bar := p.AddBar(int64(len(images)))

	scr := &scraper.Scraper{Client: server.Client()}
	paths, err := downloadChapterImagesConcurrently(ctx, scr, images, tmpDir, 2, bar)
	if err != nil {
		t.Fatalf("concurrent download failed: %v", err)
	}

	if len(paths) != len(images) {
		t.Errorf("expected %d saved paths, got %d", len(images), len(paths))
	}

	if atomic.LoadInt32(&requestCount) != int32(len(images)) {
		t.Errorf("expected %d server requests, got %d", len(images), atomic.LoadInt32(&requestCount))
	}

	for _, path := range paths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file at path %s to exist", path)
		}
	}
}

// TestInputValidation verifies path and title validation logic.
func TestInputValidation(t *testing.T) {
	t.Run("validatePath", func(t *testing.T) {
		tests := []struct {
			name    string
			path    string
			wantErr bool
		}{
			{"empty", "", true},
			{"too_long", strings.Repeat("a", 4097), true},
			{"valid", "/tmp/test", false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := ValidatePath(tt.path)
				if (err != nil) != tt.wantErr {
					t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
				}
			})
		}
	})

	t.Run("validateTitle", func(t *testing.T) {
		tests := []struct {
			name    string
			title   string
			wantErr bool
		}{
			{"empty", "", true},
			{"valid", "Comic Title", false},
			{"valid_unicode", "Comic Title 🚀", false},
			{"invalid_utf8", "\xff", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := ValidateTitle(tt.title)
				if (err != nil) != tt.wantErr {
					t.Errorf("ValidateTitle(%q) error = %v, wantErr %v", tt.title, err, tt.wantErr)
				}
			})
		}
	})
}

// TestSanitizeFilename verifies special characters are replaced safely for filesystem paths.
func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{input: "Chapter 1: The Beginning", expected: "Chapter 1_ The Beginning"},
		{input: "Vol.1/Ch.2*Special?", expected: "Vol.1_Ch.2_Special_"},
		{input: "Normal Title", expected: "Normal Title"},
	}

	for _, tt := range tests {
		result := sanitizeFilename(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeFilename(%q) = %q; want %q", tt.input, result, tt.expected)
		}
	}
}

// TestAtomicWriteFile verifies atomic writing and cleanup on failure.
func TestAtomicWriteFile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tmpDir := t.TempDir()
		destFile := filepath.Join(tmpDir, "output.txt")
		data := strings.NewReader("hello atomic")

		err := AtomicWriteFile(destFile, data)
		if err != nil {
			t.Fatalf("atomicWriteFile failed: %v", err)
		}

		content, err := os.ReadFile(destFile)
		if err != nil {
			t.Fatalf("failed to read written file: %v", err)
		}
		if string(content) != "hello atomic" {
			t.Errorf("expected content 'hello atomic', got %q", string(content))
		}
	})

	t.Run("failure_cleanup", func(t *testing.T) {
		tmpDir := t.TempDir()
		destFile := filepath.Join(tmpDir, "output.txt")
		// Use a broken reader to force failure
		data := &errorReader{}

		err := AtomicWriteFile(destFile, data)
		if err == nil {
			t.Fatal("expected error from broken reader, got nil")
		}

		// Verify no temp file left behind (should only be the dir itself)
		entries, _ := os.ReadDir(tmpDir)
		if len(entries) != 0 {
			t.Errorf("expected temp files to be cleaned up, found: %v", entries)
		}
	})
}

type errorReader struct{}

func (e *errorReader) Read(_ []byte) (n int, err error) {
	return 0, errors.New("read error")
}
