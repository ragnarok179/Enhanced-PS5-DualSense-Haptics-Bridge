param(
    [string]$RepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
)

$ErrorActionPreference = 'Stop'
$RepositoryRoot = [System.IO.Path]::GetFullPath($RepositoryRoot)
$Output = Join-Path $RepositoryRoot 'SHA256SUMS.txt'
# START_BRIDGE.bat exists only so the V1.1 updater can recognize the new layout.
# It is intentionally not part of the managed installation manifest.
$LegacyLauncherMarker = Join-Path $RepositoryRoot 'START_BRIDGE.bat'
$LegacyUpdaterBat = Join-Path $RepositoryRoot 'UPDATE_BRIDGE.bat'
$LegacyUpdaterPs1 = Join-Path $RepositoryRoot 'Tools and diagnostics\Updater\Update-Bridge.ps1'
# Runtime user preferences are deliberately unmanaged so updates never overwrite them.
$UserSettingsFile = Join-Path $RepositoryRoot 'Tools and diagnostics\Config\user_settings.json'
$PendingCompatibilityFile = Join-Path $RepositoryRoot 'Tools and diagnostics\Config\pending_bridge_compatibility.json'

function Get-RepositorySha256([string]$Path) {
    $bytes = [System.IO.File]::ReadAllBytes($Path)

    # Git stores text files with LF line endings in the repository. A Windows
    # working tree may use CRLF, so normalize text bytes before hashing. This
    # keeps SHA256SUMS.txt identical to the files downloaded from GitHub.
    $isBinary = $bytes -contains [byte]0
    if (-not $isBinary) {
        $normalized = New-Object System.Collections.Generic.List[byte]
        for ($i = 0; $i -lt $bytes.Length; $i++) {
            if ($bytes[$i] -eq 13) {
                if (($i + 1) -lt $bytes.Length -and $bytes[$i + 1] -eq 10) {
                    $i++
                }
                $normalized.Add(10)
            } else {
                $normalized.Add($bytes[$i])
            }
        }
        $bytes = $normalized.ToArray()
    }

    $sha = [System.Security.Cryptography.SHA256]::Create()
    try {
        return ([System.BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-', '').ToLowerInvariant()
    } finally {
        $sha.Dispose()
    }
}

$files = Get-ChildItem -LiteralPath $RepositoryRoot -File -Recurse | Where-Object {
    $_.FullName -ne $Output -and
    $_.FullName -ne $LegacyLauncherMarker -and
    $_.FullName -ne $LegacyUpdaterBat -and
    $_.FullName -ne $LegacyUpdaterPs1 -and
    $_.FullName -ne $UserSettingsFile -and
    $_.FullName -ne $PendingCompatibilityFile -and
    $_.FullName -notmatch '[\\/]\.git[\\/]' -and
    $_.FullName -notmatch '[\\/]Tools and diagnostics[\\/]Diagnostics[\\/]Logs[\\/](?!\.gitkeep$)'
} | Sort-Object FullName

$lines = foreach ($file in $files) {
    $relative = $file.FullName.Substring($RepositoryRoot.Length)
    $relative = $relative -replace '^[\\/]+', ''
    $relative = $relative -replace '\\', '/'
    $hash = Get-RepositorySha256 $file.FullName
    "$hash  ./$relative"
}

[System.IO.File]::WriteAllLines($Output, $lines, (New-Object System.Text.UTF8Encoding($false)))
Write-Host "Updated $Output with $($lines.Count) managed files."
