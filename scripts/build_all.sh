#!/usr/bin/env bash
# UniverScan Cross-Platform Multi-Target Build Script (Bash)
# Builds standalone static binaries for Windows, macOS (Apple Silicon & Intel), and Linux.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
BIN_DIR="${ROOT_DIR}/bin"
DIST_DIR="${ROOT_DIR}/dist"

echo "=== Building UniverScan Standalone Binaries ==="
echo "Project Root: ${ROOT_DIR}"

mkdir -p "${BIN_DIR}" "${DIST_DIR}"

# Ensure web assets are synced into cmd/universcan/web for go:embed
mkdir -p "${ROOT_DIR}/cmd/universcan/web"
cp -r "${ROOT_DIR}/web/"* "${ROOT_DIR}/cmd/universcan/web/"

LDFLAGS="-s -w -X main.Version=0.1.0"

# 1. Windows x86_64
echo "--> Compiling Windows x86_64 (bin/universcan.exe)..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -buildvcs=false -ldflags="${LDFLAGS}" -o "${BIN_DIR}/universcan.exe" "${ROOT_DIR}/cmd/universcan"

# 2. macOS Apple Silicon (ARM64)
echo "--> Compiling macOS Apple Silicon (bin/universcan-darwin-arm64)..."
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -buildvcs=false -ldflags="${LDFLAGS}" -o "${BIN_DIR}/universcan-darwin-arm64" "${ROOT_DIR}/cmd/universcan"

# 3. macOS Intel (x86_64)
echo "--> Compiling macOS Intel (bin/universcan-darwin-amd64)..."
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -buildvcs=false -ldflags="${LDFLAGS}" -o "${BIN_DIR}/universcan-darwin-amd64" "${ROOT_DIR}/cmd/universcan"

# 4. Linux x86_64
echo "--> Compiling Linux x86_64 (bin/universcan-linux-amd64)..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false -ldflags="${LDFLAGS}" -o "${BIN_DIR}/universcan-linux-amd64" "${ROOT_DIR}/cmd/universcan"

echo ""
echo "=== Compiled Binaries in ${BIN_DIR} ==="
ls -lh "${BIN_DIR}"

# 5. Linux Packaging (if running on Linux)
if [[ "$(uname -s)" == "Linux" ]]; then
    if [[ -x "${ROOT_DIR}/packaging/linux/build_appimage.sh" ]]; then
        echo "--> Running Linux packaging script..."
        bash "${ROOT_DIR}/packaging/linux/build_appimage.sh"
    fi
fi

# 6. macOS Packaging (if running on macOS)
if [[ "$(uname -s)" == "Darwin" ]]; then
    if [[ -x "${ROOT_DIR}/packaging/macos/build_app.sh" ]]; then
        echo "--> Running macOS packaging script..."
        bash "${ROOT_DIR}/packaging/macos/build_app.sh"
    fi
fi

echo ""
echo "Build complete successfully!"
