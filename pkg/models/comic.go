package models

import "time"

// ChapterItem represents an individual chapter for UI selection and download queuing.
type ChapterItem struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Number      float64   `json:"number"`
	URL         string    `json:"url"`
	Selected    bool      `json:"selected"`
	SliceCount  int       `json:"slice_count,omitempty"`
	PublishedAt time.Time `json:"published_at,omitempty"`
}

// ImageSlice represents a single image panel/slice within a webtoon chapter strip.
type ImageSlice struct {
	Index    int    `json:"index"`
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Path     string `json:"path,omitempty"`
	Size     int64  `json:"size,omitempty"`
}

// ComicMetadata holds high-level information about a webtoon series.
type ComicMetadata struct {
	Title       string        `json:"title"`
	Author      string        `json:"author"`
	CoverURL    string        `json:"cover_url"`
	SourceURL   string        `json:"source_url"`
	Chapters    []ChapterItem `json:"chapters"`
	TotalSlices int           `json:"total_slices,omitempty"`
}

// DownloadTask represents an active item in the processing queue.
type DownloadTask struct {
	Chapter   ChapterItem  `json:"chapter"`
	Slices    []ImageSlice `json:"slices"`
	OutputDir string       `json:"output_dir"`
	Status    string       `json:"status"` // e.g., "QUEUED", "DOWNLOADING", "COMPILING", "DONE", "ERROR"
	Error     error        `json:"-"`
}
