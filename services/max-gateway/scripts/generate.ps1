$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '../../..')).Path
Push-Location (Join-Path $root 'services/issue-service')
try {
    buf generate ../../contracts/proto --path ../../contracts/proto/smartquarter/issue/v1/issue.proto
    if ($LASTEXITCODE) { throw 'Issue generation failed' }
} finally { Pop-Location }

Push-Location (Join-Path $root 'services/community-service')
try {
    buf generate ../../contracts/proto --path ../../contracts/proto/smartquarter/community/v1/community.proto
    if ($LASTEXITCODE) { throw 'Community generation failed' }
} finally { Pop-Location }
Push-Location (Join-Path $root 'services/max-gateway')
try {
    buf generate ../../contracts/proto --path ../../contracts/proto/smartquarter/issue/v1/issue.proto --path ../../contracts/proto/smartquarter/community/v1/community.proto --path ../../contracts/proto/smartquarter/identity/v1/identity.proto
    if ($LASTEXITCODE) { throw 'Gateway generation failed' }
} finally { Pop-Location }

Push-Location (Join-Path $root 'services/identity-service')
try {
    buf generate ../../contracts/proto --path ../../contracts/proto/smartquarter/identity/v1/identity.proto
    if ($LASTEXITCODE) { throw 'Identity generation failed' }
} finally { Pop-Location }
