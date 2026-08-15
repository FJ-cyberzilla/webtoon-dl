package ui

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// Theme encapsulates all UI design tokens.
type Theme struct {
	// Colors (Tokens)
	Primary    lipgloss.Color
	Secondary  lipgloss.Color
	Accent     lipgloss.Color
	Success    lipgloss.Color
	Warning    lipgloss.Color
	Error      lipgloss.Color
	Text       lipgloss.Color
	Muted      lipgloss.Color
	Background lipgloss.Color

	// Base Component Styles
	Panel  lipgloss.Style
	Header lipgloss.Style
	Key    lipgloss.Style
	Value  lipgloss.Style

	// Badges
	BadgeInfo    lipgloss.Style
	BadgeSuccess lipgloss.Style
	BadgeWarning lipgloss.Style
	BadgeError   lipgloss.Style
}

// DefaultDarkTheme provides the standard dark mode aesthetic.
func DefaultDarkTheme() *Theme {
	// Palette
	cyan := lipgloss.Color("#00D2FF")
	purple := lipgloss.Color("#AF50FF")
	emerald := lipgloss.Color("#00EB96")
	amber := lipgloss.Color("#FFAA00")
	crimson := lipgloss.Color("#FF3C64")
	slate := lipgloss.Color("#6E78A0")
	text := lipgloss.Color("#D1D5DB")

	// Base styles
	badgeBase := lipgloss.NewStyle().Bold(true).Padding(0, 1)

	return &Theme{
		Primary:    cyan,
		Secondary:  purple,
		Accent:     lipgloss.Color("#FF8C00"),
		Success:    emerald,
		Warning:    amber,
		Error:      crimson,
		Text:       text,
		Muted:      slate,
		Background: lipgloss.Color("#1A1D28"),

		Panel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(purple).
			Padding(0, 1),

		Header: lipgloss.NewStyle().Foreground(cyan).Bold(true),
		Key:    lipgloss.NewStyle().Foreground(slate).Bold(true),
		Value:  lipgloss.NewStyle().Foreground(text),

		BadgeInfo:    badgeBase.Foreground(lipgloss.Color("#000000")).Background(cyan),
		BadgeSuccess: badgeBase.Foreground(lipgloss.Color("#000000")).Background(emerald),
		BadgeWarning: badgeBase.Foreground(lipgloss.Color("#000000")).Background(amber),
		BadgeError:   badgeBase.Foreground(lipgloss.Color("#FFFFFF")).Background(crimson),
	}
}

// ActiveTheme is the global export for backward compatibility.
var ActiveTheme = DefaultDarkTheme()

// --- Backward compatibility aliases (to be migrated) ---

var (
	ColorB1         = ActiveTheme.Primary
	ColorB2         = ActiveTheme.Secondary
	ColorMuted      = ActiveTheme.Muted
	ColorText       = ActiveTheme.Text
	ColorSuccess    = ActiveTheme.Success
	ColorWarning    = ActiveTheme.Warning
	ColorError      = ActiveTheme.Error
	ColorHeaderFg   = ActiveTheme.Background // Or a suitable color
	ColorB4         = ActiveTheme.Secondary  // Placeholder mapping
	ColorSubtle     = ActiveTheme.Muted
	ColorCmdPurple  = ActiveTheme.Secondary
	ColorVintageGrn = ActiveTheme.Success
	KeyStyle        = ActiveTheme.Key
	ValStyle        = ActiveTheme.Value
	BadgeInfo       = ActiveTheme.BadgeInfo
	BadgeSuccess    = ActiveTheme.BadgeSuccess
	BadgeWarning    = ActiveTheme.BadgeWarning
	BadgeError      = ActiveTheme.BadgeError
	HeaderStyle     = ActiveTheme.Header
	TitleStyle      = ActiveTheme.Header.Bold(true)
	DiagnosticBox   = ActiveTheme.Panel
)

// -----------------------------------------------------------------------------
// 🖥️ Responsive UI Rendering Helpers
// -----------------------------------------------------------------------------

// GetTermWidth returns current stdout terminal width, defaulting to 80 on fail.
func GetTermWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

// RenderBanner renders a responsive header that adapts if zoomed in (< 65 cols).
func RenderBanner() string {
	width := GetTermWidth()
	var sb strings.Builder

	// Use ActiveTheme for colors
	if width >= 65 {
		// Simplified example - full refactoring would use theme tokens
		l1 := lipgloss.NewStyle().Foreground(ActiveTheme.Primary).Render("  █   █ █████ ████  █████ █████ █████ ███╗   ██╗    ████╗  ██╗   ")
		sb.WriteString("\n" + l1 + "\n")
	} else {
		compactTitle := lipgloss.NewStyle().Foreground(ActiveTheme.Primary).Bold(true).Render("⚡ WEBTOON-DL")
		sb.WriteString("\n  " + compactTitle + "\n")
	}
	return sb.String()
}

// RenderPanel creates a container panel that automatically adjusts to terminal width.
func RenderPanel(title string, content string) string {
	width := GetTermWidth()
	panelWidth := width - 4
	if panelWidth < 30 {
		panelWidth = 30
	}

	// Use ActiveTheme.Panel
	style := ActiveTheme.Panel.Width(panelWidth)
	header := ActiveTheme.Header.Render(" " + title + " ")

	return style.Render(header + "\n" + content)
}

// -----------------------------------------------------------------------------
// 🖥️ Responsive Custom Slog Console Handler
// -----------------------------------------------------------------------------
type AestheticConsoleHandler struct {
	w io.Writer
}

func NewAestheticConsoleHandler(w io.Writer) *AestheticConsoleHandler {
	return &AestheticConsoleHandler{w: w}
}

func (h *AestheticConsoleHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *AestheticConsoleHandler) Handle(_ context.Context, r slog.Record) error {
	var badge lipgloss.Style

	switch {
	case r.Level >= slog.LevelError:
		badge = ActiveTheme.BadgeError
	case r.Level >= slog.LevelWarn:
		badge = ActiveTheme.BadgeWarning
	case r.Level >= slog.LevelInfo:
		badge = ActiveTheme.BadgeInfo
	default:
		badge = lipgloss.NewStyle().Foreground(ActiveTheme.Muted)
	}

	// Format line using ActiveTheme tokens
	line := fmt.Sprintf("%s %s %s",
		lipgloss.NewStyle().Foreground(ActiveTheme.Muted).Render(r.Time.Format("15:04:05")),
		badge.Render(" LOG "),
		lipgloss.NewStyle().Foreground(ActiveTheme.Text).Render(r.Message),
	)

	fmt.Fprintln(h.w, line)
	return nil
}

func (h *AestheticConsoleHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *AestheticConsoleHandler) WithGroup(_ string) slog.Handler      { return h }
