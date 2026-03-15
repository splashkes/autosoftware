# AS Plan: Realization Execution Layer and Runtime Orchestration

## Purpose

This plan closes a core gap in AS: the kernel can discover, inspect, grow, and
proxy realizations, but it still cannot boot and manage them itself.

That is no longer a small UX issue. It is a kernel architecture gap.

The goal is to make any complete runnable realization launchable by the kernel
under an explicit runtime model, while preserving the existing project
boundaries:

- seeds remain source intent
- realizations remain compiled intent
- the registry remains accepted truth
- execution state remains runtime state
- runtime always selects a realization explicitly

This plan should be read together with:

- `kernel/decision_log.md`
- `kernel/philosophy.md`
- `docs/seed_execution_model.md`
- `plans/03-runtime-bootstrap-activation-and-local-dev-plan.md`
- `plans/06-realization-testing-and-live-validation-plan.md`
- `plans/08-public-and-internal-api-split-plan.md`
- `kernel/security.md` section H8

---

## Why This Needs a Real Architecture

Today the kernel does four things correctly:

1. discovers realizations from the repo
2. classifies them as runnable when `runtime.yaml` exists
3. shows run metadata in the boot console
4. reverse proxies to a realization that is already listening

What it does not do is the actual launch, isolation, health gating, route
registration, restart, stop, or activation management.

That means the current `Run` concept is weaker than the product language
implies. It really means:

- "this realization carries a runtime recipe"
- not "the kernel can boot and manage it"

The fix should not be `webd` shelling out to `go run`.
That would violate the trusted-kernel boundary, weaken security, and hard-code
local-only behavior into the public web surface.

---

## Non-Negotiable Design Constraints

The execution layer should preserve these rules.

### 1. Runtime boots realizations, not unresolved seeds

Execution must target a realization reference directly.
If a caller asks to run a seed, the kernel must first resolve that seed to a
realization through explicit policy.

### 2. Execution state is runtime state, not registry state

Launch requests, live executions, health, logs, failures, route bindings,
activation pointers, and rollout events do not belong in the append-only
registry.
They belong in runtime storage beside jobs, sessions, and feedback-loop data.

### 3. Seed-authored runtime manifests are hostile input

`runtime.yaml` is authored inside a seed/realization boundary and must be
validated as hostile input before any launch occurs.
The security recommendations in `kernel/security.md` H8 are mandatory.

### 4. One control plane, multiple execution backends

The kernel needs one execution model that works across:

- local development
- preview environments
- DOKS production

The implementation backend may differ, but the execution contract should not.

### 5. Route intent is repo data; live upstreams are runtime data

`subdomain` and `path_prefix` in `realization.yaml` express desired route
identity.
They must not be treated as proof that a live upstream exists.

### 6. Public API and execution authority stay separate

Execution commands should not live on the long-term public API surface.
They belong behind the internal API boundary described in
`plans/08-public-and-internal-api-split-plan.md`.

---

## Core Model

The execution layer should formalize four distinct things that are currently
blurred together.

### 1. Authoring Runtime Manifest

This is the realization-authored `runtime.yaml`.
It declares how the realization expects to run:

- runtime family
- working directory
- entrypoint
- declared environment
- declared health surface
- declared route intent

This remains part of compiled intent inside the realization.

### 2. Execution Package

This is the kernel-approved runnable package for a target backend.

For local development, that may be a validated source launch plan.
For shared environments such as DOKS, it should be a sealed runtime package,
normally an OCI image plus digest and launch metadata.

This distinction matters:

- source manifests are useful for local authoring
- sealed packages are required for shared execution

The kernel should not treat raw seed-authored shell commands as production
authority.

### 3. Execution Session

A launch creates an explicit execution session with its own identity.

An execution session is:

- tied to one realization reference
- tied to one execution backend
- ephemeral
- observable
- restartable or stoppable
- independently routable

This is the object that represents "the thing that is actually running."

### 4. Activation Mapping

Execution and activation are not the same.

- execution asks: "is there a healthy live instance of this realization?"
- activation asks: "which realization should receive canonical traffic?"

This separation is required for:

- preview runs
- canary and shadow traffic
- rollback
- side-by-side realization variants for the same seed

---

## Proposed Runtime States

The current single `CanRun` flag is too weak.
The kernel should distinguish at least these states:

- `defined`: contract exists, no launchable runtime yet
- `dev_runnable`: source-backed runtime can be launched locally
- `packaged`: sealed runtime package exists for shared execution
- `launch_requested`: a launch job exists
- `starting`: backend provisioned, health gate not passed yet
- `healthy`: execution session is up and routable
- `degraded`: serving but failing health or incident thresholds
- `stopped`: intentionally stopped
- `failed`: launch or runtime failure
- `active`: currently selected for canonical traffic

`Runnable` should become more precise in the code and UI.
In practice the kernel needs to answer:

- can this run locally?
- can this run in shared infrastructure?
- is it running now?
- is it active now?

---

## Architecture

### Control Plane

Add a new internal kernel area:

- `kernel/internal/execution/`

Responsibilities:

