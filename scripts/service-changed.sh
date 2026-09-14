#!/usr/bin/env bash
# Print whether a service needs publishing. Unknown base => rebuild safely.
set -euo pipefail
base="${1:?base commit required}"
head="${2:?head commit required}"
service="${3:?service required}"

if [[ ! -f "services/$service/go.mod" ]]; then
  echo "Unknown service: $service" >&2
  exit 1
fi
if [[ "$base" =~ ^0+$ ]] || ! git cat-file -e "$base^{commit}" 2>/dev/null; then
  echo true
  exit 0
fi

paths="$(git diff --name-only --no-renames "$base" "$head")"
while IFS= read -r path; do
  case "$path" in
    "services/$service/"*) echo true; exit 0 ;;
    services/*|docs/*|web/*|README.md|*.md) ;;
    "") ;;
    *) echo true; exit 0 ;; # Contracts, CI, Go workspace and shared configuration.
  esac
done <<< "$paths"
echo false

