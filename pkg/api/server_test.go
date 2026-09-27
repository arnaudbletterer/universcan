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
