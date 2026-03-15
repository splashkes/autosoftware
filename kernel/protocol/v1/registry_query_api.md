# Registry Query API

This document defines the Phase 2 read API for the append-only registry.

Boundary:

- define the public read surface for accepted registry state
- define resource identity and list/detail route shapes
- define pagination and cursor semantics
- define how objects, claims, schemas, rows, and change sets relate
- do not define write/append endpoints in this document
- do not restate materializer rules beyond what the query API must expose

## Purpose

The registry query API exists so:

- workers can synchronize accepted rows deterministically
- operators can inspect accepted history
- agents can traverse accepted objects, claims, schemas, and change sets
- web surfaces can browse accepted state without inventing private read models

This API is for accepted registry history, not for normal application use.
Normal app use should still flow through seed-specific commands and
projections.

## Core Rule

The API must expose accepted history in registry order.

That means:

- row ordering is authoritative
- cursors advance by accepted row order
- list endpoints must not reorder rows by timestamp
- object, claim, and schema detail may summarize current understanding, but
  they must preserve links back to accepted rows and change sets

## Resource Identity

Phase 2 uses stable global identifiers for registry resources.

Required identifiers:

- `registry_id`
- `change_set_id`
- `row_id`
- `object_id`
- `claim_id`
- `schema_id`
- `schema_version_id`

Required properties:

- identifiers are opaque strings to clients
- identifiers are globally stable within one registry
- object detail does not rely on `seed_id + kind` as primary identity
- schema detail distinguishes schema identity from schema version identity

Human-readable fields such as `kind`, `seed_id`, `reference`, or `name` may
appear in payloads, but they are not the canonical IDs.

## Common Response Shape

Every successful response should return JSON.

Detail responses should use:

```json
{
  "resource": { "...": "..." },
  "discovery": { "...": "..." }
}
```

List responses should use:

```json
{
  "items": [{ "...": "..." }],
  "page": {
    "limit": 100,
    "next_after": "..."
  },
  "discovery": { "...": "..." }
}
```

The API may add summary fields beside `items`, but it should not omit the page
block from cursorable list endpoints.

## Error Shape

Errors should return:

```json
{
  "error": {
    "code": "not_found",
    "message": "object not found"
  }
}
```

Required error codes:

- `bad_request`
- `not_found`
- `conflict`
- `forbidden`
- `internal`

Do not return raw storage errors to clients.

## Pagination and Cursor Rules

Cursor semantics must be explicit and consistent.

### For Ordered Row Streams

`GET /v1/registry/rows` is the synchronization surface.

Rules:

- rows are returned in ascending registry order
- `after` means strictly greater than the supplied last-seen row cursor
- if `after` is omitted, the stream begins at the earliest retained row
- `limit` defaults to `100`
- `limit` maximum is `1000`
- `next_after` is the last returned `row_id`
- if fewer than `limit` rows are returned, the client must still trust
  `next_after`

Conceptually:

- request: `GET /v1/registry/rows?after=row_100&limit=100`
- result: rows with `row_id > row_100`

### For Non-Row List Endpoints

Object, claim, schema, and change-set list endpoints may use cursor pagination
too, but the cursor must reflect the route's declared ordering.

Rules:

- each endpoint must document its ordering key
- `next_after` must be opaque to clients
- clients must not derive meaning from non-row cursors

## Discovery Root

`GET /v1/registry/status`

Purpose:

- health and identity of the registry authority
- high-level discovery entry point

Required response fields:

- `registry_id`
- `status`
- `api_version`
- `discovery.rows`
- `discovery.change_sets`
- `discovery.objects`
- `discovery.claims`
- `discovery.schemas`

Example:

```json
{
  "resource": {
    "registry_id": "as-public",
    "status": "ok",
    "api_version": "v1"
  },
  "discovery": {
    "rows": "/v1/registry/rows",
    "change_sets": "/v1/registry/change-sets",
    "objects": "/v1/registry/objects",
    "claims": "/v1/registry/claims",
    "schemas": "/v1/registry/schemas"
  }
}
```

## Change Sets

### List

`GET /v1/registry/change-sets`

Purpose:

- browse accepted mutation boundaries
- support audit and review lookup

Allowed query params:

- `after`
- `limit`
- `accepted_by`
- `source_reference`

Required item fields:

- `change_set_id`
- `registry_id`
- `accepted_at`
- `accepted_by`
- `idempotency_key`
- `source_reference`
- `row_count`
- `first_row_id`
- `last_row_id`
- `self`

Ordering:

- ascending by accepted registry position

### Detail

`GET /v1/registry/change-sets/{change_set_id}`

