package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"prismscan/pkg/api"
	"prismscan/pkg/storage"
)

//go:embed all:web
var embeddedWebFS embed.FS

func main() {
	portFlag := flag.Int("port", 8765, "HTTP server port")
	hostFlag := flag.String("host", "127.0.0.1", "HTTP server host")
	noBrowser := flag.Bool("no-browser", false, "Do not open browser automatically")
	configDir := flag.String("config", "", "Custom configuration directory")
	flag.Parse()

	log.Println("=====================================================")
	log.Println("  PrismScan — Universal Native Document Scanner v2.0")
	log.Println("=====================================================")

	// Extract "web" subdirectory from embedded filesystem
	webFS, err := fs.Sub(embeddedWebFS, "web")
	if err != nil {
		log.Fatalf("Failed to initialize embedded web filesystem: %v", err)
	}

	cfgDir := *configDir
	if cfgDir == "" {
		cfgDir = storage.GetDefaultSettingsDir()
	}

	server := api.NewServer(cfgDir, webFS)
	server.StartBackgroundMonitor(3 * time.Second)
	defer server.StopBackgroundMonitor()

	router := server.Routes()

	// Bind to preferred port or find a free port
	addr := fmt.Sprintf("%s:%d", *hostFlag, *portFlag)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("Port %d is in use, finding an alternative port...", *portFlag)
		listener, err = net.Listen("tcp", fmt.Sprintf("%s:0", *hostFlag))
		if err != nil {
			log.Fatalf("Failed to bind network listener: %v", err)
		}
	}

	actualPort := listener.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://127.0.0.1:%d", actualPort)

	httpServer := &http.Server{
		Handler:      router,
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	// Channel to catch OS shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Launch HTTP server in background goroutine
	serverErrChan := make(chan error, 1)
	go func() {
		log.Printf("PrismScan server running at: %s", url)
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			serverErrChan <- err
		}
	}()

	// Open browser if requested
	if !*noBrowser {
		go func() {
			time.Sleep(200 * time.Millisecond)
			openBrowser(url)
		}()
	}

	// Wait for signal or server failure
	select {
	case err := <-serverErrChan:
		log.Fatalf("Server error: %v", err)
	case sig := <-sigChan:
		log.Printf("Received signal %v, shutting down PrismScan gracefully...", sig)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	} else {
		log.Println("PrismScan exited cleanly.")
	}
}

func openBrowser(url string) {
	// Try Chrome app mode first for native window experience
	var appCmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// Check common Chrome paths for app window mode
		chromePaths := []string{
			os.ExpandEnv(`%ProgramFiles%\Google\Chrome\Application\chrome.exe`),
			os.ExpandEnv(`%ProgramFiles(x86)%\Google\Chrome\Application\chrome.exe`),
			os.ExpandEnv(`%LocalAppData%\Google\Chrome\Application\chrome.exe`),
			os.ExpandEnv(`%ProgramFiles(x86)%\Microsoft\Edge\Application\msedge.exe`),
			os.ExpandEnv(`%ProgramFiles%\Microsoft\Edge\Application\msedge.exe`),
		}
		for _, cp := range chromePaths {
			if _, err := os.Stat(cp); err == nil {
				appCmd = exec.Command(cp, fmt.Sprintf("--app=%s", url))
				if err := appCmd.Start(); err == nil {
					return
				}
			}
		}
		_ = exec.Command("cmd", "/c", "start", url).Start()

	case "darwin":
		_ = exec.Command("open", url).Start()

	default: // Linux / BSD
		_ = exec.Command("xdg-open", url).Start()
	}
}
