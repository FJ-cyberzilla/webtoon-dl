package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// DashboardStats holds operational parameters to display in the command center.
type DashboardStats struct {
	TargetURL string
	Workers   int
	RPS       float64
	OutputDir string
	Quality   int
	MaxWidth  float64
	Status    string
}

// RenderCommandCenter generates a high-aesthetic terminal dashboard banner.
func RenderCommandCenter(stats DashboardStats, activeContent string, logs []string, overrideWidth int) string {
	width := overrideWidth
	if width <= 0 {
		width = GetTermWidth()
	}

	// 1. Banner
	banner := RenderDashboardBanner(width)

	// 2. Navigation Bar
	navBar := RenderNavBar(stats.Status, width)

	// 3. Telemetry Row
	telemetry := RenderTelemetry(stats, width)

	// 4. Main Workspace
	workspaceWidth := width - 4
	if workspaceWidth < 20 {
		workspaceWidth = 20
	}
	workspaceStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ActiveTheme.Secondary).
		Padding(0, 1).
		Width(workspaceWidth)

	workspace := workspaceStyle.Render(activeContent)

	// 5. Activity Feed
	feed := RenderActivityFeed(logs, width)

	return lipgloss.JoinVertical(lipgloss.Left,
		banner,
		navBar,
		telemetry,
		workspace,
		feed,
	)
}

// RenderDashboardBanner generates a responsive banner that switches to a compact title when zoomed in (< 65 cols).
func RenderDashboardBanner(width int) string {
	if width >= 65 {
		// Wide ASCII Block Banner with 5-Step Gradient
		l1 := lipgloss.NewStyle().Foreground(ActiveTheme.Primary).Render("  █   █ █████ ████  █████ █████ █████ ███╗   ██╗    ████╗  ██╗   ")
		l2 := lipgloss.NewStyle().Foreground(ActiveTheme.Secondary).Render("  █   █ █     █   █   █   █ █   █ █   █ ████╗  ██║    █   █  █     ")
		l3 := lipgloss.NewStyle().Foreground(ActiveTheme.Secondary).Render("  █ █ █ ███   ████    █   █   █ █   █ ██╔██╗ ██║    █   █  █     ")
		l4 := lipgloss.NewStyle().Foreground(ActiveTheme.Secondary).Render("  ███║█ █     █   █   █   █   █ █   █ ██║╚██╗██║    █   █  █     ")
		l5 := lipgloss.NewStyle().Foreground(ActiveTheme.Secondary).Render("  █   █ █████ ████    █   █████ █████ ██║ ╚████║    ████╗  █████ ")

		sep := lipgloss.NewStyle().Foreground(ActiveTheme.Muted).Render("  ─────────────────────────────────────────────────────────────")
		tagline := lipgloss.NewStyle().Foreground(ActiveTheme.Accent).Bold(true).Render("  N E X T - G E N   ★   W E B T O O N   D O W N L O A D E R")
		subtitle := lipgloss.NewStyle().Foreground(ActiveTheme.Accent).Faint(true).Render("  with FJ™ - Cybertronic Systems")

		logoBlock := strings.Join([]string{l1, l2, l3, l4, l5, sep, tagline, subtitle}, "\n")

		boxWidth := width - 4
		if boxWidth < 65 {
			boxWidth = 65
		}

		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ActiveTheme.Secondary).
			Padding(0, 1).
			Width(boxWidth).
			Render(logoBlock)
	}

	// Compact Banner when terminal is narrow or zoomed in
	title := lipgloss.NewStyle().Foreground(ActiveTheme.Primary).Bold(true).Render("⚡ WEBTOON-DL COMMAND CENTER")
	tagline := lipgloss.NewStyle().Foreground(ActiveTheme.Accent).Bold(true).Render("N E X T - G E N   D O W N L O A D E R")
	subtitle := lipgloss.NewStyle().Foreground(ActiveTheme.Accent).Faint(true).Render("with FJ™ - Cybertronic Systems")

	compactBlock := fmt.Sprintf("%s\n%s\n%s", title, tagline, subtitle)

	boxWidth := width - 4
	if boxWidth < 20 {
		boxWidth = 20
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorB2).
		Padding(0, 1).
		Width(boxWidth).
		Render(compactBlock)
}

