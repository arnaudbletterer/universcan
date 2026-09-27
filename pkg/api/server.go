package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"universcan/pkg/diagnostic"
	"universcan/pkg/pdf"
	"universcan/pkg/scanner"
	"universcan/pkg/storage"
)

// ScanJobStatus tracks live scan operation status.
type ScanJobStatus struct {
	IsScanning        bool    `json:"is_scanning"`
	Message           string  `json:"message"`
	Percent           int     `json:"percent"`
	Error             *string `json:"error"`
	CurrentPagesCount int     `json:"current_pages_count"`
	LatestPageID      *string `json:"latest_page_id"`
}

// FullStatus includes hardware telemetry and the scan job status.
type FullStatus struct {
	Online        bool          `json:"online"`
	IP            string        `json:"ip"`
	Model         string        `json:"model"`
	DisplayStatus string        `json:"display_status"`
	IsSleeping    bool          `json:"is_sleeping"`
	DocInADF      bool          `json:"doc_in_adf"`
	HasADF        bool          `json:"has_adf"`
	TonerPercent  int           `json:"toner_percent"`
	DrumPercent   int           `json:"drum_percent"`
	MACAddress    string        `json:"mac_address"`
	ScanJob       ScanJobStatus `json:"scan_job"`
}

// Server provides the REST API for UniverScan.
type Server struct {
	SessionManager   *storage.SessionManager
	SettingsDir      string
	UniversalScanner *scanner.UniversalScanner
	Diagnostics      *diagnostic.Collector
	WebFS            fs.FS

	scanJob      ScanJobStatus
	cachedStatus FullStatus
	scanMu       sync.Mutex
	statusMu     sync.RWMutex
	stopMonitor  chan struct{}
}

// NewServer creates a new API server instance.
func NewServer(baseDir string, webFS fs.FS) *Server {
	if baseDir == "" {
		baseDir = storage.GetDefaultSettingsDir()
	}

	settings := storage.LoadSettings(baseDir)
	targetIP := settings.TargetIP
	if targetIP == "" {
		targetIP = "192.168.1.50"
	}

	sm := storage.NewSessionManager(baseDir)
	uScan := scanner.NewUniversalScanner(targetIP)
	diag := diagnostic.NewCollector()

	srv := &Server{
		SessionManager:   sm,
		SettingsDir:      baseDir,
		UniversalScanner: uScan,
		Diagnostics:      diag,
		WebFS:            webFS,
		scanJob: ScanJobStatus{
			IsScanning:        false,
			Message:           "Idle",
			Percent:           0,
			Error:             nil,
			CurrentPagesCount: sm.TotalPages(),
			LatestPageID:      nil,
		},
		cachedStatus: FullStatus{
			Online:        false,
			IP:            targetIP,
			Model:         "Universal Scanner",
			DisplayStatus: "Connecting...",
			IsSleeping:    false,
			DocInADF:      false,
			HasADF:        true,
			TonerPercent:  0,
			DrumPercent:   0,
			ScanJob: ScanJobStatus{
				IsScanning:        false,
				Message:           "Idle",
				Percent:           0,
				CurrentPagesCount: sm.TotalPages(),
			},
		},
		stopMonitor: make(chan struct{}),
	}

	return srv
}

// StartBackgroundMonitor starts periodic non-blocking hardware polling.
func (s *Server) StartBackgroundMonitor(interval time.Duration) {
	if interval <= 0 {
		interval = 3 * time.Second
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-s.stopMonitor:
				return
			case <-ticker.C:
				s.scanMu.Lock()
				scanning := s.scanJob.IsScanning
				s.scanMu.Unlock()

				if !scanning {
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					st, err := s.UniversalScanner.GetStatus(ctx)
					cancel()

					s.statusMu.Lock()
					if err == nil && st != nil {
						s.cachedStatus.Online = st.Online
						s.cachedStatus.IP = st.IP
						if st.Model != "" {
							s.cachedStatus.Model = st.Model
						}
						s.cachedStatus.DisplayStatus = st.DisplayStatus
						s.cachedStatus.IsSleeping = st.IsSleeping
						s.cachedStatus.DocInADF = st.DocInADF
						s.cachedStatus.HasADF = st.HasADF
						if st.TonerPercent > 0 {
							s.cachedStatus.TonerPercent = st.TonerPercent
						}
						s.cachedStatus.MACAddress = st.MACAddress
					} else {
						s.cachedStatus.Online = false
						s.cachedStatus.DisplayStatus = "Scanner offline"
					}
					s.statusMu.Unlock()
				}
			}
		}
	}()
}

