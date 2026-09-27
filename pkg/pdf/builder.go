package pdf

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// PageItem represents a page with its image and orientation.
type PageItem struct {
	Image    image.Image
	Rotation int // 0, 90, 180, 270
}

// BuildOptions holds PDF compilation options.
type BuildOptions struct {
	Filename   string
	OutputDir  string
	DPI        int
	ColorMode  string // "auto", "color", "grayscale"
	JPEGQuality int   // defaults to 88
}

// GetDefaultScansDir returns ~/Documents/Scans in a cross-platform manner.
func GetDefaultScansDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, "Documents", "Scans")
}

// OpenFileInOS opens the document or folder using the operating system's default viewer.
func OpenFileInOS(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default: // linux, freebsd, etc.
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

// BuildPDF compiles multiple scanned pages into a standard, multi-page PDF document.
func BuildPDF(pages []PageItem, opts BuildOptions) (string, error) {
	if len(pages) == 0 {
		return "", errors.New("no pages provided to compile")
	}

	outDir := opts.OutputDir
	if outDir == "" {
		outDir = GetDefaultScansDir()
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	filename := strings.TrimSpace(opts.Filename)
	if filename == "" {
		filename = fmt.Sprintf("Scan_%s.pdf", time.Now().Format("2006-01-02_15-04-05"))
	}
	if !strings.HasSuffix(strings.ToLower(filename), ".pdf") {
		filename += ".pdf"
	}

	// Collision resolution: ensure we never overwrite an existing file
	baseName := strings.TrimSuffix(filename, filepath.Ext(filename))
	ext := filepath.Ext(filename)
	finalPath := filepath.Join(outDir, filename)
	counter := 1
	for {
		if _, err := os.Stat(finalPath); os.IsNotExist(err) {
			break
		}
		finalPath = filepath.Join(outDir, fmt.Sprintf("%s_%d%s", baseName, counter, ext))
		counter++
	}

	dpi := opts.DPI
	if dpi <= 0 {
		dpi = 150
	}

	quality := opts.JPEGQuality
	if quality <= 0 {
		quality = 88
	}

	colorMode := strings.ToLower(opts.ColorMode)
	forceGray := colorMode == "grayscale" || colorMode == "gray" || colorMode == "mono" || colorMode == "b&w"

	// PDF generator
	var pdfBuf bytes.Buffer
	offsets := []int{0} // 1-indexed object offset table

	// PDF Header
	pdfBuf.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")

	numPages := len(pages)
	// Object 1: Catalog
	// Object 2: Pages Root
	// For each page i:
	//   Page Obj: 3 + (i-1)*3
	//   Image Obj: 3 + (i-1)*3 + 1
	//   Content Obj: 3 + (i-1)*3 + 2

	catalogObjNum := 1
	pagesObjNum := 2

	// Placeholder for Catalog and Pages Root
	offsets = append(offsets, pdfBuf.Len())
	pdfBuf.WriteString(fmt.Sprintf("%d 0 obj\n<< /Type /Catalog /Pages %d 0 R >>\nendobj\n", catalogObjNum, pagesObjNum))

	// Write Pages Root
	var kids []string
	for i := 0; i < numPages; i++ {
		pageObjNum := 3 + i*3
		kids = append(kids, fmt.Sprintf("%d 0 R", pageObjNum))
	}

	offsets = append(offsets, pdfBuf.Len())
	pdfBuf.WriteString(fmt.Sprintf("%d 0 obj\n<< /Type /Pages /Kids [%s] /Count %d >>\nendobj\n",
		pagesObjNum, strings.Join(kids, " "), numPages))

	for i, pageItem := range pages {
		img := RotateImage(pageItem.Image, pageItem.Rotation)
		bounds := img.Bounds()
		wPx := bounds.Dx()
		hPx := bounds.Dy()

		// Calculate page size in PDF points (72 points = 1 inch)
		wPt := float64(wPx) * 72.0 / float64(dpi)
		hPt := float64(hPx) * 72.0 / float64(dpi)

		pageObjNum := 3 + i*3
		imgObjNum := 3 + i*3 + 1
		contentObjNum := 3 + i*3 + 2

		// 1. Write Page Object
		offsets = append(offsets, pdfBuf.Len())
		pdfBuf.WriteString(fmt.Sprintf("%d 0 obj\n<< /Type /Page /Parent %d 0 R /MediaBox [0 0 %.2f %.2f] /Resources << /XObject << /Im%d %d 0 R >> >> /Contents %d 0 R >>\nendobj\n",
			pageObjNum, pagesObjNum, wPt, hPt, i+1, imgObjNum, contentObjNum))

		// 2. Encode image to JPEG
		var isGray bool
		if forceGray {
			isGray = true
		} else {
			_, isGray = img.(*image.Gray)
		}

		var jpegData bytes.Buffer
		if isGray {
			grayImg := toGray(img)
			if err := jpeg.Encode(&jpegData, grayImg, &jpeg.Options{Quality: quality}); err != nil {
				return "", fmt.Errorf("jpeg encoding failed: %w", err)
			}
		} else {
			if err := jpeg.Encode(&jpegData, img, &jpeg.Options{Quality: quality}); err != nil {
				return "", fmt.Errorf("jpeg encoding failed: %w", err)
			}
		}

		colorSpace := "/DeviceRGB"
		if isGray {
			colorSpace = "/DeviceGray"
		}

		// Write Image XObject
		offsets = append(offsets, pdfBuf.Len())
		pdfBuf.WriteString(fmt.Sprintf("%d 0 obj\n<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace %s /BitsPerComponent 8 /Filter /DCTDecode /Length %d >>\nstream\r\n",
			imgObjNum, wPx, hPx, colorSpace, jpegData.Len()))
		pdfBuf.Write(jpegData.Bytes())
		pdfBuf.WriteString("\r\nendstream\nendobj\n")

		// 3. Write Content Stream: scale & place image
		contentStream := fmt.Sprintf("q\n%.2f 0 0 %.2f 0 0 cm\n/Im%d Do\nQ\n", wPt, hPt, i+1)
		offsets = append(offsets, pdfBuf.Len())
		pdfBuf.WriteString(fmt.Sprintf("%d 0 obj\n<< /Length %d >>\nstream\n%sendstream\nendobj\n",
			contentObjNum, len(contentStream), contentStream))
	}

	// Write XRef table
	xrefOffset := pdfBuf.Len()
	totalObjects := len(offsets) // includes 0th
	pdfBuf.WriteString(fmt.Sprintf("xref\n0 %d\n", totalObjects))
	pdfBuf.WriteString("0000000000 65535 f \r\n")
	for i := 1; i < totalObjects; i++ {
		pdfBuf.WriteString(fmt.Sprintf("%010d 00000 n \r\n", offsets[i]))
	}

	// Write Trailer
	pdfBuf.WriteString(fmt.Sprintf("trailer\n<< /Size %d /Root %d 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		totalObjects, catalogObjNum, xrefOffset))

	if err := os.WriteFile(finalPath, pdfBuf.Bytes(), 0644); err != nil {
		return "", fmt.Errorf("failed to write PDF file: %w", err)
	}

	return finalPath, nil
}

// RotateImage rotates an image clockwise by 0, 90, 180, or 270 degrees.
func RotateImage(src image.Image, angle int) image.Image {
	angle = ((angle % 360) + 360) % 360
	if angle == 0 {
		return src
	}

	bounds := src.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	switch angle {
	case 90:
		// Clockwise 90: width and height swap, (x, y) -> (h - 1 - y, x)
		dst := image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.Set(h-1-y, x, src.At(bounds.Min.X+x, bounds.Min.Y+y))
			}
		}
		return dst

	case 180:
		// 180: (x, y) -> (w - 1 - x, h - 1 - y)
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.Set(w-1-x, h-1-y, src.At(bounds.Min.X+x, bounds.Min.Y+y))
			}
		}
		return dst

	case 270:
		// Clockwise 270: (x, y) -> (y, w - 1 - x)
		dst := image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.Set(y, w-1-x, src.At(bounds.Min.X+x, bounds.Min.Y+y))
			}
		}
		return dst

	default:
		return src
	}
}

func toGray(src image.Image) *image.Gray {
	if g, ok := src.(*image.Gray); ok {
		return g
	}
	bounds := src.Bounds()
	gray := image.NewGray(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.GrayModel.Convert(src.At(x, y))
			gray.Set(x, y, c)
		}
	}
	return gray
}
