param([ValidateSet('up','test','stop')][string]$Action = 'up')
$ErrorActionPreference = 'Stop'
Push-Location (Join-Path $PSScriptRoot '..')
try {
    if (-not (Test-Path -LiteralPath '.env.local')) {
        $password = [Convert]::ToHexString([System.Security.Cryptography.RandomNumberGenerator]::GetBytes(24)).ToLowerInvariant()
        $secret = [Convert]::ToHexString([System.Security.Cryptography.RandomNumberGenerator]::GetBytes(24)).ToLowerInvariant()
        $lines = @("POSTGRES_PASSWORD=$password", 'AWS_ACCESS_KEY_ID=issue-local', "AWS_SECRET_ACCESS_KEY=$secret")
        [System.IO.File]::WriteAllLines((Join-Path (Get-Location) '.env.local'), $lines)
    }
    Get-Content -LiteralPath '.env.local' | ForEach-Object {
        $entry = $_ -split '=', 2
        if ($entry.Length -eq 2) { [Environment]::SetEnvironmentVariable($entry[0], $entry[1], 'Process') }
    }
    $env:GOWORK = 'off'
    $compose = @('compose', '--env-file', '.env.local', '-f', 'compose.local.yml')
    if ($Action -eq 'stop') {
        & docker @compose --profile app stop
        if ($LASTEXITCODE -ne 0) { throw 'Could not stop local services' }
        return
    }
    if ($Action -eq 'test') {
        # This dedicated test database is reset by migration integration tests.
        & docker @compose --profile app stop issue-service
        if ($LASTEXITCODE -ne 0) { throw 'Could not stop local application before tests' }
    }
    & docker @compose up -d --wait postgres redis minio
    if ($LASTEXITCODE -ne 0) { throw 'Could not start local dependencies' }
    & go run ./cmd/devstorage
    if ($LASTEXITCODE -ne 0) { throw 'Could not initialize local bucket' }
    if ($Action -eq 'up') {
        & docker @compose --profile app build issue-service
        if ($LASTEXITCODE -ne 0) { throw 'Image build failed' }
        & docker @compose --profile app run --rm --no-deps issue-service migrate up
        if ($LASTEXITCODE -ne 0) { throw 'Migration failed' }
        & docker @compose --profile app up -d --wait issue-service
        if ($LASTEXITCODE -ne 0) { throw 'Application did not become ready' }
        Write-Output 'gRPC: localhost:18082; readiness: http://localhost:18083/readyz'
    } else {
        $env:TEST_DATABASE_URL = 'postgres://issue:' + $env:POSTGRES_PASSWORD + '@localhost:15432/issue_test?sslmode=disable'
        $env:TEST_S3_ENDPOINT = 'http://localhost:19000'
        & go test ./...
        if ($LASTEXITCODE -ne 0) { throw 'Unit tests failed' }
        & go test -tags=integration -count=1 -timeout=120s -v ./tests/integration
        if ($LASTEXITCODE -ne 0) { throw 'Integration tests failed' }
        Write-Output 'Tests passed. Run local.ps1 up to restart the application.'
    }
} finally { Pop-Location }
