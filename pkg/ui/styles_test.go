package ui

import (
	"strings"
	"testing"
)

func TestRenderBanner(t *testing.T) {
	output := RenderBanner()
	if output == "" {
		t.Error("RenderBanner returned empty string")
	}

	// Basic check for some expected content
	// The banner might be the full ASCII art or the compact version depending on terminal width.
	if !strings.Contains(output, "█") && !strings.Contains(output, "WEBTOON-DL") {
		t.Errorf("RenderBanner output missing expected content, got: %q", output)
	}
}

func TestRenderPanel(t *testing.T) {
	title := "Test Panel"
	content := "Test Content"
	output := RenderPanel(title, content)

	if output == "" {
		t.Error("RenderPanel returned empty string")
	}

	if !strings.Contains(output, title) {
		t.Errorf("RenderPanel output missing title: %s", title)
	}

	if !strings.Contains(output, content) {
		t.Errorf("RenderPanel output missing content: %s", content)
	}
}