// StopBackgroundMonitor stops the background hardware monitor.
func (s *Server) StopBackgroundMonitor() {
	close(s.stopMonitor)
}

// Routes builds and returns the http.Handler router.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// REST APIs
	mux.HandleFunc("GET /api/status", s.handleGetStatus)
	mux.HandleFunc("POST /api/scan", s.handleStartScan)
	mux.HandleFunc("GET /api/pages", s.handleListPages)
	mux.HandleFunc("GET /api/pages/{id}/image", s.handleGetPageImage)
	mux.HandleFunc("POST /api/pages/{id}/rotate", s.handleRotatePagePath)
	mux.HandleFunc("POST /api/pages/rotate", s.handleRotatePageBody)
	mux.HandleFunc("POST /api/pages/reorder", s.handleReorderPages)
	mux.HandleFunc("POST /api/pages/duplex-interleave", s.handleDuplexInterleave)
	mux.HandleFunc("DELETE /api/pages/{id}", s.handleDeletePage)
	mux.HandleFunc("POST /api/pages/clear", s.handleClearPages)
	mux.HandleFunc("POST /api/export/pdf", s.handleExportPDF)
	mux.HandleFunc("GET /api/settings", s.handleGetSettings)
	mux.HandleFunc("POST /api/settings", s.handleUpdateSettings)
	mux.HandleFunc("POST /api/wake", s.handleWakeScanner)
	mux.HandleFunc("POST /api/open-folder", s.handleOpenFolder)
	mux.HandleFunc("GET /api/discover", s.handleDiscoverScanners)
	mux.HandleFunc("GET /api/diagnostics", s.handleGetDiagnostics)
	mux.HandleFunc("POST /api/diagnostics/probe", s.handleProbeDiagnostics)

	// Projects APIs
	mux.HandleFunc("GET /api/projects", s.handleListProjects)
	mux.HandleFunc("POST /api/projects/new", s.handleCreateProject)
	mux.HandleFunc("POST /api/projects/{id}/load", s.handleLoadProject)
	mux.HandleFunc("DELETE /api/projects/{id}", s.handleDeleteProject)
	mux.HandleFunc("PATCH /api/projects/{id}", s.handleRenameProject)

	// Static Web Assets
	if s.WebFS != nil {
		fileServer := http.FileServer(http.FS(s.WebFS))

		// Serve static directory, assets, and index
		mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/")
			if path == "" || path == "index.html" {
				data, err := fs.ReadFile(s.WebFS, "index.html")
				if err == nil {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(data)
					return
				}
			}
			if strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".css") {
				w.Header().Set("Cache-Control", "no-cache, must-revalidate")
			}
			fileServer.ServeHTTP(w, r)
		})
	}

	return corsMiddleware(mux)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg, "detail": msg})
}

// --- Handler Implementations ---

func (s *Server) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	s.statusMu.RLock()
	st := s.cachedStatus
	s.statusMu.RUnlock()

	s.scanMu.Lock()
	st.ScanJob = s.scanJob
	st.ScanJob.CurrentPagesCount = s.SessionManager.TotalPages()
	s.scanMu.Unlock()

	writeJSON(w, http.StatusOK, st)
}

type ScanRequest struct {
	Source string `json:"source"`
	DPI    int    `json:"dpi"`
	IP     string `json:"ip"`
	Color  string `json:"color"`
}

func (s *Server) handleStartScan(w http.ResponseWriter, r *http.Request) {
	s.scanMu.Lock()
	if s.scanJob.IsScanning {
		s.scanMu.Unlock()
		writeError(w, http.StatusConflict, "A scan job is already in progress.")
		return
	}

	var req ScanRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	settings := storage.LoadSettings(s.SettingsDir)
	if req.Source == "" {
		req.Source = settings.Source
		if req.Source == "" {
			req.Source = "Flatbed"
		}
	}
	if req.DPI <= 0 {
		req.DPI = settings.DPI
		if req.DPI <= 0 {
			req.DPI = 150
		}
	}
	if req.IP == "" {
		req.IP = settings.TargetIP
		if req.IP == "" {
			req.IP = "192.168.1.50"
		}
	}
	if req.Color == "" {
		req.Color = settings.ColorMode
		if req.Color == "" {
			req.Color = "color"
		}
	}

	s.UniversalScanner.IP = req.IP
	s.scanJob = ScanJobStatus{
		IsScanning:        true,
		Message:           "Initializing scanner...",
		Percent:           10,
		Error:             nil,
		CurrentPagesCount: s.SessionManager.TotalPages(),
		LatestPageID:      nil,
	}
	s.scanMu.Unlock()

	go s.performScan(req)

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Scan initiated",
	})
}

