# UniverScan — Universal Driverless Document Scanner

> **One simple app for the universe of scanners.**  
> Scan uniformly, simply, and driverless across Windows, macOS, and Linux.

UniverScan is a modern, fast, zero-dependency native document scanner application. Built in pure Go with an embedded web UI, it requires no Python, no manufacturer bloated software, and no external runtime dependencies.

---

## Why UniverScan?

- **Universal Multi-Vendor Compatibility**:
  - **Apple AirScan / eSCL (Mopria Scan)**: Supported by virtually all modern scanners from HP, Canon, Epson, Brother, Lexmark, Xerox, Kyocera, Ricoh, and Pantum.
  - **WS-Scan / WSD**: Microsoft-certified Web Services for Devices protocol.
  - **Samsung / Xerox Raw TCP (Port 9400)**: Direct protocol for Samsung Xpress, MultiXpress, and Xerox WorkCentre lines.
- **Designed for Everyday People**:
  - **Windows**: 1-click setup installer (`UniverScan-Setup.exe`) — requires no admin rights, installs to user space or Program Files, and creates Desktop and Start Menu shortcuts.
  - **macOS**: Universal `.app` bundle & `.dmg` supporting both Apple Silicon (M1/M2/M3/M4) and Intel Macs.
  - **Linux**: Standalone static binary and `.tar.gz` bundle with `.desktop` launcher.
- **Modern & Sober (No AI-Slop)**:
  - Clean, high-contrast, tactile UI with large readable controls.
  - **Smart feeder detection**: Automatically selects Flatbed Glass and greys out the feeder option with a reassuring notice when no sheets are loaded.
  - **Guided 2-sided duplex scanning**: Step-by-step physical guide (rotate stack 180° flat like a steering wheel — no upside-down confusion) with automatic page interleaving (1, 2, 3...).
  - **Drag-and-drop page reordering** and high-resolution inspection modal.
- **1-Click Hardware Diagnostics**:
  - Probes scanner endpoints (eSCL, WSD, Port 9400, SNMP) and generates a formatted Markdown report.
  - 1-click **"Copy Diagnostic Report"** and **"Save Report (.json)"** so users can report device quirks to developers or AI assistants for instant patch creation.

---

## Building Locally

### Prerequisites
- Go 1.24+ (Windows, macOS, or Linux)
- (Optional on Windows) Inno Setup 6 for compiling `UniverScan-Setup.exe`

### Build All Platforms
```powershell
# Windows PowerShell:
.\scripts\build_all.ps1

# Linux / macOS Bash:
./scripts/build_all.sh
```

Compiled binaries will be generated in `bin/`:
- `bin/universcan.exe` (Windows x86_64)
- `bin/universcan-darwin-arm64` (macOS Apple Silicon)
- `bin/universcan-darwin-amd64` (macOS Intel)
- `bin/universcan-linux-amd64` (Linux x86_64)

The Windows installer is generated at:
- `dist/windows/UniverScan-Setup.exe`

---

## Automated GitHub Actions Releases

This repository includes fully automated GitHub Actions workflows:

### 1. CI Validation (`.github/workflows/ci.yml`)
Runs automated Go unit tests and build verification across `ubuntu-latest`, `windows-latest`, and `macos-latest` on every push and pull request.

### 2. Multi-Platform Release (`.github/workflows/release.yml`)
Triggered automatically on git tags (e.g. `v0.1.0`) or via manual **Run workflow** in the GitHub Actions tab.

It automatically:
1. Compiles optimized binaries for Windows, macOS (Universal ARM64+Intel), and Linux.
2. Compiles `UniverScan-Setup.exe` using Inno Setup on Windows runners.
3. Packages `UniverScan-macOS.dmg` and `UniverScan-macOS.zip` on macOS runners.
4. Packages `universcan-linux-x86_64.tar.gz` on Linux runners.
5. Computes `SHA256SUMS.txt` checksums.
6. Publishes a complete GitHub Release with all binary assets and release notes attached.

### How to Release a New Version
```bash
git tag v0.1.0
git push origin v0.1.0
```
Or navigate to **Actions** → **Release Multi-Platform Packages** → **Run workflow** in the GitHub web interface.
