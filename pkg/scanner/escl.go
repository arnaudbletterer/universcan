package scanner

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"
	"time"
)

// ESCLClient communicates with Apple AirScan / Mopria Scan (eSCL) compatible scanners.
type ESCLClient struct {
	IP         string
	Port       int
	UseHTTPS   bool
	HTTPClient *http.Client
}

// NewESCLClient creates a new eSCL client.
func NewESCLClient(ip string, port int, useHTTPS bool) *ESCLClient {
	if port <= 0 {
		if useHTTPS {
			port = 443
		} else {
			port = 80
		}
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &ESCLClient{
		IP:       ip,
		Port:     port,
		UseHTTPS: useHTTPS,
		HTTPClient: &http.Client{
			Transport: tr,
			Timeout:   45 * time.Second,
		},
	}
}

func (c *ESCLClient) baseURL() string {
	scheme := "http"
	if c.UseHTTPS {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, c.IP, c.Port)
}

// ESCLCapabilities models scanner capabilities returned by /eSCL/ScannerCapabilities.
type ESCLCapabilities struct {
	XMLName    xml.Name `xml:"ScannerCapabilities"`
	MakeAndModel string `xml:"MakeAndModel"`
	Manufacturer string `xml:"Manufacturer"`
	HasPlaten  bool
	HasADF     bool
	ADFDuplex  bool
}

// ESCLStatus models scanner status returned by /eSCL/ScannerStatus.
type ESCLStatus struct {
	XMLName  xml.Name `xml:"ScannerStatus"`
	State    string   `xml:"State"`
	AdfState string   `xml:"AdfState"`
}

// IsAvailable quickly tests if the eSCL endpoint responds on /eSCL/ScannerCapabilities or ScannerStatus.
func (c *ESCLClient) IsAvailable(timeout time.Duration) bool {
	if timeout <= 0 {
		timeout = 800 * time.Millisecond
	}
	client := &http.Client{
		Transport: c.HTTPClient.Transport,
		Timeout:   timeout,
	}

	url := fmt.Sprintf("%s/eSCL/ScannerCapabilities", c.baseURL())
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err == nil {
		_ = resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}

	url2 := fmt.Sprintf("%s/eSCL/ScannerStatus", c.baseURL())
	req2, err := http.NewRequest("GET", url2, nil)
	if err != nil {
		return false
	}
	resp2, err := client.Do(req2)
	if err == nil {
		_ = resp2.Body.Close()
		return resp2.StatusCode == http.StatusOK
	}

	return false
}

// GetCapabilities queries /eSCL/ScannerCapabilities.
func (c *ESCLClient) GetCapabilities(ctx context.Context) (*ESCLCapabilities, error) {
	url := fmt.Sprintf("%s/eSCL/ScannerCapabilities", c.baseURL())
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("eSCL capabilities error HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	caps := &ESCLCapabilities{}
	_ = xml.Unmarshal(body, caps)

	// Robust detection of Platen and ADF tags
	bStr := strings.ToLower(string(body))
	caps.HasPlaten = strings.Contains(bStr, "platen")
	caps.HasADF = strings.Contains(bStr, "adf")
	caps.ADFDuplex = strings.Contains(bStr, "adfduplex") || strings.Contains(bStr, "duplex")

	return caps, nil
}

// GetStatus queries /eSCL/ScannerStatus.
func (c *ESCLClient) GetStatus(ctx context.Context) (*ESCLStatus, error) {
	url := fmt.Sprintf("%s/eSCL/ScannerStatus", c.baseURL())
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("eSCL status error HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	st := &ESCLStatus{}
	_ = xml.Unmarshal(body, st)

	bStr := strings.ToLower(string(body))
	if strings.Contains(bStr, "adfempty") {
		st.AdfState = "AdfEmpty"
	} else if strings.Contains(bStr, "adfloaded") {
		st.AdfState = "AdfLoaded"
	}

	return st, nil
}

// ScanPage creates a scan job and retrieves the image over eSCL.
func (c *ESCLClient) ScanPage(
	ctx context.Context,
	source string,
	dpi int,
	color string,
	progress func(msg string, pct int),
) (image.Image, error) {
	if dpi <= 0 {
		dpi = 150
	}

	esclSource := "Platen"
	if strings.EqualFold(source, "ADF") {
		esclSource = "Adf"
	}

	colorMode := "RGB24"
	if strings.EqualFold(color, "Grayscale") || strings.EqualFold(color, "Grayscale8") || strings.EqualFold(color, "gray") {
		colorMode = "Grayscale8"
	}

	if progress != nil {
		progress(fmt.Sprintf("Connecting to eSCL / AirScan service (%s, %d DPI)...", esclSource, dpi), 15)
	}

	// 1. Post ScanSettings to /eSCL/ScanJobs
	// Standard A4 dimensions in 300 DPI points: 2480 x 3508
	widthPts := 2550
	heightPts := 3508

	jobXML := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<scan:ScanSettings xmlns:scan="http://schemas.hp.com/imaging/escl/2011/05/03" xmlns:pwg="http://www.pwg.org/schemas/2010/12/sm">
  <pwg:Version>2.0</pwg:Version>
  <pwg:ScanRegions>
    <pwg:ScanRegion>
      <pwg:XOffset>0</pwg:XOffset>
      <pwg:YOffset>0</pwg:YOffset>
      <pwg:Width>%d</pwg:Width>
      <pwg:Height>%d</pwg:Height>
    </pwg:ScanRegion>
  </pwg:ScanRegions>
  <scan:InputSource>%s</scan:InputSource>
  <scan:ColorMode>%s</scan:ColorMode>
  <scan:XResolution>%d</scan:XResolution>
  <scan:YResolution>%d</scan:YResolution>
  <pwg:DocumentFormat>image/jpeg</pwg:DocumentFormat>
</scan:ScanSettings>`, widthPts, heightPts, esclSource, colorMode, dpi, dpi)

	postURL := fmt.Sprintf("%s/eSCL/ScanJobs", c.baseURL())
	req, err := http.NewRequestWithContext(ctx, "POST", postURL, bytes.NewBufferString(jobXML))
	if err != nil {
		return nil, fmt.Errorf("failed to create eSCL job request: %w", err)
	}
	req.Header.Set("Content-Type", "text/xml")

	if progress != nil {
		progress("Submitting eSCL scan job to hardware...", 30)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("eSCL POST ScanJobs failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict || resp.StatusCode == http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		if strings.Contains(strings.ToLower(string(body)), "empty") || strings.Contains(strings.ToLower(string(body)), "nodocument") {
			return nil, ErrNoDocument
		}
		return nil, fmt.Errorf("eSCL job creation rejected (%d): %s", resp.StatusCode, string(body))
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("eSCL unexpected status %d: %s", resp.StatusCode, string(body))
	}

	location := resp.Header.Get("Location")
	if location == "" {
		return nil, errors.New("eSCL scanner did not return job Location header")
	}

	// Make location absolute if relative
	docURL := location
	if !strings.HasPrefix(location, "http://") && !strings.HasPrefix(location, "https://") {
		if strings.HasPrefix(location, "/") {
			docURL = fmt.Sprintf("%s%s", c.baseURL(), location)
		} else {
			docURL = fmt.Sprintf("%s/%s", c.baseURL(), location)
		}
	}
	if !strings.HasSuffix(docURL, "/NextDocument") {
		docURL = fmt.Sprintf("%s/NextDocument", strings.TrimSuffix(docURL, "/"))
	}

	if progress != nil {
		progress("Downloading scanned page from hardware...", 60)
	}

	// 2. Fetch NextDocument
	docReq, err := http.NewRequestWithContext(ctx, "GET", docURL, nil)
	if err != nil {
		return nil, err
	}

	docResp, err := c.HTTPClient.Do(docReq)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve eSCL document: %w", err)
	}
	defer docResp.Body.Close()

	if docResp.StatusCode == http.StatusNotFound || docResp.StatusCode == http.StatusGone {
		return nil, ErrNoDocument
	}
	if docResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(docResp.Body)
		return nil, fmt.Errorf("failed to fetch eSCL document (status %d): %s", docResp.StatusCode, string(body))
	}

	rawBytes, err := io.ReadAll(docResp.Body)
	if err != nil {
		return nil, err
	}

	// Check if JPEG stream inside
	startIdx := bytes.Index(rawBytes, []byte{0xff, 0xd8, 0xff})
	var imgData []byte
	if startIdx != -1 {
		endIdx := bytes.LastIndex(rawBytes, []byte{0xff, 0xd9})
		if endIdx != -1 && endIdx > startIdx {
			imgData = rawBytes[startIdx : endIdx+2]
		} else {
			imgData = rawBytes[startIdx:]
		}
	} else {
		imgData = rawBytes
	}

	if progress != nil {
		progress("Decoding image stream...", 90)
	}

	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode eSCL image: %w", err)
	}

	if progress != nil {
		progress("Scan complete!", 100)
	}

	return img, nil
}
