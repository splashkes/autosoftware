# Public and Internal API Split Plan

## Purpose

This plan defines how `apid` should eventually be split into two separate API
surfaces:

- a **public operational API** for normal client, UI, integration, and agent use
- an **internal control API** for kernel runtime operations, growth jobs, and
  operator-only capabilities

The goal is to align deployment shape with the architecture Autosoftware
already describes, without breaking the current single-binary implementation
before the rest of the system is ready.

---

## Problem

Today, `cmd/apid/` mixes several kinds of traffic behind one server:

- contract discovery
- public seed commands and projections
- growth workflow endpoints
- runtime identity and web-state endpoints
- runtime job queue endpoints
- registry catalog endpoints

That is acceptable for local development and early deployment, but it creates
an avoidable boundary problem in production:

- the architecture wants a public machine-facing API
- the implementation currently mounts operator and runtime-control routes on
  the same listener
- the security model is still permissive, with session resolution but not hard
  auth enforcement
- deployment has to compensate with ingress rules instead of the binary
  expressing the boundary directly

The result is that "make `apid` public" is correct at the architecture level,
but overly broad at the route level.

---

## Current State

### What `apid` does now

`cmd/apid/` currently registers:

- `ContractsAPI`
- `RuntimeAPI`
- `GrowthAPI`
- `RegistryAPI`

This means one process currently serves:

- `GET /v1/contracts`
- `POST /v1/commands/realizations.grow`
- `GET /v1/projections/realization-growth/...`
- `POST /v1/runtime/...`
- `GET /v1/runtime/...`
- `GET /v1/registry/...`

### Why this is awkward

These routes do not all have the same intended audience.

- Public app and integration clients should be able to use semantic commands,
  projections, and contract discovery.
- Public registry readers should be able to inspect accepted state.
- Workers, operators, and kernel internals should own runtime jobs, growth
  orchestration, and low-level runtime state mutations.

That split is conceptual in the docs already, but not structural in the code.

---

## Desired End State

Autosoftware should expose two machine-facing servers:

### 1. Public API

This is the external operational contract.

Responsibilities:

- public contract discovery
- public seed commands
- public seed projections
- public registry query and browse routes
- public machine-facing integration surface for bots, agents, and third-party
  tools

Suggested binary:

- `cmd/publicapid/`

Suggested hostnames:

- `api.<domain>`
- or split registry further as `registry.<domain>` if desired

### 2. Internal API

This is the control-plane and runtime-management contract.

Responsibilities:

- runtime identity/session primitives
- handles, access links, publications, and state transitions
- runtime jobs and worker claim/complete/fail flows
- growth job enqueue and inspection
- operator-only inspection and future admin endpoints

Suggested binary:

- `cmd/internalapid/`

Suggested exposure:

- private network only
- cluster-internal service only
- optionally reachable through a VPN, bastion, or authenticated operator proxy

---

## API Ownership Model

The route split should follow intent, not current implementation convenience.

### Public API routes

These belong on the public operational surface:

- `GET /v1/contracts`
- `GET /v1/contracts/{seed_id}/{realization_id}`
- public seed command routes under `/v1/commands/{seed_id}/...`
- public seed projection routes under `/v1/projections/{seed_id}/...`
- public registry read routes under `/v1/registry/...`

Notes:

- The public API should expose semantic operations, not raw projection-table
  CRUD.
- Registry browse routes may remain in `registryd` instead of the public API
  if AS keeps registry authority as its own service. That is still compatible
  with this plan.

### Internal API routes

These should move off the public listener:

- `GET /v1/runtime/health`
- all `/v1/runtime/...` mutation routes
- worker queue claim/complete/fail routes
- growth queue routes:
  - `POST /v1/commands/realizations.grow`
  - `GET /v1/projections/realization-growth/...`

Rationale:

- They are control-plane behavior, not normal app usage.
- They mutate shared runtime state directly.
- They are tightly coupled to workers, operators, and kernel internals.
- They will require stricter authentication and authorization than ordinary
  public app commands.

---

## Principles

### 1. Public API is semantic

The public API should describe the app and registry the way outside callers
think about them:

- commands
- projections
- contracts
- registry resources

It should not expose low-level runtime mechanics as the default external
surface.

### 2. Internal API is operational

The internal API should own:

- shared runtime primitives
- queue management
- growth orchestration
- worker coordination
- privileged system actions

This is the kernel control plane.

### 3. Route intent matters more than package history

Do not preserve the current route grouping just because the code was first
written in one binary. The split should follow long-term meaning.

### 4. Split listener first, refactor internals second

The first successful step is separate binaries or servers with explicit route
ownership. Internal package cleanup can follow.

---

## Migration Strategy

### Phase 1: Document and gate the existing mixed surface

Before introducing new binaries:

