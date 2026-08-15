// pkg/ui/integrated.go
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/FJ-cyberzilla/webtoon-dl/pkg/config"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// IntegratedModel combines all UI components into one cohesive dashboard
type IntegratedModel struct {
	Main   MainModel
	State  ViewState
	Width  int
	Height int

	// Input for Comic Name
	NameInput textinput.Model

	// Platform
	Platform string // "webtoon" or "comicland"

	// Download status
	Downloading bool
	Progress    float64
	ETA         time.Duration
	Speed       string

	// System info
	CPU    string
	Memory string
	Disk   string
}

// NewIntegratedModel creates the complete unified dashboard
func NewIntegratedModel(comicTitle string, categories []string, items []ChapterItem, cfg *config.Config) IntegratedModel {
	main := NewMainModel(comicTitle, categories, items, cfg)

	ti := textinput.New()
	ti.Placeholder = "Enter Comic Name here (e.g. Tower of God)..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 50

	return IntegratedModel{
		Main:      main,
		State:     StateSelector,
		NameInput: ti,
		Platform:  "webtoon",
		CPU:       "OPTIMAL",
		Memory:    "STABLE",
		Disk:      "STABLE",
	}
}

func (m IntegratedModel) Init() tea.Cmd {
	return tea.Batch(
		m.Main.Init(),
		tickStatus(),
	)
}

func (m IntegratedModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height

		// Propagate to main model
		var cmd tea.Cmd
		var model tea.Model
		model, cmd = m.Main.Update(msg)
		m.Main = model.(MainModel)
		cmds = append(cmds, cmd)

	case tea.KeyMsg:
		if m.Main.Config.URL == "" {
			return m.handleNameInputState(msg)
		}
		return m.handleMainState(msg)

	case StatusTickMsg:
		m.CPU = msg.CPU
		m.Memory = msg.Memory
		m.Disk = msg.Disk
		cmds = append(cmds, tickStatus())

	default:
		// Pass to main model
		var cmd tea.Cmd
		var model tea.Model
		model, cmd = m.Main.Update(msg)
		m.Main = model.(MainModel)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m IntegratedModel) handleNameInputState(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		name := m.NameInput.Value()
		// Construct a mock or search URL based on platform
		var url string
		if m.Platform == "webtoon" {
			url = fmt.Sprintf("https://www.webtoons.com/en/search?keyword=%s", name)
		} else {
			url = fmt.Sprintf("https://comicland.org/search?keyword=%s", name)
		}
		m.Main.Config.URL = url
		m.Main.Logs = append(m.Main.Logs, fmt.Sprintf("INFO: Searching for comic '%s' on %s...", name, strings.ToUpper(m.Platform)))
		return m, nil
	case "p", "P":
		return m.togglePlatform()
	case "q", "ctrl+c":
		return m, tea.Quit
	default:
		var cmd tea.Cmd
		m.NameInput, cmd = m.NameInput.Update(msg)
		return m, cmd
	}
}

func (m IntegratedModel) handleMainState(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "p", "P":
		return m.togglePlatform()
	case "q", "ctrl+c":
		return m, tea.Quit
	default:
		// Pass to main model
		var cmd tea.Cmd
		var model tea.Model
		model, cmd = m.Main.Update(msg)
		m.Main = model.(MainModel)
		return m, cmd
	}
}

func (m IntegratedModel) togglePlatform() (tea.Model, tea.Cmd) {
	if m.Platform == "webtoon" {
		m.Platform = "comicland"
	} else {
		m.Platform = "webtoon"
	}
	m.Main.Logs = append(m.Main.Logs,
		fmt.Sprintf("INFO: Switched to %s platform.", m.Platform))
	return m, nil
}

func (m IntegratedModel) View() string {
	var sb strings.Builder

	// 1. Banner
	sb.WriteString(RenderDashboardBanner(m.Width))
	sb.WriteString("\n")

	// 2. Platform Toggle
	sb.WriteString(m.renderPlatformToggle())
	sb.WriteString("\n")

	// 3. Main Content or Comic Name Input
	if m.Main.Config.URL == "" {
		sb.WriteString("\n\n")
		prompt := "Please enter a Comic Name for " + strings.ToUpper(m.Platform) + ":"
		sb.WriteString("  " + lipgloss.NewStyle().Foreground(ColorB1).Bold(true).Render(prompt) + "\n")
		sb.WriteString("  " + m.NameInput.View() + "\n")
	} else {
		mainView := m.Main.View()
		sb.WriteString(mainView)
	}

	// 4. Download Progress (if active)
	if m.Downloading {
		sb.WriteString("\n")
		sb.WriteString(m.renderDownloadBar())
	}

	// 5. System Status Footer
	sb.WriteString("\n")
	sb.WriteString(m.renderStatusFooter())

	return sb.String()
}

