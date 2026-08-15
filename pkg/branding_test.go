package pkg

import (
	"strings"
	"testing"
)

func TestGetNotice(t *testing.T) {
	notice := GetNotice()
	if !strings.Contains(notice, Copyright) {
		t.Errorf("Expected notice to contain Copyright, got: %s", notice)
	}
	if !strings.Contains(notice, RepoURL) {
		t.Errorf("Expected notice to contain RepoURL, got: %s", notice)
	}
}

func TestGetBranding(t *testing.T) {
	branding := GetBranding()
	if branding != Branding {
		t.Errorf("Expected branding %s, got: %s", Branding, branding)
	}
}
