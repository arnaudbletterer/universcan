package storage

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ScannedPage represents a single scanned page in session memory.
type ScannedPage struct {
	ID        string      `json:"id"`
	Rotation  int         `json:"rotation"`
	Width     int         `json:"width"`
	Height    int         `json:"height"`
	CreatedAt time.Time   `json:"created_at"`
	File      string      `json:"file"`
	Image     image.Image `json:"-"`
}

// Project represents a saved scan project.
type Project struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	Pages     []*ScannedPage `json:"pages"`
}

// SessionManager manages active scanned pages and multi-project persistence.
type SessionManager struct {
	BaseDir      string
	sessionDir   string
	projectsDir  string
	pages        map[string]*ScannedPage
	order        []string
	activeProjID string
	mu           sync.RWMutex
}

// NewSessionManager creates a SessionManager rooted in baseDir (defaults to ~/.universcan).
func NewSessionManager(baseDir string) *SessionManager {
	if baseDir == "" {
		baseDir = GetDefaultSettingsDir()
	}

	sm := &SessionManager{
		BaseDir:     baseDir,
		sessionDir:  filepath.Join(baseDir, "session"),
		projectsDir: filepath.Join(baseDir, "projects"),
		pages:       make(map[string]*ScannedPage),
		order:       make([]string, 0),
	}

	_ = os.MkdirAll(sm.sessionDir, 0755)
	_ = os.MkdirAll(sm.projectsDir, 0755)

	sm.LoadActiveProject()
	return sm
}

// GenerateID produces an 8-character random hexadecimal string.
func GenerateID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// AddPage stores a scanned image in the active session and saves to disk.
func (sm *SessionManager) AddPage(img image.Image) *ScannedPage {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	id := GenerateID()
	bounds := img.Bounds()
	page := &ScannedPage{
		ID:        id,
		Rotation:  0,
		Width:     bounds.Dx(),
		Height:    bounds.Dy(),
		CreatedAt: time.Now().UTC(),
		File:      fmt.Sprintf("%s.png", id),
		Image:     img,
	}

	sm.pages[id] = page
	sm.order = append(sm.order, id)

	sm.saveToDiskLocked()
	return page
}

// GetPage returns a page by ID.
func (sm *SessionManager) GetPage(id string) (*ScannedPage, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	p, ok := sm.pages[id]
	return p, ok
}

// ListPages returns all active pages in their current ordered sequence.
func (sm *SessionManager) ListPages() []*ScannedPage {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var result []*ScannedPage
	for _, id := range sm.order {
		if p, ok := sm.pages[id]; ok {
			result = append(result, p)
		}
	}
	return result
}

// RotatePage rotates a page by deltaAngle (+/- 90, 180, etc.).
func (sm *SessionManager) RotatePage(id string, deltaAngle int) (int, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	p, ok := sm.pages[id]
	if !ok {
		return 0, errors.New("page not found")
	}

	p.Rotation = ((p.Rotation+deltaAngle)%360 + 360) % 360
	sm.saveToDiskLocked()
	return p.Rotation, nil
}

// DeletePage removes a page from active session and disk.
func (sm *SessionManager) DeletePage(id string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, ok := sm.pages[id]; !ok {
		return false
	}

	delete(sm.pages, id)
	var newOrder []string
	for _, pid := range sm.order {
		if pid != id {
			newOrder = append(newOrder, pid)
		}
	}
	sm.order = newOrder

	// Remove image file from session directory
	_ = os.Remove(filepath.Join(sm.sessionDir, fmt.Sprintf("%s.png", id)))
	if sm.activeProjID != "" {
		_ = os.Remove(filepath.Join(sm.projectsDir, sm.activeProjID, fmt.Sprintf("%s.png", id)))
	}

	sm.saveToDiskLocked()
	return true
}

// ReorderPages re-sequences the pages based on the provided IDs.
func (sm *SessionManager) ReorderPages(newOrder []string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var valid []string
	for _, id := range newOrder {
		if _, ok := sm.pages[id]; ok {
			valid = append(valid, id)
		}
	}

	if len(valid) == len(sm.order) {
		sm.order = valid
		sm.saveToDiskLocked()
		return true
	}
	return false
}

