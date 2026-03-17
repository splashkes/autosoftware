# Design

This seed defines the first authoritative registry browser as a public trust
surface.

## Request Interpretation

The request is not just for "another admin screen."
It is for a public-facing observability layer that lets a user or community
member answer:

- what objects exist
- what claims exist about them
- which schema version governs a claim
- which change set and rows accepted that state
- how an agent should retrieve the same truth through the API

The browser should therefore be designed as a read-only lens over accepted
registry state, not as a convenience dashboard detached from the ledger.

The first realization should use the registry APIs that already exist today as
its primary source of truth:

- `GET /v1/registry/catalog`
- `GET /v1/registry/realizations`
- `GET /v1/registry/realization?reference=...`
- `GET /v1/registry/commands`
- `GET /v1/registry/command?reference=...&name=...`
- `GET /v1/registry/projections`
- `GET /v1/registry/projection?reference=...&name=...`
- `GET /v1/registry/objects`
- `GET /v1/registry/object?seed_id=...&kind=...`
- `GET /v1/registry/schemas`
- `GET /v1/registry/schema?ref=...`

Claims, schema versions, change sets, and rows remain part of the long-term
target, but the first realization should not invent private substitute APIs
for them.

## Early Design Check

Before implementation begins, confirm these points with the user:

1. The first realization should optimize for trust and clarity rather than a
   dense power-user console.
2. The UI should use progressive disclosure: simple overview first, deeper
   detail on demand.
3. The authoritative API remains the primary machine interface.
   The browser may help people discover it, but must not replace it.
4. The first realization should be built on the current catalog routes, with
   deeper ledger resources added only when their real routes exist.

If any of those are wrong, the realization should be redirected before code is
produced.

## Approach Candidates

### A. Authoritative Browser

A conservative server-rendered browser with:

- a simple landing page with counts, resource types, and "how to use the API"
- searchable list pages
- detail pages with expandable raw fields and provenance links
- explicit links to the authoritative registry routes backing each screen

This should be the first realization.

### B. Rich Explorer

A heavier client-side explorer with panes, saved filters, cursor inspection,
and side-by-side comparisons.

This may become a later realization, but should not be the first pass because
it risks making the trust surface harder to understand.

### C. Ledger Reading Room

A refinement realization that keeps the same authoritative routes and read-only
trust posture, but changes the human-facing information architecture.

This realization should:

- lead with systems and plain-language purpose before ontology buckets
- treat commands as actions and projections as read models in human-facing copy
- separate domain software from registry-internal meta resources
- show lifecycle, availability, and surface kind as distinct axes
- place summaries, relationships, and provenance ahead of raw route strings

This should be a later refinement of the seed, not a replacement for the first
authoritative browser.

## Scope

Included in this seed:

- read-only browsing for registry resources
- simple search and facet filtering for list views
- deep detail views with IDs, provenance, and schema linkage
- explicit API discovery and agent usage instructions
- visible public/private redaction rules where relevant
- grouped data-layout rendering when a governed thing declares
  shared/public/private/runtime-only sections
- graph-relation rendering when a contract declares explicit domain relations

Included in the first realization:

- registry catalog overview
- realizations, commands, projections, objects, and schemas
- list/detail navigation built on the current read-only registry routes
- explicit identification of current catalog projections versus future ledger
  resources

Included in later realizations of this seed when the authoritative routes
exist:

- claims
- schema versions
- change sets
- rows

Out of scope for this seed:

- append or mutation routes
- claim editing, moderation, or direct authoring tools
- remote federation synchronization UI
- private operator-only tools that are not safe for public understanding

## Rendering Model

The first realization uses server-rendered HTML.

The projection paths defined in the interaction contract (e.g.
`/v1/projections/0006-registry-browser/objects`) serve structured JSON for
agent and programmatic access.

The browser routes serve HTML pages that consume the same backing catalog data.
Browser routes live under a seed-local prefix (e.g. `/browser/`) and are
distinct from projection paths.

Each HTML page should include a visible link to the corresponding projection
or authoritative catalog route so a user can switch from the rendered view to
the machine-readable source.