func (s *Server) performScan(req ScanRequest) {
	progressCb := func(msg string, pct int) {
		s.scanMu.Lock()
		s.scanJob.Message = msg
		s.scanJob.Percent = pct
		s.scanMu.Unlock()
	}

	var scannedIDs []string
	isFlatbed := strings.EqualFold(req.Source, "Flatbed") || strings.EqualFold(req.Source, "Platen")

	defer func() {
		s.scanMu.Lock()
		s.scanJob.IsScanning = false
		s.scanMu.Unlock()
	}()

	ctx := context.Background()

	if isFlatbed {
		progressCb("Scanning from flatbed glass...", 30)
		img, err := s.UniversalScanner.ScanPage(ctx, "Flatbed", req.DPI, req.Color, progressCb)
		if err != nil {
			errStr := err.Error()
			s.scanMu.Lock()
			s.scanJob.Error = &errStr
			s.scanJob.Message = fmt.Sprintf("Error: %s", errStr)
			s.scanMu.Unlock()
			return
		}
		if img != nil {
			p := s.SessionManager.AddPage(img)
			scannedIDs = append(scannedIDs, p.ID)
			s.scanMu.Lock()
			s.scanJob.LatestPageID = &p.ID
			s.scanJob.CurrentPagesCount = s.SessionManager.TotalPages()
			s.scanJob.Message = "Flatbed scan complete!"
			s.scanJob.Percent = 100
			s.scanMu.Unlock()
		}
	} else {
		// ADF Multi-page loop
		pageNum := 1
		for {
			progressCb(fmt.Sprintf("Scanning sheet %d from feeder...", pageNum), min(85, 15+pageNum*5))
			img, err := s.UniversalScanner.ScanPage(ctx, "ADF", req.DPI, req.Color, progressCb)
			if err != nil {
				if errors.Is(err, scanner.ErrNoDocument) || strings.Contains(strings.ToLower(err.Error()), "nodocument") {
					if pageNum == 1 {
						errStr := "No document detected in the Automatic Document Feeder (ADF). If your document is on the flatbed glass, please select 'Flatbed Glass' in the settings."
						s.scanMu.Lock()
						s.scanJob.Error = &errStr
						s.scanJob.Message = errStr
						s.scanMu.Unlock()
						return
					}
					// Finished all sheets
					break
				}
				errStr := err.Error()
				s.scanMu.Lock()
				s.scanJob.Error = &errStr
				s.scanJob.Message = fmt.Sprintf("Scan error: %s", errStr)
				s.scanMu.Unlock()
				return
			}
			if img == nil {
				if pageNum == 1 {
					errStr := "No document detected in the Automatic Document Feeder (ADF). If your document is on the flatbed glass, please select 'Flatbed Glass' in the settings."
					s.scanMu.Lock()
					s.scanJob.Error = &errStr
					s.scanJob.Message = errStr
					s.scanMu.Unlock()
					return
				}
				break
			}

			p := s.SessionManager.AddPage(img)
			scannedIDs = append(scannedIDs, p.ID)

			s.scanMu.Lock()
			s.scanJob.LatestPageID = &p.ID
			s.scanJob.CurrentPagesCount = s.SessionManager.TotalPages()
			s.scanJob.Message = fmt.Sprintf("Page %d captured! Feeding next sheet...", pageNum)
			s.scanJob.Percent = min(90, 20+pageNum*10)
			s.scanMu.Unlock()

			pageNum++
			time.Sleep(300 * time.Millisecond)
		}

		s.scanMu.Lock()
		s.scanJob.Message = fmt.Sprintf("Successfully scanned %d page(s)!", len(scannedIDs))
		s.scanJob.Percent = 100
		s.scanMu.Unlock()
	}
}

