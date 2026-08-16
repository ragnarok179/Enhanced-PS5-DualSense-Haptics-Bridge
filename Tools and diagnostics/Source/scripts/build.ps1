$ErrorActionPreference = "Stop"
$source = Split-Path -Parent $PSScriptRoot
$tools = Split-Path -Parent $source
$repository = Split-Path -Parent $tools
$bridge = Join-Path $tools "Bridge"
$startBridge = Join-Path $repository "START_BRIDGE.exe"
$startBridgeAndBeamNG = Join-Path $repository "START_BRIDGE_AND_BEAMNG.exe"
$updateBridge = Join-Path $repository "UPDATE_BRIDGE.exe"

New-Item -ItemType Directory -Force -Path $bridge | Out-Null

Push-Location $source
try {
  go test ./...

  $env:GOOS = "windows"
  $env:GOARCH = "amd64"
  $env:CGO_ENABLED = "0"

  go build -trimpath -buildvcs=false -ldflags "-s -w -buildid=" -tags bluetooth -o (Join-Path $bridge "EnhancedPS5DualSenseHapticsBluetooth.exe") .
  go build -trimpath -buildvcs=false -ldflags "-s -w -buildid=" -tags usb -o (Join-Path $bridge "EnhancedPS5DualSenseHapticsUSB.exe") .

  # One launcher implementation and one build. The executable selects its mode
  # from its own filename, so the second public launcher is an identical copy.
  go build -trimpath -buildvcs=false -ldflags "-s -w -buildid=" -o $startBridge ./launcher
  Copy-Item -LiteralPath $startBridge -Destination $startBridgeAndBeamNG -Force

  # The updater runs from a temporary copy of itself so it can safely replace
  # UPDATE_BRIDGE.exe during an update without a second helper executable.
  go build -trimpath -buildvcs=false -ldflags "-s -w -buildid=" -o $updateBridge ./updater

  Write-Host "Windows bridge executables rebuilt in: $bridge"
  Write-Host "Public launchers and updater rebuilt in: $repository"
} finally {
  Pop-Location
}
