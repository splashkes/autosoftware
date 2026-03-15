# AS Plan: Registry Navigation and Agent Alignment

The goal is to make the object and schema registry navigable for both humans
and agents without creating two incompatible surfaces.

This plan splits the work into:

- Phase 1: a repo-derived registry catalog that exposes object and schema
  navigation from the currently implemented realization contracts
- Phase 2: a true append-only registry browser backed by `registryd` and
  `internal/registry/`

The key rule is that the web UI and agent-facing APIs must share one read
model. The UI may render richer affordances, but it should not invent data or
relationships that the API cannot return.

## Why This Split Is Necessary

Today the system already has:

- validated `interaction_contract.yaml` files
- domain objects with `schema_ref`
- commands and projections with input/result schema references
- a human-facing `webd` surface
- an agent-facing `apid` surface

Today the system does not yet have:

- a real append-only registry query layer in `registryd`
- implemented ledger storage and replay APIs in `internal/registry/`
- first-class claim browsing
- versioned registry cursors, change-set browsing, or row inspection

That means the next useful step is not to pretend the full registry exists.
The next useful step is to expose the registry-adjacent structure that the repo
already knows and validate that the machine and human surfaces agree on it.

## Phase 1: Repo-Derived Registry Catalog

Phase 1 is a registry catalog projection built from local realization
contracts.

It should expose:

- realizations that declare contracts
- domain object declarations per seed and realization
- schema references used by objects, commands, and command results
- the relationships between objects, schemas, commands, and projections

It should not claim to be:

- the accepted append-only ledger
- the source of truth for claims
- a complete replayable registry history

### Phase 1 Read Model

Build one shared catalog in `kernel/internal/registry/` that produces:

- catalog summary counts
- object summaries
- object detail records
- schema summaries
- schema detail records

The shared catalog should canonicalize schema references relative to the
contract file so the same schema can be recognized across multiple
realizations.

### Phase 1 APIs

Expose the same catalog through both `apid` and `webd`:

- `GET /v1/registry/catalog`
- `GET /v1/registry/objects`
- `GET /v1/registry/object?seed_id=...&kind=...`
- `GET /v1/registry/schemas`
- `GET /v1/registry/schema?ref=...`

API responses should include discovery links so agents can traverse from
summary to detail without guessing route shapes.

### Phase 1 Web Surface

Add a registry explorer panel to `webd` that consumes the same shared catalog.

The first pass should support:

- opening the registry explorer from the boot surface
- scanning object and schema summaries
- expanding object detail to inspect linked realizations, commands, and
  projections
- expanding schema detail to inspect where that schema is used

The first pass does not need:

- in-modal live filtering
- pagination
- editing
- acceptance or mutation flows

### Phase 1 Success Criteria

- agents can discover object and schema structure via stable JSON APIs
- humans can inspect the same structure in `webd`
- schema references are canonicalized consistently
- tests verify both the catalog builder and the API output shape

## Phase 2: True Registry Browser

Phase 2 begins once `registryd` and `internal/registry/` hold real append-only
state.

The browser should then shift from contract-derived structure to accepted
ledger state.

### Phase 2 Data Model

Implement real registry storage and query support for:

- change sets
- ordered rows
- object creation rows
- claim creation rows
- schema objects and schema versions
- cursors and replay windows

### Phase 2 APIs

At minimum, the true registry should expose:

- `GET /v1/registry/status`
- `GET /v1/registry/change-sets`
- `GET /v1/registry/change-sets/{change_set_id}`
- `GET /v1/registry/rows?after=...&limit=...`
- `GET /v1/registry/objects`
- `GET /v1/registry/objects/{object_id}`
- `GET /v1/registry/claims`
- `GET /v1/registry/claims/{claim_id}`
- `GET /v1/registry/schemas`
- `GET /v1/registry/schemas/{schema_id}`
- `GET /v1/registry/schema-versions/{version_id}`

The Phase 1 catalog routes should either:

- remain as compatibility projections over accepted state, or
- redirect clients toward the true ledger-native resources

### Phase 2 Web Surface

The web explorer should add:

- change-set timeline browsing
- row-level inspection
- cursor-based replay navigation
- claim history per object
- schema version history and adoption views
- federation source hints once remote registries are implemented

### Phase 2 Success Criteria

- the UI is browsing accepted registry state, not repo scaffolding
- agents can traverse accepted objects, claims, schemas, and rows directly
- human and agent surfaces still share one read model
- replay and cursor semantics are visible enough for debugging

## Execution Order

1. Add this plan.
2. Build the Phase 1 shared catalog in `internal/registry/`.
3. Expose synchronized JSON routes in `apid` and `webd`.
4. Add the Phase 1 `webd` explorer panel.
5. Test the catalog and HTTP API shape.
6. Leave the code organized so Phase 2 can replace the backing store without
   changing the outer contract more than necessary.
