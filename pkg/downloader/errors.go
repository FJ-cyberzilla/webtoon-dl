package downloader

import (
	"errors"
	"fmt"
	"syscall"
)

var (
	ErrNetwork = errors.New("network error")
	ErrHTTP    = errors.New("http error")
	ErrIO      = errors.New("io error")
	ErrNoSpace = errors.New("insufficient disk space")
)

// CheckDiskSpace verifies if the given path has enough free space.
// We use a safe threshold (e.g., 50MB) for required free space.
func CheckDiskSpace(path string, requiredBytes uint64) error {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return fmt.Errorf("failed to stat filesystem: %w", err)
	}

	// Available blocks * size per block
	availableBytes := uint64(stat.Bavail) * uint64(stat.Bsize)
	if availableBytes < requiredBytes {
		return fmt.Errorf("%w: required %d bytes, available %d bytes", ErrNoSpace, requiredBytes, availableBytes)
	}
	return nil
}