// InterleaveDuplex calculates the correct logical page order for 2-sided (duplex) scanning.
// Pass 1 scanned sheet fronts (F1, F2, ... Fn).
// Pass 2 scanned sheet backs from the exit tray turned into the feeder tray (Rn, Rn-1, ... R1).
// Pass 2 pages are reversed to restore sheet order (R1, ... Rn), then interleaved with Pass 1.
func InterleaveDuplex(baseIDs, pass1IDs, pass2IDs []string) []string {
	reversedPass2 := make([]string, len(pass2IDs))
	for i, id := range pass2IDs {
		reversedPass2[len(pass2IDs)-1-i] = id
	}

	maxLen := len(pass1IDs)
	if len(reversedPass2) > maxLen {
		maxLen = len(reversedPass2)
	}

	interleaved := make([]string, 0, len(pass1IDs)+len(reversedPass2))
	for i := 0; i < maxLen; i++ {
		if i < len(pass1IDs) {
			interleaved = append(interleaved, pass1IDs[i])
		}
		if i < len(reversedPass2) {
			interleaved = append(interleaved, reversedPass2[i])
		}
	}

	result := make([]string, 0, len(baseIDs)+len(interleaved))
	result = append(result, baseIDs...)
	result = append(result, interleaved...)
	return result
}

// InterleaveDuplexPages calculates the duplex order and applies it to the active session.
func (sm *SessionManager) InterleaveDuplexPages(baseIDs, pass1IDs, pass2IDs []string) ([]string, bool) {
	newOrder := InterleaveDuplex(baseIDs, pass1IDs, pass2IDs)
	ok := sm.ReorderPages(newOrder)
	return newOrder, ok
}

// Clear removes all pages from the active session.
func (sm *SessionManager) Clear() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for id := range sm.pages {
		_ = os.Remove(filepath.Join(sm.sessionDir, fmt.Sprintf("%s.png", id)))
	}
	sm.pages = make(map[string]*ScannedPage)
	sm.order = make([]string, 0)
	sm.saveToDiskLocked()
}

// TotalPages returns count of pages in active session.
func (sm *SessionManager) TotalPages() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.order)
}

// --- Persistence & Projects ---

func (sm *SessionManager) saveToDiskLocked() {
	// 1. Save session directory
	manifestPath := filepath.Join(sm.sessionDir, "session.json")
	var manifestList []*ScannedPage
	for _, id := range sm.order {
		p, ok := sm.pages[id]
		if !ok {
			continue
		}
		manifestList = append(manifestList, p)

		// Persist image PNG if not already written
		pngPath := filepath.Join(sm.sessionDir, p.File)
		if _, err := os.Stat(pngPath); os.IsNotExist(err) && p.Image != nil {
			f, err := os.Create(pngPath)
			if err == nil {
				_ = png.Encode(f, p.Image)
				_ = f.Close()
			}
		}
	}

	data, err := json.MarshalIndent(manifestList, "", "  ")
	if err == nil {
		_ = os.WriteFile(manifestPath, data, 0644)
	}

	// 2. Also persist to active project if one is open
	if sm.activeProjID != "" {
		projDir := filepath.Join(sm.projectsDir, sm.activeProjID)
		_ = os.MkdirAll(projDir, 0755)

		for _, p := range manifestList {
			projPng := filepath.Join(projDir, p.File)
			if _, err := os.Stat(projPng); os.IsNotExist(err) && p.Image != nil {
				f, err := os.Create(projPng)
				if err == nil {
					_ = png.Encode(f, p.Image)
					_ = f.Close()
				}
			}
		}

		projFile := filepath.Join(projDir, "project.json")
		var curProj Project
		raw, _ := os.ReadFile(projFile)
		_ = json.Unmarshal(raw, &curProj)
		if curProj.ID == "" {
			curProj.ID = sm.activeProjID
			curProj.Name = "Current Document"
			curProj.CreatedAt = time.Now().UTC()
		}
		curProj.UpdatedAt = time.Now().UTC()
		curProj.Pages = manifestList

		pData, _ := json.MarshalIndent(curProj, "", "  ")
		_ = os.WriteFile(projFile, pData, 0644)
	}
}

// LoadActiveProject loads the active project or legacy session into memory.
func (sm *SessionManager) LoadActiveProject() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	activeFile := filepath.Join(sm.BaseDir, "active_project.txt")
	if data, err := os.ReadFile(activeFile); err == nil {
		pID := string(data)
		if pID != "" {
			projPath := filepath.Join(sm.projectsDir, pID, "project.json")
			if pData, err := os.ReadFile(projPath); err == nil {
				var proj Project
				if json.Unmarshal(pData, &proj) == nil {
					sm.activeProjID = proj.ID
					sm.loadPagesFromSliceLocked(filepath.Join(sm.projectsDir, pID), proj.Pages)
					return
				}
			}
		}
	}

	// Fallback to session.json
	sessFile := filepath.Join(sm.sessionDir, "session.json")
	if sData, err := os.ReadFile(sessFile); err == nil {
		var pages []*ScannedPage
		if json.Unmarshal(sData, &pages) == nil {
			sm.loadPagesFromSliceLocked(sm.sessionDir, pages)
		}
	}
}

