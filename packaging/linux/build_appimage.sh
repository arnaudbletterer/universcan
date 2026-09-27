#!/usr/bin/env bash
set -euo pipefail

# UniverScan Linux AppImage & Tarball Packaging Script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
DIST_DIR="${ROOT_DIR}/dist/linux"
APP_DIR="${DIST_DIR}/AppDir"

echo "=== Packaging UniverScan for Linux ==="
rm -rf "${APP_DIR}"
mkdir -p "${APP_DIR}/usr/bin"
mkdir -p "${APP_DIR}/usr/share/applications"
mkdir -p "${APP_DIR}/usr/share/icons/hicolor/scalable/apps"

# Copy binary
BINARY_SRC="${ROOT_DIR}/bin/universcan-linux-amd64"
if [[ ! -f "${BINARY_SRC}" ]]; then
    if [[ -f "${ROOT_DIR}/bin/universcan" ]]; then
        BINARY_SRC="${ROOT_DIR}/bin/universcan"
    else
        echo "Error: Linux binary ${BINARY_SRC} not found. Run scripts/build_all.sh first."
        exit 1
    fi
fi

cp "${BINARY_SRC}" "${APP_DIR}/usr/bin/universcan"
chmod +x "${APP_DIR}/usr/bin/universcan"

# Desktop integration files
cp "${SCRIPT_DIR}/universcan.desktop" "${APP_DIR}/usr/share/applications/universcan.desktop"
cp "${SCRIPT_DIR}/universcan.desktop" "${APP_DIR}/universcan.desktop"
if [[ -f "${ROOT_DIR}/web/assets/logo.svg" ]]; then
    cp "${ROOT_DIR}/web/assets/logo.svg" "${APP_DIR}/usr/share/icons/hicolor/scalable/apps/universcan.svg"
    cp "${ROOT_DIR}/web/assets/logo.svg" "${APP_DIR}/universcan.svg"
fi

# Create AppRun launcher script
cat > "${APP_DIR}/AppRun" << 'EOF'
#!/bin/sh
SELF=$(readlink -f "$0")
HERE=${SELF%/*}
export PATH="${HERE}/usr/bin:${PATH}"
exec "${HERE}/usr/bin/universcan" "$@"
EOF
chmod +x "${APP_DIR}/AppRun"

# Package tar.gz bundle
echo "Creating tarball ${DIST_DIR}/universcan-linux-x86_64.tar.gz..."
tar -czf "${DIST_DIR}/universcan-linux-x86_64.tar.gz" -C "${APP_DIR}" .

# If appimagetool is installed on system, build AppImage
if command -v appimagetool &>/dev/null; then
    echo "Building standalone AppImage with appimagetool..."
    appimagetool "${APP_DIR}" "${DIST_DIR}/UniverScan-x86_64.AppImage"
    echo "Created AppImage: ${DIST_DIR}/UniverScan-x86_64.AppImage"
else
    echo "Note: appimagetool not found on PATH. Tarball created at ${DIST_DIR}/universcan-linux-x86_64.tar.gz"
fi

echo "=== Linux Packaging Complete ==="
