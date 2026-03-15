# Authoritative Browser Realization

This realization is the first planned implementation of the registry browser
seed.

It should make the public registry feel trustworthy, not mysterious.
That means:

- simple navigation first
- deeper authoritative detail on demand
- visible links from UI screens to the API routes that back them
- no mutation controls

## Current API First

This realization should use the existing registry APIs as its main data
source:

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

It should not create substitute seed-local APIs for those same read surfaces.
New browse APIs should be added only where the registry itself does not yet
provide the required authoritative route.

## Intended User Experience

The realization should provide:

- a registry home page with counts, resource categories, and a short
  explanation of the registry model
- searchable list pages for realizations, commands, projections, objects, and
  schemas
- deep detail views for the current routes above
- explicit signposting for future claims, change sets, rows, and schema
  versions when those authoritative routes are not yet present
- explicit copyable API examples for agents and advanced users

## Agent Access

Agents should use the authoritative registry API directly.

Expected agent workflow:

1. `GET /v1/registry/catalog` to discover the current compatibility layer
2. follow the current list routes for the relevant resource type
3. use the current detail routes for exact identifiers
4. switch to `GET /v1/registry/status` and stricter ledger routes once those
   routes are actually implemented

Agents should not rely on HTML scraping when authoritative routes exist.

## Realization Boundary

This realization may add convenience projections for the browser shell, but it
must not replace the authoritative registry routes with private browser-only
queries.

The browser should always make it obvious which route is authoritative for the
screen being viewed.

Claims, change sets, rows, and schema versions belong in the broader seed
target, but this realization should expose them only when their real registry
routes exist.
