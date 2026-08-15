package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/FJ-cyberzilla/webtoon-dl/pkg/config"
	"github.com/FJ-cyberzilla/webtoon-dl/pkg/downloader"
	"github.com/FJ-cyberzilla/webtoon-dl/pkg/pdf"
	"github.com/FJ-cyberzilla/webtoon-dl/pkg/scraper"
	"github.com/FJ-cyberzilla/webtoon-dl/pkg/ui"
)

var (
	v       *viper.Viper
	cfgFile string

	rootCmd = &cobra.Command{
		Use:     "webtoon-dl",
		Version: "2.1.7",
		Short:   "webtoon-dl is a fast, concurrent CLI scraper that compiles Webtoons into PDF format.",
		Long: `A production-ready CLI tool built in Go to download webtoon series,
process long-strip images concurrently, and convert chapters into scaled PDFs.`,
		RunE: runRoot,
	}
)

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	v = viper.New()

	// Global / Persistent Flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file path (default is ./config.yaml)")

	// Local Flags
	rootCmd.Flags().StringP("url", "u", "", "Webtoon series URL (Required)")
	rootCmd.Flags().StringP("output", "o", "./downloads", "Output directory for generated PDFs")
	rootCmd.Flags().IntP("workers", "w", 5, "Number of concurrent chapter/image download workers")
	rootCmd.Flags().Float64("rps", 10.0, "Global rate limit in requests per second")
	rootCmd.Flags().Int("burst", 10, "Maximum burst tokens allowed for rate limiter")
	rootCmd.Flags().Float64("max-width", 1000.0, "Maximum PDF page width in points (0 = unconstrained)")
	rootCmd.Flags().Int("quality", 75, "JPEG compression quality (1-100, 0 = keep original format)")
	rootCmd.Flags().Float64("scale", 1.0, "Resolution scale factor (0.1 to 1.0)")
	rootCmd.Flags().String("log-path", "./logs", "Directory to store runtime logs")

	// Bind Cobra flags to Viper
	_ = v.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	_ = v.BindPFlag("url", rootCmd.Flags().Lookup("url"))
	_ = v.BindPFlag("output_dir", rootCmd.Flags().Lookup("output"))
	_ = v.BindPFlag("workers", rootCmd.Flags().Lookup("workers"))
	_ = v.BindPFlag("rps", rootCmd.Flags().Lookup("rps"))
	_ = v.BindPFlag("burst", rootCmd.Flags().Lookup("burst"))
	_ = v.BindPFlag("max_width", rootCmd.Flags().Lookup("max-width"))
	_ = v.BindPFlag("quality", rootCmd.Flags().Lookup("quality"))
	_ = v.BindPFlag("scale", rootCmd.Flags().Lookup("scale"))
	_ = v.BindPFlag("log_path", rootCmd.Flags().Lookup("log-path"))

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runRoot(_ *cobra.Command, _ []string) error {
	// 1. Load combined configuration
	cfg, err := config.Load(v)
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	var startupLogs []string

	// 2. Validate required parameters
	if err := downloader.ValidatePath(cfg.OutputDir); err != nil {
		return fmt.Errorf("invalid output directory: %w", err)
	}

	if cfg.URL == "" {
		startupLogs = append(startupLogs, "INFO: Dashboard initialized. Please enter URL in settings.")
	} else {
		startupLogs = append(startupLogs, fmt.Sprintf("INFO: Starting webtoon-dl for target: %s", cfg.URL))
		startupLogs = append(startupLogs, fmt.Sprintf("INFO: Settings: Workers=%d | MaxWidth=%.0fpt | Quality=%d%%", cfg.Workers, cfg.MaxWidth, cfg.Quality))
	}

	// 3. Initialize Shared Components
	scr := scraper.NewScraper(cfg.RPS, cfg.Burst)
	limiter := downloader.NewRateLimiter(cfg.RPS, cfg.Burst)

	// Prepare PDF Options from config
	pdfOpts := []pdf.Option{
		pdf.WithMaxWidth(cfg.MaxWidth),
		pdf.WithJPEGQuality(cfg.Quality),
		pdf.WithScaleFactor(cfg.Scale),
	}

	_ = limiter
	_ = scr
	_ = pdfOpts

	// 4. Initialize and start BubbleTea TUI
	startupLogs = append(startupLogs, "SUCCESS: Initialization complete. Launching unified dashboard...")

	model := ui.NewIntegratedModel(cfg.URL, []string{}, []ui.ChapterItem{}, cfg)
	model.Main.Logs = append(model.Main.Logs, startupLogs...)

	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("UI error: %w", err)
	}

	return nil
}
