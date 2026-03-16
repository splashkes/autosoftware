# Ledger Reading Room Realization

This realization is a runnable refinement of the registry browser seed.

It keeps the current authoritative browser intact and proposes a second,
human-first reading room built on the same accepted registry state.

The intent is not to weaken rigor.
It is to change the order in which rigor is introduced.

## Why This Exists

The direct catalog browser is useful for exact inspection, but it starts with
internal ontology terms:

- realizations
- commands
- projections
- objects
- schemas

A newcomer usually starts elsewhere:

- what software systems exist
- what those systems govern
- what actions they permit
- how those actions are seen
- why the record should be trusted

This realization reorders the browse experience around those questions.

## Core Refinements

- lead with systems before ontology buckets
- show plain-language purpose before stable IDs
- treat commands as actions in human-facing copy
- treat projections as read models in human-facing copy
- separate registry internals from domain software
- show lifecycle, availability, and surface kind as distinct concepts
- keep authoritative routes visible, but secondary to explanation

## Authority Model

This realization does not replace the authoritative registry API.

It should continue to derive from:

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

The refinement is in presentation, grouping, glossary, and wayfinding.
It should not require kernel changes or new non-authoritative hidden APIs.

## Intended Surfaces

The reading room should introduce these primary browse areas:

- systems
- governed things
- actions
- read models
- contracts
- registry internals

Every page in those areas should still point back to the exact authoritative
route or routes that ground the view.

## Artifacts In This Realization

- `artifacts/information_architecture.md`
- `artifacts/page_blueprints.md`
- `artifacts/route_map.yaml`

These artifacts describe the refined browse model without changing the current
running realization.