func (sm *SessionManager) loadPagesFromSliceLocked(dir string, pages []*ScannedPage) {
	sm.pages = make(map[string]*ScannedPage)
	sm.order = make([]string, 0)

	for _, p := range pages {
		imgPath := filepath.Join(dir, p.File)
		f, err := os.Open(imgPath)
		if err == nil {
			img, err := png.Decode(f)
			_ = f.Close()
			if err == nil {
				p.Image = img
				sm.pages[p.ID] = p
				sm.order = append(sm.order, p.ID)
			}
		}
	}
}

// ListProjects returns all persistent projects found on disk.
func (sm *SessionManager) ListProjects() ([]Project, string) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var projects []Project
	entries, err := os.ReadDir(sm.projectsDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				pFile := filepath.Join(sm.projectsDir, e.Name(), "project.json")
				data, err := os.ReadFile(pFile)
				if err == nil {
					var p Project
					if json.Unmarshal(data, &p) == nil {
						projects = append(projects, p)
					}
				}
			}
		}
	}
	return projects, sm.activeProjID
}

// CreateProject initializes a brand new project and sets it active.
func (sm *SessionManager) CreateProject(name string) *Project {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if name == "" {
		name = fmt.Sprintf("Scan Project %s", time.Now().Format("2006-01-02 15:04"))
	}

	id := fmt.Sprintf("proj_%s_%s", time.Now().Format("20060102_150405"), GenerateID())
	projDir := filepath.Join(sm.projectsDir, id)
	_ = os.MkdirAll(projDir, 0755)

	proj := &Project{
		ID:        id,
		Name:      name,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Pages:     []*ScannedPage{},
	}

	pData, _ := json.MarshalIndent(proj, "", "  ")
	_ = os.WriteFile(filepath.Join(projDir, "project.json"), pData, 0644)

	// Set as active project
	sm.activeProjID = id
	_ = os.WriteFile(filepath.Join(sm.BaseDir, "active_project.txt"), []byte(id), 0644)

	// Reset in-memory session for new project
	sm.pages = make(map[string]*ScannedPage)
	sm.order = make([]string, 0)
	_ = os.Remove(filepath.Join(sm.sessionDir, "session.json"))

	return proj
}

// LoadProject activates an existing project and loads its pages.
func (sm *SessionManager) LoadProject(id string) (*Project, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	projDir := filepath.Join(sm.projectsDir, id)
	data, err := os.ReadFile(filepath.Join(projDir, "project.json"))
	if err != nil {
		return nil, errors.New("project not found")
	}

	var proj Project
	if err := json.Unmarshal(data, &proj); err != nil {
		return nil, err
	}

	sm.activeProjID = id
	_ = os.WriteFile(filepath.Join(sm.BaseDir, "active_project.txt"), []byte(id), 0644)

	sm.loadPagesFromSliceLocked(projDir, proj.Pages)
	return &proj, nil
}

// DeleteProject deletes a project from disk.
func (sm *SessionManager) DeleteProject(id string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	projDir := filepath.Join(sm.projectsDir, id)
	_ = os.RemoveAll(projDir)

	if sm.activeProjID == id {
		sm.activeProjID = ""
		_ = os.Remove(filepath.Join(sm.BaseDir, "active_project.txt"))
		sm.pages = make(map[string]*ScannedPage)
		sm.order = make([]string, 0)
	}
	return true
}

// RenameProject renames a project.
func (sm *SessionManager) RenameProject(id, newName string) (*Project, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	projFile := filepath.Join(sm.projectsDir, id, "project.json")
	data, err := os.ReadFile(projFile)
	if err != nil {
		return nil, errors.New("project not found")
	}

	var proj Project
	if err := json.Unmarshal(data, &proj); err != nil {
		return nil, err
	}

	proj.Name = newName
	proj.UpdatedAt = time.Now().UTC()

	pData, _ := json.MarshalIndent(proj, "", "  ")
	_ = os.WriteFile(projFile, pData, 0644)
	return &proj, nil
}
