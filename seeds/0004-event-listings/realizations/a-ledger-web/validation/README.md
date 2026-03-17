# Validation

Validation evidence for this realization should cover:

- kernel launch-time injection of `AS_RUNTIME_DATABASE_URL`
- organizer create, update, publish, unpublish, cancel, and archive flows
- public directory, calendar, and detail rendering from materialized state
- per-event ledger/history browsing in both HTML and JSON projections
- stable handle behavior across edits

Current local validation target:

- `go test ./...` inside `artifacts/event-listings-app`
- `go test ./...` inside `kernel`
- manual smoke run against a local Postgres-backed launch

## Evidence captured on March 16, 2026

- `go test ./...` passed in `artifacts/event-listings-app`
- `go test ./...` passed in `kernel`
- unit coverage confirms:
  - stable handles survive edits
  - public directory excludes draft and archived events
  - multi-day events render across covered calendar days
  - ledger history records snapshot and status claims

## Remaining manual validation

- end-to-end browser smoke against a live Postgres-backed launch
- HTTP checks for:
  - `GET /healthz`
  - `GET /`
  - `GET /calendar`
  - `GET /events/{slug}/ledger`
  - `GET /v1/projections/0004-event-listings/events`
  - `GET /v1/projections/0004-event-listings/events/{event_id}/ledger`
  - all new command endpoints under `/v1/commands/0004-event-listings/`
