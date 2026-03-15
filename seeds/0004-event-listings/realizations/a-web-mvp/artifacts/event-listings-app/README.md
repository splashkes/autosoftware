# Event Listings MVP

Server-rendered Go MVP for the `0004-event-listings` seed.

Features included:

- organizer login and event management workspace
- draft, published, canceled, and archived lifecycle actions
- stable public event URLs that survive edits
- public upcoming list, month calendar, and event detail pages
- keyword search plus category, location, and date-range filtering
- JSON endpoints aligned with the realization contract

Run locally:

```bash
go run .
```

Environment:

- `AS_ADDR` defaults to `127.0.0.1:8096`
- `AS_ADMIN_PASSWORD` defaults to `admin`

Organizer login:

- visit `/admin/login`
- sign in with the configured password
