package ui

import (
	"strings"
	"testing"
)

func TestChapterSelectorView(t *testing.T) {
	items := []ChapterItem{
		{ID: "1", Title: "Chapter 1", Selected: true},
		{ID: "2", Title: "Chapter 2", Selected: false},
	}
	m := InitialChapterModel(items)

	view := m.View()

	if !strings.Contains(view, "CHAPTER SELECTOR") {
		t.Errorf("View() missing expected title: %s", view)
	}
	if !strings.Contains(view, "Chapter 1") {
		t.Errorf("View() missing item: %s", view)
	}
	if !strings.Contains(view, "Chapter 2") {
		t.Errorf("View() missing item: %s", view)
	}
}
