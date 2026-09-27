package scanner

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestNormalizeStatusText(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Pret a copier", "Ready to copy"},
		{"Prêt à imprimer", "Ready to print"},
		{"En veille", "Sleep Mode"},
		{"Numerisation en cours", "Scanning..."},
		{"Bourrage papier", "Paper Jam"},
		{"Toner vide", "Toner Empty"},
		{"Toner bas", "Toner Low"},
		{"Bac vide", "Out of Paper"},
		{"Capot ouvert", "Cover Open"},
		{"Some Other Status", "Some Other Status"},
	}

	for _, tt := range tests {
		got := NormalizeStatusText(tt.input)
		if got != tt.expected {
			t.Errorf("NormalizeStatusText(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

func TestSamsungDriverGetStatus(t *testing.T) {
	driver := NewSamsungScannerDriver("192.168.1.50", 9400)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	status, err := driver.GetStatus(ctx)
	if err != nil {
		t.Fatalf("GetStatus returned error: %v", err)
	}
	if status == nil {
		t.Fatal("status is nil")
	}

	t.Logf("Scanner online=%v, model=%q, status=%q, sleeping=%v, toner=%d%%, doc_in_adf=%v",
		status.Online, status.Model, status.DisplayStatus, status.IsSleeping, status.TonerPercent, status.DocInADF)
}

func TestSamsungMockServerProtocol(t *testing.T) {
	// Start mock TCP port 9400 server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 128)
		for {
			n, err := conn.Read(buf)
			if err != nil || n == 0 {
				return
			}
			req := buf[:n]
			if len(req) >= 4 && req[0] == 0x1b && req[1] == 0xa8 {
				cmd := req[2]
				switch cmd {
				case 0x12: // INQUIRY
					resp := make([]byte, 70)
					resp[0] = 0x00
					resp[1] = 0x00 // Success
					resp[0x26] = 0x01 // Has ADF
					resp[0x35] = 0x02 // Doc in ADF
					_, _ = conn.Write(resp)
				case 0x16: // RESERVE_UNIT
					resp := make([]byte, 32)
					resp[1] = 0x00 // Success
					_, _ = conn.Write(resp)
				case 0x17: // RELEASE_UNIT
					resp := make([]byte, 32)
					_, _ = conn.Write(resp)
				case 0x24: // SET_WINDOW
					resp := make([]byte, 32)
					resp[1] = 0x00
					_, _ = conn.Write(resp)
				case 0x31: // OBJECT_POSITION
					resp := make([]byte, 32)
					resp[1] = 0x00
					_, _ = conn.Write(resp)
				case 0x28: // READ
					resp := make([]byte, 32)
					resp[1] = 0x00
					resp[3] = 0x81 // finalBlock
					// blockLen: 16 bytes
					resp[4] = 0; resp[5] = 0; resp[6] = 0; resp[7] = 16
					// vertical: 4 lines
					resp[8] = 0; resp[9] = 4
					// horizontal: 4 pixels
					resp[10] = 0; resp[11] = 4
					_, _ = conn.Write(resp)
				case 0x29: // READ_IMAGE
					data := make([]byte, 16)
					for i := range data {
						data[i] = byte(i * 15)
					}
					_, _ = conn.Write(data)
				}
			}
		}
	}()

	driver := NewSamsungScannerDriver("127.0.0.1", port)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	img, err := driver.ScanPageRaw(ctx, "ADF", 150, nil)
	if err != nil {
		t.Fatalf("ScanPageRaw mock failed: %v", err)
	}
	if img == nil {
		t.Fatal("img is nil")
	}
	bounds := img.Bounds()
	if bounds.Dx() != 4 || bounds.Dy() != 4 {
		t.Errorf("expected 4x4 image, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}
