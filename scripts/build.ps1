param(
    [string]$PublisherConfig,
    [switch]$Release,
    [string]$Output
)
$ErrorActionPreference = 'Stop'
$MclRoot = Split-Path -Parent $PSScriptRoot
$MclTarget = go env -json GOOS GOARCH GOHOSTOS GOHOSTARCH | ConvertFrom-Json
if ($LASTEXITCODE -ne 0) { throw 'Cannot read Go target settings.' }
$MclBuildArgs = @('run', './cmd/build', '-goos', $MclTarget.GOOS, '-goarch', $MclTarget.GOARCH)
if ($Release) { $MclBuildArgs += '-release' }
if ($Output) { $MclBuildArgs += @('-output', $Output) }
if ($PublisherConfig) { $MclBuildArgs += @('-publisher-config', [IO.Path]::GetFullPath($PublisherConfig)) }
$MclOriginalOS, $MclOriginalArch = $env:GOOS, $env:GOARCH
Push-Location -LiteralPath $MclRoot
try {
    # Run the build tool on the host, even when GOOS/GOARCH select another target.
    $env:GOOS, $env:GOARCH = $MclTarget.GOHOSTOS, $MclTarget.GOHOSTARCH
    go @MclBuildArgs
    if ($LASTEXITCODE -ne 0) { throw 'Native build failed.' }
} finally {
    $env:GOOS, $env:GOARCH = $MclOriginalOS, $MclOriginalArch
    Pop-Location
}
