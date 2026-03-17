# Event Listings MVP

Server-rendered Go MVP for the `0004-event-listings` seed.

Features included:

- organizer login and event management workspace
- richer organizer authoring with cover image URL, organizer identity, tags, venue notes, and editorial blurb
- draft, published, canceled, and archived lifecycle actions
- stable public event URLs that survive edits
- stable by-ID event projection for alternate clients
- private organizer workspace projection exposed through the same contract
- public upcoming list, curated discovery modules, month calendar, and event detail pages
- keyword search plus category, location, and date-range filtering
- social/share affordances including recommendation copy, save/share signals, related-event suggestions, and copy-link action
- file-backed local persistence so events survive restarts
- JSON endpoints aligned with the realization contract

Run locally:

```bash
go run .
```

Environment:

- `AS_ADDR` — unix socket path (e.g. `/tmp/as-realizations/0004-event-listings--a-web-mvp.sock`); falls back to `127.0.0.1:8096` if unset
- `AS_ADMIN_PASSWORD` defaults to `admin`
- `AS_DATA_FILE` defaults to `data/events.json`
- `AS_SERVICE_TOKEN` enables service-token access to organizer projections and commands

Organizer login:

- visit `/admin/login`
- sign in with the configured password
