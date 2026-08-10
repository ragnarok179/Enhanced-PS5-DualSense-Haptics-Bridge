$ErrorActionPreference = "Stop"
$source = Split-Path -Parent $PSScriptRoot
Push-Location $source
try {
  go test ./...
  $env:GOOS = "windows"
  $env:GOARCH = "amd64"
  $env:CGO_ENABLED = "0"
  go vet -tags bluetooth ./...
  go test -c -tags bluetooth -o $env:TEMP\DualSensePhysicsHaptics_tests_bluetooth.exe .
  go test -c -tags usb -o $env:TEMP\DualSensePhysicsHaptics_tests_usb.exe .
  Remove-Item -Force -ErrorAction SilentlyContinue $env:TEMP\DualSensePhysicsHaptics_tests_bluetooth.exe
  Remove-Item -Force -ErrorAction SilentlyContinue $env:TEMP\DualSensePhysicsHaptics_tests_usb.exe
  Write-Host "All tests completed successfully."
} finally {
  Pop-Location
}
