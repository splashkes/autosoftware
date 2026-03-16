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

## Boot Execution

The homepage only renders a real `Run` action when both of these are true:

- `AS_RUNTIME_DATABASE_URL` is set so runtime state is available
- `AS_BOOT_EXECUTION_ENABLED=true` is present for `webd`

If `AS_BOOT_EXECUTION_ENABLED` is missing, runnable realizations degrade to
`Show Run`, the launch route is not registered, and no preview path under
`/__runs/<execution_id>/` can be created from the homepage.

In DOKS, keep `webd` and `execd` in the same pod and set
`AS_BOOT_EXECUTION_ENABLED=true` in the shared runtime config so
`autosoftware.app` can launch realizations and bind their preview and stable
routes.

Launch health checks are owned by `execd`, not by the browser. Use
`AS_EXECUTION_HEALTH_TIMEOUT_SECONDS` to control how long `execd` waits for a
new realization to become healthy before marking the launch failed. The current
DOKS manifest sets this to `180`.

Because `registryd` and `apid` run as separate services in DOKS, launched
realizations must receive service-reachable capability URLs rather than the raw
listener addresses. The production stack now sets
`AS_EXECD_REGISTRY_BASE_URL`, `AS_EXECD_PUBLIC_API_BASE_URL`, and
`AS_EXECD_INTERNAL_API_BASE_URL` for `execd`.

Healthy realizations are kept hot until one of these happens:

- an operator explicitly stops the execution
- the process exits or is terminated by execution budget enforcement
- the pod restarts and `execd` reconciles runtime state on startup

There is no separate idle timeout that shuts down healthy realizations.

## Production Release

The AS production stack is released from GitHub Actions, not from an operator's
local shell.

Canonical workflow:

- merge to `main` through a PR
- GitHub Actions builds one kernel image with `docker buildx`
- the workflow pushes a SHA-tagged image to the configured registry repository
- the workflow records the immutable digest and deploys by digest
- Kubernetes manifests are rendered from templates in `deploy/doks/`
- the deploy job verifies cluster services through Kubernetes port-forwards
  instead of probing the public Cloudflare edge

Workflow file:

- `.github/workflows/as-prod-release.yml`

### GitHub configuration split

Keep production config out of the repo.

Environment secrets:

- `DIGITALOCEAN_ACCESS_TOKEN`
- `AS_KUBECONFIG_B64`
- `AS_RUNTIME_DATABASE_URL`

Environment vars:

- `AS_IMAGE_REPOSITORY`
- `AS_NAMESPACE`
- `AS_BASE_DOMAIN`
- `AS_WEB_HOST`
- `AS_REGISTRY_HOST`
- `AS_API_HOST`
- `AS_GITHUB_URL`
- `AS_IMAGE_PULL_SECRET`
- `AS_EDGE_TLS_SECRET`

Future additions that should also live in environment secrets:

- `AS_INTERNAL_API_TOKEN`
- TLS key material if certificate automation is added
- realization-specific secret environment for shared deployments

### DOKS topology

The current AS deployment shape is:

- `as-apid` Deployment + ClusterIP Service
- `as-registryd` Deployment + ClusterIP Service
- `as-materializerd` Deployment + ClusterIP Service
- `as-webd` Deployment + ClusterIP Service
- `webd` and `execd` run in the same `as-webd` pod

`execd` is colocated with `webd` because the current shared execution path is
still localhost-backed and source-launched. This is transitional. The longer-
term target is packaged realization runtimes launched as separate workloads.

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
