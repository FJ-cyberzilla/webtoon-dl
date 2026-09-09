package downloader

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

var bufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 32*1024)
	},
}

// Archiver defines the interface for archiving chapter content.
type Archiver interface {
	Archive(srcDir, dstArchive string) error
}

// CBZArchiver implements Archiver for .cbz files.
type CBZArchiver struct{}

// Archive bundles all images in srcDir into a .cbz archive at dstCBZ.
func (a *CBZArchiver) Archive(srcDir, dstCBZ string) error {
	tempFile, tempPath, err := createTempCBZFile(dstCBZ)
	if err != nil {
		return err
	}
	defer cleanupTempFile(tempFile, tempPath)

	if err := writeToZip(tempFile, srcDir); err != nil {
		return err
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

func createTempCBZFile(dstCBZ string) (*os.File, string, error) {
	tempFile, err := os.CreateTemp(filepath.Dir(dstCBZ), ".tmp-cbz-*")
	if err != nil {
		return nil, "", fmt.Errorf("failed creating temp CBZ file: %w", err)
	}
	return tempFile, tempFile.Name(), nil
}

func cleanupTempFile(tempFile *os.File, tempPath string) {
	tempFile.Close()
	if _, err := os.Stat(tempPath); err == nil {
		if err := os.Remove(tempPath); err != nil {
			log.Printf("failed to remove temp CBZ file: %v", err)
		}
	}
}

func writeToZip(tempFile *os.File, srcDir string) error {
	zipWriter := zip.NewWriter(tempFile)
	defer zipWriter.Close()

	files, err := getImageFiles(srcDir)
	if err != nil {
		return err
	}

	for _, fileName := range files {
		if err := addFileToZip(zipWriter, srcDir, fileName); err != nil {
			return fmt.Errorf("failed adding file to zip: %w", err)
		}
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

	buf := bufferPool.Get().([]byte)
	defer bufferPool.Put(buf)

	_, err = io.CopyBuffer(writer, fileToZip, buf)
	if err != nil {
		return fmt.Errorf("failed copying image to zip: %w", err)
	}
	return nil
}
