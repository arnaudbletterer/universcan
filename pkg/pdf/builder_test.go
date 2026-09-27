package pdf

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func createTestImage(w, h int, c color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func TestBuildPDFValidStructure(t *testing.T) {
	tempDir := t.TempDir()

	img1 := createTestImage(300, 400, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	img2 := createTestImage(300, 400, color.RGBA{R: 0, G: 255, B: 0, A: 255})

	pages := []PageItem{
		{Image: img1, Rotation: 0},
		{Image: img2, Rotation: 90},
	}

	opts := BuildOptions{
		Filename:  "TestDoc.pdf",
		OutputDir: tempDir,
		DPI:       150,
		ColorMode: "color",
	}

	path, err := BuildPDF(pages, opts)
	if err != nil {
		t.Fatalf("BuildPDF failed: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("generated PDF does not exist at %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read generated PDF: %v", err)
	}

	// Verify standard PDF header
	if !bytes.HasPrefix(data, []byte("%PDF-1.4")) {
		t.Errorf("PDF does not start with standard header %%PDF-1.4")
	}

	// Verify xref and trailer
	if !bytes.Contains(data, []byte("xref")) {
		t.Errorf("PDF missing xref table")
	}
	if !bytes.Contains(data, []byte("/Type /Catalog")) {
		t.Errorf("PDF missing Catalog")
	}
	if !bytes.Contains(data, []byte("/Count 2")) {
		t.Errorf("PDF missing page count 2")
	}
	if !bytes.Contains(data, []byte("%%EOF")) {
		t.Errorf("PDF missing %%EOF marker")
	}
}

func TestCollisionFreeNaming(t *testing.T) {
	tempDir := t.TempDir()
	img := createTestImage(100, 100, color.White)
	pages := []PageItem{{Image: img, Rotation: 0}}

	opts := BuildOptions{
		Filename:  "MyScan.pdf",
		OutputDir: tempDir,
		DPI:       150,
	}

	path1, err := BuildPDF(pages, opts)
	if err != nil {
		t.Fatalf("first build failed: %v", err)
	}
	if filepath.Base(path1) != "MyScan.pdf" {
		t.Errorf("expected MyScan.pdf, got %s", filepath.Base(path1))
	}

	path2, err := BuildPDF(pages, opts)
	if err != nil {
		t.Fatalf("second build failed: %v", err)
	}
	if filepath.Base(path2) != "MyScan_1.pdf" {
		t.Errorf("expected MyScan_1.pdf, got %s", filepath.Base(path2))
	}

	path3, err := BuildPDF(pages, opts)
	if err != nil {
		t.Fatalf("third build failed: %v", err)
	}
	if filepath.Base(path3) != "MyScan_2.pdf" {
		t.Errorf("expected MyScan_2.pdf, got %s", filepath.Base(path3))
	}
}

func TestRotateImage(t *testing.T) {
	// Create an asymmetric image: 10 wide, 20 high
	img := createTestImage(10, 20, color.Black)

	r0 := RotateImage(img, 0)
	if r0.Bounds().Dx() != 10 || r0.Bounds().Dy() != 20 {
		t.Errorf("rotation 0 size mismatch: %v", r0.Bounds())
	}

	r90 := RotateImage(img, 90)
	if r90.Bounds().Dx() != 20 || r90.Bounds().Dy() != 10 {
		t.Errorf("rotation 90 size mismatch: %v", r90.Bounds())
	}

	r180 := RotateImage(img, 180)
	if r180.Bounds().Dx() != 10 || r180.Bounds().Dy() != 20 {
		t.Errorf("rotation 180 size mismatch: %v", r180.Bounds())
	}

	r270 := RotateImage(img, 270)
	if r270.Bounds().Dx() != 20 || r270.Bounds().Dy() != 10 {
		t.Errorf("rotation 270 size mismatch: %v", r270.Bounds())
	}
}
