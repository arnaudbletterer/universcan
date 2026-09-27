# Building from Source & Architecture 🛠️

UniverScan is engineered with modern Go, standard embedded web assets, and zero external runtime dependencies.

---

## Architectural Principles

1. **Zero External Runtime Dependencies:** No Python runtime, no NodeJS, no Electron, no GTK/Qt dynamic linking issues. A single static Go binary (`CGO_ENABLED=0`) contains both the multi-vendor scanning engine and the embedded web frontend.
2. **True Cross-Platform Binary Output:**
   - **Windows:** 64-bit (`amd64`) and 32-bit (`386` for legacy Windows 7/8/10 machines).
   - **macOS:** Apple Silicon (`arm64` M1-M4) and Intel (`amd64`).
   - **Linux:** 64-bit (`amd64`), 32-bit (`386`), and ARM64 (`arm64` for Raspberry Pi 3/4/5).
3. **Driverless Multi-Protocol Scanning:**
   - Apple AirScan / eSCL REST XML (`pkg/scanner/escl.go`)
   - Microsoft WS-Scan SOAP (`pkg/scanner/wsd.go`)
   - Samsung/Xerox Direct Port 9400 TCP (`pkg/scanner/samsung.go`)
   - SNMP Telemetry UDP Port 161 (`pkg/scanner/snmp.go`)
   - Automatic Negotiator (`pkg/scanner/universal.go`)

---

## Local Build Instructions

### Prerequisites
- [Go 1.22+](https://go.dev/dl/)
- (Optional for Windows installer) [Inno Setup 6](https://jrsoftware.org/isdl.php)

### Compiling Locally

```bash
# Clone the repository
git clone https://github.com/arnaudbletterer/universcan.git
cd universcan

# Run all unit and integration tests
go test -v ./...

# Run the app locally
go run ./cmd/universcan
```

### Multi-Target Cross-Compilation

```powershell
# On Windows (PowerShell):
.\scripts\build_all.ps1

# On Linux or macOS (Bash):
./scripts/build_all.sh
```

---

## Packaging & Distribution

- **Windows Setup:** Compiled with Inno Setup using `packaging/windows/installer.iss`. It bundles both 32-bit and 64-bit binaries in a single 4.5 MB executable that installs seamlessly without administrator privileges.
- **macOS App & DMG:** Packaged with `packaging/macos/build_app.sh` into a standalone `.app` bundle.
- **Linux Tarball:** Static binary tarball with desktop launcher icon.
- **Automated CI/CD:** GitHub Actions workflows in `.github/workflows/release.yml` automatically build, test, sign checksums, and publish release binaries whenever a `v*` tag is pushed.
