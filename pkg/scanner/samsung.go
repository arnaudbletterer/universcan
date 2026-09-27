package scanner

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ScannerStatus contains real-time hardware telemetry and state.
type ScannerStatus struct {
	Online        bool   `json:"online"`
	IP            string `json:"ip"`
	Model         string `json:"model"`
	DisplayStatus string `json:"display_status"`
	IsSleeping    bool   `json:"is_sleeping"`
	DocInADF      bool   `json:"doc_in_adf"`
	HasADF        bool   `json:"has_adf"`
	TonerPercent  int    `json:"toner_percent"`
	DrumPercent   int    `json:"drum_percent"`
	MACAddress    string `json:"mac_address"`
}

// SamsungScannerDriver communicates with Samsung/Xerox MFP scanners over raw TCP 9400,
// SyncThru HTTP (port 80), SNMP (port 161), and WS-Scan (port 8018).
type SamsungScannerDriver struct {
	IP         string
	Port       int
	HTTPClient *http.Client
	mu         sync.Mutex
}

// NewSamsungScannerDriver creates a new driver instance for the target IP.
func NewSamsungScannerDriver(ip string, port int) *SamsungScannerDriver {
	if ip == "" {
		ip = "192.168.1.50"
	}
	if port <= 0 {
		port = 9400
	}
	return &SamsungScannerDriver{
		IP:   ip,
		Port: port,
		HTTPClient: &http.Client{
			Timeout: 2500 * time.Millisecond,
		},
	}
}

// NormalizeStatusText translates printer LCD status into clean English.
func NormalizeStatusText(text string) string {
	t := strings.TrimSpace(text)
	tl := strings.ToLower(t)

	if strings.Contains(tl, "pret a copier") || strings.Contains(tl, "prêt à copier") {
		return "Ready to copy"
	} else if strings.Contains(tl, "pret a imprimer") || strings.Contains(tl, "prêt à imprimer") {
		return "Ready to print"
	} else if tl == "pret" || tl == "prêt" {
		return "Ready"
	} else if strings.Contains(tl, "en veille") || strings.Contains(tl, "veille") || strings.Contains(tl, "sleep") {
		return "Sleep Mode"
	} else if strings.Contains(tl, "numerisation") || strings.Contains(tl, "numérisation") || strings.Contains(tl, "scanning") {
		return "Scanning..."
	} else if strings.Contains(tl, "impression") || strings.Contains(tl, "printing") {
		return "Printing..."
	} else if strings.Contains(tl, "copie") || strings.Contains(tl, "copying") {
		return "Copying..."
	} else if strings.Contains(tl, "traitement") || strings.Contains(tl, "processing") {
		return "Processing..."
	} else if strings.Contains(tl, "rechauffement") || strings.Contains(tl, "réchauffement") || strings.Contains(tl, "chauffage") || strings.Contains(tl, "warming") {
		return "Warming up..."
	} else if strings.Contains(tl, "bourrage") || strings.Contains(tl, "jam") {
		return "Paper Jam"
	} else if strings.Contains(tl, "toner") {
		if strings.Contains(tl, "vide") || strings.Contains(tl, "epuise") || strings.Contains(tl, "épuisé") || strings.Contains(tl, "empty") {
			return "Toner Empty"
		}
		if strings.Contains(tl, "bas") || strings.Contains(tl, "faible") || strings.Contains(tl, "low") {
			return "Toner Low"
		}
	} else if strings.Contains(tl, "bac vide") || strings.Contains(tl, "pas de papier") || strings.Contains(tl, "manque papier") || strings.Contains(tl, "out of paper") {
		return "Out of Paper"
	} else if strings.Contains(tl, "porte") || strings.Contains(tl, "capot") || strings.Contains(tl, "cover open") {
		return "Cover Open"
	}
	return t
}

