package downloader

import (
	"archive/zip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPackCBZ_Success(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "chapter_1")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create source directory: %v", err)
	}

	// Create test image files
	_ = os.WriteFile(filepath.Join(srcDir, "001.jpg"), []byte("img-1"), 0644)
	_ = os.WriteFile(filepath.Join(srcDir, "002.png"), []byte("img-2"), 0644)
	_ = os.WriteFile(filepath.Join(srcDir, "ignore.txt"), []byte("txt"), 0644)

	cbzPath := filepath.Join(tmpDir, "chapter_1.cbz")
	if err := PackCBZ(srcDir, cbzPath); err != nil {
		t.Fatalf("PackCBZ failed: %v", err)
	}

	// Verify zip contents
	r, err := zip.OpenReader(cbzPath)
	if err != nil {
		t.Fatalf("failed to open CBZ archive: %v", err)
	}
	defer r.Close()

	if len(r.File) != 2 {
		t.Errorf("expected 2 image files in CBZ, found %d", len(r.File))
	}

	expectedFiles := map[string]bool{"001.jpg": true, "002.png": true}
	for _, f := range r.File {
		if !expectedFiles[f.Name] {
			t.Errorf("unexpected file in CBZ: %s", f.Name)
		}
	}
}

func TestEngine_DownloadChapterPages_EndToEnd(t *testing.T) {
	// Mock HTTP Server serving HTML & Images
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/chapter-1":
			w.WriteHeader(http.StatusOK)
			htmlContent := fmt.Sprintf(`
				<html>
					<body>
						<div id="_imageList">
							<img data-url="%s/img1.jpg" />
							<img src="%s/img2.jpg" />
						</div>
					</body>
				</html>
			`, "http://"+r.Host, "http://"+r.Host)
			_, _ = w.Write([]byte(htmlContent))
		case "/img1.jpg", "/img2.jpg":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("mock-image-bytes"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	engine := NewEngine(2, true, false)
	engine.Client = server.Client()

	tmpDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	chapterURL := server.URL + "/chapter-1"
	err := engine.DownloadChapterPages(ctx, chapterURL, "Chapter 1: The Beginning", tmpDir)
	if err != nil {
		t.Fatalf("expected DownloadChapterPages to succeed, got: %v", err)
	}

	expectedCBZ := filepath.Join(tmpDir, "Chapter 1_ The Beginning.cbz")
	if _, err := os.Stat(expectedCBZ); os.IsNotExist(err) {
		t.Errorf("expected CBZ output file at %s", expectedCBZ)
	}

	// Verify raw folder was cleaned up because KeepRaw = false
	rawDir := filepath.Join(tmpDir, "Chapter 1_ The Beginning")
	if _, err := os.Stat(rawDir); !os.IsNotExist(err) {
		t.Errorf("expected raw folder %s to be deleted when KeepRaw=false", rawDir)
	}
}

func TestEngine_DownloadChapterPages_NoImagesFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body><p>No Images Here</p></body></html>"))
	}))
	defer server.Close()

	engine := NewEngine(1, false, true)
	engine.Client = server.Client()

	tmpDir := t.TempDir()
	err := engine.DownloadChapterPages(context.Background(), server.URL, "Empty Chapter", tmpDir)
	if err == nil {
		t.Fatal("expected error when no images found, got nil")
	}
}
