package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Settings holds user preferences.
type Settings struct {
	OutputDir    string `json:"output_dir"`
	ColorMode    string `json:"color_mode"`
	DPI          int    `json:"dpi"`
	Source       string `json:"source"`
	Sides        string `json:"sides"`
	OpenAfter    bool   `json:"open_after"`
	NamingPrefix string `json:"naming_prefix"`
	TargetIP     string `json:"target_ip"`
}

var (
	settingsMu sync.RWMutex
)

// GetDefaultSettingsDir returns ~/.universcan (falling back to ~/.prism_scan if already present).
func GetDefaultSettingsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	uniDir := filepath.Join(home, ".universcan")
	if _, err := os.Stat(uniDir); err == nil {
		return uniDir
	}
	prismDir := filepath.Join(home, ".prism_scan")
	if _, err := os.Stat(prismDir); err == nil {
		return prismDir
	}
	return uniDir
}

// DefaultSettings returns the initial default configuration.
func DefaultSettings() Settings {
	home, _ := os.UserHomeDir()
	defaultScans := filepath.Join(home, "Documents", "Scans")
	return Settings{
		OutputDir:    defaultScans,
		ColorMode:    "color",
		DPI:          150,
		Source:       "Flatbed", // Defaults to Flatbed if ADF empty
		Sides:        "2",
		OpenAfter:    true,
		NamingPrefix: "Scan",
		TargetIP:     "192.168.1.50",
	}
}

// LoadSettings reads ~/.universcan/settings.json (or ~/.prism_scan/settings.json fallback) or returns defaults.
func LoadSettings(configDir string) Settings {
	settingsMu.RLock()
	defer settingsMu.RUnlock()

	if configDir == "" {
		configDir = GetDefaultSettingsDir()
	}

	settingsPath := filepath.Join(configDir, "settings.json")
	defaults := DefaultSettings()

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return defaults
	}

	var loaded Settings
	if err := json.Unmarshal(data, &loaded); err != nil {
		return defaults
	}

	// Apply non-empty loaded fields over defaults
	if loaded.OutputDir == "" {
		loaded.OutputDir = defaults.OutputDir
	}
	if loaded.ColorMode == "" {
		loaded.ColorMode = defaults.ColorMode
	}
	if loaded.DPI <= 0 {
		loaded.DPI = defaults.DPI
	}
	if loaded.Source == "" {
		loaded.Source = defaults.Source
	}
	if loaded.Sides == "" {
		loaded.Sides = defaults.Sides
	}
	if loaded.NamingPrefix == "" {
		loaded.NamingPrefix = defaults.NamingPrefix
	}
	if loaded.TargetIP == "" {
		loaded.TargetIP = defaults.TargetIP
	}

	return loaded
}

// SaveSettings writes updated settings to ~/.universcan/settings.json (or active configDir).
func SaveSettings(newSettings Settings, configDir string) (Settings, error) {
	settingsMu.Lock()
	defer settingsMu.Unlock()

	if configDir == "" {
		configDir = GetDefaultSettingsDir()
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return newSettings, err
	}

	settingsPath := filepath.Join(configDir, "settings.json")
	data, err := json.MarshalIndent(newSettings, "", "  ")
	if err != nil {
		return newSettings, err
	}

	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return newSettings, err
	}

	return newSettings, nil
}
