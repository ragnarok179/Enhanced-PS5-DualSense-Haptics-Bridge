$ErrorActionPreference = 'Stop'
$release = 'V1.42'

$diagnosticsDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$rootDir = Split-Path -Parent $diagnosticsDir
$bridgeDir = Join-Path $rootDir 'Bridge'
$launcher = Join-Path $rootDir 'START_BRIDGE.exe'
$usb = Join-Path $bridgeDir 'EnhancedPS5DualSenseHapticsUSB.exe'
$bluetooth = Join-Path $bridgeDir 'EnhancedPS5DualSenseHapticsBluetooth.exe'
$logDir = Join-Path $diagnosticsDir 'Logs'

function Stop-ProcessTree([int]$ProcessId) {
    if ($ProcessId -le 0) { return }
    & taskkill.exe /PID $ProcessId /T /F 2>$null | Out-Null
}

function Require-BridgeFiles {
    if (-not (Test-Path -LiteralPath $launcher -PathType Leaf) -or
        -not (Test-Path -LiteralPath $usb -PathType Leaf) -or
        -not (Test-Path -LiteralPath $bluetooth -PathType Leaf)) {
        Write-Host 'ERROR: Bridge files are incomplete. Reinstall or update the Bridge.' -ForegroundColor Red
        exit 2
    }
}

function New-LogPath([string]$Prefix) {
    New-Item -ItemType Directory -Path $logDir -Force | Out-Null
    $stamp = Get-Date -Format 'yyyyMMdd_HHmmss'
    return Join-Path $logDir ($Prefix + '_' + $release + '_' + $stamp + '.log')
}

function Invoke-AutoBridge([string[]]$BridgeArgs) {
    Push-Location $rootDir
    try {
        & $launcher @BridgeArgs | Out-Host
        $code = $LASTEXITCODE
        if ($null -eq $code) { $code = 0 }
        return [int]$code
    }
    finally {
        Pop-Location
    }
}

