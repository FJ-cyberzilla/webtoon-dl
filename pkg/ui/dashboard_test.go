package ui

import (
	"strings"
	"testing"
)

func TestRenderCommandCenter(t *testing.T) {
	stats := DashboardStats{
		TargetURL: "https://example.com",
		Workers:   5,
		OutputDir: "./downloads",
		Quality:   75,
		MaxWidth:  1000,
		Status:    "READY",
	}

	logs := []string{"INFO: Test log 1", "SUCCESS: Test log 2"}
	activeContent := "Active Content View"
	width := 80

	output := RenderCommandCenter(stats, activeContent, logs, width)

	if output == "" {
		t.Error("RenderCommandCenter returned empty string")
	}

	// Basic check for some expected strings in the output
	expectedStrings := []string{
		"WEBTOON",
		"NETWORK",
		"COMPILER",
		"SYSTEM",
		"Active Content View",
		"RECENT ACTIVITY",
		"Test log 1",
	}

	for _, s := range expectedStrings {
		if !strings.Contains(output, s) {
			t.Errorf("RenderCommandCenter output missing expected string: %s", s)
		}
	}
}

func TestRenderDashboardBanner(t *testing.T) {
	// Wide
	outputWide := RenderDashboardBanner(100)
	if !strings.Contains(outputWide, "W E B T O O N") {
		t.Errorf("RenderDashboardBanner(100) missing expected string. Output: %s", outputWide)
	}

	// Narrow
	outputNarrow := RenderDashboardBanner(40)
	if !strings.Contains(outputNarrow, "WEBTOON") {
		t.Errorf("RenderDashboardBanner(40) missing expected string. Output: %s", outputNarrow)
	}
}

func TestRenderTelemetry(t *testing.T) {
	stats := DashboardStats{
		TargetURL: "https://example.com",
		Workers:   5,
		Quality:   75,
		MaxWidth:  1000,
	}

	// Wide
	outputWide := RenderTelemetry(stats, 100)
	if !strings.Contains(outputWide, "NETWORK") || !strings.Contains(outputWide, "COMPILER") {
		t.Errorf("RenderTelemetry(100) missing panels, got: %q", outputWide)
	}

	// Narrow
	outputNarrow := RenderTelemetry(stats, 40)
	if !strings.Contains(outputNarrow, "NETWORK") || !strings.Contains(outputNarrow, "COMPILER") {
		t.Errorf("RenderTelemetry(40) missing panels, got: %q", outputNarrow)
	}
}
