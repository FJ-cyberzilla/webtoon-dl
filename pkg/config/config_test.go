package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.URL != "https://comicland.org/home" {
		t.Errorf("Expected default URL, got %s", cfg.URL)
	}
	if cfg.Workers != 5 {
		t.Errorf("Expected 5 workers, got %d", cfg.Workers)
	}
}

func TestLoad(t *testing.T) {
	v := viper.New()
	// Set a mock config file path to avoid reading actual files
	v.Set("url", "https://test.com")

	cfg, err := Load(v)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.URL != "https://test.com" {
		t.Errorf("Expected test.com, got %s", cfg.URL)
	}
}

func TestUpdateAPIKeys(t *testing.T) {
	cfg := DefaultConfig()
	sd := "sd-key"
	sec := "sec-key"
	api := "api-token"

	err := cfg.UpdateAPIKeys(sd, sec, api)
	if err != nil {
		t.Fatalf("Failed to update keys: %v", err)
	}

	// Verify .env file creation
	if _, err := os.Stat(".env"); os.IsNotExist(err) {
		t.Error(".env file not created")
	}

	// Cleanup
	_ = os.Remove(".env")
}
