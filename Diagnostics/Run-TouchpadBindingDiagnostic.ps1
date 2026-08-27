param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('USB', 'Bluetooth')]
    [string]$Mode
)

$ErrorActionPreference = 'Stop'

$diagnosticsDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$rootDir = Split-Path -Parent $diagnosticsDir
$bridgeDir = Join-Path $rootDir 'Bridge'
$logDir = Join-Path $diagnosticsDir 'Logs'

if ($Mode -eq 'USB') {
    $exePath = Join-Path $bridgeDir 'EnhancedPS5DualSenseHapticsUSB.exe'
    $bridgeArgs = @('--diagnostic-touchpad-binding')
    $logPrefix = 'TOUCHPAD_BINDING_USB'
    $title = 'DualSense touchpad / BeamNG binding diagnostic - USB'
} else {
    $exePath = Join-Path $bridgeDir 'EnhancedPS5DualSenseHapticsBluetooth.exe'
    $bridgeArgs = @('--diagnostic-touchpad-binding')
    $logPrefix = 'TOUCHPAD_BINDING_BLUETOOTH'
    $title = 'DualSense touchpad / BeamNG binding diagnostic - Bluetooth'
}

if (-not (Test-Path -LiteralPath $exePath -PathType Leaf)) {
    Write-Host "ERROR: Bridge executable not found:" -ForegroundColor Red
    Write-Host $exePath
    exit 2
}

New-Item -ItemType Directory -Path $logDir -Force | Out-Null
$stamp = Get-Date -Format 'yyyyMMdd_HHmmss'
$logPath = Join-Path $logDir ($logPrefix + '_' + $stamp + '.log')

$header = @(
    $title,
    ('-' * $title.Length),
    ('Started: ' + (Get-Date -Format 'yyyy-MM-dd HH:mm:ss.fff')),
    ('Executable: ' + $exePath),
    ('Arguments: ' + ($bridgeArgs -join ' ')),
    '',
    'Keep this window open while testing in BeamNG.',
    '1. Open Options > Controls > Bindings.',
    '2. Open Add/Edit Binding so BeamNG is waiting for an input.',
    '3. Touch/tap the DualSense touchpad.',
    '4. Close/cancel the binding window, then test the mouse again.',
    '',
    ('Log file: ' + $logPath),
    ''
)

$header | Tee-Object -FilePath $logPath

$exitCode = 0
Push-Location $bridgeDir
try {
    & $exePath @bridgeArgs 2>&1 | Tee-Object -FilePath $logPath -Append
    $exitCode = $LASTEXITCODE
    if ($null -eq $exitCode) { $exitCode = 0 }
}
catch {
    ('DIAGNOSTIC LAUNCH ERROR: ' + $_.Exception.Message) | Tee-Object -FilePath $logPath -Append
    $exitCode = 3
}
finally {
    Pop-Location
}

'' | Tee-Object -FilePath $logPath -Append
('Finished: ' + (Get-Date -Format 'yyyy-MM-dd HH:mm:ss.fff')) | Tee-Object -FilePath $logPath -Append
('Exit code: ' + $exitCode) | Tee-Object -FilePath $logPath -Append

Write-Host ''
if (Test-Path -LiteralPath $logPath -PathType Leaf) {
    $size = (Get-Item -LiteralPath $logPath).Length
    Write-Host ('Log saved successfully (' + $size + ' bytes):') -ForegroundColor Green
    Write-Host $logPath
} else {
    Write-Host 'ERROR: diagnostic ended but the log file was not created.' -ForegroundColor Red
    $exitCode = 4
}

exit $exitCode
