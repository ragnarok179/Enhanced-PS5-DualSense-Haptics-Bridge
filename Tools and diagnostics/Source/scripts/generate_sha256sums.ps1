param(
    [string]$RepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
)

$ErrorActionPreference = 'Stop'
$RepositoryRoot = [System.IO.Path]::GetFullPath($RepositoryRoot)
$Output = Join-Path $RepositoryRoot 'SHA256SUMS.txt'

$files = Get-ChildItem -LiteralPath $RepositoryRoot -File -Recurse | Where-Object {
    $_.FullName -ne $Output -and
    $_.FullName -notmatch '[\\/]\.git[\\/]' -and
    $_.FullName -notmatch '[\\/]Tools and diagnostics[\\/]Diagnostics[\\/]Logs[\\/](?!\.gitkeep$)'
} | Sort-Object FullName

$lines = foreach ($file in $files) {
    $relative = $file.FullName.Substring($RepositoryRoot.Length)
    $relative = $relative -replace '^[\\/]+', ''
    $relative = $relative -replace '\\', '/'
    $hash = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
    "$hash  ./$relative"
}

[System.IO.File]::WriteAllLines($Output, $lines, (New-Object System.Text.UTF8Encoding($false)))
Write-Host "Updated $Output with $($lines.Count) managed files."