// GetStatus retrieves multi-source hardware telemetry (SyncThru, Port 9400 sensor, SNMP, WSD).
func (d *SamsungScannerDriver) GetStatus(ctx context.Context) (*ScannerStatus, error) {
	status := &ScannerStatus{
		Online:        false,
		IP:            d.IP,
		Model:         "Samsung M2070 Series",
		DisplayStatus: "Offline",
		IsSleeping:    false,
		DocInADF:      false,
		HasADF:        true,
		TonerPercent:  0,
		DrumPercent:   0,
		MACAddress:    "",
	}

	// 1. SyncThru Web Service (port 80)
	syncThruURL := fmt.Sprintf("http://%s/sws/app/information/home/home.json", d.IP)
	req, err := http.NewRequestWithContext(ctx, "GET", syncThruURL, nil)
	if err == nil {
		req.Header.Set("User-Agent", "UniverScan/0.1.0")
		resp, err := d.HTTPClient.Do(req)
		if err == nil {
			status.Online = true
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()

			var rawMap map[string]any
			if json.Unmarshal(body, &rawMap) == nil {
				if idMap, ok := rawMap["identity"].(map[string]any); ok {
					if m, ok := idMap["model_name"].(string); ok && m != "" {
						status.Model = m
					}
					if mac, ok := idMap["mac_addr"].(string); ok {
						status.MACAddress = mac
					}
				}
				if stMap, ok := rawMap["status"].(map[string]any); ok {
					if s1, ok := stMap["status1"].(string); ok && s1 != "" {
						norm := NormalizeStatusText(s1)
						status.DisplayStatus = norm
						if strings.Contains(strings.ToLower(s1), "veille") || strings.Contains(strings.ToLower(s1), "sleep") {
							status.IsSleeping = true
						}
					}
				}
				if tbMap, ok := rawMap["toner_black"].(map[string]any); ok {
					if rem, ok := tbMap["remaining"].(float64); ok {
						status.TonerPercent = int(rem)
					}
				}
			}
		}
	}

	// 2. Query hardware sensor on Port 9400 for ADF paper sensor (only if awake)
	if !status.IsSleeping {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", d.IP, d.Port), 1200*time.Millisecond)
		if err == nil {
			status.Online = true
			// CMD_INQUIRY: 1b a8 12 00
			_, _ = conn.Write([]byte{0x1b, 0xa8, 0x12, 0x00})
			inq := make([]byte, 70)
			_, _ = io.ReadFull(conn, inq)
			_ = conn.Close()

			if len(inq) >= 70 && (inq[1] == 0x00 || inq[1] == 0x08) {
				status.HasADF = (inq[0x26] & 0x03) != 0
				if inq[1] == 0x00 {
					status.DocInADF = (inq[0x35] == 0x02)
				}
			}
		}
	}

	// 3. Fallback/Enrichment via SNMP UDP 161 (if toner level wasn't populated)
	if status.TonerPercent == 0 {
		snmp := NewSNMPClient(d.IP)
		snmpCtx, snmpCancel := context.WithTimeout(ctx, 600*time.Millisecond)
		info, err := snmp.QueryDevice(snmpCtx)
		snmpCancel()
		if err == nil && info != nil {
			if info.TonerPercent > 0 {
				status.TonerPercent = info.TonerPercent
			}
			if status.Model == "" && info.DeviceDescription != "" {
				status.Model = info.DeviceDescription
			}
		}
	}

	// 4. Check WS-Scan availability on port 8018
	wsd := NewWSDClient(d.IP, 8018)
	if wsd.IsAvailable(500 * time.Millisecond) {
		status.Online = true
		status.HasADF = true
		if status.DisplayStatus == "Offline" || status.DisplayStatus == "Scanner Unreachable" {
			status.DisplayStatus = "Ready"
		}
	}

	if !status.Online {
		status.DisplayStatus = "Scanner Unreachable"
	}

	return status, nil
}

// IsWSDAvailable returns true if WS-Scan responds on port 8018.
func (d *SamsungScannerDriver) IsWSDAvailable(timeout time.Duration) bool {
	wsd := NewWSDClient(d.IP, 8018)
	return wsd.IsAvailable(timeout)
}

