$ErrorActionPreference = "Stop"
$source = Split-Path -Parent $PSScriptRoot
$tools = Split-Path -Parent $source
$bridge = Join-Path $tools "Bridge"
New-Item -ItemType Directory -Force -Path $bridge | Out-Null
Push-Location $source
try {
  go test ./...
  $env:GOOS = "windows"
  $env:GOARCH = "amd64"
  $env:CGO_ENABLED = "0"
  go build -trimpath -ldflags "-s -w" -tags bluetooth -o (Join-Path $bridge "EnhancedPS5DualSenseHapticsBluetooth.exe") .
  go build -trimpath -ldflags "-s -w" -tags usb -o (Join-Path $bridge "EnhancedPS5DualSenseHapticsUSB.exe") .
  Write-Host "Windows executables rebuilt in: $bridge"
} finally {
  Pop-Location
}