function Invoke-LoggedAutoBridge([string[]]$BridgeArgs, [string]$Prefix, [string]$Purpose) {
    $logPath = New-LogPath $Prefix
    $started = Get-Date
    $stdoutPath = $logPath + '.stdout.tmp'
    $stderrPath = $logPath + '.stderr.tmp'

    Write-Host ''
    Write-Host $Purpose
    Write-Host 'The controller transport is detected automatically.'
    Write-Host 'Disconnecting and reconnecting the controller will not stop the diagnostic.'
    Write-Host ('Log: ' + $logPath)
    Write-Host ''
    Write-Host 'Press Enter when the test is finished.' -ForegroundColor Cyan

    $proc = Start-Process -FilePath $launcher -ArgumentList $BridgeArgs -WorkingDirectory $rootDir `
        -RedirectStandardOutput $stdoutPath -RedirectStandardError $stderrPath -PassThru -NoNewWindow
    try {
        [void](Read-Host)
    }
    finally {
        if (-not $proc.HasExited) {
            Stop-ProcessTree $proc.Id
            try { $proc.WaitForExit(3000) | Out-Null } catch {}
        }
    }

    $content = New-Object System.Collections.Generic.List[string]
    $content.Add('Enhanced PS5 DualSense Haptics ' + $release + ' - diagnostic log')
    $content.Add('Started: ' + $started.ToString('yyyy-MM-dd HH:mm:ss.fff'))
    $content.Add('Finished: ' + (Get-Date -Format 'yyyy-MM-dd HH:mm:ss.fff'))
    $content.Add('')
    if (Test-Path -LiteralPath $stdoutPath) {
        foreach ($line in (Get-Content -LiteralPath $stdoutPath -ErrorAction SilentlyContinue)) { $content.Add([string]$line) }
    }
    if (Test-Path -LiteralPath $stderrPath) {
        $stderr = @(Get-Content -LiteralPath $stderrPath -ErrorAction SilentlyContinue)
        if ($stderr.Count -gt 0) {
            $content.Add('')
            $content.Add('Errors:')
            foreach ($line in $stderr) { $content.Add([string]$line) }
        }
    }
    $content | Set-Content -LiteralPath $logPath -Encoding UTF8
    Remove-Item -LiteralPath $stdoutPath,$stderrPath -Force -ErrorAction SilentlyContinue

    if (Test-Path -LiteralPath $logPath -PathType Leaf) {
        $size = (Get-Item -LiteralPath $logPath).Length
        Write-Host ('Diagnostic log saved (' + $size + ' bytes).') -ForegroundColor Green
        Write-Host $logPath
        return 0
    }

    Write-Host 'ERROR: the diagnostic log could not be created.' -ForegroundColor Red
    return 4
}

function Invoke-HardwareSummary {
    $logPath = New-LogPath 'HARDWARE'
    $lines = New-Object System.Collections.Generic.List[string]
    $lines.Add('Enhanced PS5 DualSense Haptics ' + $release + ' - hardware diagnostic')
    $lines.Add('Started: ' + (Get-Date -Format 'yyyy-MM-dd HH:mm:ss.fff'))
    $lines.Add('')

    $lines.Add('USB controller probe:')
    $out = & $usb --probe 2>&1
    $code = $LASTEXITCODE
    if ($out) { $lines.AddRange([string[]]$out) }
    $lines.Add('Result: ' + $(if ($code -eq 0) { 'detected' } elseif ($code -eq 1) { 'not detected' } else { 'error' }))
    $lines.Add('')

    $lines.Add('Bluetooth controller probe:')
    $out = & $bluetooth --probe 2>&1
    $code = $LASTEXITCODE
    if ($out) { $lines.AddRange([string[]]$out) }
    $lines.Add('Result: ' + $(if ($code -eq 0) { 'detected' } elseif ($code -eq 1) { 'not detected' } else { 'error' }))
    $lines.Add('')

    $lines.Add('USB audio endpoints:')
    $out = & $usb --list-audio 2>&1
    if ($out) { $lines.AddRange([string[]]$out) }
    $lines.Add('')

    $lines.Add('Windows controller devices:')
    try {
        $devices = Get-PnpDevice -PresentOnly -ErrorAction SilentlyContinue |
            Where-Object { $_.FriendlyName -match 'DualSense|Wireless Controller|Sony Interactive Entertainment' -or $_.InstanceId -match 'VID_054C' } |
            Sort-Object Class,FriendlyName |
            Format-Table -AutoSize Class,Status,FriendlyName,InstanceId |
            Out-String -Width 240
        $lines.Add($devices.TrimEnd())
    }
    catch {
        $lines.Add('Windows device information is unavailable.')
    }

    $lines | Set-Content -LiteralPath $logPath -Encoding UTF8
    Write-Host ''
    Write-Host 'Hardware diagnostic saved.' -ForegroundColor Green
    Write-Host $logPath
    return 0
}

Require-BridgeFiles

while ($true) {
    Clear-Host
    Write-Host ('Enhanced PS5 DualSense Haptics ' + $release + ' - Diagnostics')
    Write-Host 'USB / Bluetooth selection is automatic.'
    Write-Host ''
    Write-Host '1. General support diagnostic'
    Write-Host '2. Haptics left / right / center test'
    Write-Host '3. Extended inputs test'
    Write-Host '4. Motion inputs test'
    Write-Host '5. Touchpad mouse test'
    Write-Host '6. Touchpad / BeamNG binding diagnostic'
    Write-Host '7. Hardware diagnostic'
    Write-Host '8. Audio output diagnostic'
    Write-Host '9. Feel profile information'
    Write-Host '0. Exit'
    Write-Host ''

    $choice = Read-Host 'Choose a diagnostic'
    $code = 0

    switch ($choice) {
        '1' {
            $code = Invoke-LoggedAutoBridge @('--diagnostic-status') 'GENERAL' 'General diagnostic is running. Reproduce the problem in BeamNG.'
        }
        '2' {
            Write-Host ''
            Write-Host 'The connected controller is detected automatically.'
            $code = Invoke-AutoBridge @('--test-stereo')
            [void](Read-Host 'Press Enter to continue')
        }
        '3' {
            Write-Host ''
            Write-Host 'Use the touchpad and controller inputs when requested.'
            $code = Invoke-AutoBridge @('--test-extended-inputs')
            [void](Read-Host 'Press Enter to continue')
        }
        '4' {
            Write-Host ''
            Write-Host 'Follow the motion test instructions shown below.'
            $code = Invoke-AutoBridge @('--test-motion-inputs')
            [void](Read-Host 'Press Enter to continue')
        }
        '5' {
            Write-Host ''
            Write-Host 'The touchpad will control the Windows pointer during this test.'
            Write-Host 'Press Ctrl+C when finished.'
            $code = Invoke-AutoBridge @('--test-touchpad-mouse')
            [void](Read-Host 'Press Enter to continue')
        }
        '6' {
            $code = Invoke-LoggedAutoBridge @('--diagnostic-touchpad-binding') 'TOUCHPAD_BINDING' 'Touchpad / BeamNG binding diagnostic is running.'
        }
        '7' {
            $code = Invoke-HardwareSummary
            [void](Read-Host 'Press Enter to continue')
        }
        '8' {
            Write-Host ''
            & $usb --list-audio
            $code = $LASTEXITCODE
            [void](Read-Host 'Press Enter to continue')
        }
        '9' {
            Write-Host ''
            $code = Invoke-AutoBridge @('--show-feel-profile')
            [void](Read-Host 'Press Enter to continue')
        }
        '0' { exit 0 }
        default { continue }
    }

    if ($code -ne 0) {
        Write-Host ('Diagnostic finished with an error (' + $code + ').') -ForegroundColor Yellow
        Start-Sleep -Seconds 1
    }
}
