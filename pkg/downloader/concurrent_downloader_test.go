package downloader

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/FJ-cyberzilla/webtoon-dl/pkg/scraper"
)

func TestNewDownloader_Defaults(t *testing.T) {
	dl := NewDownloader(0)
	if dl.WorkerCount != 4 {
		t.Errorf("expected default worker count of 4, got %d", dl.WorkerCount)
	}
	if dl.Client == nil {
		t.Fatal("expected non-nil http.Client")
	}
}

func TestDownloadChapter_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify required headers
		if r.Header.Get("Referer") != "https://www.webtoons.com/" {
			t.Errorf("expected Referer header, got %q", r.Header.Get("Referer"))
		}
		if r.Header.Get("User-Agent") == "" {
			t.Error("expected non-empty User-Agent header")
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("fake-image-bytes"))
	}))
	defer server.Close()

	dl := NewDownloader(2)
	dl.Client = server.Client()

	tmpDir := t.TempDir()
	images := []scraper.ChapterImage{
		{URL: server.URL + "/1.jpg", Format: "jpg"},
		{URL: server.URL + "/2.png", Format: "png"},
		{URL: server.URL + "/3.webp", Format: ""}, // Defaults to .jpg
	}

	paths, err := dl.DownloadChapter(context.Background(), images, tmpDir)
	if err != nil {
		t.Fatalf("expected DownloadChapter to succeed, got: %v", err)
	}

	if len(paths) != 3 {
		t.Fatalf("expected 3 paths returned, got %d", len(paths))
	}

	expectedExts := []string{".jpg", ".png", ".jpg"}
	for i, path := range paths {
		if filepath.Ext(path) != expectedExts[i] {
			t.Errorf("file %d: expected extension %s, got %s", i, expectedExts[i], filepath.Ext(path))
		}

		content, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("failed to read downloaded file %s: %v", path, err)
		}
		if string(content) != "fake-image-bytes" {
			t.Errorf("unexpected file content: %s", string(content))
		}
	}
}

func TestDownloadChapter_EmptyImages(t *testing.T) {
	dl := NewDownloader(2)
	paths, err := dl.DownloadChapter(context.Background(), nil, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error for empty image slice: %v", err)
	}
	if paths != nil {
		t.Errorf("expected nil paths slice, got %v", paths)
	}
}

func TestDownloadChapter_HTTPErrorAborts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fail" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	dl := NewDownloader(2)
	dl.Client = server.Client()

	tmpDir := t.TempDir()
	images := []scraper.ChapterImage{
		{URL: server.URL + "/fail", Format: "jpg"},
		{URL: server.URL + "/ok1", Format: "jpg"},
		{URL: server.URL + "/ok2", Format: "jpg"},
	}

	_, err := dl.DownloadChapter(context.Background(), images, tmpDir)
	if err == nil {
		t.Fatal("expected error on 404 response, got nil")
	}
}
