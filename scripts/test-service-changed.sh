#!/usr/bin/env bash
set -euo pipefail
selector="$(cd "$(dirname "$0")" && pwd)/service-changed.sh"
temp_root="$(cd "${TMPDIR:-/tmp}" && pwd -P)"
fixture="$(mktemp -d "$temp_root/smartquarter-ci.XXXXXXXX")"
# Clean up only this newly created, resolved temporary directory.
[[ "$(cd "$fixture" && pwd -P)" == "$temp_root"/smartquarter-ci.* ]]
trap 'rm -rf -- "$fixture"' EXIT
cd "$fixture"
git init -q
git config user.email ci@example.invalid
git config user.name "CI test"
git config core.autocrlf false
mkdir -p services/issue-service services/identity-service contracts/events docs
touch services/issue-service/go.mod services/identity-service/go.mod
git add .
git commit -qm initial
initial="$(git rev-parse HEAD)"
expect() {
  local actual
  actual="$(bash "$selector" "$1" "$2" "$3")"
  if [[ "$actual" != "$4" ]]; then
    echo "Expected $4 for $3 ($1..$2), got $actual" >&2
    exit 1
  fi
}
echo change > services/issue-service/main.go
git add .
git commit -qm service
service_commit="$(git rev-parse HEAD)"
expect "$initial" "$service_commit" issue-service true
expect "$initial" "$service_commit" identity-service false
echo docs > docs/guide.md
git add .
git commit -qm docs
docs_commit="$(git rev-parse HEAD)"
expect "$service_commit" "$docs_commit" issue-service false
echo '{}' > contracts/events/event.json
git add .
git commit -qm contract
contract_commit="$(git rev-parse HEAD)"
expect "$docs_commit" "$contract_commit" identity-service true
git mv services/issue-service/main.go services/identity-service/main.go
git commit -qm rename
rename_commit="$(git rev-parse HEAD)"
expect "$contract_commit" "$rename_commit" issue-service true
expect "$contract_commit" "$rename_commit" identity-service true
git rm -q services/identity-service/main.go
git commit -qm deletion
expect "$rename_commit" HEAD identity-service true
expect 0000000000000000000000000000000000000000 HEAD issue-service true
expect missing-base HEAD issue-service true
expect HEAD HEAD issue-service false
echo "Service selection tests passed"
