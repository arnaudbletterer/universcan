# UniverScan 📄✨

**The simple, universal document scanner for everyone — from kids to grandparents.**

[![GitHub Release](https://img.shields.io/github/v/release/arnaudbletterer/universcan?style=flat-square)](https://github.com/arnaudbletterer/universcan/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg?style=flat-square)](LICENSE)
[![Platform](https://img.shields.io/badge/Platforms-Windows%20%7C%20macOS%20%7C%20Linux-lightgrey?style=flat-square)](https://github.com/arnaudbletterer/universcan/releases)
[![Privacy](https://img.shields.io/badge/Privacy-100%25%20Local%20%26%20Offline-success?style=flat-square)](SECURITY.md)

---

## What is UniverScan?

Most scanner software is complicated, cluttered with dozens of technical options, or requires installing massive driver packages.

**UniverScan makes scanning delightfully simple.** It works driverless over your home network with almost any brand of scanner (Samsung, HP, Canon, Epson, Brother, Xerox, Lexmark, Kyocera, and more). It runs completely on your own computer with zero cloud telemetry, no accounts, and no subscriptions.

```
[ 1. Put Paper In ]  ──▶  [ 2. Click "Scan Now" ]  ──▶  [ 3. Click "Save as PDF" ]
```

---

## 🚀 Quick Download

| Operating System | Download Link | Compatibility Notes |
| :--- | :--- | :--- |
| **Windows** | [**Download UniverScan-Setup.exe**](https://github.com/arnaudbletterer/universcan/releases/latest/download/UniverScan-Setup.exe) | Single installer for **Windows 11, 10, 8, & Windows 7** (both 64-bit and 32-bit). No admin password needed! |
| **macOS (Apple)** | [**Download UniverScan-macOS.dmg**](https://github.com/arnaudbletterer/universcan/releases/latest/download/UniverScan-macOS.dmg) | Universal app for **Apple Silicon** (M1/M2/M3/M4) and **Intel Macs**. |
| **Linux** | [**Download Linux Package**](https://github.com/arnaudbletterer/universcan/releases/latest) | Standalone static binaries for **64-bit x86**, **32-bit legacy PCs**, and **Raspberry Pi ARM64**. Zero library locks! |

---

## ✨ Key Features

- **Just One Click to Scan:** Place your sheets in the top tray or on the glass, and click **Scan Now**.
- **Works with Older and Newer Computers:** From modern Windows 11 PCs and Apple Silicon MacBooks to 15-year-old Windows 7 netbooks and Raspberry Pi boards.
- **Smart Tray Sensing:** Automatically detects when the top tray has paper. If it's empty, it smoothly switches to the scanner glass under the lid with a reassuring notice.
- **Guided 2-Sided Scanning:** If your feeder only scans one side, UniverScan guides you with a picture to turn the stack like a steering wheel (180° flat) and automatically interleaves the pages in exact order (**1, 2, 3...**).
- **Drag & Drop Page Reordering:** Rotate pages with one click or drag pages to reorder them before saving.
- **Collision-Free Auto-Naming:** Every scan is saved neatly with the current date and time (e.g. `Scan_2026-09-27_1015.pdf`) in your personal `Documents/Scans` folder.
- **1-Click Help Tool:** If your scanner isn't responding, click **Help** > **Copy Help Information**. You can paste it into an email or message to a friend, technician, or AI assistant to solve the problem instantly.
- **100% Offline & Private:** Zero tracking, zero cloud servers, zero ads. Your documents stay safe on your computer.

---

## 📖 Documentation

Read our friendly, jargon-free user manual built with Zensical:
- [Getting Started](docs/getting-started.md)
- [How to Scan](docs/how-to-scan.md)
- [2-Sided Scanning Guide](docs/two-sided-guide.md)
- [Supported Scanner Brands](docs/supported-scanners.md)
- [Problem Solver & Help](docs/troubleshooting.md)
- [Building from Source](docs/developer-guide.md)

---

## 🛠️ Building from Source

UniverScan is written in modern Go with embedded web assets and zero runtime dependencies.

```bash
# Clone the repository
git clone https://github.com/arnaudbletterer/universcan.git
cd universcan

# Run all automated tests
go test -v ./...

# Run the app locally
go run ./cmd/universcan
```

### Compile All Platforms & Architectures
```powershell
# On Windows (PowerShell):
.\scripts\build_all.ps1

# On Linux or macOS (Bash):
./scripts/build_all.sh
```

---

## 🤝 Community & Contributing

UniverScan is free and open-source software built for the entire world. Contributions, translations, bug reports, and scanner compatibility tests are warmly welcomed!

- Read our [Contributing Guidelines](CONTRIBUTING.md) to learn how to help.
- Please adhere to our [Code of Conduct](CODE_OF_CONDUCT.md).
- Read our [Security Policy](SECURITY.md) for details on privacy and vulnerability reporting.

---

## 📄 License

This project is licensed under the permissive **[MIT License](LICENSE)**. You are free to use, share, modify, and distribute it anywhere in the world.
