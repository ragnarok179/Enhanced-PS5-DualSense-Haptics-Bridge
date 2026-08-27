$ErrorActionPreference = 'Stop'
$source = Split-Path -Parent $PSScriptRoot
$repository = Split-Path -Parent $source
$bridge = Join-Path $repository 'Bridge'
$startBridge = Join-Path $repository 'START_BRIDGE.exe'
$startBridgeAndBeamNG = Join-Path $repository 'START_BRIDGE_AND_BEAMNG.exe'
$updateBridge = Join-Path $repository 'UPDATE_BRIDGE.exe'

New-Item -ItemType Directory -Force -Path $bridge | Out-Null

Push-Location $source
try {
    go test ./...
    go test -race ./...

    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'
    $env:CGO_ENABLED = '0'

    go build -trimpath -buildvcs=false -ldflags '-s -w -buildid=' -tags bluetooth -o (Join-Path $bridge 'EnhancedPS5DualSenseHapticsBluetooth.exe') .
    go build -trimpath -buildvcs=false -ldflags '-s -w -buildid=' -tags usb -o (Join-Path $bridge 'EnhancedPS5DualSenseHapticsUSB.exe') .
    go build -trimpath -buildvcs=false -ldflags '-s -w -buildid=' -o $startBridge ./launcher
    Copy-Item -LiteralPath $startBridge -Destination $startBridgeAndBeamNG -Force
    go build -trimpath -buildvcs=false -ldflags '-s -w -buildid=' -o $updateBridge ./updater

    Write-Host "Bridge V1.4 Windows binaries rebuilt in $repository"
} finally {
    Pop-Location
}
