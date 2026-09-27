# PrismScan — Universal Driverless Network Scanner

A modern, fast, zero-dependency native document scanner application for Windows, macOS, and Linux. 

Built in pure Go with an embedded web UI. No Python, no drivers, and no external runtime dependencies required.

---

## Key Features

- **Universal Multi-Vendor Compatibility**:
  - **Apple AirScan / eSCL (Mopria Scan)**: Supported by virtually all modern scanners from HP, Canon, Epson, Brother, Lexmark, Xerox, Kyocera, and Ricoh.
  - **WS-Scan / WSD**: Windows-certified Web Services for Devices.
  - **Samsung / Xerox Raw TCP (Port 9400)**: Direct protocol for Samsung Xpress and MultiXpress multifunction devices.
- **Non-Technical User Friendly**:
  - Windows: 1-click installer (`PrismScan-Setup.exe`) — no administrative elevation required, creates Desktop and Start Menu shortcuts.
  - macOS: Universal `.app` bundle & `.dmg` supporting both Apple Silicon (M1/M2/M3/M4) and Intel Macs.
  - Linux: Standalone binary and `.tar.gz` bundle with `.desktop` menu integration.
- **Sober, Modern Design (No AI-Slop)**:
  - Clean, high-contrast, tactile UI with large readable controls.
  - Smart empty-feeder detection: automatically selects Flatbed Glass and greys out the feeder option with a reassuring notice when no sheets are loaded.
  - Guided 2-sided duplex scanning with a physical 3-step visual guide (rotate stack 180° flat like a steering wheel).
  - Drag-and-drop page reordering and inspection modal.
- **1-Click Hardware Diagnostics**:
  - Probes scanner endpoints (eSCL, WSD, Port 9400, SNMP) and generates a formatted Markdown report.
  - 1-click **"Copy Diagnostic Report"** and **"Save Report (.json)"** to easily share device telemetry with developers or AI assistants to add hardware quirks.

---

## Building Locally

### Prerequisites
- Go 1.24+ (Windows, macOS, or Linux)
- (Optional on Windows) Inno Setup 6 for compiling `PrismScan-Setup.exe`

### Build All Platforms
```powershell
# Windows PowerShell:
.\scripts\build_all.ps1

# Linux / macOS Bash:
./scripts/build_all.sh
```

Compiled binaries will be generated in `bin/`:
- `bin/prismscan.exe` (Windows x86_64)
- `bin/prismscan-darwin-arm64` (macOS Apple Silicon)
- `bin/prismscan-darwin-amd64` (macOS Intel)
- `bin/prismscan-linux-amd64` (Linux x86_64)

The Windows installer is output to:
- `dist/windows/PrismScan-Setup.exe`

---

## Automated GitHub Actions Releases

This repository includes fully automated GitHub Actions workflows:

### 1. CI Validation (`.github/workflows/ci.yml`)
Runs automated Go unit tests and build verification across `ubuntu-latest`, `windows-latest`, and `macos-latest` on every push and pull request.

### 2. Multi-Platform Release (`.github/workflows/release.yml`)
Triggered automatically on git tags (e.g. `v2.0.0`) or via manual **Run workflow** in the GitHub Actions tab.

It automatically:
1. Compiles optimized binaries for Windows, macOS (Universal ARM64+Intel), and Linux.
2. Compiles `PrismScan-Setup.exe` using Inno Setup on Windows runners.
3. Packages `PrismScan-macOS.dmg` and `PrismScan-macOS.zip` on macOS runners.
4. Packages `prismscan-linux-x86_64.tar.gz` on Linux runners.
5. Computes `SHA256SUMS.txt` checksums.
6. Publishes a complete GitHub Release with all binary assets and release notes attached.

### How to Release a New Version
```bash
git tag v2.0.0
git push origin v2.0.0
```
Or navigate to **Actions** → **Release Multi-Platform Packages** → **Run workflow** in the GitHub web interface.
