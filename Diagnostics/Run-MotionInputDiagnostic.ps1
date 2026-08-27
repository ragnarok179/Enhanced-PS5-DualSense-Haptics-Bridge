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
    $bridgeArgs = @('--test-motion-inputs')
    $logPrefix = 'MOTION_INPUTS_USB'
    $title = 'DualSense gyro / accelerometer diagnostic - USB'
} else {
    $exePath = Join-Path $bridgeDir 'EnhancedPS5DualSenseHapticsBluetooth.exe'
    $bridgeArgs = @('--test-motion-inputs')
    $logPrefix = 'MOTION_INPUTS_BLUETOOTH'
    $title = 'DualSense gyro / accelerometer diagnostic - Bluetooth'
}

if (-not (Test-Path -LiteralPath $exePath -PathType Leaf)) {
    Write-Host 'ERROR: Bridge executable not found:' -ForegroundColor Red
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
    'TEST SEQUENCE:',
    '1. Put the controller flat and do not touch it for about 5 seconds.',
    '2. Pitch: nose/front up then down, slowly, then faster.',
    '3. Yaw: rotate flat left then right, slowly, then faster.',
    '4. Roll: tilt left side then right side, slowly, then faster.',
    '5. Put it flat and still again for about 5 seconds.',
    '6. Leave the controller still and wait. The diagnostic stops automatically at 80 s and prints MOTION_DIAG_SUMMARY.',
    '',
    'The log contains raw IMU values, calibrated physical values, orientation, timing/jitter,',
    'dropout indicators, saturation counters, quiet-noise statistics and a final summary.',
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
    Write-Host ('Motion log saved successfully (' + $size + ' bytes):') -ForegroundColor Green
    Write-Host $logPath
    Write-Host ''
    Write-Host 'Attach this .log file when requesting support.' -ForegroundColor Cyan
} else {
    Write-Host 'ERROR: diagnostic ended but the log file was not created.' -ForegroundColor Red
    $exitCode = 4
}

exit $exitCode