func (s *Server) handleListPages(w http.ResponseWriter, r *http.Request) {
	pages := s.SessionManager.ListPages()
	type pageResp struct {
		ID       string `json:"id"`
		Width    int    `json:"width"`
		Height   int    `json:"height"`
		Rotation int    `json:"rotation"`
	}

	var resp []pageResp
	for _, p := range pages {
		resp = append(resp, pageResp{
			ID:       p.ID,
			Width:    p.Width,
			Height:   p.Height,
			Rotation: p.Rotation,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"pages": resp})
}

func (s *Server) handleGetPageImage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	page, ok := s.SessionManager.GetPage(id)
	if !ok || page.Image == nil {
		writeError(w, http.StatusNotFound, "Page not found")
		return
	}

	thumb := r.URL.Query().Get("thumb") == "1" || r.URL.Query().Get("thumb") == "true"
	rotated := pdf.RotateImage(page.Image, page.Rotation)

	var outputImg image.Image = rotated
	if thumb {
		outputImg = createThumbnail(rotated, 400, 600)
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-cache")
	_ = jpeg.Encode(w, outputImg, &jpeg.Options{Quality: 85})
}

func createThumbnail(src image.Image, maxW, maxH int) image.Image {
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	if srcW <= maxW && srcH <= maxH {
		return src
	}

	ratioW := float64(maxW) / float64(srcW)
	ratioH := float64(maxH) / float64(srcH)
	ratio := min(ratioW, ratioH)

	dstW := int(float64(srcW) * ratio)
	dstH := int(float64(srcH) * ratio)
	if dstW <= 0 {
		dstW = 1
	}
	if dstH <= 0 {
		dstH = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	for y := 0; y < dstH; y++ {
		srcY := bounds.Min.Y + int(float64(y)/ratio)
		for x := 0; x < dstW; x++ {
			srcX := bounds.Min.X + int(float64(x)/ratio)
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}

type RotateRequest struct {
	PageID string `json:"page_id"`
	Angle  int    `json:"angle"`
}

func (s *Server) handleRotatePagePath(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req RotateRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	angle := req.Angle
	if angle == 0 {
		angle = 90
	}

	rot, err := s.SessionManager.RotatePage(id, angle)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "rotation": rot})
}

func (s *Server) handleRotatePageBody(w http.ResponseWriter, r *http.Request) {
	var req RotateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PageID == "" {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	angle := req.Angle
	if angle == 0 {
		angle = 90
	}

	rot, err := s.SessionManager.RotatePage(req.PageID, angle)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "rotation": rot})
}

type ReorderRequest struct {
	Order   []string `json:"order"`
	PageIDs []string `json:"page_ids"`
}

func (s *Server) handleReorderPages(w http.ResponseWriter, r *http.Request) {
	var req ReorderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	order := req.Order
	if len(order) == 0 && len(req.PageIDs) > 0 {
		order = req.PageIDs
	}

	if len(order) == 0 {
		writeError(w, http.StatusBadRequest, "order or page_ids is required")
		return
	}

	if !s.SessionManager.ReorderPages(order) {
		writeError(w, http.StatusBadRequest, "reorder failed: mismatch in page IDs or count")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "order": order})
}

type DuplexInterleaveRequest struct {
	BaseIDs  []string `json:"base_ids"`
	Pass1IDs []string `json:"pass1_ids"`
	Pass2IDs []string `json:"pass2_ids"`
}

func (s *Server) handleDuplexInterleave(w http.ResponseWriter, r *http.Request) {
	var req DuplexInterleaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	order, ok := s.SessionManager.InterleaveDuplexPages(req.BaseIDs, req.Pass1IDs, req.Pass2IDs)
	if !ok {
		writeError(w, http.StatusBadRequest, "duplex interleave failed: mismatch in page IDs or count")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"order":   order,
	})
}

