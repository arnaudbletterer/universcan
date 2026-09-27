#!/usr/bin/env bash
# PrismScan Cross-Platform Multi-Target Build Script (Bash)
# Builds standalone static binaries for Windows, macOS (Apple Silicon & Intel), and Linux.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
BIN_DIR="${ROOT_DIR}/bin"
DIST_DIR="${ROOT_DIR}/dist"

echo "=== Building PrismScan Standalone Binaries ==="
echo "Project Root: ${ROOT_DIR}"

mkdir -p "${BIN_DIR}" "${DIST_DIR}"

# Ensure web assets are synced into cmd/prismscan/web for go:embed
mkdir -p "${ROOT_DIR}/cmd/prismscan/web"
cp -r "${ROOT_DIR}/web/"* "${ROOT_DIR}/cmd/prismscan/web/"

LDFLAGS="-s -w -X main.Version=2.0.0"

# 1. Windows x86_64
echo "--> Compiling Windows x86_64 (bin/prismscan.exe)..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o "${BIN_DIR}/prismscan.exe" "${ROOT_DIR}/cmd/prismscan"

# 2. macOS Apple Silicon (ARM64)
echo "--> Compiling macOS Apple Silicon (bin/prismscan-darwin-arm64)..."
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o "${BIN_DIR}/prismscan-darwin-arm64" "${ROOT_DIR}/cmd/prismscan"

# 3. macOS Intel (x86_64)
echo "--> Compiling macOS Intel (bin/prismscan-darwin-amd64)..."
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o "${BIN_DIR}/prismscan-darwin-amd64" "${ROOT_DIR}/cmd/prismscan"

# 4. Linux x86_64
echo "--> Compiling Linux x86_64 (bin/prismscan-linux-amd64)..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o "${BIN_DIR}/prismscan-linux-amd64" "${ROOT_DIR}/cmd/prismscan"

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
