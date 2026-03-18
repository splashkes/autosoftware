# Design

This seed defines a calendar orchestration system that sits between many
external calendars and one internal production calendar.

## Product Shape

The system should behave like a time-network router:

- ingest from upstream calendars and event sources
- normalize those inputs into a common event model
- present operators with lineage, health, and conflict visibility
- propagate approved production events out to selected downstream calendars

## Operational Model

### Sources

Sources may include:

- Google calendars
- CalDAV or Apple calendars
- ICS feeds
- event platform APIs
- venue calendars
- internal production calendars

Each source should track its external identity, sync status, and most recent
successful incremental fetch.

### Canonical Event Layer

Imported events should be mapped into one canonical event model with explicit
provenance:

- where the event came from
- what upstream identifier it corresponds to
- when it was last seen or changed
- how it has been transformed for internal use

The canonical layer is the place where operators reason about conflicts,
authority, and downstream propagation.

### Authority

One production calendar should be designated as the authoritative outbound
source. Not every upstream source should be allowed to write everywhere else.

This keeps the system understandable:

- sources inform the hub
- the hub normalizes and adjudicates
- the production calendar drives propagation outward

### Propagation

Propagation should be explicit and observable. Each outbound edge needs
delivery state such as:

- `pending`
- `pushed`
- `failed`

Operators should be able to see what was attempted, what succeeded, and what
needs retry or intervention.

## UI Direction

The primary UI should feel operational rather than consumer-facing. Useful
views include:

- a signal board showing source and destination health
- event provenance and revision detail
- a propagation graph for each event
- a timeline across calendars
- filters for conflicts, unsynced changes, and propagation failures

## Kernel Boundary

This seed should remain mostly seed-local. Potential kernel support may help
with credential storage, scheduling, and artifact lineage, but the core product
logic belongs inside the seed's future realizations.

## Realization Guidance

Future realizations should prioritize:

- deterministic ingestion and propagation state
- clear provenance for every event
- operator visibility into sync failures and conflicts
- narrow, explicit authority rules between imported calendars and production
  calendars
