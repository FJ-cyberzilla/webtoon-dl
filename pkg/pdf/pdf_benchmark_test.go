package pdf

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// setupBenchmarkImages creates high-resolution test images to simulate realistic workloads.
func setupBenchmarkImages(b *testing.B, count int, format string) (string, []string) {
	b.Helper()
	tmpDir := b.TempDir()
	paths := make([]string, count)

	width, height := 3000, 4000 // Typical high-res page size
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill with patterns to prevent unrealistic pure-solid compression ratios
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if (x+y)%2 == 0 {
				img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
			} else {
				img.Set(x, y, color.RGBA{R: 255, G: uint8(x % 256), B: uint8(y % 256), A: 255})
			}
		}
	}

	for i := 0; i < count; i++ {
		filename := filepath.Join(tmpDir, fmt.Sprintf("bench_page_%d.%s", i, format))
		f, err := os.Create(filename)
		if err != nil {
			b.Fatalf("failed to create benchmark image: %v", err)
		}

		if format == "jpg" || format == "jpeg" {
			err = jpeg.Encode(f, img, &jpeg.Options{Quality: 90})
		} else {
			err = png.Encode(f, img)
		}
		f.Close()

		if err != nil {
			b.Fatalf("failed to encode benchmark image: %v", err)
		}
		paths[i] = filename
	}

	return tmpDir, paths
}

func BenchmarkCreatePDF(b *testing.B) {
	const pageCount = 5

	b.Run("DirectEmbed_PNG", func(b *testing.B) {
		tmpDir, paths := setupBenchmarkImages(b, pageCount, "png")
		outPath := filepath.Join(tmpDir, "out_direct.pdf")

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			err := CreatePDF(paths, outPath)
			if err != nil {
				b.Fatalf("CreatePDF failed: %v", err)
			}
		}
	})

	b.Run("DirectEmbed_JPEG", func(b *testing.B) {
		tmpDir, paths := setupBenchmarkImages(b, pageCount, "jpg")
		outPath := filepath.Join(tmpDir, "out_direct_jpg.pdf")

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			err := CreatePDF(paths, outPath)
			if err != nil {
				b.Fatalf("CreatePDF failed: %v", err)
			}
		}
	})

	b.Run("Compressed_JPEG_Quality_75", func(b *testing.B) {
		tmpDir, paths := setupBenchmarkImages(b, pageCount, "png")
		outPath := filepath.Join(tmpDir, "out_compressed.pdf")

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			err := CreatePDF(paths, outPath, WithJPEGQuality(75))
			if err != nil {
				b.Fatalf("CreatePDF failed: %v", err)
			}
		}
	})

	b.Run("Downsampled_And_Compressed", func(b *testing.B) {
		tmpDir, paths := setupBenchmarkImages(b, pageCount, "png")
		outPath := filepath.Join(tmpDir, "out_scaled_compressed.pdf")

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			err := CreatePDF(
				paths,
				outPath,
				WithScaleFactor(0.5),
				WithJPEGQuality(75),
				WithMaxWidth(1000),
			)
			if err != nil {
				b.Fatalf("CreatePDF failed: %v", err)
			}
		}
	})
}
