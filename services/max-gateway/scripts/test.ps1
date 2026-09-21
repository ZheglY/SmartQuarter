$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '../../..')).Path
& (Join-Path $root 'services/issue-service/scripts/local.ps1') up
if ($LASTEXITCODE) { throw 'Issue test stack failed' }
Get-Content -LiteralPath (Join-Path $root 'services/issue-service/.env.local') | ForEach-Object {
    $pair = $_ -split '=', 2
    if ($pair.Length -eq 2) { [Environment]::SetEnvironmentVariable($pair[0], $pair[1], 'Process') }
}
$env:TEST_DATABASE_URL = 'postgres://issue:' + $env:POSTGRES_PASSWORD + '@localhost:15432/issue_test?sslmode=disable'
$env:GATEWAY_E2E = '1'
$env:GOWORK = 'off'
Push-Location (Join-Path $root 'services/max-gateway')
try {
    go test ./...
    if ($LASTEXITCODE) { throw 'Gateway unit tests failed' }
    go test -tags=integration -count=1 -timeout=180s -v ./tests/integration ./internal/state ./internal/notification
    if ($LASTEXITCODE) { throw 'Gateway integration tests failed' }
} finally { Pop-Location }