The browser does not proxy or wrap the authoritative registry API. It reads
the same catalog data and renders it as HTML. Agents should use the projection
paths or the authoritative registry routes directly.

## Human Experience

### Simple Layer

The default experience should help a new visitor orient quickly:

- what the registry is
- which resource types exist
- where to start browsing
- how to search
- how to switch from a convenience summary to authoritative detail

This layer should prefer plain language, short explanations, and obvious next
steps.

### Deep Layer

Every major resource that has an authoritative route should support deeper
inspection with:

- stable IDs
- timestamps and acceptance provenance
- links to related resources that currently exist
- explicit incoming and outgoing relation context when the contract declares it
- supersession chains for claims when claim routes exist
- schema identity separated from schema version identity when schema-version
  routes exist
- raw payload or structured field display where safe
- grouped public/private/shared/runtime layout display when the contract
  declares it

The deep layer should make it possible to answer disputes or ambiguity without
leaving the browser.

## Registry Resource Model

This seed treats the following as first-class target resources:

- objects
- claims
- schemas
- schema versions
- change sets
- rows

In the first realization, the concrete entry points should be:

- realizations
- commands
- projections
- objects
- schemas

Objects and schemas are the main human entry points today.
Claims, change sets, rows, and schema versions remain the accountability layer
to expose once the true routes land.

## Affected Surfaces

### Browser UI

The UI should expose:

- a registry home page
- list views for the resource types currently available
- detail views for the resource types currently available
- visible provenance trails from catalog summaries into deeper current detail
- inline API references and copyable route examples

### Authoritative API

For the first realization, the browser should use the existing registry
catalog routes directly:

- `GET /v1/registry/catalog`
- `GET /v1/registry/realizations`
- `GET /v1/registry/realization?reference=...`
- `GET /v1/registry/commands`
- `GET /v1/registry/command?reference=...&name=...`
- `GET /v1/registry/projections`
- `GET /v1/registry/projection?reference=...&name=...`
- `GET /v1/registry/objects`
- `GET /v1/registry/object?seed_id=...&kind=...`
- `GET /v1/registry/schemas`
- `GET /v1/registry/schema?ref=...`

The first realization should use these routes as much as possible rather than
adding seed-local replacement APIs.

When the stricter ledger-backed routes from
`kernel/protocol/v1/registry_query_api.md` are implemented, later realizations
of this seed should add claims, schema versions, change sets, and rows on top
of those true routes instead of inventing a custom browse layer.

### Agent Guidance

The product must explicitly tell agent authors:

- start with `GET /v1/registry/catalog` for the current compatibility layer
- use the current list and detail routes directly
- use `GET /v1/registry/status` and the stricter discovery root once the true
  ledger-backed routes exist
- do not scrape the browser HTML for authoritative data

This guidance should live in the UI and in seed-local realization docs.

## Data And Claim Changes

This seed does not change the registry write model.
It changes how accepted registry state is observed.

Expected derived claims or projections may include:

- resource counts and browse summaries
- faceted search indexes over public registry resources
- projection metadata that links UI pages back to authoritative routes

The underlying authoritative record remains the accepted object, claim,
schema, change-set, and row history.

## Interface Changes

The first realization should provide seed-specific browser routes or
projections only where needed for presentation, while clearly pointing back to
the current authoritative registry API.

The browser should make these distinctions obvious:

- seed-local presentation route versus authoritative registry route
- human-readable label versus stable identifier
- current catalog projection versus future ledger-backed resource

## Artifact Changes

Expected realization artifacts include:

- browser routes and templates
- read-only search and facet UI components
- detail panels or pages for provenance inspection
- API documentation snippets or copyable request examples
- validation evidence showing that no mutation path was introduced

## Rollback Or Supersession Notes

This seed should supersede ad hoc registry inspection patterns that require
source diving or private implementation knowledge.

If a later realization improves the browser, it should still preserve:

- read-only behavior
- authoritative route linkage
- explicit provenance paths
- agent-facing API instructions

The first realization should also preserve compatibility with the existing
catalog routes instead of bypassing them prematurely.

See also:

- `kernel/protocol/v1/registry_query_api.md`
- `plans/07-registry-navigation-and-agent-alignment-plan.md`
