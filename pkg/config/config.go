package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config holds all runtime settings for the webtoon downloader.
type Config struct {
	URL              string  `mapstructure:"url"`
	OutputDir        string  `mapstructure:"output_dir"`
	Workers          int     `mapstructure:"workers"`
	RPS              float64 `mapstructure:"rps"`
	Burst            int     `mapstructure:"burst"`
	MaxWidth         float64 `mapstructure:"max_width"`
	Quality          int     `mapstructure:"quality"`
	Scale            float64 `mapstructure:"scale"`
	LogPath          string  `mapstructure:"log_path"`
	ConfigFile       string  `mapstructure:"config"`
	CacheDir         string  `mapstructure:"cache_dir"`
	ScraperDogAPIKey string  `mapstructure:"scraper_dog_api_key"`
	SecondaryAPIKey  string  `mapstructure:"secondary_api_key"`
	ApifyToken       string  `mapstructure:"apify_token"`
}

// DefaultConfig returns the baseline default settings.
func DefaultConfig() *Config {
	return &Config{
		URL:       "https://comicland.org/home",
		OutputDir: GetDefaultDownloadDir(),
		Workers:   5,
		RPS:       10.0,
		Burst:     10,
		MaxWidth:  1000.0,
		Quality:   75,
		Scale:     1.0,
		LogPath:   "./logs",
		CacheDir:  "./cache",
	}
}

// GetDefaultDownloadDir returns an idiomatic default download location based on OS/environment.
func GetDefaultDownloadDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current relative working directory if home cannot be resolved
		return "./downloads"
	}

	// Termux detection
	if isTermux() {
		// If Termux shared storage exists, save to main device downloads
		sharedDownloads := "/sdcard/Download/webtoon-dl"
		if _, err := os.Stat("/sdcard/Download"); err == nil {
			return sharedDownloads
		}
		// Fallback to internal Termux home
		return filepath.Join(home, "downloads")
	}

	// Standard Linux / WSL / macOS XDG fallback
	return filepath.Join(home, "Downloads", "webtoon-dl")
}

func isTermux() bool {
	// Termux sets PREFIX to /data/data/com.termux/files/usr
	return os.Getenv("PREFIX") != "" && runtime.GOOS == "linux"
}

// UpdateAPIKeys saves the API keys to a .env file and updates the current config.
func (c *Config) UpdateAPIKeys(sdKey, secKey, apifyToken string) error {
	c.ScraperDogAPIKey = sdKey
	c.SecondaryAPIKey = secKey
	c.ApifyToken = apifyToken

	envMap := make(map[string]string)
	_ = godotenv.Load(".env") // Load existing if any

	envMap["WEBTOON_SCRAPER_DOG_API_KEY"] = sdKey
	envMap["WEBTOON_SECONDARY_API_KEY"] = secKey
	envMap["WEBTOON_APIFY_TOKEN"] = apifyToken

	if err := godotenv.Write(envMap, ".env"); err != nil {
		return fmt.Errorf("failed to write .env file: %w", err)
	}
	return nil
}

// Load reads config from viper (which handles config files, env vars, and flags).
func Load(v *viper.Viper) (*Config, error) {
	_ = godotenv.Load() // Load .env file into environment
	cfg := DefaultConfig()

	// 1. Setup Environment Variables (e.g. WEBTOON_WORKERS=8)
	v.SetEnvPrefix("WEBTOON")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	v.AutomaticEnv()

	// 2. Read Config File if specified or found in default locations
	if cfgFile := v.GetString("config"); cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.AddConfigPath(".")
		v.AddConfigPath("./configs")
		v.SetConfigName("config")
		v.SetConfigType("yaml")
	}

	if err := v.ReadInConfig(); err != nil {
		// Ignore error if config file is missing; fallback to flags/defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok && v.GetString("config") != "" {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// 3. Unmarshal combined settings into struct
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	return cfg, nil
}
