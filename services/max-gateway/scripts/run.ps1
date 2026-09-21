$ErrorActionPreference = 'Stop'
Push-Location (Join-Path $PSScriptRoot '..')
try {
    if (-not (Test-Path -LiteralPath '.env.local')) { throw 'Copy .env.example to .env.local and fill MAX settings first' }
    Get-Content -LiteralPath '.env.local' | ForEach-Object {
        if ($_ -notmatch '^\s*#' -and $_ -match '=') {
            $pair = $_ -split '=', 2
            [Environment]::SetEnvironmentVariable($pair[0], $pair[1], 'Process')
        }
    }
    $env:GOWORK = 'off'
    go run ./cmd/app
    if ($LASTEXITCODE) { throw 'Gateway stopped with error' }
} finally { Pop-Location }
