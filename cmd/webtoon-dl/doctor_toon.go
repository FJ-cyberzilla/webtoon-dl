package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/FJ-cyberzilla/webtoon-dl/pkg/httpclient"
	"github.com/FJ-cyberzilla/webtoon-dl/pkg/ui"
)

var doctorToonCmd = &cobra.Command{
	Use:   "doctortoon [URL]",
	Short: "Run network and system diagnostics to troubleshoot Cloudflare blocking and environment health",
	Long: ui.HeaderStyle.Render(" SYSTEM DIAGNOSTICS & DOCTOR ") + "\n" +
		"Audits network connectivity, checks target webtoon host response headers for Cloudflare\n" +
		"anti-bot challenges, and verifies local disk permissions.",
	Args: cobra.MaximumNArgs(1),
	Run:  runDoctorToonDiagnostics,
}

func init() {
	rootCmd.AddCommand(doctorToonCmd)
}

type CheckResult struct {
	Name    string
	Passed  bool
	Message string
	Detail  string
}

func runDoctorToonDiagnostics(_ *cobra.Command, args []string) {
	fmt.Fprintln(os.Stdout)
	// Render Header Box
	headerText := ui.TitleStyle.Render(" WEBTOON-DL DIAGNOSTICS CONTROL CENTER  v2.1.7")
	fmt.Fprintln(os.Stdout, ui.DiagnosticBox.Render(headerText))
	fmt.Fprintln(os.Stdout)

	// Render Table Header
	headerStyle := lipgloss.NewStyle().Foreground(ui.ColorMuted).Bold(true).Padding(0, 1)
	fmt.Fprintln(os.Stdout, headerStyle.Render("  STATUS     SUBSYSTEM           METRIC / LATENCY        DETAILS"))
	fmt.Fprintln(os.Stdout, ui.KeyStyle.Render(" ───────────────────────────────────────────────────────────────────"))

	targetURL := "https://www.webtoons.com"
	if len(args) > 0 && args[0] != "" {
		targetURL = args[0]
	}

	outDir := viper.GetString("output")
	if outDir == "" {
		outDir = "./downloads"
	}

	checks := []CheckResult{
		checkInternetConnectivity(),
		checkTargetHost(targetURL),
		checkDiskPermissions(outDir),
		checkLogsDirectory("./logs"),
	}

	allPassed := true
	for _, check := range checks {
		var status string
		if check.Passed {
			status = ui.BadgeSuccess.Render(" [ OK ] ")
		} else {
			status = ui.BadgeError.Render(" [FAIL] ")
			allPassed = false
		}

		// Row formatting
		fmt.Fprintf(os.Stdout, "  %s %-18s %-23s %s\n", status, check.Name, check.Message, check.Detail)
	}

	fmt.Fprintln(os.Stdout, ui.KeyStyle.Render("\n ───────────────────────────────────────────────────────────────────"))
	if allPassed {
		fmt.Fprintln(os.Stdout, ui.BadgeSuccess.Render(" ❯ SYSTEM HEALTH: 100% OPERATIONAL "))
	} else {
		fmt.Fprintln(os.Stdout, ui.BadgeError.Render(" ❯ SYSTEM HEALTH: ISSUES DETECTED "))
	}
	fmt.Fprintln(os.Stdout)
}

// 1. Audit DNS & Base WAN Connection
func checkInternetConnectivity() CheckResult {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://1.1.1.1")
	if err != nil {
		return CheckResult{
			Name:    "Internet Connectivity",
			Passed:  false,
			Message: "Unable to reach public DNS endpoint (1.1.1.1)",
			Detail:  fmt.Sprintf("Error: %v", err),
		}
	}
	_ = resp.Body.Close()

	return CheckResult{
		Name:    "Internet Connectivity",
		Passed:  true,
		Message: "Active internet connection verified.",
	}
}

// 2. Audit Target Webtoon Server & Cloudflare Anti-Bot Status
func checkTargetHost(targetURL string) CheckResult {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return CheckResult{
			Name:    "Webtoon Target Reachability",
			Passed:  false,
			Message: "Invalid target URL format.",
			Detail:  err.Error(),
		}
	}

	// Spoof desktop browser user agent
	req.Header.Set("User-Agent", httpclient.DefaultUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")

	client := &http.Client{}
	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start).Round(time.Millisecond)

	if err != nil {
		return CheckResult{
			Name:    "Webtoon Target Reachability",
			Passed:  false,
			Message: fmt.Sprintf("Failed to connect to %s", targetURL),
			Detail:  fmt.Sprintf("Network error: %v", err),
		}
	}
	defer resp.Body.Close()

	serverHeader := resp.Header.Get("Server")
	cfRay := resp.Header.Get("CF-RAY")
	cfMitigation := resp.Header.Get("cf-mitigated")

	// Check for Cloudflare challenge status codes
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == 429 {
		detailMsg := fmt.Sprintf("HTTP Status %d | Server: %s", resp.StatusCode, serverHeader)
		if cfRay != "" {
			detailMsg += fmt.Sprintf(" | CF-Ray ID: %s", cfRay)
		}

		return CheckResult{
			Name:    "Cloudflare Anti-Bot Status",
			Passed:  false,
			Message: "Cloudflare challenge or rate-limit active on target domain.",
			Detail:  detailMsg + "\n       Recommendation: Try lowering request rate (-r flag) or rotating User-Agent headers.",
		}
	}

	successDetail := fmt.Sprintf("HTTP Status: %d %s | Latency: %v", resp.StatusCode, resp.Status, latency)
	if cfRay != "" || cfMitigation != "" {
		successDetail += fmt.Sprintf(" | Protected by Cloudflare (CF-Ray: %s)", cfRay)
	}

	return CheckResult{
		Name:    "Webtoon Target Reachability",
		Passed:  true,
		Message: fmt.Sprintf("Successfully established session with %s", targetURL),
		Detail:  successDetail,
	}
}

// 3. Audit Local File System Permissions for Output Directory
func checkDiskPermissions(outDir string) CheckResult {
	if err := os.MkdirAll(outDir, 0750); err != nil {
		return CheckResult{
			Name:    "Disk Output Permissions",
			Passed:  false,
			Message: fmt.Sprintf("Cannot create output directory at %s", outDir),
			Detail:  err.Error(),
		}
	}

	testFile := filepath.Join(outDir, ".permission_check.tmp")
	if err := os.WriteFile(testFile, []byte("test"), 0600); err != nil {
		return CheckResult{
			Name:    "Disk Output Permissions",
			Passed:  false,
			Message: fmt.Sprintf("Output directory %s is not writable", outDir),
			Detail:  err.Error(),
		}
	}
	_ = os.Remove(testFile)

	return CheckResult{
		Name:    "Disk Output Permissions",
		Passed:  true,
		Message: fmt.Sprintf("Output directory '%s' is writable.", outDir),
	}
}

// 4. Audit Log Directory
func checkLogsDirectory(logDir string) CheckResult {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return CheckResult{
			Name:    "Log Directory",
			Passed:  false,
			Message: fmt.Sprintf("Cannot create logs directory at %s", logDir),
			Detail:  err.Error(),
		}
	}

	return CheckResult{
		Name:    "Log Directory",
		Passed:  true,
		Message: fmt.Sprintf("Logging directory '%s' verified.", logDir),
	}
}
