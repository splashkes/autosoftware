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

## Runtime Database

The kernel optionally connects to a Postgres database for runtime features
(identity, sessions, communications, guardrails). Set the connection string
via environment variable:

    AS_RUNTIME_DATABASE_URL=postgres://user:pass@host:port/dbname

Set `AS_RUNTIME_AUTO_MIGRATE=true` to apply schema files from
`kernel/db/runtime/*.sql` on startup.

Without a database URL, the kernel runs in reduced mode — seed authoring,
growth, materialization, and the boot console all work without it.

### Local Postgres (Docker)

Helper scripts for a disposable local instance:

- `./scripts/as-postgres-up.sh` — start and bootstrap
- `./scripts/as-postgres-down.sh` — stop
- `./scripts/as-postgres-psql.sh` — connect
- `./scripts/as-postgres-reset.sh` — destroy and recreate

Override defaults via `.env.postgres` (copy from `.env.postgres.example`).
Default port is `54329`.

## Local Run Script

The fastest way to validate the local kernel stack:

`./scripts/local-run.sh`

The script starts all kernel services, runs tests, and verifies endpoints.
If a runtime database is configured, it bootstraps that first.
On exit it stops Go services.

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
