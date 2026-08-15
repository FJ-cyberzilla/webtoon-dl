package cache

import (
	"container/list"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")

	cm, err := NewManager(cacheDir, 50)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	if cm.capacity != 50 {
		t.Errorf("Expected capacity 50, got %d", cm.capacity)
	}

	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		t.Error("Cache directory was not created")
	}
}

func TestNewManager_InvalidCapacity(t *testing.T) {
	tmpDir := t.TempDir()
	cm, _ := NewManager(tmpDir, 0) // Should default to 100

	if cm.capacity != 100 {
		t.Errorf("Expected default capacity 100, got %d", cm.capacity)
	}
}

func TestSetAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	cm, _ := NewManager(tmpDir, 10)

	key := "test-key"
	data := []byte("test-data")

	if err := cm.Set(key, data); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	retrieved, found := cm.Get(key, time.Hour)
	if !found {
		t.Fatal("Key not found")
	}

	if string(retrieved) != string(data) {
		t.Errorf("Expected %s, got %s", data, retrieved)
	}
}

func TestLRU(t *testing.T) {
	tmpDir := t.TempDir()
	// Capacity of 2
	cm, _ := NewManager(tmpDir, 2)

	if err := cm.Set("k1", []byte("v1")); err != nil {
		t.Fatalf("Set k1 failed: %v", err)
	}
	if err := cm.Set("k2", []byte("v2")); err != nil {
		t.Fatalf("Set k2 failed: %v", err)
	}
	if err := cm.Set("k3", []byte("v3")); err != nil {
		t.Fatalf("Set k3 failed: %v", err)
	} // Evicts k1 from memory

	// Memory buffer capacity verification
	cm.mu.RLock()
	lenMem := len(cm.memBuffer)
	cm.mu.RUnlock()

	if lenMem > 2 {
		t.Errorf("Memory buffer exceeded capacity: got %d", lenMem)
	}
}
func TestTTLRules(t *testing.T) {
	tmpDir := t.TempDir()
	cm, _ := NewManager(tmpDir, 10)

	key := "ttl-key"
	if err := cm.Set(key, []byte("data")); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	if _, found := cm.Get(key, time.Second); !found {
		t.Error("Fresh item not found")
	}
}
func TestPurgeExpired(t *testing.T) {
	tmpDir := t.TempDir()
	cm, _ := NewManager(tmpDir, 10)

	// Set an item
	if err := cm.Set("k1", []byte("v1")); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Manually set modification time in the past
	filePath := cm.getFilePath("k1")
	pastTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filePath, pastTime, pastTime); err != nil {
		t.Fatalf("Chtimes failed: %v", err)
	}

	purged, err := cm.PurgeExpired(time.Minute)
	if err != nil {
		t.Fatalf("PurgeExpired failed: %v", err)
	}

	if purged != 1 {
		t.Errorf("Expected 1 purged item, got %d", purged)
	}

	if cm.HasKey("k1") {
		t.Error("Cache item still exists after PurgeExpired")
	}
}

func TestHasImage(t *testing.T) {
	tmpDir := t.TempDir()
	cm, _ := NewManager(tmpDir, 10)

	filePath := filepath.Join(tmpDir, "test.png")
	if err := os.WriteFile(filePath, []byte("data"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if !cm.HasImage(filePath) {
		t.Error("HasImage failed to detect existing file")
	}

	if cm.HasImage(filepath.Join(tmpDir, "nonexistent.png")) {
		t.Error("HasImage detected nonexistent file")
	}
}

func TestGetExpired(t *testing.T) {
	tmpDir := t.TempDir()
	cm, _ := NewManager(tmpDir, 10)

	key := "expired-key"
	if err := cm.Set(key, []byte("data")); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Clear memory buffer to force disk read
	cm.mu.Lock()
	cm.memBuffer = make(map[string]*list.Element)
	cm.lruList.Init()
	cm.mu.Unlock()

	// Manually set modification time in the past
	filePath := cm.getFilePath(key)
	pastTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filePath, pastTime, pastTime); err != nil {
		t.Fatalf("Chtimes failed: %v", err)
	}

	if _, found := cm.Get(key, time.Minute); found {
		t.Error("Expired item was returned by Get()")
	}
}

func TestClear(t *testing.T) {
	tmpDir := t.TempDir()
	cm, _ := NewManager(tmpDir, 10)

	if err := cm.Set("k1", []byte("v1")); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if err := cm.Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	if cm.HasKey("k1") {
		t.Error("Cache not empty after Clear()")
	}
}

func TestHasKey_DiskOnly(t *testing.T) {
	tmpDir := t.TempDir()
	cm, _ := NewManager(tmpDir, 10)

	key := "disk-only-key"
	if err := cm.Set(key, []byte("data")); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Force clear memory to leave only disk
	cm.mu.Lock()
	cm.memBuffer = make(map[string]*list.Element)
	cm.lruList.Init()
	cm.mu.Unlock()

	if !cm.HasKey(key) {
		t.Error("HasKey failed to find disk-only key")
	}
}

func TestGet_ReadFileError(t *testing.T) {
	tmpDir := t.TempDir()
	cm, _ := NewManager(tmpDir, 10)

	key := "error-key"
	if err := cm.Set(key, []byte("data")); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Make file unreadable
	filePath := cm.getFilePath(key)
	if err := os.Chmod(filePath, 0000); err != nil {
		t.Fatalf("Chmod failed: %v", err)
	}
	defer func() {
		if err := os.Chmod(filePath, 0644); err != nil {
			t.Logf("Failed to reset permissions: %v", err)
		}
	}() // Reset for cleanup

	// Clear memory to force disk read
	cm.mu.Lock()
	cm.memBuffer = make(map[string]*list.Element)
	cm.lruList.Init()
	cm.mu.Unlock()

	_, found := cm.Get(key, 0)
	if found {
		t.Error("Expected Get to fail due to unreadable file, but it succeeded")
	}
}
