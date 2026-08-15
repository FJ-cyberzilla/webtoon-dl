package pkg

// Branding information embedded in the binary
const (
	Author      = "FJ-cyberzilla"
	ProjectName = "Webtoon-dl"
	Copyright   = "Copyright (C) 2026 FJ-cyberzilla"
	RepoURL     = "https://github.com/FJ-cyberzilla/webtoon-dl"
	Branding    = "FJ™ - Cybertronic Systems"
)

// GetNotice returns the legal notice
func GetNotice() string {
	return Copyright + " (" + RepoURL + ")"
}

// GetBranding returns branding info
func GetBranding() string {
	return Branding
}