// Wake sends Wake-on-LAN broadcast and continuously attempts RESERVE_UNIT until the scanner wakes up.
func (d *SamsungScannerDriver) Wake(ctx context.Context, timeout time.Duration, progress func(msg string, pct int)) (bool, error) {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}

	if progress != nil {
		progress("Sending wake signal to scanner...", 10)
	}

	// Attempt Wake-on-LAN magic packet broadcast if MAC is known
	status, _ := d.GetStatus(ctx)
	macStr := "84:25:19:75:BF:37"
	if status != nil && status.MACAddress != "" {
		macStr = status.MACAddress
	}
	sendWoL(macStr, d.IP)

	startTime := time.Now()
	for time.Since(startTime) < timeout {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
		}

		elapsed := time.Since(startTime).Seconds()
		pct := int(15 + (elapsed/timeout.Seconds())*75)
		if pct > 90 {
			pct = 90
		}
		if progress != nil {
			progress(fmt.Sprintf("Waking scanner hardware (%ds)...", int(elapsed)), pct)
		}

		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", d.IP, d.Port), 3*time.Second)
		if err == nil {
			// CMD_RESERVE_UNIT: 1b a8 16 00
			_, _ = conn.Write([]byte{0x1b, 0xa8, 0x16, 0x00})
			res := make([]byte, 32)
			n, _ := conn.Read(res)
			if n >= 2 && res[1] == 0x00 {
				// Successfully reserved! Release it now
				_, _ = conn.Write([]byte{0x1b, 0xa8, 0x17, 0x00})
				_, _ = conn.Read(res)
				_ = conn.Close()
				if progress != nil {
					progress("Scanner is ready!", 100)
				}
				return true, nil
			}
			_ = conn.Close()
		}

		time.Sleep(1 * time.Second)
	}

	return false, errors.New("timed out waiting for scanner to wake")
}

func sendWoL(macStr, targetIP string) {
	hw, err := net.ParseMAC(macStr)
	if err != nil || len(hw) != 6 {
		return
	}

	var packet []byte
	packet = append(packet, bytes.Repeat([]byte{0xff}, 6)...)
	for i := 0; i < 16; i++ {
		packet = append(packet, hw...)
	}

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.IPv4bcast, Port: 9})
	if err == nil {
		_, _ = conn.Write(packet)
		_ = conn.Close()
	}

	target := net.ParseIP(targetIP)
	if target != nil {
		connTarget, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: target, Port: 9})
		if err == nil {
			_, _ = connTarget.Write(packet)
			_ = connTarget.Close()
		}
	}
}

// ScanPage scans a single page, prioritizing driverless WS-Scan and falling back to raw TCP Port 9400.
func (d *SamsungScannerDriver) ScanPage(
	ctx context.Context,
	source string,
	dpi int,
	color string,
	progress func(msg string, pct int),
) (image.Image, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Prioritize WS-Scan if available
	if d.IsWSDAvailable(500 * time.Millisecond) {
		wsd := NewWSDClient(d.IP, 8018)
		img, err := wsd.ScanPage(ctx, source, dpi, color, progress)
		if err == nil {
			return img, nil
		}
		if errors.Is(err, ErrNoDocument) {
			return nil, ErrNoDocument
		}
		// If WS-Scan had a non-empty error, fall back to raw port 9400
	}

	return d.scanPageRawLocked(ctx, source, dpi, progress)
}

// ScanPageRaw performs the raw TCP port 9400 scan directly.
func (d *SamsungScannerDriver) ScanPageRaw(
	ctx context.Context,
	source string,
	dpi int,
	progress func(msg string, pct int),
) (image.Image, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.scanPageRawLocked(ctx, source, dpi, progress)
}

