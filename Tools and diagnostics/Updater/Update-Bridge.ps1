param(
    [Parameter(Mandatory = $true)]
    [string]$InstallRoot
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$RepoOwner = 'ragnarok179'
$RepoName = 'Enhanced-PS5-DualSense-Haptics-Bridge'
$Branch = 'main'
$ArchiveUrl = "https://github.com/$RepoOwner/$RepoName/archive/refs/heads/$Branch.zip"
$ManifestName = 'SHA256SUMS.txt'

function Write-Step([string]$Text) {
    Write-Host "[UPDATE] $Text"
}

function Normalize-RelativePath([string]$Path) {
    return $Path.Replace('/', '\')
}

function Read-ChecksumManifest([string]$Path) {
    $map = @{}
    foreach ($line in Get-Content -LiteralPath $Path -Encoding UTF8) {
        if ([string]::IsNullOrWhiteSpace($line)) { continue }
        if ($line -match '^([0-9a-fA-F]{64})\s+\./(.+)$') {
            $relative = Normalize-RelativePath $Matches[2].Trim()
            $map[$relative] = $Matches[1].ToLowerInvariant()
        }
    }
    if ($map.Count -eq 0) {
        throw "Checksum manifest is empty or invalid: $Path"
    }
    return $map
}

function Get-Sha256([string]$Path) {
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Verify-Package([string]$Root, [hashtable]$Manifest) {
    foreach ($entry in $Manifest.GetEnumerator()) {
        $path = Join-Path $Root $entry.Key
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
            throw "Package verification failed: missing '$($entry.Key)'."
        }
        $actual = Get-Sha256 $path
        if ($actual -ne $entry.Value) {
            throw "Package verification failed: SHA-256 mismatch for '$($entry.Key)'."
        }
    }
}

function Copy-WithParent([string]$Source, [string]$Destination) {
    $parent = Split-Path -Parent $Destination
    if ($parent -and -not (Test-Path -LiteralPath $parent)) {
        New-Item -ItemType Directory -Path $parent -Force | Out-Null
    }
    Copy-Item -LiteralPath $Source -Destination $Destination -Force
}

# Be defensive about paths coming from cmd.exe. A quoted argument whose original
# directory ended in a backslash can arrive with a stray quote on some Windows
# command-line parsing paths.
$InstallRoot = $InstallRoot.Trim().Trim('"')
if ([string]::IsNullOrWhiteSpace($InstallRoot)) {
    throw 'The installation folder path is empty.'
}
$InstallRoot = [System.IO.Path]::GetFullPath($InstallRoot)
$localManifestPath = Join-Path $InstallRoot $ManifestName

if (Test-Path -LiteralPath (Join-Path $InstallRoot '.git') -PathType Container) {
    Write-Host '[ERROR] This folder is a Git working copy. Use git pull instead of the public updater.'
    exit 4
}

$bridgeProcesses = @('EnhancedPS5DualSenseHapticsUSB', 'EnhancedPS5DualSenseHapticsBluetooth', 'START_BRIDGE', 'START_BRIDGE_AND_BEAMNG')
foreach ($processName in $bridgeProcesses) {
    if (Get-Process -Name $processName -ErrorAction SilentlyContinue) {
        Write-Host "[ERROR] The Bridge is currently running. Close it before updating."
        exit 3
    }
}

$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("EnhancedDualSenseUpdate_" + [Guid]::NewGuid().ToString('N'))
$downloadZip = Join-Path $tempRoot 'repository.zip'
$extractRoot = Join-Path $tempRoot 'extract'
$backupRoot = Join-Path $tempRoot 'backup'

try {
    New-Item -ItemType Directory -Path $tempRoot, $extractRoot, $backupRoot -Force | Out-Null

    Write-Step "Downloading the current GitHub $Branch branch..."
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    $headers = @{ 'User-Agent' = 'Enhanced-PS5-DualSense-Haptics-Updater' }
    Invoke-WebRequest -UseBasicParsing -Uri $ArchiveUrl -OutFile $downloadZip -Headers $headers

    Write-Step 'Extracting update package...'
    Expand-Archive -LiteralPath $downloadZip -DestinationPath $extractRoot -Force

    $remoteManifestFile = Get-ChildItem -LiteralPath $extractRoot -Filter $ManifestName -File -Recurse | Select-Object -First 1
    if (-not $remoteManifestFile) {
        throw "The downloaded repository does not contain $ManifestName. Update cancelled."
    }

    $remoteRoot = Split-Path -Parent $remoteManifestFile.FullName
    if (-not (Test-Path -LiteralPath (Join-Path $remoteRoot 'START_BRIDGE.exe'))) {
        throw 'The downloaded repository layout is not recognized. Update cancelled.'
    }

    # Do not let a local updater-enabled package downgrade itself against an older
    # GitHub main branch that does not contain the updater yet. This is especially
    # useful while preparing the first updater-enabled public release.
    $requiredUpdaterFiles = @(
        'UPDATE_BRIDGE.bat',
        'Tools and diagnostics\Updater\Update-Bridge.ps1'
    )
    $remoteUpdaterReady = $true
    foreach ($required in $requiredUpdaterFiles) {
        if (-not (Test-Path -LiteralPath (Join-Path $remoteRoot $required) -PathType Leaf)) {
            $remoteUpdaterReady = $false
            break
        }
    }
    if (-not $remoteUpdaterReady) {
        Write-Host '[INFO] The updater-enabled version is not published on GitHub main yet.'
        Write-Host '[INFO] No files were changed. Publish this package first, then run UPDATE_BRIDGE.bat again.'
        exit 0
    }

    $remoteManifest = Read-ChecksumManifest $remoteManifestFile.FullName
    Write-Step 'Verifying downloaded files with SHA-256...'
    Verify-Package $remoteRoot $remoteManifest

    $localManifest = @{}
    if (Test-Path -LiteralPath $localManifestPath -PathType Leaf) {
        $localManifest = Read-ChecksumManifest $localManifestPath
    }

    $newFiles = New-Object System.Collections.Generic.List[string]
    $changedFiles = New-Object System.Collections.Generic.List[string]
    $removedFiles = New-Object System.Collections.Generic.List[string]

    foreach ($relative in ($remoteManifest.Keys | Sort-Object)) {
        $localPath = Join-Path $InstallRoot $relative
        if (-not (Test-Path -LiteralPath $localPath -PathType Leaf)) {
            $newFiles.Add($relative)
            continue
        }
        if ((Get-Sha256 $localPath) -ne $remoteManifest[$relative]) {
            $changedFiles.Add($relative)
        }
    }

    foreach ($relative in ($localManifest.Keys | Sort-Object)) {
        if (-not $remoteManifest.ContainsKey($relative)) {
            $localPath = Join-Path $InstallRoot $relative
            if (Test-Path -LiteralPath $localPath -PathType Leaf) {
                $removedFiles.Add($relative)
            }
        }
    }

    if (($newFiles.Count + $changedFiles.Count + $removedFiles.Count) -eq 0) {
        Write-Host '[OK] The Bridge files are already up to date.'
        exit 0
    }

    Write-Host ''
    Write-Host 'Update found:'
    Write-Host "  New files:     $($newFiles.Count)"
    Write-Host "  Changed files: $($changedFiles.Count)"
    Write-Host "  Removed files: $($removedFiles.Count)"

    if ($newFiles.Count -gt 0) {
        Write-Host ''
        Write-Host 'New:'
        $newFiles | ForEach-Object { Write-Host "  + $_" }
    }
    if ($changedFiles.Count -gt 0) {
        Write-Host ''
        Write-Host 'Changed:'
        $changedFiles | ForEach-Object { Write-Host "  * $_" }
    }
    if ($removedFiles.Count -gt 0) {
        Write-Host ''
        Write-Host 'Obsolete managed files:'
        $removedFiles | ForEach-Object { Write-Host "  - $_" }
    }

    Write-Host ''
    Write-Host 'Your diagnostic logs and files that are not managed by SHA256SUMS.txt will not be deleted.'
    $answer = Read-Host 'Install this update? [Y/N]'
    if ($answer -notmatch '^(y|yes)$') {
        Write-Step 'Update cancelled by user.'
        exit 0
    }

    # Backup every managed local file that may be changed or removed.
    $toBackup = @()
    foreach ($relative in $changedFiles) { $toBackup += $relative }
    foreach ($relative in $removedFiles) { $toBackup += $relative }
    foreach ($relative in $toBackup) {
        $source = Join-Path $InstallRoot $relative
        if (Test-Path -LiteralPath $source -PathType Leaf) {
            Copy-WithParent $source (Join-Path $backupRoot $relative)
        }
    }
    if (Test-Path -LiteralPath $localManifestPath -PathType Leaf) {
        Copy-Item -LiteralPath $localManifestPath -Destination (Join-Path $backupRoot $ManifestName) -Force
    }

    $createdDuringUpdate = New-Object System.Collections.Generic.List[string]

    try {
        Write-Step 'Installing new and changed files...'
        foreach ($relative in ($remoteManifest.Keys | Sort-Object)) {
            $source = Join-Path $remoteRoot $relative
            $destination = Join-Path $InstallRoot $relative
            if (-not (Test-Path -LiteralPath $destination -PathType Leaf)) {
                $createdDuringUpdate.Add($relative)
            }
            Copy-WithParent $source $destination
        }

        Write-Step 'Removing obsolete managed files...'
        foreach ($relative in $removedFiles) {
            $target = Join-Path $InstallRoot $relative
            if (Test-Path -LiteralPath $target -PathType Leaf) {
                Remove-Item -LiteralPath $target -Force
            }
        }

        # The manifest is replaced last so an interrupted update does not look complete.
        Copy-Item -LiteralPath $remoteManifestFile.FullName -Destination $localManifestPath -Force

        Write-Step 'Verifying installed files...'
        Verify-Package $InstallRoot $remoteManifest
        Write-Host '[OK] Update installed successfully.'
    }
    catch {
        Write-Host "[ERROR] Update failed: $($_.Exception.Message)"
        Write-Step 'Restoring previous files...'

        foreach ($relative in $createdDuringUpdate) {
            $target = Join-Path $InstallRoot $relative
            if (Test-Path -LiteralPath $target -PathType Leaf) {
                Remove-Item -LiteralPath $target -Force -ErrorAction SilentlyContinue
            }
        }

        foreach ($relative in $toBackup) {
            $backup = Join-Path $backupRoot $relative
            if (Test-Path -LiteralPath $backup -PathType Leaf) {
                Copy-WithParent $backup (Join-Path $InstallRoot $relative)
            }
        }

        $manifestBackup = Join-Path $backupRoot $ManifestName
        if (Test-Path -LiteralPath $manifestBackup -PathType Leaf) {
            Copy-Item -LiteralPath $manifestBackup -Destination $localManifestPath -Force
        }

        throw
    }
}
catch {
    Write-Host "[ERROR] $($_.Exception.Message)"
    exit 1
}
finally {
    if (Test-Path -LiteralPath $tempRoot) {
        Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
}
