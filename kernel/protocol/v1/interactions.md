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
Do not treat first-party private screens as an exception to this rule.

The public operational API should normalize the interaction contract, not erase
the domain vocabulary.

Model that vocabulary as a graph of stable domain objects and explicit
relations, not as one large flattened record unless the seed is genuinely that
simple.
If the seed has meaningful related things such as versions, actors, venues,
media assets, or attendee records, declare them as first-class domain objects or
relation edges rather than hiding them inside one object's prose summary.

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
- seed-local `domain_relations` when the seed has meaningful graph edges
- public `commands`
- public `projections`
- baseline `consistency` semantics

When a seed depends on partially shared data boundaries, the contract should
also declare:

- object `data_layout`
- projection `data_views`

When a seed depends on graph traversal for reuse, the contract should also
declare:

- the related object kinds
- the explicit edge kinds connecting them
- the visibility of those edges when it differs between audiences
- any meaningful relation attributes, such as role, position, or timestamps

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

Graph-first guidance:

- model stable things as domain objects with stable IDs
- when content-bearing objects have meaningful edit history, model immutable
  version objects rather than silently overwriting one mutable record
- when authorship or editorial provenance matters, model the related actor or
  profile object explicitly
- if a public route uses a slug or handle, keep a stable machine identity for
  the underlying object
- if historical pages must remain stable when related objects change, make the
  snapshot-versus-reference rule explicit in the seed docs

If the object has a meaningful public/private/runtime boundary, it should also
declare `data_layout` using the seed-local `PUBLIC_PRIVATE_DATA.md` document or
an equivalent boundary definition:

- `shared_metadata`
- `public_payload`
- `private_payload`
- `runtime_only`

Each declared section should summarize the slice and list the named fields that
belong to it.

## Domain Relations

When the seed has meaningful graph edges, `domain_relations` should declare
them directly.

Each relation entry should define:

- `kind`
- `summary`
- `from_kinds`
- `to_kinds`
- `cardinality`
- `visibility`
- `schema_ref`
- `capabilities`

Optional relation metadata:

- `attributes`

Use relations for durable semantics such as:

- object-to-version linkage
- object-to-actor provenance
- object-to-media attachment
- object-to-venue or object-to-series membership
- attendee, registration, or check-in edges when the seed carries participation
  data

If a query can be phrased as "what X are related to Y under rule Z", that is a
strong signal the underlying model should include an explicit relation rather
than forcing every client to rediscover it through ad hoc filtering.

`visibility` is relation-level and may differ from node-level visibility:

- `public`
- `private`
- `mixed`
- `runtime_only`

`attributes` should be used when the edge itself carries truth, such as media
role, ordering, timestamps, or approval state.

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
- `auth_modes`
- `capabilities`
- `freshness`

Path rule:

- projections live under `/v1/projections/{seed_id}/...`

Freshness values:

- `materialized`
- `direct`
- `eventual`

If auth mode changes which object slices are visible, the projection should
also declare `data_views`:

- `auth_modes`
- `sections`
- `summary`

`sections` should be a subset of the object's data-layout buckets.

## Private And Split Read Surfaces

First-party private views are still part of the operational contract.

Rules:

- if the first-party UI can see or manage private state, the realization must
  declare a corresponding projection with non-anonymous `auth_modes`
- if one object has both public metadata and private full content, model those
  as separate projections rather than hiding the private read path behind
  server-only code
- if only a digest or very small metadata surface is public, declare that
  metadata-facing projection explicitly and keep the fuller projection private
- if projection auth modes expose different data slices, declare that split in
  `data_views` so the visibility boundary is machine-readable
- if a public route is keyed by a mutable-friendly handle or slug, also expose
  a stable object-id projection so alternate clients are not forced to treat
  the handle as the primary identity

This is the minimum shape needed for another UI, agent, or integration to
reuse the same contracts and data boundary decisions as the first-party app.

## Extensibility

Seeds should evolve additively.

Rules:

- if multiple clients should rely on a new truth, add it to the canonical graph
- if one realization needs extra structured truth that may later become shared,
  prefer a namespaced extension field or a new graph node over an untyped blob
- if a field exists only for rendering or transport convenience, keep it
  `runtime_only`
- do not silently repurpose an existing canonical field for a new meaning
- document which optional subgraphs are deferred versus canonical-but-not-yet
  fully operationalized

## Process For New Seeds

If an agent starts from only the seed directory and kernel specs, the intended
process is:

1. Read `brief.md`, `design.md`, `acceptance.md`, and the selected
   `realization.yaml`.
2. Identify the seed-local domain objects and graph relations that must exist at
   runtime.
3. Map those objects and relations to shared kernel capabilities in
   `interaction_contract.yaml`.
4. Declare the commands that both UI and third-party callers will use.
5. Declare the projections those callers will read, including private or
   metadata-only variants when visibility differs by audience.
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
