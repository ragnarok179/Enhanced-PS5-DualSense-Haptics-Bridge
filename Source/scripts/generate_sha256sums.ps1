param(
    [string]$PackageRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
)

$ErrorActionPreference = 'Stop'
$PackageRoot = [System.IO.Path]::GetFullPath($PackageRoot)
$Output = Join-Path $PackageRoot 'SHA256SUMS.txt'

# Only these paths belong to the updater-managed public runtime package.
$ManagedRootFiles = @(
    'START_BRIDGE.exe',
    'START_BRIDGE_AND_BEAMNG.exe',
    'UPDATE_BRIDGE.exe',
    'README.md',
    'LICENSE',
    'THIRD_PARTY_NOTICES.md',
    'COMPATIBILITY.md'
)
$ManagedDirectories = @('Bridge', 'Config', 'Diagnostics')

function Test-PublicManagedFile([string]$FullName) {
    if ($FullName -eq $Output) { return $false }
    $relative = $FullName.Substring($PackageRoot.Length).TrimStart('\','/')
    $relative = $relative -replace '\\','/'

    if ($relative -eq 'Config/user_settings.json') { return $false }
    if ($relative -match '^Diagnostics/Logs/') { return $false }

    if ($ManagedRootFiles -contains $relative) { return $true }
    foreach ($directory in $ManagedDirectories) {
        if ($relative.StartsWith($directory + '/', [System.StringComparison]::OrdinalIgnoreCase)) {
            return $true
        }
    }
    return $false
}

$files = Get-ChildItem -LiteralPath $PackageRoot -File -Recurse |
    Where-Object { Test-PublicManagedFile $_.FullName } |
    Sort-Object FullName

$lines = foreach ($file in $files) {
    $relative = $file.FullName.Substring($PackageRoot.Length).TrimStart('\','/') -replace '\\','/'
    $hash = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
    "$hash  ./$relative"
}

[System.IO.File]::WriteAllLines($Output, $lines, (New-Object System.Text.UTF8Encoding($false)))
Write-Host "Updated $Output with $($lines.Count) public runtime files."
