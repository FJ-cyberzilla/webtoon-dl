package ui

import (
	"github.com/FJ-cyberzilla/webtoon-dl/pkg/config"
	tea "github.com/charmbracelet/bubbletea"
)

type ViewState int

const (
	StateSelector ViewState = iota
	StateSettings
)

type MainModel struct {
	State         ViewState
	SelectorModel Model
	SettingsModel SettingsModel
	Config        *config.Config
	Logs          []string
	Width         int
}

func NewMainModel(comicTitle string, categories []string, items []ChapterItem, cfg *config.Config) MainModel {
	initWidth := GetTermWidth()

	return MainModel{
		State:         StateSelector,
		SelectorModel: NewChapterSelector(comicTitle, categories, items, cfg.CacheDir, 5, 3),
		SettingsModel: NewSettingsModel(cfg),
		Config:        cfg,
		Logs: []string{
			"Main engine initialized.",
			"Config loaded from local environment.",
		},
		Width: initWidth,
	}
}

func (m MainModel) Init() tea.Cmd {
	return nil
}

func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width

		// Propagate window dimensions to sub-models so child widgets resize correctly
		var selCmd, setCmd tea.Cmd
		var selModel tea.Model

		selModel, selCmd = m.SelectorModel.Update(msg)
		m.SelectorModel = selModel.(Model)
		m.SettingsModel, setCmd = m.SettingsModel.Update(msg)

		cmds = append(cmds, selCmd, setCmd)
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		// Global navigation toggles
		switch msg.String() {
		case "s", "S":
			if m.State == StateSelector {
				m.State = StateSettings
				m.Logs = append(m.Logs, "INFO: Entered API settings configuration.")
				return m, nil
			}
		case "esc":
			if m.State == StateSettings {
				m.State = StateSelector
				m.Logs = append(m.Logs, "INFO: Returned to main chapter workspace.")
				return m, nil
			}
		}
	}

	// Delegate input processing based on active state
	var cmd tea.Cmd
	if m.State == StateSettings {
		m.SettingsModel, cmd = m.SettingsModel.Update(msg)
	} else {
		var selModel tea.Model
		selModel, cmd = m.SelectorModel.Update(msg)
		m.SelectorModel = selModel.(Model)
	}

	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m MainModel) View() string {
	var activeContent string
	if m.State == StateSettings {
		activeContent = m.SettingsModel.View()
	} else {
		activeContent = m.SelectorModel.View()
	}

	status := "READY"
	if m.State == StateSettings {
		status = "BUSY"
	}

	stats := DashboardStats{
		TargetURL: m.Config.URL,
		Workers:   m.Config.Workers,
		OutputDir: m.Config.OutputDir,
		Quality:   m.Config.Quality,
		MaxWidth:  m.Config.MaxWidth,
		Status:    status,
	}

	return RenderCommandCenter(stats, activeContent, m.Logs, m.Width)
}