- validate runtime manifests
- resolve realization references to execution specs
- create launch/stop/restart/promote jobs
- track execution sessions
- monitor health and lifecycle
- publish live route bindings
- expose internal execution projections

Add a dedicated internal daemon:

- `kernel/cmd/execd/`

Responsibilities:

- claim execution jobs from runtime storage
- talk to the configured backend
- update execution session state
- write route bindings
- stream logs and events into runtime storage

This keeps `webd` and `apid` from becoming process managers.

### API Surface

Execution should be internal-only from the start, even if the public/internal
API split is still in progress.

Planned internal commands:

- `POST /v1/commands/realizations.launch`
- `POST /v1/commands/realizations.stop`
- `POST /v1/commands/realizations.restart`
- `POST /v1/commands/realizations.activate`
- `POST /v1/commands/realizations.deactivate`

Planned internal projections:

- `GET /v1/projections/realization-execution/sessions`
- `GET /v1/projections/realization-execution/sessions/{execution_id}`
- `GET /v1/projections/realization-execution/routes`
- `GET /v1/projections/realization-execution/logs/{execution_id}`
- `GET /v1/projections/realization-execution/activation`

Use `runtime_jobs` for command handoff, but do not use it as the durable source
of live execution state.

### Backend Interface

The control plane should target an internal backend interface, for example:

- `Validate(spec)`
- `Launch(spec) -> execution session`
- `Stop(execution_id)`
- `Restart(execution_id)`
- `Inspect(execution_id)`
- `StreamLogs(execution_id)`

Planned backends:

- `localprocess`
- `kubernetes`

The backend decides how the workload is run.
The kernel decides what should run and how it is tracked.

---

## Data Model

Add runtime tables for execution state.

### `runtime_realization_executions`

One row per execution session.

Suggested fields:

- `execution_id`
- `reference`
- `seed_id`
- `realization_id`
- `backend`
- `mode` (`preview`, `candidate`, `active`, `shadow`, `local`)
- `status`
- `route_subdomain`
- `route_path_prefix`
- `upstream_addr`
- `execution_package_ref`
- `launched_by_principal_id`
- `launched_by_session_id`
- `request_id`
- `started_at`
- `healthy_at`
- `stopped_at`
- `last_error`
- `metadata`

### `runtime_realization_execution_events`

Append-only operational timeline for one execution session.

Examples:

- launch_requested
- validation_failed
- backend_created
- health_passed
- health_failed
- route_registered
- stopped
- crashed
- promoted
- demoted

### `runtime_realization_activation`

Maps stable traffic identities to the currently active realization or execution.

Examples:

- seed default route
- explicit preview alias
- canary alias
- shadow alias

### `runtime_realization_route_bindings`

Current live route table produced by the execution layer.

This is what `webd` should consult, directly or through a cache, instead of
freezing route bindings at process start from repo metadata alone.

---

## Routing Model

The kernel needs two classes of routes.

### 1. Stable routes

These are the user-facing canonical routes derived from activation.

Examples:

- `notepad.autosoftware.app`
- `/notepad/`

Only an active execution should receive stable traffic.

### 2. Preview routes

These are explicit execution-session routes for operator testing and live
validation.

Examples:

- `exec-abc123.autosoftware.app`
- `/__runs/exec-abc123/`

This avoids a major current limitation: two realizations of the same seed
cannot safely run side by side if the only route identity comes from the seed's
static `subdomain` or `path_prefix`.

The routing flow should become:

1. realization metadata declares route intent
2. activation selects which execution owns stable traffic
3. execution layer publishes live route bindings
4. `webd` proxies from route binding to live upstream

---

## Packaging Model

The execution layer should support two launch classes.

### Local Source Launch

For local development only.

The kernel validates:

- runtime family is allowlisted
- entrypoint stays within the realization tree
- working directory stays within the realization tree
- environment keys are allowlisted
- launch happens in a temp execution workspace, not against arbitrary host
  paths

This is the bridge from the current manual model to a real local kernel-run
experience.

### Shared Sealed Launch

For preview and production environments.

The kernel should launch a sealed runtime package, preferably:

- an OCI image
- pinned by digest
- tied back to the realization reference and artifact hashes

This is the right model for DOKS.

The important rule is:

- shared infrastructure should run packaged artifacts
- not raw seed-authored `go run`, `node`, or `python` commands directly

That keeps production execution deterministic, reviewable, and safe.

## Kernel Capability Wiring

Realizations should not discover kernel services through ad hoc hard-coded
localhost URLs.

Instead, the execution layer should inject a small stable capability contract,
such as:

- registry query base URL
- public API base URL, if allowed for that realization
- internal API base URL, only when explicitly required
- runtime callback endpoints
- execution identity values such as `seed_id`, `realization_id`, and
  `execution_id`

That contract should be kernel-owned and allowlisted.
Repo-authored manifests may declare that a capability is required, but they
should not control the final privileged endpoint values in shared
infrastructure.

## Manifest Evolution

`runtime.yaml` should remain the authoring manifest, but it needs clearer
structure over time.

Planned additions:

