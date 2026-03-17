# Validation

Validation evidence for this realization should cover:

- kernel launch-time injection of `AS_RUNTIME_DATABASE_URL`
- organizer create, update, publish, unpublish, cancel, and archive flows
- public directory, calendar, and detail rendering from materialized state
- per-event ledger/history browsing in both HTML and JSON projections
- stable by-ID event projection for alternate clients
- private organizer workspace projection with auth enforcement
- command auth enforcement for create, update, publish, unpublish, cancel, and archive
- stable handle behavior across edits

Current local validation target:

- `go test ./...` inside `artifacts/event-listings-app`
- `go test ./...` inside `kernel`
- manual smoke run against a local Postgres-backed launch

## Evidence captured on March 17, 2026

- `go test ./...` passed in `artifacts/event-listings-app`
- `go test ./...` passed in `kernel`
- unit coverage confirms:
  - stable handles survive edits
  - public directory excludes draft and archived events
  - multi-day events render across covered calendar days
  - ledger history records snapshot and status claims with non-zero revision counts
- local Postgres-backed smoke validation passed against `postgres://postgres:postgres@127.0.0.1:54329/as_local?sslmode=disable`
- HTTP checks passed:
  - `GET /healthz`
  - `GET /`
  - `GET /events/harbour-lights-night-market/ledger`
  - `GET /v1/projections/0004-event-listings/events`
  - `GET /v1/projections/0004-event-listings/events/by-id/{event_id}`
  - `GET /v1/projections/0004-event-listings/events/by-id/{event_id}/ledger`
  - `GET /v1/projections/0004-event-listings/admin/events`
  - `POST /v1/commands/0004-event-listings/events.create`
  - `POST /v1/commands/0004-event-listings/events.publish`
  - `POST /v1/commands/0004-event-listings/events.update`
- browser-rendered HTML checks passed:
  - event detail page shows the ledger entry point
  - ledger page renders accepted claim history with both `event.snapshot` and `event.status`
- API parity checks passed:
  - anonymous callers cannot write through `/v1/commands/0004-event-listings/*`
  - organizer or service-token callers can read draft organizer projections and write through the declared command surface
- raw database checks passed:
  - `as_event_listings_objects` populated
  - `as_event_listings_claims` populated
  - `as_event_listings_materialized_events` shows the smoke event with `revision_count = 4` and `status = published`
