# PrismScan Cross-Platform Multi-Target Build Script (PowerShell)
# Builds standalone static binaries for Windows, macOS (Apple Silicon & Intel), and Linux.

$ErrorActionPreference = "Stop"

$RootDir = Split-Path -Parent $PSScriptRoot
$BinDir = Join-Path $RootDir "bin"
$DistDir = Join-Path $RootDir "dist"

Write-Host "=== Building PrismScan Standalone Binaries ===" -ForegroundColor Cyan
Write-Host "Project Root: $RootDir"

if (-not (Test-Path $BinDir)) {
    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
}

# Ensure web assets are synced into cmd/prismscan/web for go:embed
$CmdWebDir = Join-Path $RootDir "cmd\prismscan\web"
if (-not (Test-Path $CmdWebDir)) {
    New-Item -ItemType Directory -Force -Path $CmdWebDir | Out-Null
}
Copy-Item -Recurse -Force (Join-Path $RootDir "web\*") $CmdWebDir

# Go flags for compact, stripped binaries with embedded assets
$LdFlags = "-s -w -X main.Version=2.0.0"

# 1. Windows x86_64
Write-Host "--> Compiling Windows x86_64 (bin/prismscan.exe)..." -ForegroundColor Yellow
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -buildvcs=false -ldflags $LdFlags -o "$BinDir/prismscan.exe" "$RootDir/cmd/prismscan"
if ($LASTEXITCODE -ne 0) { throw "Windows build failed" }

# 2. macOS Apple Silicon (ARM64)
Write-Host "--> Compiling macOS Apple Silicon (bin/prismscan-darwin-arm64)..." -ForegroundColor Yellow
$env:GOOS = "darwin"
$env:GOARCH = "arm64"
$env:CGO_ENABLED = "0"
go build -buildvcs=false -ldflags $LdFlags -o "$BinDir/prismscan-darwin-arm64" "$RootDir/cmd/prismscan"
if ($LASTEXITCODE -ne 0) { throw "macOS ARM64 build failed" }

# 3. macOS Intel (x86_64)
Write-Host "--> Compiling macOS Intel (bin/prismscan-darwin-amd64)..." -ForegroundColor Yellow
$env:GOOS = "darwin"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -buildvcs=false -ldflags $LdFlags -o "$BinDir/prismscan-darwin-amd64" "$RootDir/cmd/prismscan"
if ($LASTEXITCODE -ne 0) { throw "macOS AMD64 build failed" }

# 4. Linux x86_64
Write-Host "--> Compiling Linux x86_64 (bin/prismscan-linux-amd64)..." -ForegroundColor Yellow
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -buildvcs=false -ldflags $LdFlags -o "$BinDir/prismscan-linux-amd64" "$RootDir/cmd/prismscan"
if ($LASTEXITCODE -ne 0) { throw "Linux build failed" }

# Reset environment variables
$env:GOOS = ""
$env:GOARCH = ""
$env:CGO_ENABLED = ""

Write-Host "`n=== Compiled Binaries in $BinDir ===" -ForegroundColor Green
Get-ChildItem -Path $BinDir | Select-Object Name, @{Name="Size (MB)"; Expression={ "{0:N2}" -f ($_.Length / 1MB) }}, LastWriteTime | Format-Table -AutoSize

# 5. Check for Inno Setup compiler (ISCC)
$IsccPaths = @(
    "iscc.exe",
    "${env:ProgramFiles(x86)}\Inno Setup 6\iscc.exe",
    "${env:ProgramFiles}\Inno Setup 6\iscc.exe",
    "${env:LOCALAPPDATA}\Programs\Inno Setup 6\iscc.exe"
)

$Iscc = $null
foreach ($path in $IsccPaths) {
    if (Get-Command $path -ErrorAction SilentlyContinue) {
        $Iscc = $path
        break
    }
    if (Test-Path $path) {
        $Iscc = $path
        break
    }
}

if ($Iscc) {
    Write-Host "`n--> Found Inno Setup at $Iscc. Building Windows Setup Installer..." -ForegroundColor Cyan
    $IssScript = Join-Path $RootDir "packaging\windows\installer.iss"
    & $Iscc $IssScript
    Write-Host "Windows Installer generated in $DistDir\windows" -ForegroundColor Green
} else {
    Write-Host "`n[Notice] Inno Setup compiler (ISCC) not found on PATH. To build PrismScan-Setup.exe, install Inno Setup 6 and compile packaging\windows\installer.iss." -ForegroundColor Gray
}

Write-Host "`nBuild complete successfully!" -ForegroundColor Green
