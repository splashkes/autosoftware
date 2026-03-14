# Kernel Runbook

This document is for operational procedures.

Boundary:

- describe how to run, validate, inspect, and operate the kernel
- describe step-by-step sequences
- do not restate architecture rationale here
- do not duplicate protocol definitions here

## Scope

The runbook should answer operational questions such as:

- how to start local kernel services
- how to pin a local run to a realization
- how to validate a seed before merge
- how to validate a realization before merge
- how an accepted realization is appended to the registry
- how to trigger or inspect materialization
- how to inspect registry and materializer state
- how to inspect browser and request incidents for one realization

## Local Postgres Bootstrap

The local kernel runtime expects a Postgres instance for runtime tables.

Bootstrap flow:

1. Optionally copy `.env.postgres.example` to `.env.postgres` and override
   database name, user, password, or port.
2. Run `./scripts/as-postgres-up.sh` from the repo root.
3. Verify access with `./scripts/as-postgres-psql.sh -c '\\dt runtime_*'`.
4. Stop the service with `./scripts/as-postgres-down.sh`.
5. If local state becomes disposable, recreate it with
   `./scripts/as-postgres-reset.sh`.
6. For login-time autostart on macOS, bootstrap the LaunchAgent in
   `~/Library/LaunchAgents/com.splash.as-postgres.plist`.

Notes:

- the service runs via `compose.yaml`
- the container listens on `localhost:${AS_POSTGRES_PORT:-54329}`
- every `kernel/db/runtime/*.sql` file is applied during bootstrap
- runtime data persists in the Docker volume `as_as-postgres-data`
- the autostart helper is `scripts/as-postgres-autostart.sh`

## Local Run Script

The fastest way to validate the local kernel stack is:

`./scripts/local-run.sh`

This script:

1. bootstraps Postgres
2. shows recent Postgres output
3. verifies runtime tables
4. runs `go test ./...` in `kernel/`
5. starts `apid`, `registryd`, `materializerd`, and `webd`
6. streams prefixed Postgres and Go service logs to the console
7. checks the runtime health, contract, registry, realization, and
   materialization endpoints

The script keeps running until interrupted.
On exit it stops the Go services and leaves Postgres up.

## Planned Operational Flow

1. Prepare the founding seed and kernel configuration.
2. Start the registry service.
3. Start the materializer.
4. Start web and API surfaces as needed.
5. Select or pin the realization to test or serve.
6. Author or refine one seed in `seeds/`.
7. Produce or refine a realization for that seed.
8. Run the realization and capture browser or request failures in runtime
   feedback-loop storage.
9. Validate the realization against the seed-level acceptance criteria.
10. Review incidents, test runs, and agent findings tied to that realization.
11. Merge the realization through review.
12. Append the accepted realization to the registry.
13. Confirm materialization completed successfully.

As the implementation solidifies, this file should become the source of truth
for actual commands and procedures.