Required detail fields:

- all summary fields
- `rows`
- `provenance`
- `artifacts`

The embedded `rows` block may be truncated, but it must include row discovery
links.

## Rows

### List

`GET /v1/registry/rows`

This is the authoritative synchronization stream.

Allowed query params:

- `after`
- `limit`
- `row_type`
- `change_set_id`
- `object_id`

Required item fields:

- `row_id`
- `registry_id`
- `change_set_id`
- `row_type`
- `object_id`
- `claim_id` when present
- `schema_id` when present
- `schema_version_id` when present
- `accepted_at`
- `payload`
- `self`

Initial required row types:

- `object.create`
- `claim.create`

The route may later add more row types without changing pagination semantics.

### Detail

`GET /v1/registry/rows/{row_id}`

Required detail fields:

- all list item fields
- `position`
- `provenance`
- `supersedes` when applicable
- `superseded_by` when applicable

## Objects

### List

`GET /v1/registry/objects`

Purpose:

- browse stable identities referenced by claims

Allowed query params:

- `after`
- `limit`
- `kind`
- `seed_id`
- `schema_id`
- `source_reference`

Required item fields:

- `object_id`
- `registry_id`
- `kind`
- `seed_id` when applicable
- `created_at`
- `created_by`
- `create_row_id`
- `self`

Ordering:

- ascending by object creation position

### Detail

`GET /v1/registry/objects/{object_id}`

Required detail fields:

- all summary fields
- `current_claims`
- `claim_history`
- `related_schema_ids`
- `first_change_set_id`

Rules:

- `current_claims` is a convenience view only
- `claim_history` must link back to accepted rows and claims
- clients must be able to ignore `current_claims` and still reconstruct
  history from `claim_history`

## Claims

### List

`GET /v1/registry/claims`

Purpose:

- browse accepted assertions and supersession chains

Allowed query params:

- `after`
- `limit`
- `object_id`
- `schema_id`
- `schema_version_id`
- `claim_type`
- `change_set_id`

Required item fields:

- `claim_id`
- `registry_id`
- `object_id`
- `claim_type`
- `schema_id`
- `schema_version_id`
- `create_row_id`
- `change_set_id`
- `accepted_at`
- `supersedes` when applicable
- `self`

Ordering:

- ascending by claim creation position

### Detail

`GET /v1/registry/claims/{claim_id}`

Required detail fields:

- all summary fields
- `payload`
- `superseded_by` when applicable
- `provenance`

Rules:

- claim detail must never require floating schema interpretation
- claim detail must identify the exact `schema_version_id` that governs the
  claim

## Schemas

### List

`GET /v1/registry/schemas`

Purpose:

- browse schema objects separately from schema versions

Allowed query params:

- `after`
- `limit`
- `kind`
- `name`
- `published`

Required item fields:

- `schema_id`
- `registry_id`
- `kind`
- `name`
- `latest_version_id`
- `created_at`
- `self`

Ordering:

- ascending by schema object creation position

### Detail

`GET /v1/registry/schemas/{schema_id}`

Required detail fields:

- all summary fields
- `versions`
- `used_by_claim_count`
- `used_by_object_count`

The detail response may include a compact current-version summary, but it must
not collapse schema identity and schema version identity into one record.

## Schema Versions

### Detail

`GET /v1/registry/schema-versions/{schema_version_id}`

Required detail fields:

- `schema_version_id`
- `schema_id`
- `version`
- `status`
- `published_at`
- `artifact`
- `create_row_id`
- `used_by_claims`
- `self`

Rules:

- published schema versions are immutable
- clients should treat version identifiers as permanent

## Visibility and Redaction

The public registry may expose only public accepted history.

Rules:

- private claims may be omitted entirely from public list endpoints
- if a private row is omitted, row ordering exposed to a public client must
  still remain internally consistent
- redaction must not cause one public row to appear to have been accepted
  before another public row if the opposite is true in registry order

If the system cannot satisfy that rule with omission alone, it should use a
public registry authority with its own accepted history rather than leaking a
partial private ledger.

## Compatibility With Phase 1 Catalog

The Phase 1 catalog routes are convenience projections:

- `GET /v1/registry/catalog`
- `GET /v1/registry/object?...`
- `GET /v1/registry/schema?...`

In Phase 2 these routes may remain, but only as derived projections over the
true registry resources defined here.

They must not become the only way agents can inspect accepted registry state.

## Out Of Scope

This document does not define:

- append/write endpoints
- signature formats
- remote federation synchronization
- retention or archival policy
- materializer APIs

Those should be specified separately once the read contract above is stable.
