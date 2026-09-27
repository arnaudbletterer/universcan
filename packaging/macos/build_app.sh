#!/usr/bin/env bash
set -euo pipefail

# UniverScan macOS .app Bundle and DMG Packaging Script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
DIST_DIR="${ROOT_DIR}/dist/macos"
APP_NAME="UniverScan.app"
APP_BUNDLE="${DIST_DIR}/${APP_NAME}"
CONTENTS="${APP_BUNDLE}/Contents"
MACOS_DIR="${CONTENTS}/MacOS"
RESOURCES_DIR="${CONTENTS}/Resources"

echo "=== Packaging UniverScan for macOS ==="
mkdir -p "${MACOS_DIR}" "${RESOURCES_DIR}"

# Select binary or build universal binary if both architectures exist
if [[ -f "${ROOT_DIR}/bin/universcan-darwin-arm64" && -f "${ROOT_DIR}/bin/universcan-darwin-amd64" ]] && command -v lipo &>/dev/null; then
    echo "Creating Universal Binary (ARM64 + AMD64) with lipo..."
    lipo -create -output "${MACOS_DIR}/universcan" "${ROOT_DIR}/bin/universcan-darwin-arm64" "${ROOT_DIR}/bin/universcan-darwin-amd64"
    chmod +x "${MACOS_DIR}/universcan"
elif [[ -f "${ROOT_DIR}/bin/universcan-darwin-arm64" ]]; then
    echo "Using ARM64 binary..."
    cp "${ROOT_DIR}/bin/universcan-darwin-arm64" "${MACOS_DIR}/universcan"
    chmod +x "${MACOS_DIR}/universcan"
elif [[ -f "${ROOT_DIR}/bin/universcan-darwin-amd64" ]]; then
    echo "Using AMD64 binary..."
    cp "${ROOT_DIR}/bin/universcan-darwin-amd64" "${MACOS_DIR}/universcan"
    chmod +x "${MACOS_DIR}/universcan"
elif [[ -f "${ROOT_DIR}/bin/universcan" ]]; then
    cp "${ROOT_DIR}/bin/universcan" "${MACOS_DIR}/universcan"
    chmod +x "${MACOS_DIR}/universcan"
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
echo "Creating ZIP archive ${DIST_DIR}/UniverScan-macOS.zip..."
(cd "${DIST_DIR}" && zip -r -q "UniverScan-macOS.zip" "${APP_NAME}")

# Create native Apple Installer Package (.pkg) if pkgbuild is available
if command -v pkgbuild &>/dev/null; then
    echo "Creating native macOS Installer Package ${DIST_DIR}/UniverScan-macOS.pkg..."
    pkgbuild --component "${APP_BUNDLE}" --install-location "/Applications" "${DIST_DIR}/UniverScan-macOS.pkg" || {
        echo "Warning: pkgbuild component failed, building root package..."
        STAGING_PKG="${DIST_DIR}/pkg_staging/Applications"
        mkdir -p "${STAGING_PKG}"
        cp -R "${APP_BUNDLE}" "${STAGING_PKG}/"
        pkgbuild --root "${DIST_DIR}/pkg_staging" --identifier "com.universcan.app" --version "0.1.0" "${DIST_DIR}/UniverScan-macOS.pkg"
        rm -rf "${DIST_DIR}/pkg_staging"
    }
fi

# Create Drag-to-Install DMG if hdiutil is available (native on macOS)
if command -v hdiutil &>/dev/null; then
    echo "Creating Drag-to-Install DMG ${DIST_DIR}/UniverScan-macOS.dmg..."
    DMG_STAGING="${DIST_DIR}/dmg_staging"
    rm -rf "${DMG_STAGING}"
    mkdir -p "${DMG_STAGING}"
    
    # Copy app bundle
    cp -R "${APP_BUNDLE}" "${DMG_STAGING}/"
    
    # Create symlink to /Applications for easy drag-to-install
    ln -s /Applications "${DMG_STAGING}/Applications"

    # Add quick install instructions file for beginners
    cat << 'EOF' > "${DMG_STAGING}/How to Install.txt"
UniverScan for macOS
====================

To install:
1. Drag the "UniverScan" app icon into the "Applications" folder right next to it.
2. Open UniverScan from your Applications folder or Launchpad.

Enjoy simple, universal document scanning!
EOF

    hdiutil create -volname "UniverScan" -srcfolder "${DMG_STAGING}" -ov -format UDZO "${DIST_DIR}/UniverScan-macOS.dmg"
    rm -rf "${DMG_STAGING}"
fi

echo "=== macOS Packaging Complete ==="
echo "App bundle: ${APP_BUNDLE}"
echo "Zip archive: ${DIST_DIR}/UniverScan-macOS.zip"
[[ -f "${DIST_DIR}/UniverScan-macOS.pkg" ]] && echo "Pkg installer: ${DIST_DIR}/UniverScan-macOS.pkg"
[[ -f "${DIST_DIR}/UniverScan-macOS.dmg" ]] && echo "DMG archive: ${DIST_DIR}/UniverScan-macOS.dmg"
