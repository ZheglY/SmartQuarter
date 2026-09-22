# Identity Service

Owns identity_db: MAX users, houses and RESIDENT/CHAIRMAN/ADMIN memberships.
Shared source: ../../contracts/proto/smartquarter/identity/v1/identity.proto.
Gateway uses this protobuf, validates DTOs and checks membership on each protected request.

Required DATABASE_URL must target identity_db or identity_test*. Defaults:
IDENTITY_GRPC_PORT=50051, IDENTITY_HTTP_PORT=8081, LOG_LEVEL=info.
Run `go run ./cmd/app`; environment files must be loaded by the shell/Compose.
Embedded Goose migrations run once on startup. No development seed runs automatically.

HTTP: /livez, /readyz (DB ping), /metrics.
gRPC: UpsertMaxUser, GetUserContext, GetMembership, ListMemberships;
grpc.health.v1.Health/Check checks DB readiness. RPC deadline capped at 10 seconds.
SIGTERM drains servers with a bounded shutdown; listener failures terminate the process.

GetUserContext selects only an ACTIVE default membership. A revoked/missing default
falls back to the first active membership; users without active membership get an
empty default house and can still log in. Unknown/zero/noncanonical UUIDs are rejected.

## Operator provisioning

From deploy/server, after migrations:

```bash
docker compose exec identity-service /app provision \
  --max-user-id VERIFIED_MAX_ID \
  --house-id YOUR_CANONICAL_UUID \
  --house-name 'Дом 1' --address 'Улица, дом' --city 'Город' \
  --role CHAIRMAN
```

Use a real verified MAX user ID, never a phone number. Generate a house UUID once;
reuse it for residents. For an existing house omit its descriptive fields.
The command creates the user if necessary and upserts one membership atomically;
repeat execution does not duplicate records. A normal MAX login updates the name.
Use --status INACTIVE to revoke membership or --role RESIDENT to remove management
rights. Existing sessions are rechecked at each protected request.
The command is available only through server/container console, not a public endpoint.

Tests: `GOWORK=off go test ./...`. Full database/gRPC/workflow verification:
`docker compose -f deploy/test/compose.yaml run --build --rm test` from repository root.
Regenerate Go stubs using buf.gen.yaml and the shared proto source.