func (d *SamsungScannerDriver) scanPageRawLocked(
	ctx context.Context,
	source string,
	dpi int,
	progress func(msg string, pct int),
) (image.Image, error) {
	if dpi <= 0 {
		dpi = 150
	}

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", d.IP, d.Port), 25*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to scanner on port %d: %w", d.Port, err)
	}
	defer conn.Close()

	if progress != nil {
		progress(fmt.Sprintf("Connecting to scanner at %s...", d.IP), 10)
	}

	// 1. CMD_INQUIRY (0x1b, 0xa8, 0x12, 0x00)
	if _, err := conn.Write([]byte{0x1b, 0xa8, 0x12, 0x00}); err != nil {
		return nil, fmt.Errorf("inquiry send failed: %w", err)
	}
	inq := make([]byte, 70)
	if _, err := io.ReadFull(conn, inq); err != nil {
		return nil, fmt.Errorf("inquiry receive failed: %w", err)
	}

	// Wait if scanner warming up (status == 0x08)
	for retries := 0; len(inq) >= 2 && inq[1] == 0x08 && retries < 40; retries++ {
		time.Sleep(500 * time.Millisecond)
		if _, err := conn.Write([]byte{0x1b, 0xa8, 0x12, 0x00}); err != nil {
			return nil, err
		}
		if _, err := io.ReadFull(conn, inq); err != nil {
			return nil, err
		}
	}

	isADF := strings.EqualFold(source, "ADF")
	if isADF && len(inq) >= 70 && inq[1] == 0x00 {
		hasADF := (inq[0x26] & 0x03) != 0
		docInADF := (inq[0x35] == 0x02)
		if hasADF && !docInADF {
			return nil, ErrNoDocument
		}
	}

	sourceCode := byte(0x40) // Flatbed / Platen
	if isADF {
		sourceCode = byte(0x20) // ADF
	}

	// 2. CMD_RESERVE_UNIT (0x1b, 0xa8, 0x16, 0x00)
	if progress != nil {
		progress("Reserving scanner unit...", 20)
	}
	if _, err := conn.Write([]byte{0x1b, 0xa8, 0x16, 0x00}); err != nil {
		return nil, fmt.Errorf("reserve send failed: %w", err)
	}
	res := make([]byte, 32)
	if _, err := io.ReadFull(conn, res); err != nil {
		return nil, fmt.Errorf("reserve receive failed: %w", err)
	}

	for retries := 0; len(res) >= 2 && res[1] == 0x08 && retries < 40; retries++ {
		time.Sleep(500 * time.Millisecond)
		if _, err := conn.Write([]byte{0x1b, 0xa8, 0x16, 0x00}); err != nil {
			return nil, err
		}
		if _, err := io.ReadFull(conn, res); err != nil {
			return nil, err
		}
		if progress != nil {
			progress(fmt.Sprintf("Scanner warming up... (%ds)", retries/2), min(35, 20+retries))
		}
	}

	if len(res) < 2 || res[1] != 0 {
		return nil, fmt.Errorf("scanner reserve failed with status code 0x%02x", res[1])
	}

	// Helper to release unit on failure
	defer func() {
		_, _ = conn.Write([]byte{0x1b, 0xa8, 0x17, 0x00})
		dummy := make([]byte, 32)
		_, _ = conn.Read(dummy)
	}()

	// 3. CMD_SET_WINDOW (0x1b, 0xa8, 0x24, 0x13, 0x30, ...)
	dpiCode := byte(2) // 150 DPI
	if dpi > 150 {
		dpiCode = byte(4) // 300 DPI
	}

	setWin := make([]byte, 25)
	copy(setWin[0:5], []byte{0x1b, 0xa8, 0x24, 0x13, 0x30})
	binary.BigEndian.PutUint32(setWin[5:9], 9921)   // A4 width in 1200 dpi points
	binary.BigEndian.PutUint32(setWin[9:13], 14031) // A4 height in 1200 dpi points
	setWin[13] = dpiCode
	setWin[14] = dpiCode
	setWin[19] = 0x03       // Grayscale uncompressed 8-bit bytes
	setWin[22] = 2          // Threshold
	setWin[23] = sourceCode // ADF vs Flatbed

	if progress != nil {
		progress(fmt.Sprintf("Setting scan resolution to %d DPI (%s)...", dpi, source), 35)
	}

	if _, err := conn.Write(setWin); err != nil {
		return nil, fmt.Errorf("set window send failed: %w", err)
	}
	if _, err := io.ReadFull(conn, res); err != nil {
		return nil, fmt.Errorf("set window receive failed: %w", err)
	}
	if len(res) < 2 || res[1] != 0 {
		return nil, fmt.Errorf("set window configuration failed with status 0x%02x", res[1])
	}

	// 4. CMD_OBJECT_POSITION (0x1b, 0xa8, 0x31, 0x00)
	if progress != nil {
		progress("Feeding sheet and positioning scan head...", 45)
	}
	if _, err := conn.Write([]byte{0x1b, 0xa8, 0x31, 0x00}); err != nil {
		return nil, fmt.Errorf("position send failed: %w", err)
	}
	if _, err := io.ReadFull(conn, res); err != nil {
		return nil, fmt.Errorf("position receive failed: %w", err)
	}

	for retries := 0; len(res) >= 2 && res[1] == 0x08 && retries < 40; retries++ {
		time.Sleep(1 * time.Second)
		if _, err := conn.Write([]byte{0x1b, 0xa8, 0x31, 0x00}); err != nil {
			return nil, err
		}
		if _, err := io.ReadFull(conn, res); err != nil {
			return nil, err
		}
	}

	if len(res) >= 6 && res[1] == 0x02 {
		state := uint16(res[4])<<8 | uint16(res[5])
		if (state & 0x0010) != 0 { // STATE_NO_DOCUMENT
			return nil, ErrNoDocument
		}
	}

	if len(res) < 2 || res[1] != 0 {
		return nil, fmt.Errorf("position command failed with status 0x%02x", res[1])
	}

	// 5. CMD_READ & CMD_READ_IMAGE block acquisition
	if progress != nil {
		progress("Acquiring scanned image stream...", 55)
	}

	var rawData []byte
	var finalW int
	var totalLines int

	for {
		// CMD_READ: 1b a8 28 00
		if _, err := conn.Write([]byte{0x1b, 0xa8, 0x28, 0x00}); err != nil {
			return nil, fmt.Errorf("read cmd failed: %w", err)
		}
		if _, err := io.ReadFull(conn, res); err != nil {
			return nil, fmt.Errorf("read header receive failed: %w", err)
		}

		for len(res) >= 2 && res[1] == 0x08 {
			time.Sleep(200 * time.Millisecond)
			if _, err := conn.Write([]byte{0x1b, 0xa8, 0x28, 0x00}); err != nil {
				return nil, err
			}
			if _, err := io.ReadFull(conn, res); err != nil {
				return nil, err
			}
		}

		if len(res) < 2 || res[1] != 0 {
			break
		}

		finalBlock := (res[3] == 0x81)
		blockLen := int(binary.BigEndian.Uint32(res[4:8]))
		vertical := int(binary.BigEndian.Uint16(res[8:10]))
		horizontal := int(binary.BigEndian.Uint16(res[10:12]))

		finalW = horizontal
		totalLines += vertical

		// CMD_READ_IMAGE: 1b a8 29 00
		if _, err := conn.Write([]byte{0x1b, 0xa8, 0x29, 0x00}); err != nil {
			return nil, fmt.Errorf("read image cmd failed: %w", err)
		}
		chunk := make([]byte, blockLen)
		if _, err := io.ReadFull(conn, chunk); err != nil {
			return nil, fmt.Errorf("chunk read failed (expected %d bytes): %w", blockLen, err)
		}
		rawData = append(rawData, chunk...)

		if progress != nil {
			pct := min(95, 55+int(float64(totalLines)/1800.0*38.0))
			progress(fmt.Sprintf("Receiving image: %d lines...", totalLines), pct)
		}

		if finalBlock {
			break
		}
	}

	expected := finalW * totalLines
	if finalW <= 0 || totalLines <= 0 || len(rawData) < expected {
		return nil, fmt.Errorf("corrupted image stream: got %d bytes, expected %d (%dx%d)", len(rawData), expected, finalW, totalLines)
	}

	img := image.NewGray(image.Rect(0, 0, finalW, totalLines))
	copy(img.Pix, rawData[:expected])

	if progress != nil {
		progress("Page capture complete!", 100)
	}

	return img, nil
}
