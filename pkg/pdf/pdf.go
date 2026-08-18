package pdf

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif" // Register GIF decoder for image.Decode
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/go-pdf/fpdf"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // Register WebP decoder for image.Decode
	"golang.org/x/sync/errgroup"
)

// Options holds configuration for PDF generation and image compression.
type Options struct {
	MaxWidth    float64 // Max page width in points (0 = unconstrained)
	JPEGQuality int     // JPEG compression quality (1-100, 0 = keep original format)
	ScaleFactor float64 // Pre-downsample factor (0.1 to 1.0, 1.0 = full resolution)
	Workers     int     // Number of parallel image processing workers (0 = NumCPU)
}

// Option defines a functional option setter.
type Option func(*Options)

// WithMaxWidth sets a custom maximum width constraint in points.
func WithMaxWidth(width float64) Option {
	return func(opts *Options) {
		opts.MaxWidth = width
	}
}

// WithJPEGQuality re-encodes images to JPEG at the specified quality (1-100).
func WithJPEGQuality(quality int) Option {
	return func(opts *Options) {
		if quality < 1 {
			quality = 1
		} else if quality > 100 {
			quality = 100
		}
		opts.JPEGQuality = quality
	}
}

// WithScaleFactor resizes pixel resolution before adding to PDF.
func WithScaleFactor(factor float64) Option {
	return func(opts *Options) {
		if factor > 0 && factor <= 1.0 {
			opts.ScaleFactor = factor
		}
	}
}

// WithWorkers sets the max worker goroutines for concurrent image processing.
func WithWorkers(workers int) Option {
	return func(opts *Options) {
		if workers > 0 {
			opts.Workers = workers
		}
	}
}

// bufferPool reuses bytes.Buffer instances across image processing tasks.
var bufferPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 256*1024))
	},
}