- explicitly classify every current route as public or internal
- keep `apid` public only with ingress/path-level restrictions
- prevent internal routes from being exposed broadly in production
- add documentation saying the mixed surface is transitional

Outcome:

- deployment becomes safer immediately
- no code split required yet

### Phase 2: Extract route registration into composable modules

Refactor route assembly so handlers can be registered independently.

For example:

- `RegisterPublicContractsAPI(...)`
- `RegisterPublicOperationalAPI(...)`
- `RegisterInternalRuntimeAPI(...)`
- `RegisterInternalGrowthAPI(...)`
- `RegisterRegistryReadAPI(...)`

The important part is not the exact function names. The important part is
making route ownership explicit and reusable across binaries.

Outcome:

- current `apid` can still exist temporarily
- new binaries can be introduced without duplicating handler wiring

### Phase 3: Introduce `publicapid`

Create a new public-facing API binary that serves only:

- contracts
- public operational commands and projections
- any other explicitly public machine-facing routes

Outcome:

- external integrations stop depending on the mixed `apid`

### Phase 4: Introduce `internalapid`

Create a private control-plane API binary that serves:

- runtime routes
- growth routes
- worker queue routes
- future privileged control endpoints

Outcome:

- runtime and control-plane behavior no longer share a public listener

### Phase 5: Remove mixed-surface `apid`

After consumers and deployment are moved:

- deprecate the old combined binary
- remove transitional ingress rules
- keep the public/internal boundary in code and deployment permanently aligned

Outcome:

- architecture and implementation finally match

---

## Backward Compatibility

This split should not force a sudden route break.

Recommended compatibility approach:

- keep route paths stable where possible
- change hostnames/listeners before changing path shapes
- preserve public contracts exactly during the extraction
- treat internal clients and workers as the first consumers to migrate

This means:

- external clients can keep calling the same public routes
- only the serving binary and hostname may change first
- worker and operator tooling can be migrated in a controlled sequence

---

## Deployment Implications

This split fits the planned DigitalOcean deployment cleanly.

### Near term

For the first deployment:

- `webd` public
- `registryd` public
- `apid` public with restricted internal paths
- `materializerd` internal

This is acceptable as an interim state.

### Later

After the split:

- `webd` public
- `registryd` public
- `publicapid` public
- `internalapid` private
- `materializerd` private

That is the shape the deployment should grow toward.

---

## Security Benefits

Splitting `apid` improves several things immediately:

- narrower public attack surface
- clearer auth requirements by route family
- reduced chance of accidentally exposing worker/job-control endpoints
- better least-privilege service-to-service policy
- simpler ingress and firewall rules
- easier rate limiting and observability by audience

It also makes future auth work more coherent:

- public API auth can focus on user, bot, and integration identity
- internal API auth can focus on service identity, worker identity, and
  operator privilege

---

## Design Questions To Resolve

The split leaves a few deliberate choices open.

### 1. Does registry read stay in `registryd` or move under `publicapid`?

Two valid shapes exist:

- `registryd` remains the dedicated public registry authority
- `publicapid` proxies or embeds registry read routes

Recommendation:

- keep `registryd` as its own authority
- keep `publicapid` focused on operational app APIs

That preserves the conceptual distinction already present in the architecture.

### 2. Should growth be operator-only forever?

Probably not forever, but for the near-term system it should be treated as
internal control-plane behavior, not anonymous public API.

### 3. Should all `/v1/runtime/*` routes remain internal?

Most likely yes. If some runtime capabilities eventually become public-facing,
they should be re-exposed through a deliberate public contract, not by making
the raw internal route family public by default.

---

## Suggested Implementation Work

When this work is scheduled, the implementation should include:

1. A route inventory document that marks each endpoint public or internal.
2. A route registration refactor that separates handler groups cleanly.
3. A new public API binary.
4. A new internal API binary.
5. Deployment manifests that place public and internal services on different
   listeners and network policies.
6. Tests proving public binaries do not expose internal route families.
7. Auth and authorization middleware appropriate to each surface.

---

## Success Criteria

This plan is complete when:

- public machine-facing routes are served by a dedicated public API binary
- runtime/growth/operator routes are served only by a private API binary
- deployment no longer relies on ingress tricks to express the core boundary
- public and internal route families have distinct auth models
- the architecture docs and implementation say the same thing

---

## Summary

`apid` is currently doing two jobs:

- acting as the public machine-facing API
- acting as the internal runtime and growth control plane

That is useful for early development but wrong as a long-term boundary.

The correct end state is:

- a public operational API for commands, projections, contracts, and possibly
  registry-adjacent read integration
- a private internal API for runtime primitives, jobs, growth, and operator
  control

The transition should happen in stages so current deployment can proceed now
without blocking on a larger refactor.