// RenderNavBar renders a responsive navigation header row.
func RenderNavBar(status string, width int) string {
	titleText := "🖥️ WEBTOON COMMAND CENTER v2.1.7"
	clockText := "🕒 " + time.Now().Format("15:04:05")

	statusBadge := BadgeSuccess.Render(" SYSTEM READY ")
	if status == "BUSY" {
		statusBadge = BadgeWarning.Render(" PROCESSING ")
	} else if status == "ERROR" {
		statusBadge = BadgeError.Render(" CRITICAL ")
	}

	navWidth := width - 4
	if navWidth < 20 {
		navWidth = 20
	}

	// Dynamic Layout for Narrow Terminals (< 70 cols)
	if width < 70 {
		titleStyle := ActiveTheme.Header.Background(ActiveTheme.Secondary).Padding(0, 1)
		clockStyle := lipgloss.NewStyle().Foreground(ActiveTheme.Primary).Bold(true)

		left := titleStyle.Render(titleText)
		line := fmt.Sprintf("%s  %s  %s", left, statusBadge, clockStyle.Render(clockText))

		return lipgloss.NewStyle().
			Width(navWidth).
			MarginBottom(1).
			Render(line)
	}

	// Full Multi-Section Layout for Normal Terminals
	titleStyle := ActiveTheme.Header.Background(ActiveTheme.Secondary).Padding(0, 2)
	clockStyle := lipgloss.NewStyle().Foreground(ActiveTheme.Primary).Background(ActiveTheme.Muted).Padding(0, 1).Bold(true)

	left := titleStyle.Render(titleText)
	center := lipgloss.NewStyle().Align(lipgloss.Center).Render(statusBadge)
	right := clockStyle.Render(clockText)

	// Distribute space proportionally
	space := navWidth - lipgloss.Width(left) - lipgloss.Width(center) - lipgloss.Width(right)
	if space < 2 {
		space = 2
	}
	gap := strings.Repeat(" ", space/2)

	return lipgloss.NewStyle().
		MarginBottom(1).
		Render(lipgloss.JoinHorizontal(lipgloss.Center, left, gap, center, gap, right))
}

// RenderTelemetry renders operational metric panels (Network, Compiler, System).
func RenderTelemetry(stats DashboardStats, width int) string {
	netPanel := lipgloss.JoinVertical(lipgloss.Left,
		ActiveTheme.Header.Foreground(ActiveTheme.Primary).Render("⚡ NETWORK"),
		fmt.Sprintf("%s %s", ActiveTheme.Key.Render("Target:"), ActiveTheme.Value.Render(truncateURL(stats.TargetURL, 16))),
		fmt.Sprintf("%s %s", ActiveTheme.Key.Render("Workers:"), ActiveTheme.Value.Render(fmt.Sprintf("%d", stats.Workers))),
	)

	ioPanel := lipgloss.JoinVertical(lipgloss.Left,
		ActiveTheme.Header.Foreground(ActiveTheme.Secondary).Render("⚙️ COMPILER"),
		fmt.Sprintf("%s %s", ActiveTheme.Key.Render("Quality:"), ActiveTheme.Value.Render(fmt.Sprintf("%d%%", stats.Quality))),
		fmt.Sprintf("%s %s", ActiveTheme.Key.Render("Max W:  "), ActiveTheme.Value.Render(fmt.Sprintf("%.0fpt", stats.MaxWidth))),
	)

	sysPanel := lipgloss.JoinVertical(lipgloss.Left,
		ActiveTheme.Header.Foreground(ActiveTheme.Muted).Render("🏥 SYSTEM"),
		fmt.Sprintf("%s %s", ActiveTheme.Key.Render("CPU:"), ActiveTheme.BadgeSuccess.Render("OPTIMAL")),
		fmt.Sprintf("%s %s", ActiveTheme.Key.Render("MEM:"), ActiveTheme.BadgeSuccess.Render("STABLE")),
	)

	panelStyle := ActiveTheme.Panel.BorderForeground(ActiveTheme.Muted)

	// Stack panels vertically when terminal width is narrow (< 85 cols)
	if width < 85 {
		panelWidth := width - 4
		if panelWidth < 20 {
			panelWidth = 20
		}

		// Option A: Add MarginBottom(1) to panelStyle when stacked
		stackedStyle := panelStyle.Width(panelWidth).MarginBottom(1)

		return lipgloss.JoinVertical(lipgloss.Left,
			stackedStyle.Render(netPanel),
			stackedStyle.Render(ioPanel),
			stackedStyle.Render(sysPanel),
		)
	}

	// 3-Column Horizontal Layout for Wide Terminals
	panelWidth := (width - 10) / 3
	if panelWidth < 22 {
		panelWidth = 22
	}

	return lipgloss.JoinHorizontal(lipgloss.Top,
		panelStyle.Width(panelWidth).Render(netPanel),
		panelStyle.Width(panelWidth).Render(ioPanel),
		panelStyle.Width(panelWidth).Render(sysPanel),
	)
}

