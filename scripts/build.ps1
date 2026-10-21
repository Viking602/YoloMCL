param(
    [string]$PublisherConfig,
    [switch]$Release,
    [string]$Output = '.cache\MCL-dev.exe'
)
$ErrorActionPreference = 'Stop'
$MclRoot = Split-Path -Parent $PSScriptRoot
$MclConfig = @{ microsoftClientId = ''; curseForgeProxyUrl = ''; curseForgeApiKey = '' }
if ($PublisherConfig) {
    $MclInput = Get-Content -LiteralPath $PublisherConfig -Raw | ConvertFrom-Json
    $MclConfig.microsoftClientId = [string]$MclInput.microsoftClientId
    $MclConfig.curseForgeProxyUrl = [string]$MclInput.curseForgeProxyUrl
    $MclConfig.curseForgeApiKey = [string]$MclInput.curseForgeApiKey
}
if (!$MclConfig.curseForgeApiKey -and $env:MCL_CURSEFORGE_API_KEY) { $MclConfig.curseForgeApiKey = $env:MCL_CURSEFORGE_API_KEY }
if (!$MclConfig.microsoftClientId -and $env:MCL_MICROSOFT_CLIENT_ID) { $MclConfig.microsoftClientId = $env:MCL_MICROSOFT_CLIENT_ID }
if ($MclConfig.microsoftClientId -and $MclConfig.microsoftClientId -notmatch '^[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$') { throw 'Invalid publisher Microsoft client ID.' }
if ($MclConfig.curseForgeProxyUrl) {
    $MclUri = [uri]$MclConfig.curseForgeProxyUrl
    if (!$MclUri.IsAbsoluteUri -or $MclUri.Scheme -ne 'https' -or $MclUri.UserInfo -or $MclUri.Query -or $MclUri.Fragment) { throw 'Catalog service requires a valid HTTPS URL.' }
}
if ($Release -and !$MclConfig.microsoftClientId) { throw 'Release requires a publisher Microsoft app ID. Players never enter these values.' }
# CurseForge key: env var, else the project's .env. Never echoed.
$MclCfKey = $env:CURSEFORGE_API_KEY
if (!$MclCfKey) {
    $MclEnvFile = Join-Path $MclRoot '.env'
    if (Test-Path -LiteralPath $MclEnvFile) {
        foreach ($MclLine in Get-Content -LiteralPath $MclEnvFile) {
            if ($MclLine -match '^\s*(export\s+)?CURSEFORGE_API_KEY\s*=\s*(.*)$') { $MclCfKey = $Matches[2].Trim().Trim('"', "'") }
        }
    }
}
$MclCfObf = ''
if ($MclCfKey) {
    $MclMask = [Text.Encoding]::UTF8.GetBytes('mcl-cf-9f3a7c1e-launcher')  # must match internal/distribution/cfkey.go
    $MclKeyBytes = [Text.Encoding]::UTF8.GetBytes($MclCfKey.Trim())
    $MclCfObf = -join (0..($MclKeyBytes.Length - 1) | ForEach-Object { '{0:x2}' -f ($MclKeyBytes[$_] -bxor $MclMask[$_ % $MclMask.Length]) })
    Write-Output 'CurseForge key: embedded (obfuscated)'
} else {
    Write-Output 'CurseForge key: not found (CF features need env/.env at runtime)'
}
if ($Release -and !$MclCfObf -and !$MclConfig.curseForgeApiKey -and !$MclConfig.curseForgeProxyUrl) { throw 'Release requires a CurseForge API key.' }
$MclEncoded = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes(($MclConfig | ConvertTo-Json -Compress)))
$MclFlags = "-s -w -H=windowsgui -X mcl/internal/distribution.encoded=$MclEncoded -X mcl/internal/distribution.cfKeyObf=$MclCfObf"
Push-Location -LiteralPath $MclRoot
try {
    $MclOutDir = Split-Path -Parent $Output
    if ($MclOutDir) { New-Item -ItemType Directory -Path $MclOutDir -Force | Out-Null }
    go build -trimpath -ldflags $MclFlags -o $Output .
    if ($LASTEXITCODE -ne 0) { throw 'Native build failed.' }
    Write-Output "Native launcher: $(Resolve-Path -LiteralPath $Output)"
} finally { Pop-Location }
