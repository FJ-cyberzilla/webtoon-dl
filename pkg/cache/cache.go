package cache

import (
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type lruEntry struct {
	key   string
	value []byte
}

// Manager handles two-tier caching: high-speed memory LRU + atomic disk persistence.
type Manager struct {
	baseDir   string
	mu        sync.RWMutex
	capacity  int
	lruList   *list.List
	memBuffer map[string]*list.Element
}

// NewManager initializes disk cache and an in-memory LRU layer.
// maxMemItems limits how many items stay in RAM (e.g., 50–200 slices).
func NewManager(baseDir string, maxMemItems int) (*Manager, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	if maxMemItems <= 0 {
		maxMemItems = 100 // Default memory capacity
	}

	return &Manager{
		baseDir:   baseDir,
		capacity:  maxMemItems,
		lruList:   list.New(),
		memBuffer: make(map[string]*list.Element),
	}, nil
}

// Get checks Memory LRU first, falling back to Disk on miss.
func (cm *Manager) Get(key string, maxAge time.Duration) ([]byte, bool) {
	// 1. Memory Tier (Fast Path)
	cm.mu.Lock()
	if elem, exists := cm.memBuffer[key]; exists {
		cm.lruList.MoveToFront(elem)
		data := elem.Value.(*lruEntry).value
		cm.mu.Unlock()
		return data, true
	}
	cm.mu.Unlock()

	// 2. Disk Tier (Fallback Path)
	filePath := cm.getFilePath(key)
	info, err := os.Stat(filePath)
	if err != nil || info.Size() == 0 {
		return nil, false // Miss or corrupt zero-byte entry
	}

	// Check TTL expiration
	if maxAge > 0 && time.Since(info.ModTime()) > maxAge {
		if err := os.Remove(filePath); err != nil {
			// Log error but continue as we wanted to remove it anyway
			log.Printf("failed to remove expired cache file: %v\n", err)
		}
		return nil, false
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, false
	}

	// Populate Memory Buffer with retrieved disk data
	cm.putMem(key, data)

	return data, true
}

// Set writes data atomically to both Memory LRU and Disk.
func (cm *Manager) Set(key string, data []byte) error {
	// Populate Memory LRU
	cm.putMem(key, data)

	// Atomic Disk Write
	filePath := cm.getFilePath(key)
	tmpFile, err := os.CreateTemp(cm.baseDir, ".tmp-cache-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	var success bool
	defer func() {
		tmpFile.Close()
		if !success {
			if err := os.Remove(tmpPath); err != nil {
				log.Printf("failed to remove temp cache file: %v\n", err)
			}
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		return fmt.Errorf("failed writing to temp cache: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed syncing cache file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed closing cache file: %w", err)
	}

	if err := os.Rename(tmpPath, filePath); err != nil {
		return fmt.Errorf("failed replacing cache file: %w", err)
	}

	success = true
	return nil
}

// Internal helper to insert/update items in the RAM LRU buffer.
func (cm *Manager) putMem(key string, data []byte) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Update existing element
	if elem, exists := cm.memBuffer[key]; exists {
		cm.lruList.MoveToFront(elem)
		elem.Value.(*lruEntry).value = data
		return
	}

	// Evict oldest RAM item if capacity reached
	if cm.lruList.Len() >= cm.capacity {
		oldest := cm.lruList.Back()
		if oldest != nil {
			cm.lruList.Remove(oldest)
			kv := oldest.Value.(*lruEntry)
			delete(cm.memBuffer, kv.key)
		}
	}

	// Add new element to front
	entry := &lruEntry{key: key, value: data}
	elem := cm.lruList.PushFront(entry)
	cm.memBuffer[key] = elem
}

// HasImage verifies whether a file exists locally and is non-empty.
func (cm *Manager) HasImage(targetPath string) bool {
	info, err := os.Stat(targetPath)
	return err == nil && info.Size() > 0
}

// HasKey checks Memory buffer first, then Disk.
func (cm *Manager) HasKey(key string) bool {
	cm.mu.RLock()
	if _, exists := cm.memBuffer[key]; exists {
		cm.mu.RUnlock()
		return true
	}
	cm.mu.RUnlock()

	filePath := cm.getFilePath(key)
	info, err := os.Stat(filePath)
	return err == nil && info.Size() > 0
}

// PurgeExpired cleans up disk cache and flushes RAM LRU.
func (cm *Manager) PurgeExpired(maxAge time.Duration) (int, error) {
	cm.mu.Lock()
	cm.lruList.Init()
	cm.memBuffer = make(map[string]*list.Element)
	cm.mu.Unlock()

	entries, err := os.ReadDir(cm.baseDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read cache directory: %w", err)
	}

	purged := 0
	now := time.Now()

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".cache" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if now.Sub(info.ModTime()) > maxAge {
			if err := os.Remove(filepath.Join(cm.baseDir, entry.Name())); err == nil {
				purged++
			}
		}
	}

	return purged, nil
}

// Clear flushes both RAM buffer and disk cache.
func (cm *Manager) Clear() error {
	cm.mu.Lock()
	cm.lruList.Init()
	cm.memBuffer = make(map[string]*list.Element)
	cm.mu.Unlock()

	entries, err := os.ReadDir(cm.baseDir)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".cache" {
			if err := os.Remove(filepath.Join(cm.baseDir, entry.Name())); err != nil {
				log.Printf("failed to remove cache file: %v\n", err)
			}
		}
	}

	return nil
}

func (cm *Manager) getFilePath(key string) string {
	hash := sha256.Sum256([]byte(key))
	fileName := hex.EncodeToString(hash[:]) + ".cache"
	return filepath.Join(cm.baseDir, fileName)
}
