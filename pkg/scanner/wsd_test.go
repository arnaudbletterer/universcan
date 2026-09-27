package scanner

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func TestWSDClientMockServer(t *testing.T) {
	// Generate valid test JPEG
	testImg := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for x := 0; x < 10; x++ {
		for y := 0; y < 10; y++ {
			testImg.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var jpegBuf bytes.Buffer
	if err := jpeg.Encode(&jpegBuf, testImg, nil); err != nil {
		t.Fatalf("failed to encode test jpeg: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
		if r.Header.Get("Action") != "" || r.URL.Path == "/wsd/scan" {
			bodyBuf := new(bytes.Buffer)
			_, _ = bodyBuf.ReadFrom(r.Body)
			bStr := bodyBuf.String()

			if bytes.Contains([]byte(bStr), []byte("CreateScanJobRequest")) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"
               xmlns:sca="http://schemas.microsoft.com/windows/2006/08/wdp/scan">
  <soap:Body>
    <sca:CreateScanJobResponse>
      <sca:JobId>999</sca:JobId>
      <sca:JobToken>TESTTOKEN123</sca:JobToken>
    </sca:CreateScanJobResponse>
  </soap:Body>
</soap:Envelope>`))
				return
			}

			if bytes.Contains([]byte(bStr), []byte("RetrieveImageRequest")) {
				w.WriteHeader(http.StatusOK)
				// Return boundary or envelope containing the raw JPEG bytes
				prefix := []byte("--uuid:mime-boundary\r\nContent-Type: image/jpeg\r\n\r\n")
				suffix := []byte("\r\n--uuid:mime-boundary--\r\n")
				var fullResp []byte
				fullResp = append(fullResp, prefix...)
				fullResp = append(fullResp, jpegBuf.Bytes()...)
				fullResp = append(fullResp, suffix...)
				_, _ = w.Write(fullResp)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	u, _ := url.Parse(server.URL)
	host, portStr, _ := netSplit(u.Host)
	port, _ := strconv.Atoi(portStr)

	client := NewWSDClient(host, port)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	img, err := client.ScanPage(ctx, "Flatbed", 150, "RGB24", nil)
	if err != nil {
		t.Fatalf("WSD ScanPage failed: %v", err)
	}
	if img == nil {
		t.Fatal("scanned img is nil")
	}
	if img.Bounds().Dx() != 10 || img.Bounds().Dy() != 10 {
		t.Errorf("expected 10x10, got %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func netSplit(hostPort string) (string, string, error) {
	for i := len(hostPort) - 1; i >= 0; i-- {
		if hostPort[i] == ':' {
			return hostPort[:i], hostPort[i+1:], nil
		}
	}
	return hostPort, "80", nil
}
