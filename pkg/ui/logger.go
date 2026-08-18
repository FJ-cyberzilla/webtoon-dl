package ui

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	mu            sync.RWMutex
	fileLogger    *log.Logger
	logFileHandle *os.File
	outputWriter  io.Writer = os.Stdout
)

// -----------------------------------------------------------------------------
// 📁 Logger Initialization & Management
// -----------------------------------------------------------------------------

// InitializeLogger sets up the persistent file logger. Returns a cleanup function.
func InitializeLogger(logDir string) (func(), error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Clean file name generation without terminal formatting codes
	logFileName := fmt.Sprintf("log_%s.txt", time.Now().Format("2006-01-02"))
	logPath := filepath.Join(logDir, logFileName)

	/* #nosec G304 */
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	mu.Lock()
	if logFileHandle != nil {
		_ = logFileHandle.Close()
	}
	logFileHandle = f
	fileLogger = log.New(f, "", log.LstdFlags)
	mu.Unlock()

	cleanup := func() {
		mu.Lock()
		defer mu.Unlock()
		if logFileHandle != nil {
			_ = logFileHandle.Close()
			logFileHandle = nil
			fileLogger = nil
		}
	}

	return cleanup, nil
}

// SetOutputWriter overrides default output stream (useful for tests or alternate buffers).
func SetOutputWriter(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	outputWriter = w
}

// writeToLog writes thread-safely to the active log file without ANSI sequences.
func writeToLog(msg string) {
	mu.RLock()
	defer mu.RUnlock()
	if fileLogger != nil {
		fileLogger.Println(msg)
	}
}

// -----------------------------------------------------------------------------
// 📢 Formatted Terminal Message Printers
// -----------------------------------------------------------------------------

// PrintInfo prints informational messages with blue/cyan highlight.
func PrintInfo(message string) {
	badge := BadgeInfo.Render(" INF ")
	msg := lipgloss.NewStyle().Foreground(ColorText).Bold(true).Render(message)
	fmt.Fprintf(outputWriter, "%s %s %s\n", renderTimestamp(), badge, msg)
	writeToLog("[INFO] " + message)
}

// PrintSuccess prints success messages with emerald green highlight.
func PrintSuccess(message string) {
	badge := BadgeSuccess.Render(" SUCCESS ")
	msg := lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true).Render(message)
	fmt.Fprintf(outputWriter, "%s %s %s\n", renderTimestamp(), badge, msg)
	writeToLog("[SUCCESS] " + message)
}

// PrintError prints error messages with crimson red highlight.
func PrintError(message string) {
	badge := BadgeError.Render(" ERR ")
	msg := lipgloss.NewStyle().Foreground(ColorError).Bold(true).Render(message)
	fmt.Fprintf(outputWriter, "%s %s %s\n", renderTimestamp(), badge, msg)
	writeToLog("[ERROR] " + message)
}

// PrintWarning prints warning messages with amber highlight.
func PrintWarning(message string) {
	badge := BadgeWarning.Render(" WRN ")
	msg := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true).Render(message)
	fmt.Fprintf(outputWriter, "%s %s %s\n", renderTimestamp(), badge, msg)
	writeToLog("[WARNING] " + message)
}

// -----------------------------------------------------------------------------
// 📊 Dynamic & Responsive Progress Bar & Spinner
// -----------------------------------------------------------------------------

// PrintProgress displays a dynamically scaled progress bar on a single line that fits any terminal width.
func PrintProgress(current, total int, label string) {
	if total <= 0 {
		return
	}

	percentage := float64(current) / float64(total) * 100
	termWidth := GetTermWidth()

	// Safe length bounds to prevent negative string multiplication panics on narrow terminals (e.g. Termux/Splits)
	barLength := termWidth - 36
	if barLength > 40 {
		barLength = 40
	} else if barLength < 5 {
		barLength = 5
	}

	filled := int((percentage / 100) * float64(barLength))
	if filled > barLength {
		filled = barLength
	}
	if filled < 0 {
		filled = 0
	}

	// Blue-to-Purple Gradient Progress Bar
	filledBar := lipgloss.NewStyle().Foreground(ColorB2).Render(strings.Repeat("█", filled))
	emptyBar := lipgloss.NewStyle().Foreground(ColorMuted).Render(strings.Repeat("░", barLength-filled))

	badge := BadgeInfo.Render(" PROG ")
	stats := lipgloss.NewStyle().Foreground(ColorB1).Bold(true).Render(fmt.Sprintf("%3.0f%% (%d/%d)", percentage, current, total))

	// Safely truncate label for narrow screens
	lbl := label
	lblRunes := []rune(lbl)
	if len(lblRunes) > 20 && termWidth < 65 {
		lbl = string(lblRunes[:17]) + "..."
	}
	lblStyle := lipgloss.NewStyle().Foreground(ColorText).Render(lbl)

	line := fmt.Sprintf("\r\033[K%s %s %s%s %s %s", renderTimestamp(), badge, filledBar, emptyBar, stats, lblStyle)
	fmt.Fprint(outputWriter, line)

	if current >= total {
		fmt.Fprintln(outputWriter)
	}

	writeToLog(fmt.Sprintf("[PROGRESS] %s %3.0f%% (%d/%d)", label, percentage, current, total))
}

// StartSpinner starts a responsive waiting animation with cyan-to-purple accents.
func StartSpinner(message string, stopChan <-chan struct{}) {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	spinnerStyle := lipgloss.NewStyle().Foreground(ColorB1).Bold(true)
	msgStyle := lipgloss.NewStyle().Foreground(ColorText).Bold(true)

	if message == "" {
		message = "Processing..."
	}

	i := 0
	for {
		select {
		case <-stopChan:
			// Fully clear current active terminal line
			fmt.Fprint(outputWriter, "\r\033[K")
			return
		case <-ticker.C:
			frame := spinnerStyle.Render(frames[i%len(frames)])
			fmt.Fprintf(outputWriter, "\r\033[K%s %s %s", renderTimestamp(), frame, msgStyle.Render(message))
			i++
		}
	}
}

// Helper function to render a consistent muted timestamp
func renderTimestamp() string {
	return lipgloss.NewStyle().Foreground(ColorMuted).Render(time.Now().Format("15:04:05"))
}

// Render outputs the string representation of any UI component model.
func Render(model interface{}) string {
	if stringer, ok := model.(fmt.Stringer); ok {
		return stringer.String()
	}
	return fmt.Sprintf("%v", model)
}
