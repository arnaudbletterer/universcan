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

func TestESCLMockServer(t *testing.T) {
	// Generate sample test jpeg
	testImg := image.NewRGBA(image.Rect(0, 0, 20, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			testImg.Set(x, y, color.RGBA{R: 50, G: 150, B: 250, A: 255})
		}
	}
	var jpegBuf bytes.Buffer
	if err := jpeg.Encode(&jpegBuf, testImg, nil); err != nil {
		t.Fatalf("failed to encode test jpeg: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/eSCL/ScannerCapabilities":
			w.Header().Set("Content-Type", "text/xml")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<scan:ScannerCapabilities xmlns:scan="http://schemas.hp.com/imaging/escl/2011/05/03">
  <scan:MakeAndModel>HP LaserJet Pro MFP</scan:MakeAndModel>
  <scan:Manufacturer>HP</scan:Manufacturer>
  <scan:Platen>
    <scan:PlatenInputCaps/>
  </scan:Platen>
  <scan:Adf>
    <scan:AdfInputCaps/>
  </scan:Adf>
</scan:ScannerCapabilities>`))

		case "/eSCL/ScannerStatus":
			w.Header().Set("Content-Type", "text/xml")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<scan:ScannerStatus xmlns:scan="http://schemas.hp.com/imaging/escl/2011/05/03">
  <scan:State>Idle</scan:State>
  <scan:AdfState>AdfLoaded</scan:AdfState>
</scan:ScannerStatus>`))

		case "/eSCL/ScanJobs":
			if r.Method == http.MethodPost {
				w.Header().Set("Location", "/eSCL/ScanJobs/job456/NextDocument")
				w.WriteHeader(http.StatusCreated)
				return
			}
			w.WriteHeader(http.StatusMethodNotAllowed)

		case "/eSCL/ScanJobs/job456/NextDocument":
			w.Header().Set("Content-Type", "image/jpeg")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(jpegBuf.Bytes())

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	u, _ := url.Parse(server.URL)
	host, portStr, _ := netSplit(u.Host)
	port, _ := strconv.Atoi(portStr)

	client := NewESCLClient(host, port, false)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 1. Test Capabilities
	caps, err := client.GetCapabilities(ctx)
	if err != nil {
		t.Fatalf("GetCapabilities failed: %v", err)
	}
	if caps.Manufacturer != "HP" || !caps.HasPlaten || !caps.HasADF {
		t.Errorf("unexpected capabilities: %+v", caps)
	}

	// 2. Test Status
	st, err := client.GetStatus(ctx)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if st.State != "Idle" || st.AdfState != "AdfLoaded" {
		t.Errorf("unexpected status: %+v", st)
	}

	// 3. Test ScanPage
	img, err := client.ScanPage(ctx, "ADF", 150, "RGB24", nil)
	if err != nil {
		t.Fatalf("ScanPage failed: %v", err)
	}
	if img == nil || img.Bounds().Dx() != 20 || img.Bounds().Dy() != 20 {
		t.Errorf("unexpected image: %v", img)
	}
}

func TestUniversalScannerProtocolSwitch(t *testing.T) {
	uScan := NewUniversalScanner("192.168.1.50")

	// Set override protocol
	uScan.SetProtocol(ProtocolSamsungRaw)
	if uScan.ActiveProtocol() != ProtocolSamsungRaw {
		t.Errorf("expected ProtocolSamsungRaw, got %s", uScan.ActiveProtocol())
	}

	uScan.SetProtocol(ProtocolESCL)
	if uScan.ActiveProtocol() != ProtocolESCL {
		t.Errorf("expected ProtocolESCL, got %s", uScan.ActiveProtocol())
	}
}
