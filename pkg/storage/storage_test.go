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

func TestInterleaveDuplex(t *testing.T) {
	// Case 1: 2 sheets (4 pages)
	res1 := InterleaveDuplex(nil, []string{"P1", "P3"}, []string{"P4", "P2"})
	expected1 := []string{"P1", "P2", "P3", "P4"}
	if !slicesEqual(res1, expected1) {
		t.Errorf("case 1 failed: got %v, expected %v", res1, expected1)
	}

	// Case 2: 3 sheets (6 pages)
	res2 := InterleaveDuplex(nil, []string{"P1", "P3", "P5"}, []string{"P6", "P4", "P2"})
	expected2 := []string{"P1", "P2", "P3", "P4", "P5", "P6"}
	if !slicesEqual(res2, expected2) {
		t.Errorf("case 2 failed: got %v, expected %v", res2, expected2)
	}

	// Case 3: Pre-existing base pages
	res3 := InterleaveDuplex([]string{"baseA", "baseB"}, []string{"P1", "P3"}, []string{"P4", "P2"})
	expected3 := []string{"baseA", "baseB", "P1", "P2", "P3", "P4"}
	if !slicesEqual(res3, expected3) {
		t.Errorf("case 3 failed: got %v, expected %v", res3, expected3)
	}

	// Case 4: Unequal pages (3 fronts, 2 backs)
	res4 := InterleaveDuplex(nil, []string{"P1", "P3", "P5"}, []string{"P4", "P2"})
	expected4 := []string{"P1", "P2", "P3", "P4", "P5"}
	if !slicesEqual(res4, expected4) {
		t.Errorf("case 4 failed: got %v, expected %v", res4, expected4)
	}

	// Case 5: Empty pass 2
	res5 := InterleaveDuplex(nil, []string{"P1", "P3"}, nil)
	expected5 := []string{"P1", "P3"}
	if !slicesEqual(res5, expected5) {
		t.Errorf("case 5 failed: got %v, expected %v", res5, expected5)
	}
}

func TestSessionManagerInterleaveDuplex(t *testing.T) {
	tempDir := t.TempDir()
	sm := NewSessionManager(tempDir)

	p1 := sm.AddPage(createTestImage(20, 20))
	p3 := sm.AddPage(createTestImage(20, 20))
	p4 := sm.AddPage(createTestImage(20, 20))
	p2 := sm.AddPage(createTestImage(20, 20))

	order, ok := sm.InterleaveDuplexPages(nil, []string{p1.ID, p3.ID}, []string{p4.ID, p2.ID})
	if !ok {
		t.Fatalf("interleave failed")
	}
	expected := []string{p1.ID, p2.ID, p3.ID, p4.ID}
	if !slicesEqual(order, expected) {
		t.Errorf("interleave order mismatch: got %v, expected %v", order, expected)
	}

	pages := sm.ListPages()
	if len(pages) != 4 || pages[0].ID != p1.ID || pages[1].ID != p2.ID || pages[2].ID != p3.ID || pages[3].ID != p4.ID {
		t.Errorf("session list pages mismatch after duplex interleave: %+v", pages)
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