// RenderActivityFeed displays the recent activity log panel.
func RenderActivityFeed(logs []string, width int) string {
	if len(logs) == 0 {
		logs = []string{"System initialized. Awaiting user input..."}
	}

	// Display only the last 3 logs
	if len(logs) > 3 {
		logs = logs[len(logs)-3:]
	}

	var logLines []string
	for _, l := range logs {
		var line string
		switch {
		case strings.HasPrefix(l, "SUCCESS"):
			line = ActiveTheme.BadgeSuccess.Render(" OK ") + " " + ActiveTheme.Value.Render(l)
		case strings.HasPrefix(l, "WARNING"):
			line = ActiveTheme.BadgeWarning.Render(" WRN ") + " " + ActiveTheme.Value.Render(l)
		case strings.HasPrefix(l, "INFO"):
			line = ActiveTheme.BadgeInfo.Render(" INF ") + " " + ActiveTheme.Value.Render(l)
		case strings.HasPrefix(l, "ERROR"):
			line = ActiveTheme.BadgeError.Render(" ERR ") + " " + ActiveTheme.Value.Render(l)
		default:
			line = "    " + ActiveTheme.Value.Render(l)
		}
		logLines = append(logLines, " ❯ "+line)
	}

	feedContent := lipgloss.JoinVertical(lipgloss.Left,
		ActiveTheme.Header.Foreground(ActiveTheme.Primary).Render("--- RECENT ACTIVITY ---"),
		strings.Join(logLines, "\n"),
	)

	panelWidth := width - 4
	if panelWidth < 20 {
		panelWidth = 20
	}

	return ActiveTheme.Panel.
		BorderForeground(ActiveTheme.Muted).
		Width(panelWidth).
		MarginTop(1).
		Padding(0, 1).
		Render(feedContent)
}

func truncateURL(url string, maxLen int) string {
	runes := []rune(url)
	if len(runes) <= maxLen {
		return url
	}
	if maxLen < 4 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

func ShowDashboard() {
	stats := DashboardStats{
		TargetURL: "https://www.webtoons.com/en/fantasy/tower-of-god",
		Workers:   4,
		RPS:       12.5,
		OutputDir: "./downloads",
		Quality:   90,
		MaxWidth:  1200,
		Status:    "READY",
	}

	logs := []string{
		"INFO Initialization complete",
		"SUCCESS Connected to remote provider",
		"INFO Ready to process chapter batch",
	}

	output := RenderCommandCenter(stats, "Select an action from the menu below...", logs, 0)
	if _, err := os.Stdout.WriteString(output + "\n"); err != nil {
		log.Printf("failed to write to stdout: %v", err)
	}
}
