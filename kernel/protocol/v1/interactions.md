# Operational Interaction Contract

This document defines the normalized operational API contract for a realized
seed.

Boundary:

- define how a realized seed exposes normal app usage to both UI clients and
  third-party automation
- define the machine-readable contract every runnable realization must carry
- define the minimum linkage from seed-local domain objects to shared kernel
  capabilities
- do not restate registry append rules or materializer internals here

## Core Rule

Normal use of a realized seed must flow through the same semantic commands and
projections whether the caller is:

- the first-party UI
- a third-party integration
- an AI bot
- an operator tool

Do not make the UI the only complete interface.
Do not expose projection-table CRUD as the public operational contract.
Do not require normal clients to append raw claims directly.

The public operational API should normalize the interaction contract, not erase
the domain vocabulary.

Examples:

- `tickets.reply`
- `events.publish`
- `bids.place`

These commands may still create or interpret claims underneath, but external
callers should interact with the realized seed through semantic operations and
read models.

## Required Realization File

Every runnable realization must include `interaction_contract.yaml` beside
`realization.yaml`.

That file is the authoritative bridge between:

- the seed-local docs
- the realized API surface
- the shared kernel capabilities

The kernel validates this contract in `go test`, and `apid` exposes it through
contract-discovery endpoints.

## Required Contract Shape

`interaction_contract.yaml` must declare:

- `contract_version`
- `surface_kind`
- `seed_id`
- `realization_id`
- `summary`
- `links` back to seed design and realization docs
- supported `auth_modes`
- declared kernel `capabilities`
- seed-local `domain_objects`
- public `commands`
- public `projections`
- baseline `consistency` semantics

## Surface Kinds

Supported values:

- `interactive`
- `read_only`
- `bootstrap_only`

`interactive` realizations must expose at least one command and one projection.
`read_only` and `bootstrap_only` realizations may omit commands, but they still
need a declared projection surface.

## Capabilities

Capabilities are the shared kernel primitives a realization depends on.

Current allowed names are:

- `principals`
- `principal_identifiers`
- `memberships`
- `sessions`
- `consent`
- `auth_challenges`
- `handles`
- `access_links`
- `publications`
- `state_transitions`
- `activity_events`
- `jobs`
- `outbox`
- `threads`
- `messages`
- `assignments`
- `subscriptions`
- `notification_preferences`
- `uploads`
- `search_documents`
- `search_facets`
- `guard_decisions`
- `risk_events`

Every command and projection must reference only declared capabilities.
Every domain object must also declare the capability bindings it depends on.

## Domain Objects

Each domain object entry must define:

- `kind`
- `summary`
- `schema_ref`
- `capabilities`

`schema_ref` should point to the seed-local file that explains the object or
its schema, usually `design.md` or a dedicated schema document.

This is the minimum proof that the API surface is traceable back to the seed
docs a future agent starts from.

## Commands

Commands are the public write surface for normal app usage.

Each command must define:

- `name`
- `summary`
- `path`
- `object_kinds`
- `auth_modes`
- `idempotency`
- `input_schema_ref`
- `result_schema_ref`
- `capabilities`
- `projection`
- `consistency`

Path rule:

- commands live under `/v1/commands/{seed_id}/...`

Consistency values:

- `read_your_writes`
- `eventual`
- `async_job`

Idempotency values:

- `required`
- `recommended`
- `none`

## Projections

Projections are the public read surface for normal app usage.

Each projection must define:

- `name`
- `summary`
- `path`
- `object_kinds`
- `capabilities`
- `freshness`

Path rule:

- projections live under `/v1/projections/{seed_id}/...`

Freshness values:

- `materialized`
- `direct`
- `eventual`

## Process For New Seeds

If an agent starts from only the seed directory and kernel specs, the intended
process is:

1. Read `brief.md`, `design.md`, `acceptance.md`, and the selected
   `realization.yaml`.
2. Identify the seed-local domain objects that must exist at runtime.
3. Map those objects to shared kernel capabilities in
   `interaction_contract.yaml`.
4. Declare the commands that both UI and third-party callers will use.
5. Declare the projections those callers will read.
6. Implement the realization so the UI consumes those same commands and
   projections.
7. Verify the contract through `go test ./...`.
8. Inspect the materialized contract via `GET /v1/contracts`.

The critical rule is that a realized seed is not complete until this linkage is
defined and testable.

## Relationship To Growth

This document covers normal use of a realized seed once it exposes an
operational app surface.

The kernel growth console is a separate contract layer:

- it inspects a selected realization's current seed packet
- it queues `grow`, `tweak`, or `validate` work into `runtime_jobs`
- it exists so drafts can still be visible, inspectable, and agent-ready before
  they are runnable

That growth workflow is defined in [growth.md](growth.md).
