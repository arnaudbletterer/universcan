package scanner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // register JPEG decoder
	_ "image/png"  // register PNG decoder
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var (
	// ErrNoDocument is returned when ADF is empty or no sheets are available to scan.
	ErrNoDocument = errors.New("no document available in scanner")
)

// WSDClient executes WS-Scan (Web Services on Devices) jobs.
type WSDClient struct {
	IP         string
	Port       int
	HTTPClient *http.Client
}

// NewWSDClient creates a new WSDClient.
func NewWSDClient(ip string, port int) *WSDClient {
	if port <= 0 {
		port = 8018
	}
	return &WSDClient{
		IP:   ip,
		Port: port,
		HTTPClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

// IsAvailable checks if the WSD scan service port is responding.
func (c *WSDClient) IsAvailable(timeout time.Duration) bool {
	if timeout <= 0 {
		timeout = 500 * time.Millisecond
	}
	addr := fmt.Sprintf("%s:%d", c.IP, c.Port)
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

var (
	reJobID   = regexp.MustCompile(`<(?:[a-zA-Z0-9]+:)?JobId>(\d+)</`)
	reJobTok  = regexp.MustCompile(`<(?:[a-zA-Z0-9]+:)?JobToken>(.*?)</`)
	jpegStart = []byte{0xff, 0xd8, 0xff}
	jpegEnd   = []byte{0xff, 0xd9}
)

// ScanPage executes a full WS-Scan job cycle (CreateScanJob -> RetrieveImage).
func (c *WSDClient) ScanPage(
	ctx context.Context,
	source string,
	dpi int,
	color string,
	progress func(msg string, pct int),
) (image.Image, error) {
	wsdSource := "ADF"
	if strings.EqualFold(source, "Flatbed") || strings.EqualFold(source, "Platen") {
		wsdSource = "Platen"
	}

	colorProc := "RGB24"
	if strings.EqualFold(color, "Grayscale") || strings.EqualFold(color, "Grayscale8") || strings.EqualFold(color, "gray") {
		colorProc = "Grayscale8"
	}

	if dpi <= 0 {
		dpi = 150
	}

	endpoint := fmt.Sprintf("http://%s:%d/wsd/scan", c.IP, c.Port)
	width := 8267
	height := 11693

	if progress != nil {
		progress(fmt.Sprintf("Connecting to scanner via WS-Scan (%s, %d DPI)...", wsdSource, dpi), 15)
	}

	// 1. CreateScanJobRequest
	createUUID := fmt.Sprintf("urn:uuid:%d", time.Now().UnixNano())
	createSoap := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"
               xmlns:wsa="http://schemas.xmlsoap.org/ws/2004/08/addressing"
               xmlns:sca="http://schemas.microsoft.com/windows/2006/08/wdp/scan">
  <soap:Header>
    <wsa:To>%s</wsa:To>
    <wsa:Action>http://schemas.microsoft.com/windows/2006/08/wdp/scan/CreateScanJob</wsa:Action>
    <wsa:MessageID>%s</wsa:MessageID>
    <wsa:ReplyTo>
      <wsa:Address>http://schemas.xmlsoap.org/ws/2004/08/addressing/role/anonymous</wsa:Address>
    </wsa:ReplyTo>
  </soap:Header>
  <soap:Body>
    <sca:CreateScanJobRequest>
      <sca:ScanTicket>
        <sca:JobDescription>
          <sca:JobName>UniverScan Job</sca:JobName>
          <sca:JobOriginatingUserName>User</sca:JobOriginatingUserName>
          <sca:JobInformation>UniverScan</sca:JobInformation>
        </sca:JobDescription>
        <sca:DocumentParameters>
          <sca:Format>jfif</sca:Format>
          <sca:ImagesToTransfer>1</sca:ImagesToTransfer>
          <sca:ContentType>Mixed</sca:ContentType>
          <sca:InputSize>
            <sca:InputMediaSize>
              <sca:Width>%d</sca:Width>
              <sca:Height>%d</sca:Height>
            </sca:InputMediaSize>
          </sca:InputSize>
          <sca:InputSource>%s</sca:InputSource>
          <sca:MediaSides>
            <sca:MediaFront>
              <sca:ColorProcessing>%s</sca:ColorProcessing>
              <sca:Resolution>
                <sca:Width>%d</sca:Width>
                <sca:Height>%d</sca:Height>
              </sca:Resolution>
              <sca:ScanRegion>
                <sca:ScanRegionXOffset>0</sca:ScanRegionXOffset>
                <sca:ScanRegionYOffset>0</sca:ScanRegionYOffset>
                <sca:ScanRegionWidth>%d</sca:ScanRegionWidth>
                <sca:ScanRegionHeight>%d</sca:ScanRegionHeight>
              </sca:ScanRegion>
            </sca:MediaFront>
          </sca:MediaSides>
        </sca:DocumentParameters>
      </sca:ScanTicket>
    </sca:CreateScanJobRequest>
  </soap:Body>
</soap:Envelope>`, endpoint, createUUID, width, height, wsdSource, colorProc, dpi, dpi, width, height)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBufferString(createSoap))
	if err != nil {
		return nil, fmt.Errorf("failed to build CreateScanJob request: %w", err)
	}
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")

	if progress != nil {
		progress(fmt.Sprintf("Initiating %s scan on hardware...", wsdSource), 30)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("CreateScanJob network error: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read CreateScanJob response: %w", err)
	}
	respText := string(respBytes)

	if resp.StatusCode >= 400 {
		if strings.Contains(respText, "NoDocument") || strings.Contains(respText, "Empty") || strings.Contains(respText, "NoImagesAvailable") {
			return nil, ErrNoDocument
		}
		return nil, fmt.Errorf("WS-Scan CreateScanJob rejected (HTTP %d): %s", resp.StatusCode, respText)
	}

	jobIDMatch := reJobID.FindStringSubmatch(respText)
	jobTokMatch := reJobTok.FindStringSubmatch(respText)
	if len(jobIDMatch) < 2 || len(jobTokMatch) < 2 {
		return nil, fmt.Errorf("failed to obtain JobId/JobToken from WS-Scan response")
	}

	jobID := strings.TrimSpace(jobIDMatch[1])
	jobToken := strings.TrimSpace(jobTokMatch[1])

	// 2. RetrieveImageRequest
	if progress != nil {
		progress("Acquiring scanned page from hardware...", 60)
	}

	retUUID := fmt.Sprintf("urn:uuid:%d", time.Now().UnixNano())
	retSoap := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"
               xmlns:wsa="http://schemas.xmlsoap.org/ws/2004/08/addressing"
               xmlns:sca="http://schemas.microsoft.com/windows/2006/08/wdp/scan">
  <soap:Header>
    <wsa:To>%s</wsa:To>
    <wsa:Action>http://schemas.microsoft.com/windows/2006/08/wdp/scan/RetrieveImage</wsa:Action>
    <wsa:MessageID>%s</wsa:MessageID>
    <wsa:ReplyTo>
      <wsa:Address>http://schemas.xmlsoap.org/ws/2004/08/addressing/role/anonymous</wsa:Address>
    </wsa:ReplyTo>
  </soap:Header>
  <soap:Body>
    <sca:RetrieveImageRequest>
      <sca:DocumentDescription>
        <sca:DocumentName>SCAN001.JPG</sca:DocumentName>
      </sca:DocumentDescription>
      <sca:JobId>%s</sca:JobId>
      <sca:JobToken>%s</sca:JobToken>
    </sca:RetrieveImageRequest>
  </soap:Body>
</soap:Envelope>`, endpoint, retUUID, jobID, jobToken)

	retReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBufferString(retSoap))
	if err != nil {
		return nil, fmt.Errorf("failed to build RetrieveImage request: %w", err)
	}
	retReq.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")

	retResp, err := c.HTTPClient.Do(retReq)
	if err != nil {
		return nil, fmt.Errorf("RetrieveImage network error: %w", err)
	}
	defer retResp.Body.Close()

	rawImgBytes, err := io.ReadAll(retResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read RetrieveImage body: %w", err)
	}

	if retResp.StatusCode >= 400 || bytes.Contains(rawImgBytes, []byte("ClientErrorNoImagesAvailable")) || bytes.Contains(rawImgBytes, []byte("No images available")) {
		return nil, ErrNoDocument
	}

	startIdx := bytes.Index(rawImgBytes, jpegStart)
	if startIdx == -1 {
		if bytes.Contains(bytes.ToLower(rawImgBytes[:min(1000, len(rawImgBytes))]), []byte("fault")) {
			if bytes.Contains(rawImgBytes, []byte("NoImagesAvailable")) || bytes.Contains(rawImgBytes, []byte("Empty")) {
				return nil, ErrNoDocument
			}
			return nil, fmt.Errorf("WS-Scan Fault: %s", string(rawImgBytes))
		}
		return nil, fmt.Errorf("WS-Scan: no JPEG stream found in payload")
	}

	var jpegData []byte
	endIdx := bytes.LastIndex(rawImgBytes, jpegEnd)
	if endIdx != -1 && endIdx > startIdx {
		jpegData = rawImgBytes[startIdx : endIdx+2]
	} else {
		jpegData = rawImgBytes[startIdx:]
	}

	if progress != nil {
		progress("Processing scanned image...", 90)
	}

	img, _, err := image.Decode(bytes.NewReader(jpegData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode JPEG image from WS-Scan: %w", err)
	}

	if progress != nil {
		progress("Page scanned successfully!", 100)
	}

	return img, nil
}
