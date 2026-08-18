package downloader

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

// AtomicWriteFile writes data to a file atomically by creating a temporary file
// and renaming it to the final destination upon successful write and sync.
func AtomicWriteFile(path string, data io.Reader) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed creating directory: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, ".tmp-*.tmp")
	if err != nil {
		return fmt.Errorf("failed creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Ensure cleanup of temp file on error
	var success bool
	defer func() {
		if err := tmpFile.Close(); err != nil {
			log.Printf("failed to close temp file: %v", err)
		}
		if !success {
			if err := os.Remove(tmpPath); err != nil {
				log.Printf("failed to remove temp file: %v", err)
			}
		}
	}()

	if _, err := io.Copy(tmpFile, data); err != nil {
		return fmt.Errorf("failed writing to temp file: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed syncing file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed closing temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed renaming temp file: %w", err)
	}

	success = true
	return nil
}
