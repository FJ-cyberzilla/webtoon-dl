package ui

import (
	"strings"
	"testing"

	"github.com/FJ-cyberzilla/webtoon-dl/pkg/config"
)

func TestSettingsModel(t *testing.T) {
	cfg := &config.Config{}
	m := NewSettingsModel(cfg)

	if m.focusIndex != 0 {
		t.Errorf("Expected initial focusIndex 0, got %d", m.focusIndex)
	}

	view := m.View()
	if !strings.Contains(view, "API Settings") {
		t.Errorf("View() missing expected title: %s", view)
	}
	if !strings.Contains(view, "ScraperDog") {
		t.Errorf("View() missing expected field: %s", view)
	}
}
