package ui

import (
	"strings"

	"github.com/FJ-cyberzilla/webtoon-dl/pkg/config"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type SettingsModel struct {
	cfg        *config.Config
	sdInput    textinput.Model
	secInput   textinput.Model
	apifyInput textinput.Model
	focusIndex int
	SavedMsg   string
}

func NewSettingsModel(cfg *config.Config) SettingsModel {
	sd := textinput.New()
	sd.Placeholder = "Enter ScraperDog API Key (Optional)"
	sd.SetValue(cfg.ScraperDogAPIKey)
	sd.Focus()

	sec := textinput.New()
	sec.Placeholder = "Enter Secondary API Key (Optional)"
	sec.SetValue(cfg.SecondaryAPIKey)

	apify := textinput.New()
	apify.Placeholder = "Enter Apify API Token"
	apify.SetValue(cfg.ApifyToken)

	return SettingsModel{
		cfg:        cfg,
		sdInput:    sd,
		secInput:   sec,
		apifyInput: apify,
		focusIndex: 0,
	}
}

func (m SettingsModel) Update(msg tea.Msg) (SettingsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}

	var cmd tea.Cmd
	switch m.focusIndex {
	case 0:
		m.sdInput, cmd = m.sdInput.Update(msg)
	case 1:
		m.secInput, cmd = m.secInput.Update(msg)
	case 2:
		m.apifyInput, cmd = m.apifyInput.Update(msg)
	}

	return m, cmd
}

func (m *SettingsModel) handleKeyMsg(msg tea.KeyMsg) (SettingsModel, tea.Cmd) {
	switch msg.String() {
	case "tab", "shift+tab", "up", "down":
		m.rotateFocus()
		return *m, nil
	case "enter":
		m.saveKeys()
		return *m, nil
	}
	return *m, nil
}

func (m *SettingsModel) rotateFocus() {
	m.focusIndex = (m.focusIndex + 1) % 3
	m.sdInput.Blur()
	m.secInput.Blur()
	m.apifyInput.Blur()
	switch m.focusIndex {
	case 0:
		m.sdInput.Focus()
	case 1:
		m.secInput.Focus()
	case 2:
		m.apifyInput.Focus()
	}
}

func (m *SettingsModel) saveKeys() {
	err := m.cfg.UpdateAPIKeys(m.sdInput.Value(), m.secInput.Value(), m.apifyInput.Value())
	if err != nil {
		m.SavedMsg = "❌ Error saving keys!"
	} else {
		m.SavedMsg = "✅ Saved to .env file successfully!"
	}
}

func (m SettingsModel) View() string {
	var b strings.Builder
	b.WriteString("── ⚙️ API Settings (Auto-saves to .env) ──\n\n")

	b.WriteString("ScraperDog API Key:\n")
	b.WriteString(m.sdInput.View() + "\n\n")

	b.WriteString("Secondary API Key:\n")
	b.WriteString(m.secInput.View() + "\n\n")

	b.WriteString("Apify API Token:\n")
	b.WriteString(m.apifyInput.View() + "\n\n")

	if m.SavedMsg != "" {
		b.WriteString(m.SavedMsg + "\n\n")
	}

	b.WriteString("[Tab] Switch Field  •  [Enter] Save to .env  •  [Esc] Back\n")
	return b.String()
}
