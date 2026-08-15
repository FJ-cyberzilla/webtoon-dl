package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ChapterItem represents an individual chapter item in the selection list.
type ChapterItem struct {
	ID       string
	Title    string
	Selected bool
}

// Model holds the Bubble Tea state for the selector component.
type Model struct {
	chapters   []ChapterItem
	cursor     int
	quitted    bool
	width      int
	height     int
	offset     int // Scroll window offset
	comicTitle string
}

// InitialChapterModel creates the initial state for standalone usage.
func InitialChapterModel(chapters []ChapterItem) Model {
	return Model{
		chapters: chapters,
		cursor:   0,
		width:    GetTermWidth(),
		height:   10,
	}
}

// NewChapterSelector wraps creation for MainModel.
func NewChapterSelector(comicTitle string, _ []string, items []ChapterItem, _ string, _, _ int) Model {
	return Model{
		chapters:   items,
		cursor:     0,
		width:      GetTermWidth(),
		height:     10, // Visible window height
		comicTitle: comicTitle,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles incoming keypresses and resizes state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.handleWindowSize(msg)
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}
	return m, nil
}

func (m *Model) handleWindowSize(msg tea.WindowSizeMsg) {
	m.width = msg.Width
	if msg.Height > 15 {
		m.height = msg.Height - 12
	} else {
		m.height = 6
	}
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		m.quitted = true
		return m, tea.Quit
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case " ":
		m.toggleSelection()
	case "a":
		m.selectAll()
	case "i":
		m.invertSelection()
	case "enter":
		return m, tea.Quit
	}
	return m, nil
}

func (m *Model) moveCursor(delta int) {
	if len(m.chapters) == 0 {
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = len(m.chapters) - 1
	} else if m.cursor >= len(m.chapters) {
		m.cursor = 0
	}
	m.adjustOffset()
}

func (m *Model) toggleSelection() {
	if len(m.chapters) > 0 {
		m.chapters[m.cursor].Selected = !m.chapters[m.cursor].Selected
	}
}

func (m *Model) selectAll() {
	allSelected := true
	for _, item := range m.chapters {
		if !item.Selected {
			allSelected = false
			break
		}
	}
	for i := range m.chapters {
		m.chapters[i].Selected = !allSelected
	}
}

func (m *Model) invertSelection() {
	for i := range m.chapters {
		m.chapters[i].Selected = !m.chapters[i].Selected
	}
}

// Adjusts the scroll offset so the cursor is always visible.
func (m *Model) adjustOffset() {
	if m.height <= 0 {
		m.height = 8
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	} else if m.cursor >= m.offset+m.height {
		m.offset = m.cursor - m.height + 1
	}
}

// View renders the terminal view based on current state.
func (m Model) View() string {
	if m.quitted {
		return lipgloss.NewStyle().Foreground(ColorWarning).Render("Selection canceled by user.")
	}

	var b strings.Builder
	b.WriteString(m.renderHeader())
	if len(m.chapters) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(ColorMuted).Italic(true).Render("  No chapters available.\n"))
		return b.String()
	}

	b.WriteString(m.renderList())
	b.WriteString(m.renderFooter())
	return b.String()
}

func (m Model) renderHeader() string {
	headerTitle := "📚 CHAPTER SELECTOR"
	if m.comicTitle != "" {
		headerTitle += fmt.Sprintf(" — %s", m.comicTitle)
	}
	header := lipgloss.NewStyle().Foreground(ColorB1).Bold(true).Render(headerTitle)

	subKey := lipgloss.NewStyle().Foreground(ColorCmdPurple).Bold(true)
	subVal := lipgloss.NewStyle().Foreground(ColorText)

	helpLine := fmt.Sprintf("%s Navigation • %s Toggle • %s All • %s Invert • %s Confirm",
		subKey.Render("[↑/↓]"),
		subKey.Render("[Space]"),
		subKey.Render("[a]"),
		subKey.Render("[i]"),
		subKey.Render("[Enter]"),
	)
	return header + "\n" + subVal.Render(helpLine) + "\n\n"
}

func (m Model) renderList() string {
	var b strings.Builder
	maxLen := len(m.chapters)
	visibleHeight := m.height
	if visibleHeight > maxLen {
		visibleHeight = maxLen
	}

	end := m.offset + visibleHeight
	if end > maxLen {
		end = maxLen
	}

	maxTitleWidth := m.getMaxTitleWidth()

	for i := m.offset; i < end; i++ {
		b.WriteString(m.renderItem(i, maxTitleWidth) + "\n")
	}

	if maxLen > visibleHeight {
		scrollInfo := fmt.Sprintf("  ▲▼ showing %d-%d of %d", m.offset+1, end, maxLen)
		b.WriteString(lipgloss.NewStyle().Foreground(ColorMuted).Render(scrollInfo) + "\n")
	}
	return b.String()
}

func (m Model) getMaxTitleWidth() int {
	w := m.width - 15
	if w < 15 {
		return 15
	}
	return w
}

func (m Model) renderItem(idx int, maxTitleWidth int) string {
	item := m.chapters[idx]
	isCursor := m.cursor == idx

	cursorStr := "  "
	if isCursor {
		cursorStr = lipgloss.NewStyle().Foreground(ColorB1).Bold(true).Render("❯ ")
	}

	checkedStr := lipgloss.NewStyle().Foreground(ColorMuted).Render("○ ")
	if item.Selected {
		checkedStr = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true).Render("◉ ")
	}

	title := item.Title
	if len(title) > maxTitleWidth {
		title = title[:maxTitleWidth-3] + "..."
	}

	titleStr := ValStyle.Render(title)
	if isCursor {
		titleStr = lipgloss.NewStyle().
			Foreground(ColorB2).
			Bold(true).
			Render(title)
	}

	return fmt.Sprintf("%s%s%s", cursorStr, checkedStr, titleStr)
}

func (m Model) renderFooter() string {
	selectedCount := 0
	for _, item := range m.chapters {
		if item.Selected {
			selectedCount++
		}
	}
	maxLen := len(m.chapters)
	countStr := fmt.Sprintf(" %d / %d SELECTED ", selectedCount, maxLen)
	countBadge := BadgeInfo.Render(countStr)
	if selectedCount > 0 {
		countBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(ColorVintageGrn).
			Padding(0, 1).
			Render(countStr)
	}

	statusText := lipgloss.NewStyle().Foreground(ColorMuted).Render("READY FOR COMPILATION")
	return fmt.Sprintf("\n%s %s", countBadge, statusText)
}

// PromptChapterSelection presents the interactive menu and returns selected items.
func PromptChapterSelection(availableChapters []ChapterItem) ([]ChapterItem, error) {
	p := tea.NewProgram(InitialChapterModel(availableChapters))

	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run chapter selector: %w", err)
	}

	m, ok := finalModel.(Model)
	if !ok || m.quitted {
		return nil, fmt.Errorf("chapter selection was canceled")
	}

	// Filter and return selected items
	var selected []ChapterItem
	for _, item := range m.chapters {
		if item.Selected {
			selected = append(selected, item)
		}
	}

	if len(selected) == 0 {
		return nil, fmt.Errorf("no chapters selected")
	}

	return selected, nil
}
