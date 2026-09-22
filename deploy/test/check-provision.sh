#!/usr/bin/env bash
# Run after the acceptance stack has started. Operates only on identity_test.
set -euo pipefail
export MSYS_NO_PATHCONV=1
cd "$(dirname "$0")/../.."
compose() { docker compose -f deploy/test/compose.yaml "$@"; }
provision() { compose exec -T identity-service /app provision --max-user-id 106 --house-id 77777777-7777-4777-8777-777777777777 "$@"; }
query() { compose exec -T identity-db psql -U test -d identity_test -At -v ON_ERROR_STOP=1 -c "$1"; }

provision --house-name 'Acceptance house' --address 'Test street 1' --city Test --role CHAIRMAN
query "UPDATE users SET display_name='Preserved profile' WHERE max_user_id=106" >/dev/null
provision --role RESIDENT
test "$(query "SELECT count(*) FROM memberships m JOIN users u ON m.user_id=u.id WHERE u.max_user_id=106 AND u.display_name='Preserved profile' AND m.role='RESIDENT' AND m.status='ACTIVE'")" = 1
provision --status INACTIVE
test "$(query "SELECT count(*) FROM memberships m JOIN users u ON m.user_id=u.id WHERE u.max_user_id=106 AND m.status='INACTIVE'")" = 1
if provision --role OWNER; then
  echo 'Unknown role unexpectedly accepted' >&2
  exit 1
fi
echo 'PASS: provisioning, idempotent membership update, profile preservation and revocation'
