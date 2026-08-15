package models

import (
	"encoding/json"
	"testing"
)

func TestComicJSON(t *testing.T) {
	chapter := ChapterItem{
		ID:    "1",
		Title: "Chapter 1",
	}

	data, err := json.Marshal(chapter)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var unmarshaled ChapterItem
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if unmarshaled.ID != chapter.ID {
		t.Errorf("Expected ID %s, got %s", chapter.ID, unmarshaled.ID)
	}
}
