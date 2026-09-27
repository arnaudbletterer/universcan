package diagnostic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"prismscan/pkg/scanner"
)

// DiagnosticReport represents the full hardware telemetry report.
type DiagnosticReport struct {
	Timestamp           time.Time              `json:"timestamp"`
	TargetIP            string                 `json:"target_ip"`
	Manufacturer        string                 `json:"manufacturer"`
	Model               string                 `json:"model"`
	ResponsivePorts     []int                  `json:"responsive_ports"`
	RecommendedProtocol string                 `json:"recommended_protocol"`
	Protocols           map[string]any         `json:"protocols"`
	MarkdownSummary     string                 `json:"markdown_summary"`
}

// Collector probes the network target across all protocols and gathers diagnostics.
type Collector struct {
	lastReport   *DiagnosticReport
	mu           sync.RWMutex
}

// NewCollector creates a diagnostics collector.
func NewCollector() *Collector {
	return &Collector{}
}

// GetLastReport returns the most recently collected diagnostic report.
func (c *Collector) GetLastReport() *DiagnosticReport {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastReport
}

// GenerateReport runs comprehensive multi-protocol diagnostics against target IP.
func (c *Collector) GenerateReport(ctx context.Context, targetIP string) (*DiagnosticReport, error) {
	if targetIP == "" {
		targetIP = "192.168.1.50"
	}

	report := &DiagnosticReport{
		Timestamp:    time.Now().UTC(),
		TargetIP:     targetIP,
		Manufacturer: "Unknown",
		Model:        "Unknown Scanner",
		Protocols:    make(map[string]any),
	}

	// 1. Port scan for common scanner ports
	portsToTest := []int{80, 443, 161, 5357, 8018, 8080, 9100, 9400}
	var openPorts []int
	var portMu sync.Mutex
	var wg sync.WaitGroup

	for _, p := range portsToTest {
		port := p
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", targetIP, port), 600*time.Millisecond)
			if err == nil {
				_ = conn.Close()
				portMu.Lock()
				openPorts = append(openPorts, port)
				portMu.Unlock()
			}
		}()
	}
	wg.Wait()
	report.ResponsivePorts = openPorts

	// 2. HTTP Server Headers and SyncThru probe
	httpRes := map[string]any{"available": false}
	httpClient := &http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := httpClient.Get(fmt.Sprintf("http://%s/", targetIP))
	if err == nil {
		httpRes["available"] = true
		httpRes["server_header"] = resp.Header.Get("Server")
		_ = resp.Body.Close()
	}

	// Check SyncThru
	stResp, err := httpClient.Get(fmt.Sprintf("http://%s/sws/app/information/home/home.json", targetIP))
	if err == nil && stResp.StatusCode == 200 {
		stBytes, _ := io.ReadAll(stResp.Body)
		_ = stResp.Body.Close()
		httpRes["syncthru_json_available"] = true
		var stMap map[string]any
		if json.Unmarshal(stBytes, &stMap) == nil {
			if idMap, ok := stMap["identity"].(map[string]any); ok {
				if m, ok := idMap["model_name"].(string); ok && m != "" {
					report.Model = m
					report.Manufacturer = "Samsung"
					httpRes["model_name"] = m
				}
			}
		}
	}
	report.Protocols["http"] = httpRes

	// 3. eSCL (Apple AirScan / Mopria Scan)
	esclRes := map[string]any{"available": false}
	esclClient := scanner.NewESCLClient(targetIP, 80, false)
	if esclClient.IsAvailable(800 * time.Millisecond) {
		esclRes["available"] = true
		caps, err := esclClient.GetCapabilities(ctx)
		if err == nil && caps != nil {
			esclRes["make_and_model"] = caps.MakeAndModel
			esclRes["manufacturer"] = caps.Manufacturer
			esclRes["has_platen"] = caps.HasPlaten
			esclRes["has_adf"] = caps.HasADF
			esclRes["adf_duplex"] = caps.ADFDuplex
			if caps.Manufacturer != "" {
				report.Manufacturer = caps.Manufacturer
			}
			if caps.MakeAndModel != "" {
				report.Model = caps.MakeAndModel
			}
		}
	}
	report.Protocols["escl"] = esclRes

	// 4. Port 9400 Samsung / Xerox Raw Protocol
	samsungRes := map[string]any{"available": false}
	samsungDriver := scanner.NewSamsungScannerDriver(targetIP, 9400)
	st, err := samsungDriver.GetStatus(ctx)
	if err == nil && st != nil && st.Online {
		samsungRes["available"] = true
		samsungRes["model"] = st.Model
		samsungRes["display_status"] = st.DisplayStatus
		samsungRes["is_sleeping"] = st.IsSleeping
		samsungRes["doc_in_adf"] = st.DocInADF
		samsungRes["has_adf"] = st.HasADF
		samsungRes["toner_percent"] = st.TonerPercent
		if report.Manufacturer == "Unknown" {
			report.Manufacturer = "Samsung"
		}
		if report.Model == "Unknown Scanner" && st.Model != "" {
			report.Model = st.Model
		}
	}
	report.Protocols["samsung_raw"] = samsungRes

	// 5. WS-Scan (WSD)
	wsdRes := map[string]any{"available": false}
	wsdClient := scanner.NewWSDClient(targetIP, 8018)
	if wsdClient.IsAvailable(600 * time.Millisecond) {
		wsdRes["available"] = true
		wsdRes["port"] = 8018
	}
	report.Protocols["wsd"] = wsdRes

	// 6. SNMP
	snmpRes := map[string]any{"available": false}
	snmpClient := scanner.NewSNMPClient(targetIP)
	snmpCtx, snmpCancel := context.WithTimeout(ctx, 800*time.Millisecond)
	snmpInfo, err := snmpClient.QueryDevice(snmpCtx)
	snmpCancel()
	if err == nil && snmpInfo != nil {
		snmpRes["available"] = true
		snmpRes["device_description"] = snmpInfo.DeviceDescription
		snmpRes["console_display"] = snmpInfo.DisplayBuffer
		snmpRes["toner_percent"] = snmpInfo.TonerPercent
		if report.Model == "Unknown Scanner" && snmpInfo.DeviceDescription != "" {
			report.Model = snmpInfo.DeviceDescription
		}
	}
	report.Protocols["snmp"] = snmpRes

	// Determine recommended protocol
	if esclAvailable, _ := esclRes["available"].(bool); esclAvailable {
		report.RecommendedProtocol = "eSCL (Apple AirScan / Mopria Scan)"
	} else if samAvailable, _ := samsungRes["available"].(bool); samAvailable {
		report.RecommendedProtocol = "Samsung/Xerox Port 9400 Raw TCP"
	} else if wsdAvailable, _ := wsdRes["available"].(bool); wsdAvailable {
		report.RecommendedProtocol = "WS-Scan (Web Services on Devices)"
	} else {
		report.RecommendedProtocol = "None detected (Check power / IP connection)"
	}

	// Generate Markdown diagnostic report
	var md bytes.Buffer
	md.WriteString("# PrismScan Hardware Diagnostics Report\n\n")
	md.WriteString(fmt.Sprintf("- **Generated At:** `%s`\n", report.Timestamp.Format(time.RFC3339)))
	md.WriteString(fmt.Sprintf("- **Target IP:** `%s`\n", report.TargetIP))
	md.WriteString(fmt.Sprintf("- **Manufacturer:** `%s`\n", report.Manufacturer))
	md.WriteString(fmt.Sprintf("- **Detected Model:** `%s`\n", report.Model))
	md.WriteString(fmt.Sprintf("- **Recommended Protocol:** `%s`\n\n", report.RecommendedProtocol))

	md.WriteString("### Responsive Ports\n")
	if len(openPorts) > 0 {
		var portStrs []string
		for _, p := range openPorts {
			portStrs = append(portStrs, fmt.Sprintf("%d", p))
		}
		md.WriteString(fmt.Sprintf("- %s\n\n", strings.Join(portStrs, ", ")))
	} else {
		md.WriteString("- None\n\n")
	}

	md.WriteString("### Protocol Support Breakdown\n")
	md.WriteString(fmt.Sprintf("- **eSCL (AirScan / Mopria):** `%v`\n", esclRes["available"]))
	md.WriteString(fmt.Sprintf("- **Samsung Raw TCP (Port 9400):** `%v`\n", samsungRes["available"]))
	md.WriteString(fmt.Sprintf("- **WS-Scan / WSD (Port 8018):** `%v`\n", wsdRes["available"]))
	md.WriteString(fmt.Sprintf("- **SNMP UDP (Port 161):** `%v`\n", snmpRes["available"]))
	md.WriteString(fmt.Sprintf("- **HTTP Web UI (Port 80):** `%v`\n", httpRes["available"]))

	report.MarkdownSummary = md.String()

	c.mu.Lock()
	c.lastReport = report
	c.mu.Unlock()

	return report, nil
}
