package pdf

import (
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// Helper: generate a test image file
func createTestImage(t *testing.T, dir, filename string, width, height int, format string) string {
	t.Helper()
	imgPath := filepath.Join(dir, filename)

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}

	f, err := os.Create(imgPath)
	if err != nil {
		t.Fatalf("failed to create test image: %v", err)
	}
	defer f.Close()

	if format == "jpeg" {
		err = jpeg.Encode(f, img, &jpeg.Options{Quality: 80})
	} else {
		err = png.Encode(f, img)
	}

	if err != nil {
		t.Fatalf("failed to encode test image: %v", err)
	}

	return imgPath
}

func TestOptions(t *testing.T) {
	opts := Options{}

	WithMaxWidth(800.0)(&opts)
	if opts.MaxWidth != 800.0 {
		t.Errorf("expected MaxWidth 800.0, got %f", opts.MaxWidth)
	}

	WithJPEGQuality(75)(&opts)
	if opts.JPEGQuality != 75 {
		t.Errorf("expected JPEGQuality 75, got %d", opts.JPEGQuality)
	}

	WithJPEGQuality(150)(&opts) // Clamp test
	if opts.JPEGQuality != 100 {
		t.Errorf("expected clamped JPEGQuality 100, got %d", opts.JPEGQuality)
	}

	WithScaleFactor(0.5)(&opts)
	if opts.ScaleFactor != 0.5 {
		t.Errorf("expected ScaleFactor 0.5, got %f", opts.ScaleFactor)
	}
}

func TestCreatePDF_SuccessDirectEmbed(t *testing.T) {
	tmpDir := t.TempDir()
	img1 := createTestImage(t, tmpDir, "page1.png", 500, 700, "png")
	img2 := createTestImage(t, tmpDir, "page2.jpg", 600, 800, "jpeg")

	pdfPath := filepath.Join(tmpDir, "output.pdf")
	err := CreatePDF([]string{img1, img2}, pdfPath)
	if err != nil {
		t.Fatalf("CreatePDF failed: %v", err)
	}

	info, err := os.Stat(pdfPath)
	if err != nil {
		t.Fatalf("failed to stat output PDF: %v", err)
	}
	if info.Size() == 0 {
		t.Error("expected non-empty PDF file")
	}
}

func TestCreatePDF_WithInProcessCompression(t *testing.T) {
	tmpDir := t.TempDir()
	img1 := createTestImage(t, tmpDir, "page1.png", 1200, 1600, "png")

	pdfPath := filepath.Join(tmpDir, "compressed_output.pdf")
	err := CreatePDF(
		[]string{img1},
		pdfPath,
		WithMaxWidth(800),
		WithJPEGQuality(60),
		WithScaleFactor(0.5),
	)
	if err != nil {
		t.Fatalf("CreatePDF with options failed: %v", err)
	}

	info, err := os.Stat(pdfPath)
	if err != nil {
		t.Fatalf("failed to stat compressed PDF: %v", err)
	}
	if info.Size() == 0 {
		t.Error("expected non-empty compressed PDF file")
	}
}

func TestCreatePDF_EmptyImagesError(t *testing.T) {
	err := CreatePDF([]string{}, filepath.Join(t.TempDir(), "out.pdf"))
	if err == nil {
		t.Fatal("expected error for empty image slice, got nil")
	}
}