- declared health probe
- declared port or socket behavior
- declared route exposure intent
- declared persistent volume needs
- declared kernel capability dependencies
- declared network policy needs
- declared build/package source

The kernel should normalize this into an internal execution spec before
launching anything.

---

## Security Model

The execution layer must treat realizations as untrusted workloads.

Minimum rules:

1. allowlist runtime families
2. enforce path containment for entrypoint and working directory
3. allowlist environment variables; never pass kernel secrets through
4. run realizations in a separate process or pod boundary from kernel services
5. avoid direct network reachability to kernel internals except explicit
   capability endpoints
6. mount repo inputs read-only where possible
7. isolate writable state to per-execution workspaces or volumes
8. capture stdout, stderr, exit code, and health failures into runtime storage
9. do not let a realization bind arbitrary public ports
10. do not let `webd` or public `apid` execute arbitrary commands

For DOKS specifically, the shared-environment backend should prefer:

- one pod or deployment per execution session
- namespace/network-policy isolation
- per-execution service account with minimal privileges
- no access to the kernel database except through explicit runtime APIs

---

## DOKS Shape

For the current AS deployment, the production form should be:

- `execd` runs as an internal service in the AS cluster
- it launches realization workloads into the same AS cluster
- realization pods are separate from kernel pods
- realization pods talk to kernel APIs through internal cluster DNS only
- public traffic still enters through `webd`
- `webd` proxies to live realization upstreams using execution route bindings

The first production-capable backend should assume:

- shared DOCR for image storage is acceptable
- execution stays isolated to the AS VPC/cluster
- each launched realization gets its own deployment or workload identity

This matches the current deployment isolation goals without inventing a second
control plane.

---

## Interaction with Existing Plans

### Plan 03: Runtime Bootstrap

This plan provides the missing execution shell that Plan 03 assumed.
It keeps the kernel as the stable compiler and execution shell while allowing
app realizations to evolve quickly.

### Plan 06: Realization Testing and Live Validation

This plan provides the runtime substrate Plan 06 needs for:

- ephemeral execution
- startup probes
- canary and shadow traffic
- realization-scoped incidents
- promotion and rollback

### Plan 08: Public/Internal API Split

Execution should land on the internal side of that split.
It should not be treated as part of the public app contract.

### Plan 09: DOKS Deployment

The current DOKS stack can host the kernel services, but it does not yet host a
first-class realization executor.
This plan fills that missing layer.

---

## Rollout Plan

### Phase 1: Execution Domain and Runtime State

- add `internal/execution/` package
- add runtime tables for execution sessions, events, activation, and routes
- add manifest validator and normalized execution spec
- add internal execution commands and projections

Exit criteria:

- kernel can represent launch intent and live execution state without overloading
  `runtime_jobs`

### Phase 2: Local Backend

- implement `localprocess` backend
- launch validated local source-backed realizations from the kernel
- add health gating
- add stop and restart
- change `webd` to use dynamic route bindings

Exit criteria:

- a complete local realization can be launched by reference from the kernel
- `Run` means a real launch, not a documentation panel

### Phase 3: Packaging Pipeline

- define sealed execution package artifact shape
- produce OCI images or equivalent package outputs from realizations
- tie package digests back to realization provenance and artifact hashes
- distinguish local-only from shared-launch-capable realizations

Exit criteria:

- the kernel has a verifiable runtime package for shared execution

### Phase 4: Kubernetes Backend

- implement `kubernetes` backend
- launch one isolated workload per execution session
- add internal service discovery and route registration
- wire health and lifecycle back into runtime state

Exit criteria:

- a packaged realization can be launched and routed on DOKS by reference

### Phase 5: Activation and Live Validation

- activation mappings
- stable versus preview routes
- canary and shadow support
- failure-triggered rollback hooks

Exit criteria:

- accepted realizations can be promoted and rolled back without losing preview
  execution visibility

---

## Explicit Anti-Goals

Do not do these:

- do not make `webd` call `exec.Command` on seed-authored runtime manifests
- do not expose launch/stop commands on the public API surface
- do not use repo-authored `AS_ADDR` as the live source of routing truth
- do not store long-lived execution state only in `runtime_jobs`
- do not run production realizations from unchecked source trees on cluster
  nodes
- do not couple realization execution directly to registry acceptance

---

## Success Criteria

This plan is successful when all of these become true.

1. The kernel can launch a runnable realization by explicit reference.
2. The kernel can stop, restart, and inspect that execution.
3. `webd` routes only to live execution sessions, not static guessed upstreams.
4. Multiple realizations of the same seed can coexist through preview and
   activation routes.
5. Execution failures, health failures, and logs are visible in runtime state.
6. Local development supports source-backed launches.
7. Shared environments support sealed packaged launches.
8. Execution authority lives behind the internal API boundary.

---

## Recommended First Implementation Cut

The most defensible first cut is:

1. add the execution domain model and runtime tables
2. implement a local backend only
3. make `Run` truly launch a local realization by reference
4. move `webd` to dynamic route bindings
5. then add packaging and the Kubernetes backend

That gives the kernel a correct execution architecture before it takes on
cluster-specific complexity.
