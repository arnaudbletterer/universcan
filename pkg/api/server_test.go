package api

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"net/http"
	"net/http/httptest"
	"testing"
)

func createSampleImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 150, B: 50, A: 255})
		}
	}
	return img
}

func TestAPIServerEndpoints(t *testing.T) {
	tempDir := t.TempDir()
	srv := NewServer(tempDir, nil)
	handler := srv.Routes()

	// 1. GET /api/status
	req := httptest.NewRequest("GET", "/api/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/status returned status %d", rec.Code)
	}

	var st FullStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatalf("failed to parse status JSON: %v", err)
	}

	// 2. GET /api/settings
	req = httptest.NewRequest("GET", "/api/settings", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/settings returned status %d", rec.Code)
	}

	// 3. POST /api/settings
	updatePayload := bytes.NewBufferString(`{"dpi": 300, "source": "Flatbed"}`)
	req = httptest.NewRequest("POST", "/api/settings", updatePayload)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/settings returned status %d", rec.Code)
	}

	// 4. Populate a page into session
	img := createSampleImage(100, 100)
	page := srv.SessionManager.AddPage(img)

	// 5. GET /api/pages
	req = httptest.NewRequest("GET", "/api/pages", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/pages returned status %d", rec.Code)
	}
	var pagesList struct {
		Pages []struct {
			ID string `json:"id"`
		} `json:"pages"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &pagesList)
	if len(pagesList.Pages) != 1 || pagesList.Pages[0].ID != page.ID {
		t.Fatalf("unexpected pages list: %+v", pagesList)
	}

	// 6. GET /api/pages/{id}/image (thumb and full)
	req = httptest.NewRequest("GET", "/api/pages/"+page.ID+"/image?thumb=1", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET image thumb returned status %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("expected Content-Type image/jpeg, got %s", rec.Header().Get("Content-Type"))
	}

	// 7. POST /api/pages/{id}/rotate
	rotReq := bytes.NewBufferString(`{"angle": 90}`)
	req = httptest.NewRequest("POST", "/api/pages/"+page.ID+"/rotate", rotReq)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST rotate returned status %d", rec.Code)
	}

	// 8. POST /api/export/pdf
	exportReq := bytes.NewBufferString(`{"filename": "ApiTest.pdf", "output_dir": "` + filepathToSlash(tempDir) + `"}`)
	req = httptest.NewRequest("POST", "/api/export/pdf", exportReq)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST export returned status %d: %s", rec.Code, rec.Body.String())
	}

	// 9. DELETE /api/pages/{id}
	req = httptest.NewRequest("DELETE", "/api/pages/"+page.ID, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE page returned status %d", rec.Code)
	}
	if srv.SessionManager.TotalPages() != 0 {
		t.Errorf("expected 0 pages after deletion, got %d", srv.SessionManager.TotalPages())
	}
}

func TestAPIServerReorderAndDuplex(t *testing.T) {
	tempDir := t.TempDir()
	srv := NewServer(tempDir, nil)
	handler := srv.Routes()

	img := createSampleImage(50, 50)
	p1 := srv.SessionManager.AddPage(img)
	p2 := srv.SessionManager.AddPage(img)
	p3 := srv.SessionManager.AddPage(img)
	p4 := srv.SessionManager.AddPage(img)

	// Test 1: Reorder with "order"
	reorderPayload := bytes.NewBufferString(`{"order": ["` + p4.ID + `", "` + p3.ID + `", "` + p2.ID + `", "` + p1.ID + `"]}`)
	req := httptest.NewRequest("POST", "/api/pages/reorder", reorderPayload)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/pages/reorder with order returned status %d: %s", rec.Code, rec.Body.String())
	}
	if srv.SessionManager.ListPages()[0].ID != p4.ID {
		t.Fatalf("expected p4 first after reorder")
	}

	// Test 2: Reorder with "page_ids" (supports client drag-and-drop format)
	reorderPayload2 := bytes.NewBufferString(`{"page_ids": ["` + p1.ID + `", "` + p2.ID + `", "` + p3.ID + `", "` + p4.ID + `"]}`)
	req = httptest.NewRequest("POST", "/api/pages/reorder", reorderPayload2)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/pages/reorder with page_ids returned status %d: %s", rec.Code, rec.Body.String())
	}
	if srv.SessionManager.ListPages()[0].ID != p1.ID {
		t.Fatalf("expected p1 first after reorder")
	}

	// Test 3: Duplex Interleave endpoint
	// Pass 1 = [p1, p3], Pass 2 = [p4, p2] (scanned in reverse sheet order)
	duplexPayload := bytes.NewBufferString(`{"pass1_ids": ["` + p1.ID + `", "` + p3.ID + `"], "pass2_ids": ["` + p4.ID + `", "` + p2.ID + `"]}`)
	req = httptest.NewRequest("POST", "/api/pages/duplex-interleave", duplexPayload)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/pages/duplex-interleave returned status %d: %s", rec.Code, rec.Body.String())
	}

	pages := srv.SessionManager.ListPages()
	if len(pages) != 4 || pages[0].ID != p1.ID || pages[1].ID != p2.ID || pages[2].ID != p3.ID || pages[3].ID != p4.ID {
		t.Fatalf("unexpected page sequence after duplex interleave: %+v", pages)
	}
}

func filepathToSlash(s string) string {
	var b []byte
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			b = append(b, '/')
		} else {
			b = append(b, s[i])
		}
	}
	return string(b)
}
