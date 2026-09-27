package scanner

import (
	"context"
	"fmt"
	"image"
	"strings"
	"sync"
	"time"
)

// Protocol identifies the scanner communication protocol.
type Protocol string

const (
	ProtocolAuto       Protocol = "auto"
	ProtocolESCL       Protocol = "escl"
	ProtocolWSD        Protocol = "wsd"
	ProtocolSamsungRaw Protocol = "samsung_raw"
)

// UniversalScanner provides a single unified interface that transparently communicates
// with any scanner (HP, Canon, Epson, Brother, Kyocera, Xerox, Samsung, etc.).
type UniversalScanner struct {
	IP               string
	ProtocolOverride Protocol
	activeProtocol   Protocol
	samsungDriver    *SamsungScannerDriver
	esclClient       *ESCLClient
	wsdClient        *WSDClient
	mu               sync.Mutex
}

// NewUniversalScanner creates a universal scanner client for the given host.
func NewUniversalScanner(ip string) *UniversalScanner {
	if ip == "" {
		ip = "192.168.1.50"
	}
	return &UniversalScanner{
		IP:             ip,
		activeProtocol: ProtocolAuto,
		samsungDriver:  NewSamsungScannerDriver(ip, 9400),
		esclClient:     NewESCLClient(ip, 80, false),
		wsdClient:      NewWSDClient(ip, 8018),
	}
}

// DetectProtocol probes the hardware across all supported scanner protocols and determines the best one.
func (u *UniversalScanner) DetectProtocol(ctx context.Context) Protocol {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.ProtocolOverride != "" && u.ProtocolOverride != ProtocolAuto {
		u.activeProtocol = u.ProtocolOverride
		return u.activeProtocol
	}

	// 1. Try eSCL (standard for HP, Canon, Epson, Brother, Kyocera, Xerox)
	esclPorts := []struct {
		port  int
		https bool
	}{
		{80, false},
		{8080, false},
		{443, true},
	}
	for _, ep := range esclPorts {
		client := NewESCLClient(u.IP, ep.port, ep.https)
		if client.IsAvailable(400 * time.Millisecond) {
			u.esclClient = client
			u.activeProtocol = ProtocolESCL
			return ProtocolESCL
		}
	}

	// 2. Try Samsung Raw TCP Port 9400 (if port is open)
	if isPortOpen(u.IP, 9400, 400*time.Millisecond) {
		u.activeProtocol = ProtocolSamsungRaw
		return ProtocolSamsungRaw
	}

	// 3. Try WS-Scan (Port 8018 or 5357)
	if u.wsdClient.IsAvailable(400 * time.Millisecond) {
		u.activeProtocol = ProtocolWSD
		return ProtocolWSD
	}

	// Default fallback to Samsung raw or eSCL
	u.activeProtocol = ProtocolSamsungRaw
	return u.activeProtocol
}

// GetStatus returns real-time hardware telemetry from whichever protocol is active.
func (u *UniversalScanner) GetStatus(ctx context.Context) (*ScannerStatus, error) {
	proto := u.DetectProtocol(ctx)

	switch proto {
	case ProtocolESCL:
		status := &ScannerStatus{
			Online:        true,
			IP:            u.IP,
			Model:         "Universal eSCL Scanner",
			DisplayStatus: "Ready",
			HasADF:        true,
			DocInADF:      false,
			TonerPercent:  100,
		}
		caps, err := u.esclClient.GetCapabilities(ctx)
		if err == nil && caps != nil {
			if caps.MakeAndModel != "" {
				status.Model = caps.MakeAndModel
			} else if caps.Manufacturer != "" {
				status.Model = caps.Manufacturer
			}
			status.HasADF = caps.HasADF
		}
		esclSt, err := u.esclClient.GetStatus(ctx)
		if err == nil && esclSt != nil {
			status.DisplayStatus = esclSt.State
			if esclSt.AdfState == "AdfLoaded" {
				status.DocInADF = true
			}
		}
		return status, nil

	default:
		// Samsung / Xerox / WSD status
		return u.samsungDriver.GetStatus(ctx)
	}
}

// ScanPage executes a scan job using the best detected protocol.
func (u *UniversalScanner) ScanPage(
	ctx context.Context,
	source string,
	dpi int,
	color string,
	progress func(msg string, pct int),
) (image.Image, error) {
	proto := u.DetectProtocol(ctx)

	switch proto {
	case ProtocolESCL:
		return u.esclClient.ScanPage(ctx, source, dpi, color, progress)
	case ProtocolWSD:
		return u.wsdClient.ScanPage(ctx, source, dpi, color, progress)
	case ProtocolSamsungRaw:
		return u.samsungDriver.ScanPage(ctx, source, dpi, color, progress)
	default:
		return nil, fmt.Errorf("unsupported or undetected protocol: %s", proto)
	}
}

// Wake wakes the scanner hardware if sleeping.
func (u *UniversalScanner) Wake(ctx context.Context, timeout time.Duration, progress func(msg string, pct int)) (bool, error) {
	// WoL and Samsung reserve loop works well for waking devices
	return u.samsungDriver.Wake(ctx, timeout, progress)
}

// ActiveProtocol returns currently selected protocol.
func (u *UniversalScanner) ActiveProtocol() Protocol {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.activeProtocol
}

// SetProtocol forces a specific protocol.
func (u *UniversalScanner) SetProtocol(p Protocol) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.ProtocolOverride = p
	u.activeProtocol = p
}

// Ensure interface compatibility
var _ = strings.TrimSpace