func getBuffer() *bytes.Buffer {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

func putBuffer(buf *bytes.Buffer) {
	const maxPooledCapacity = 10 * 1024 * 1024
	if buf.Cap() > maxPooledCapacity {
		return
	}
	bufferPool.Put(buf)
}

// processedPage carries processed image payload and metadata back to the main thread.
type processedPage struct {
	Index       int
	PageWidth   float64
	PageHeight  float64
	FormatType  string
	Buffer      io.Reader
	ImagePath   string
	IsProcessed bool
}

// CreatePDF converts images into a PDF using dynamic sizing, compression options, and concurrent workers.
func CreatePDF(imagePaths []string, outputPath string, opts ...Option) error {
	if len(imagePaths) == 0 {
		return fmt.Errorf("no images provided for PDF creation")
	}

	config := getOptions(opts)
	results := make([]*processedPage, len(imagePaths))

	tasks := createTasks(imagePaths)
	g, ctx := errgroup.WithContext(context.Background())

	for w := 0; w < config.Workers; w++ {
		g.Go(func() error {
			return runWorker(ctx, tasks, results, config)
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("pdf generation worker failed: %w", err)
	}

	return assemblePDF(results, outputPath)
}

func getOptions(opts []Option) Options {
	config := Options{
		MaxWidth:    1000.0,
		JPEGQuality: 0,
		ScaleFactor: 1.0,
		Workers:     runtime.NumCPU(),
	}

	for _, opt := range opts {
		opt(&config)
	}
	return config
}

func createTasks(imagePaths []string) chan task {
	numPages := len(imagePaths)
	tasks := make(chan task, numPages)
	for i, path := range imagePaths {
		tasks <- task{Index: i, Path: path}
	}
	close(tasks)
	return tasks
}

type task struct {
	Index int
	Path  string
}

func runWorker(ctx context.Context, tasks chan task, results []*processedPage, config Options) error {
	for t := range tasks {
		select {
		case <-ctx.Done():
			return fmt.Errorf("worker execution cancelled: %w", ctx.Err())
		default:
		}

		page, err := processPage(t.Index, t.Path, config)
		if err != nil {
			return err
		}
		results[t.Index] = page
	}
	return nil
}

// assemblePDF uses fpdf to construct the final document.
func assemblePDF(results []*processedPage, outputPath string) error {
	pdf := fpdf.New("P", "pt", "Letter", "")
	for idx, page := range results {
		customSize := fpdf.SizeType{
			Wd: page.PageWidth,
			Ht: page.PageHeight,
		}
		pdf.AddPageFormat("P", customSize)

		if page.IsProcessed {
			imageKey := fmt.Sprintf("compressed_img_%d", idx)
			pdf.RegisterImageOptionsReader(imageKey, fpdf.ImageOptions{ImageType: page.FormatType}, page.Buffer)
			pdf.ImageOptions(imageKey, 0, 0, page.PageWidth, page.PageHeight, false, fpdf.ImageOptions{ImageType: page.FormatType}, 0, "")
		} else {
			ext := strings.ToUpper(filepath.Ext(page.ImagePath))
			if ext != "" {
				ext = ext[1:]
			}
			pdf.ImageOptions(page.ImagePath, 0, 0, page.PageWidth, page.PageHeight, false, fpdf.ImageOptions{ImageType: ext, ReadDpi: true}, 0, "")
		}
	}
	if err := pdf.OutputFileAndClose(outputPath); err != nil {
		return fmt.Errorf("failed to write PDF: %w", err)
	}
	return nil
}

func getImageConfig(imgPath string) (image.Config, error) {
	/* #nosec G304 */
	file, err := os.Open(imgPath)
	if err != nil {
		return image.Config{}, fmt.Errorf("failed to open image %s: %w", imgPath, err)
	}
	defer file.Close()

	imgConfig, _, err := image.DecodeConfig(file)
	if err != nil {
		return image.Config{}, fmt.Errorf("failed to decode image header for %s: %w", imgPath, err)
	}
	if imgConfig.Width <= 0 || imgConfig.Height <= 0 {
		return image.Config{}, fmt.Errorf("invalid image dimensions (%dx%d) for %s", imgConfig.Width, imgConfig.Height, imgPath)
	}
	return imgConfig, nil
}

func calculateDimensions(imgConfig image.Config, config Options) (float64, float64) {
	rawWidth := float64(imgConfig.Width)
	rawHeight := float64(imgConfig.Height)
	if config.MaxWidth > 0 && rawWidth > config.MaxWidth {
		scale := config.MaxWidth / rawWidth
		return config.MaxWidth, rawHeight * scale
	}
	return rawWidth, rawHeight
}

func checkNeedsProcessing(imgPath string, config Options) bool {
	return config.JPEGQuality > 0 || config.ScaleFactor < 1.0 || strings.ToLower(filepath.Ext(imgPath)) == ".webp"
}

func fillProcessedPage(pPage *processedPage, imgPath string, config Options) error {
	bufReader, formatType, err := processAndCompressImage(imgPath, config)
	if err != nil {
		return fmt.Errorf("failed processing page %d (%s): %w", pPage.Index, imgPath, err)
	}
	pPage.Buffer = bufReader
	pPage.FormatType = formatType
	return nil
}

// processPage handles metadata reading, scaling, and compression for a single page.
func processPage(index int, imgPath string, config Options) (*processedPage, error) {
	imgConfig, err := getImageConfig(imgPath)
	if err != nil {
		return nil, err
	}

	pageWidth, pageHeight := calculateDimensions(imgConfig, config)
	needsProcessing := checkNeedsProcessing(imgPath, config)

	pPage := &processedPage{
		Index:       index,
		PageWidth:   pageWidth,
		PageHeight:  pageHeight,
		ImagePath:   imgPath,
		IsProcessed: needsProcessing,
	}

	if needsProcessing {
		if err := fillProcessedPage(pPage, imgPath, config); err != nil {
			return nil, err
		}
	}

	return pPage, nil
}

// processAndCompressImage handles pixel scaling and JPEG compression in-memory using pooled buffers.
func processAndCompressImage(imgPath string, opts Options) (io.Reader, string, error) {
	file, err := os.Open(imgPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	srcImg, _, err := image.Decode(file)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode image: %w", err)
	}

	if opts.ScaleFactor < 1.0 {
		bounds := srcImg.Bounds()
		newWidth := int(float64(bounds.Dx()) * opts.ScaleFactor)
		newHeight := int(float64(bounds.Dy()) * opts.ScaleFactor)

		dstImg := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
		draw.CatmullRom.Scale(dstImg, dstImg.Bounds(), srcImg, bounds, draw.Over, nil)
		srcImg = dstImg
	}

	buf := getBuffer()

	if opts.JPEGQuality > 0 {
		err := jpeg.Encode(buf, srcImg, &jpeg.Options{Quality: opts.JPEGQuality})
		if err != nil {
			putBuffer(buf)
			return nil, "", fmt.Errorf("failed to encode jpeg: %w", err)
		}

		reader := bytes.NewReader(append([]byte(nil), buf.Bytes()...))
		putBuffer(buf)
		return reader, "JPG", nil
	}

	if err := png.Encode(buf, srcImg); err != nil {
		putBuffer(buf)
		return nil, "", fmt.Errorf("failed to encode png: %w", err)
	}

	reader := bytes.NewReader(append([]byte(nil), buf.Bytes()...))
	putBuffer(buf)
	return reader, "PNG", nil
}
