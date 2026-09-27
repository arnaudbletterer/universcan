#!/usr/bin/env bash
set -euo pipefail

# PrismScan macOS .app Bundle and DMG Packaging Script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
DIST_DIR="${ROOT_DIR}/dist/macos"
APP_NAME="PrismScan.app"
APP_BUNDLE="${DIST_DIR}/${APP_NAME}"
CONTENTS="${APP_BUNDLE}/Contents"
MACOS_DIR="${CONTENTS}/MacOS"
RESOURCES_DIR="${CONTENTS}/Resources"

echo "=== Packaging PrismScan for macOS ==="
mkdir -p "${MACOS_DIR}" "${RESOURCES_DIR}"

# Select binary or build universal binary if both architectures exist
if [[ -f "${ROOT_DIR}/bin/prismscan-darwin-arm64" && -f "${ROOT_DIR}/bin/prismscan-darwin-amd64" ]] && command -v lipo &>/dev/null; then
    echo "Creating Universal Binary (ARM64 + AMD64) with lipo..."
    lipo -create -output "${MACOS_DIR}/prismscan" "${ROOT_DIR}/bin/prismscan-darwin-arm64" "${ROOT_DIR}/bin/prismscan-darwin-amd64"
    chmod +x "${MACOS_DIR}/prismscan"
elif [[ -f "${ROOT_DIR}/bin/prismscan-darwin-arm64" ]]; then
    echo "Using ARM64 binary..."
    cp "${ROOT_DIR}/bin/prismscan-darwin-arm64" "${MACOS_DIR}/prismscan"
    chmod +x "${MACOS_DIR}/prismscan"
elif [[ -f "${ROOT_DIR}/bin/prismscan-darwin-amd64" ]]; then
    echo "Using AMD64 binary..."
    cp "${ROOT_DIR}/bin/prismscan-darwin-amd64" "${MACOS_DIR}/prismscan"
    chmod +x "${MACOS_DIR}/prismscan"
elif [[ -f "${ROOT_DIR}/bin/prismscan" ]]; then
    cp "${ROOT_DIR}/bin/prismscan" "${MACOS_DIR}/prismscan"
    chmod +x "${MACOS_DIR}/prismscan"
else
    echo "Error: No macOS binary found in ${ROOT_DIR}/bin."
    echo "Run scripts/build_all.sh first."
    exit 1
fi

# Copy Info.plist
cp "${SCRIPT_DIR}/Info.plist" "${CONTENTS}/Info.plist"

# Copy logo SVG to Resources
if [[ -f "${ROOT_DIR}/web/assets/logo.svg" ]]; then
    cp "${ROOT_DIR}/web/assets/logo.svg" "${RESOURCES_DIR}/AppIcon.svg"
fi

# Optional ad-hoc codesign if running on macOS
if command -v codesign &>/dev/null; then
    echo "Performing ad-hoc codesigning..."
    codesign --force --deep --sign - "${APP_BUNDLE}"
fi

# Create distribution ZIP archive
echo "Creating ZIP archive ${DIST_DIR}/PrismScan-macOS.zip..."
(cd "${DIST_DIR}" && zip -r -q "PrismScan-macOS.zip" "${APP_NAME}")

# Create DMG if hdiutil is available (native on macOS)
if command -v hdiutil &>/dev/null; then
    echo "Creating DMG archive ${DIST_DIR}/PrismScan-macOS.dmg..."
    hdiutil create -volname "PrismScan" -srcfolder "${APP_BUNDLE}" -ov -format UDZO "${DIST_DIR}/PrismScan-macOS.dmg"
fi

echo "=== macOS Packaging Complete ==="
echo "App bundle: ${APP_BUNDLE}"
echo "Zip archive: ${DIST_DIR}/PrismScan-macOS.zip"