func (s *Server) handleDeletePage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ok := s.SessionManager.DeletePage(id)
	if !ok {
		writeError(w, http.StatusNotFound, "page not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "total_pages": s.SessionManager.TotalPages()})
}

func (s *Server) handleClearPages(w http.ResponseWriter, r *http.Request) {
	s.SessionManager.Clear()
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

type ExportRequest struct {
	Filename   string `json:"filename"`
	OpenAfter  *bool  `json:"open_after"`
	DPI        int    `json:"dpi"`
	ColorMode  string `json:"color_mode"`
	OutputDir  string `json:"output_dir"`
}

func (s *Server) handleExportPDF(w http.ResponseWriter, r *http.Request) {
	var req ExportRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	settings := storage.LoadSettings(s.SettingsDir)
	outDir := req.OutputDir
	if outDir == "" {
		outDir = settings.OutputDir
	}

	colorMode := req.ColorMode
	if colorMode == "" {
		colorMode = settings.ColorMode
	}

	dpi := req.DPI
	if dpi <= 0 {
		dpi = settings.DPI
	}

	openAfter := settings.OpenAfter
	if req.OpenAfter != nil {
		openAfter = *req.OpenAfter
	}

	pages := s.SessionManager.ListPages()
	if len(pages) == 0 {
		writeError(w, http.StatusBadRequest, "No pages to export.")
		return
	}

	var pageItems []pdf.PageItem
	for _, p := range pages {
		if p.Image != nil {
			pageItems = append(pageItems, pdf.PageItem{
				Image:    p.Image,
				Rotation: p.Rotation,
			})
		}
	}

	pdfPath, err := pdf.BuildPDF(pageItems, pdf.BuildOptions{
		Filename:  req.Filename,
		OutputDir: outDir,
		DPI:       dpi,
		ColorMode: colorMode,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if openAfter {
		_ = pdf.OpenFileInOS(pdfPath)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":     true,
		"path":        pdfPath,
		"filename":    filepath.Base(pdfPath),
		"pages_count": len(pageItems),
		"output_dir":  outDir,
	})
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	st := storage.LoadSettings(s.SettingsDir)
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	current := storage.LoadSettings(s.SettingsDir)

	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if v, ok := req["output_dir"].(string); ok && v != "" {
		current.OutputDir = v
	}
	if v, ok := req["color_mode"].(string); ok && v != "" {
		current.ColorMode = v
	}
	if v, ok := req["dpi"].(float64); ok && v > 0 {
		current.DPI = int(v)
	}
	if v, ok := req["source"].(string); ok && v != "" {
		current.Source = v
	}
	if v, ok := req["sides"].(string); ok && v != "" {
		current.Sides = v
	}
	if v, ok := req["open_after"].(bool); ok {
		current.OpenAfter = v
	}
	if v, ok := req["naming_prefix"].(string); ok && v != "" {
		current.NamingPrefix = v
	}
	if v, ok := req["target_ip"].(string); ok && v != "" {
		current.TargetIP = v
		s.UniversalScanner.IP = v
	}

	saved, err := storage.SaveSettings(current, s.SettingsDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "settings": saved})
}

func (s *Server) handleWakeScanner(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	success, _ := s.UniversalScanner.Wake(ctx, 15*time.Second, nil)
	writeJSON(w, http.StatusOK, map[string]any{"success": success})
}

func (s *Server) handleOpenFolder(w http.ResponseWriter, r *http.Request) {
	settings := storage.LoadSettings(s.SettingsDir)
	outDir := settings.OutputDir
	if outDir == "" {
		outDir = pdf.GetDefaultScansDir()
	}

	_ = pdf.OpenFileInOS(outDir)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "path": outDir})
}

func (s *Server) handleDiscoverScanners(w http.ResponseWriter, r *http.Request) {
	settings := storage.LoadSettings(s.SettingsDir)
	devices := scanner.DiscoverScanners(r.Context(), settings.TargetIP)
	writeJSON(w, http.StatusOK, map[string]any{"devices": devices})
}

func (s *Server) handleGetDiagnostics(w http.ResponseWriter, r *http.Request) {
	rep := s.Diagnostics.GetLastReport()
	if rep == nil {
		settings := storage.LoadSettings(s.SettingsDir)
		var err error
		rep, err = s.Diagnostics.GenerateReport(r.Context(), settings.TargetIP)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, rep)
}

func (s *Server) handleProbeDiagnostics(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IP string `json:"ip"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.IP == "" {
		settings := storage.LoadSettings(s.SettingsDir)
		req.IP = settings.TargetIP
	}

	rep, err := s.Diagnostics.GenerateReport(r.Context(), req.IP)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

// --- Project Handlers ---

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	projects, activeID := s.SessionManager.ListProjects()
	var activeProj any
	for _, p := range projects {
		if p.ID == activeID {
			activeProj = p
			break
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"projects":       projects,
		"active_id":      activeID,
		"active_project": activeProj,
	})
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	p := s.SessionManager.CreateProject(req.Name)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "project": p})
}

func (s *Server) handleLoadProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := s.SessionManager.LoadProject(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":     true,
		"project":     p,
		"pages_count": s.SessionManager.TotalPages(),
	})
}

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.SessionManager.DeleteProject(id)
	_, activeID := s.SessionManager.ListProjects()

	writeJSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"active_id": activeID,
	})
}

func (s *Server) handleRenameProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	p, err := s.SessionManager.RenameProject(id, req.Name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "project": p})
}

// Ensure color import is used
var _ = color.RGBA{}
