# Event Listings Ledger Web

Server-rendered Go realization for the `0004-event-listings` seed.

Features included:

- organizer login and event management workspace
- richer organizer authoring with cover image URL, organizer identity, tags, venue notes, and editorial blurb
- draft, published, canceled, and archived lifecycle actions
- stable public event URLs that survive edits
- durable event object IDs backed by the shared runtime database
- append-only event claims for snapshots and lifecycle changes
- materialized current-state projections for directory, calendar, detail, and ledger views
- human-visible ledger pages for each event object
- public upcoming list, curated discovery modules, month calendar, and event detail pages
- keyword search plus category, location, and date-range filtering
- social/share affordances including recommendation copy, save/share signals, related-event suggestions, and copy-link action
- JSON endpoints aligned with the realization contract

Run locally:

```bash
AS_RUNTIME_DATABASE_URL=postgres://postgres:postgres@127.0.0.1:54329/as_local?sslmode=disable go run .
```

Environment:

- `AS_ADDR` — unix socket path (e.g. `/tmp/as-realizations/0004-event-listings--a-ledger-web.sock`); falls back to `127.0.0.1:8096` if unset
- `AS_ADMIN_PASSWORD` defaults to `admin`
- `AS_RUNTIME_DATABASE_URL` is required and points at the shared runtime database used for durable event objects and claims

Organizer login:

- visit `/admin/login`
- sign in with the configured password
