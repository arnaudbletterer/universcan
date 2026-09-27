package storage

import (
	"image"
	"image/color"
	"testing"
)

func createTestImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 100, A: 255})
		}
	}
	return img
}

func TestSettingsLoadAndSave(t *testing.T) {
	tempDir := t.TempDir()

	// Initial load should give defaults
	s := LoadSettings(tempDir)
	if s.DPI != 150 {
		t.Errorf("expected default DPI 150, got %d", s.DPI)
	}
	if s.Source != "Flatbed" {
		t.Errorf("expected default Source Flatbed, got %s", s.Source)
	}

	// Update settings
	s.DPI = 300
	s.Source = "ADF"
	s.ColorMode = "grayscale"
	saved, err := SaveSettings(s, tempDir)
	if err != nil {
		t.Fatalf("SaveSettings failed: %v", err)
	}
	if saved.DPI != 300 || saved.Source != "ADF" {
		t.Errorf("saved settings mismatch: %+v", saved)
	}

	// Reload from disk
	reloaded := LoadSettings(tempDir)
	if reloaded.DPI != 300 || reloaded.Source != "ADF" || reloaded.ColorMode != "grayscale" {
		t.Errorf("reloaded settings mismatch: %+v", reloaded)
	}
}

func TestSessionManagerLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	sm := NewSessionManager(tempDir)

	// Add 2 pages
	img1 := createTestImage(50, 50)
	img2 := createTestImage(60, 60)

	p1 := sm.AddPage(img1)
	p2 := sm.AddPage(img2)

	if sm.TotalPages() != 2 {
		t.Fatalf("expected 2 pages, got %d", sm.TotalPages())
	}

	// Rotate page 1
	rot, err := sm.RotatePage(p1.ID, 90)
	if err != nil || rot != 90 {
		t.Errorf("rotate page failed: rot=%d, err=%v", rot, err)
	}

	// Reorder
	sm.ReorderPages([]string{p2.ID, p1.ID})
	list := sm.ListPages()
	if len(list) != 2 || list[0].ID != p2.ID || list[1].ID != p1.ID {
		t.Errorf("reorder failed: %+v", list)
	}

	// Delete page 2
	if !sm.DeletePage(p2.ID) {
		t.Error("delete page failed")
	}
	if sm.TotalPages() != 1 {
		t.Errorf("expected 1 page after deletion, got %d", sm.TotalPages())
	}

	// Verify persistence: create new manager on same directory
	smReloaded := NewSessionManager(tempDir)
	reloadedList := smReloaded.ListPages()
	if len(reloadedList) != 1 || reloadedList[0].ID != p1.ID {
		t.Errorf("persistence reload mismatch: %+v", reloadedList)
	}
	if reloadedList[0].Rotation != 90 {
		t.Errorf("persisted rotation mismatch: %d", reloadedList[0].Rotation)
	}
}

func TestSessionManagerProjects(t *testing.T) {
	tempDir := t.TempDir()
	sm := NewSessionManager(tempDir)

	proj1 := sm.CreateProject("Project One")
	if proj1.Name != "Project One" {
		t.Errorf("project name mismatch: %s", proj1.Name)
	}

	img := createTestImage(40, 40)
	sm.AddPage(img)
	if sm.TotalPages() != 1 {
		t.Errorf("expected 1 page in project, got %d", sm.TotalPages())
	}

	// Rename project
	renamed, err := sm.RenameProject(proj1.ID, "Renamed One")
	if err != nil || renamed.Name != "Renamed One" {
		t.Errorf("rename failed: %+v, err=%v", renamed, err)
	}

	// Create project 2
	_ = sm.CreateProject("Project Two")
	if sm.TotalPages() != 0 {
		t.Errorf("expected 0 pages in new project, got %d", sm.TotalPages())
	}

	// Load project 1 back
	loaded, err := sm.LoadProject(proj1.ID)
	if err != nil || loaded.Name != "Renamed One" {
		t.Errorf("load project failed: %+v, err=%v", loaded, err)
	}
	if sm.TotalPages() != 1 {
		t.Errorf("expected 1 page after reloading project 1, got %d", sm.TotalPages())
	}
}
