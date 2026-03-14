# Autosoftware

Autosoftware (AS) is an experiment in software that evolves from user
requests made inside the running experience.

In the historical model, a request leaves the experience and disappears into a
separate software process. In Autosoftware, the experience itself is where
change begins. The product is not just where software is used. It is where
software continuously evolves.

## Core Loop

1. Capture the request in a seed, right in the user experience.
2. Validate the early design with the user.
3. Produce one or more realizations from named approaches, including different
   coding agents or implementation strategies.
4. Run automated validation and refine the realization.
5. Review and accept a realization, publish the accepted result, and let the
   materializer rebuild current state so the user experience updates
   automatically.

## Repo Map

- `genesis/` is the founding seed and the first complete example of the model.
- `kernel/` is the trusted machinery, including the registry, the
  materializer, and the main interface surfaces.
- `seeds/` contains evolving app changes: request, design, approaches,
  acceptance, and realizations.
- `materialized/` contains hydrated outputs derived from accepted registry
  history.

Most change should happen in `seeds/`. Changes to `kernel/` should be rarer and
should tighten registry correctness, permission validation, artifact
verification, replay/materialization determinism, or interface plumbing.

## Local Postgres For Testing

AS now includes a local Postgres dev environment for kernel/runtime testing.

1. Copy `.env.postgres.example` to `.env.postgres` if you want non-default
   credentials or a different port.
2. Start and bootstrap the database with `./scripts/as-postgres-up.sh`.
3. Connect with `./scripts/as-postgres-psql.sh`.
4. Stop it with `./scripts/as-postgres-down.sh`.
5. Reset the database volume with `./scripts/as-postgres-reset.sh`.
6. Enable login-time autostart with `launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.splash.as-postgres.plist`.

Defaults:

- host: `localhost`
- port: `54329`
- database: `as_local`
- user: `postgres`
- password: `postgres`

The bootstrap script reapplies every SQL file in `kernel/db/runtime/`, so the
runtime feedback-loop tables are created automatically and future runtime SQL
can be added there without changing the startup flow.

For persistence across machine restarts, the Postgres data lives in the Docker
volume `as_as-postgres-data`. The included LaunchAgent starts Docker Desktop if
needed and then starts the `postgres` service automatically at login.

## Local Run

To run the local kernel stack end to end, use:

`./scripts/local-run.sh`

The script:

- bootstraps and verifies Postgres
- prints recent Postgres container output
- runs `go test ./...` in `kernel/`
- starts `apid`, `registryd`, `materializerd`, and `webd`
- streams Postgres and Go service logs to the console with service prefixes
- verifies the main health, contract, and materialization endpoints

When you stop the script with `Ctrl-C`, it stops the Go services and leaves
Postgres running for the next local session.

## Read Next

- [seeds/README.md](seeds/README.md) for the seed model and authoring structure
- [kernel/public_object_registry.md](kernel/public_object_registry.md) for the
  registry and materialization model
- [kernel/public_claim_registry.md](kernel/public_claim_registry.md) for the
  claim-focused view of the same registry
- [kernel/public_schema_object_registry.md](kernel/public_schema_object_registry.md)
  for schemas as first-class objects
- [kernel/architecture.md](kernel/architecture.md) for kernel structure and
  responsibilities
- [kernel/protocol/v1/](kernel/protocol/v1/) for the deeper registry, object,
  claim, and materialization model
- [kernel/philosophy.md](kernel/philosophy.md) for deeper concepts such as
  runtime selection and the feedback loop
- [genesis/](genesis/) for the founding seed
