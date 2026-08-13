$ErrorActionPreference = "Stop"
$source = Split-Path -Parent $PSScriptRoot
$tools = Split-Path -Parent $source
$repository = Split-Path -Parent $tools
$bridge = Join-Path $tools "Bridge"
$startBridge = Join-Path $repository "START_BRIDGE.exe"
$startBridgeAndBeamNG = Join-Path $repository "START_BRIDGE_AND_BEAMNG.exe"

New-Item -ItemType Directory -Force -Path $bridge | Out-Null

Push-Location $source
try {
  go test ./...

  $env:GOOS = "windows"
  $env:GOARCH = "amd64"
  $env:CGO_ENABLED = "0"

  go build -trimpath -ldflags "-s -w" -tags bluetooth -o (Join-Path $bridge "EnhancedPS5DualSenseHapticsBluetooth.exe") .
  go build -trimpath -ldflags "-s -w" -tags usb -o (Join-Path $bridge "EnhancedPS5DualSenseHapticsUSB.exe") .

  # One launcher implementation and one build. The executable selects its mode
  # from its own filename, so the second public launcher is an identical copy.
  go build -trimpath -ldflags "-s -w" -o $startBridge ./launcher
  Copy-Item -LiteralPath $startBridge -Destination $startBridgeAndBeamNG -Force

  Write-Host "Windows bridge executables rebuilt in: $bridge"
  Write-Host "Public launchers rebuilt in: $repository"
} finally {
  Pop-Location
}