func (m IntegratedModel) renderPlatformToggle() string {
	webtoonStyle := lipgloss.NewStyle().
		Foreground(ActiveTheme.Muted).
		Padding(0, 2)

	comiclandStyle := lipgloss.NewStyle().
		Foreground(ActiveTheme.Muted).
		Padding(0, 2)

	if m.Platform == "webtoon" {
		webtoonStyle = lipgloss.NewStyle().
			Foreground(ActiveTheme.Background).
			Background(ActiveTheme.Success).
			Bold(true).
			Padding(0, 2)
	} else {
		comiclandStyle = lipgloss.NewStyle().
			Foreground(ActiveTheme.Background).
			Background(ActiveTheme.Accent).
			Bold(true).
			Padding(0, 2)
	}

	hint := lipgloss.NewStyle().
		Foreground(ActiveTheme.Muted).
		Render("[P] Platform | [Q] Exit")

	return lipgloss.JoinHorizontal(
		lipgloss.Center,
		webtoonStyle.Render("🌐 WEBTOON"),
		lipgloss.NewStyle().Padding(0, 1).Render(hint),
		comiclandStyle.Render("📚 COMICLAND"),
	)
}

func (m IntegratedModel) renderDownloadBar() string {
	width := m.Width - 20
	if width < 20 {
		width = 20
	}

	filled := int(m.Progress * float64(width))
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)

	percentStyle := lipgloss.NewStyle().
		Foreground(ColorSuccess).
		Bold(true)

	etaStyle := lipgloss.NewStyle().
		Foreground(ColorWarning).
		Bold(true)

	content := fmt.Sprintf(`
  ⬇ DOWNLOADING  %s
  %s
  %s  %s  %s
`,
		percentStyle.Render(fmt.Sprintf("%.1f%%", m.Progress*100)),
		lipgloss.NewStyle().Foreground(ColorB1).Render(bar),
		etaStyle.Render(fmt.Sprintf("ETA: %s", m.ETA.Round(time.Second))),
		lipgloss.NewStyle().Foreground(ColorMuted).Render(fmt.Sprintf("Speed: %s", m.Speed)),
		lipgloss.NewStyle().Foreground(ColorMuted).Render(fmt.Sprintf("Platform: %s", strings.ToUpper(m.Platform))),
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSuccess).
		Padding(1).
		Width(m.Width - 4).
		Render(content)
}

func (m IntegratedModel) renderStatusFooter() string {
	left := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Render(fmt.Sprintf("🕒 %s", time.Now().Format("15:04:05")))

	// Determine active API
	activeAPI := "None"
	if m.Main.Config.ApifyToken != "" {
		activeAPI = "Apify"
	} else if m.Main.Config.ScraperDogAPIKey != "" {
		activeAPI = "ScraperDog"
	}

	center := lipgloss.NewStyle().
		Foreground(ColorSuccess).
		Bold(true).
		Render(fmt.Sprintf("● CPU: %s | MEM: %s | DISK: %s | API: %s", m.CPU, m.Memory, m.Disk, activeAPI))

	right := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Render(fmt.Sprintf("Platform: %s", strings.ToUpper(m.Platform)))

	space := m.Width - lipgloss.Width(left) - lipgloss.Width(center) - lipgloss.Width(right) - 6
	if space < 2 {
		space = 2
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorMuted).
		Padding(0, 1).
		Width(m.Width - 4).
		Render(lipgloss.JoinHorizontal(
			lipgloss.Center,
			left,
			strings.Repeat(" ", space),
			center,
			strings.Repeat(" ", space),
			right,
		))
}

// StatusTickMsg updates system status
type StatusTickMsg struct {
	CPU    string
	Memory string
	Disk   string
}

func tickStatus() tea.Cmd {
	return tea.Tick(5*time.Second, func(_ time.Time) tea.Msg {
		return StatusTickMsg{
			CPU:    "OPTIMAL",
			Memory: "STABLE",
			Disk:   "STABLE",
		}
	})
}
