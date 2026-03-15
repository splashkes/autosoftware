# Validation

Validation evidence for this realization should cover:

- organizer event creation and editing
- publish and unpublish behavior
- cancel and archive behavior
- public list and calendar rendering
- event detail routing
- keyword search plus category, date, and location filtering

Current local validation target:

- `go test ./...` inside `artifacts/event-listings-app`
- manual smoke run via webd at `127.0.0.1:8090` (app listens on unix socket)
- HTTP checks for:
  - `GET /healthz`
  - `GET /`
  - `GET /calendar`
  - `GET /v1/projections/0004-event-listings/events`
  - organizer login and draft state transitions

## Evidence captured on March 15, 2026

- `go test ./...` passed in `artifacts/event-listings-app`
- `GET /healthz` returned `{"seed":"0004-event-listings","status":"ok"}`
- `GET /` rendered public listings including `Harbour Lights Night Market` and `Design Systems Field Day`
- `GET /calendar?month=2026-03` rendered `Design Systems Field Day` across multiple days and showed `Riverfront Cleanup Rally` as canceled
- `GET /v1/projections/0004-event-listings/events?category=Workshop` returned the published workshop event JSON payload
- organizer login at `/admin/login` accepted the configured password and reached the organizer workspace
- organizer workflow smoke test:
  - created draft `Codex Test Event`
  - verified stable slug `codex-test-event-6`
  - published the draft and loaded its public detail page
  - unpublished it and confirmed it disappeared from the default public directory
  - re-published, archived it, and confirmed the detail page stayed reachable with an archive notice
- canceled `Harbour Lights Night Market` from the organizer workspace and confirmed the public detail page displayed the canceled notice
- Playwright browser validation:
  - signed in through `/admin/login`
  - created draft `Playwright Browser Check`
  - verified generated stable slug `playwright-browser-check-6`
  - published the event from the organizer table
  - opened the public detail page and confirmed the fixed URL note and event facts
  - opened the month calendar and confirmed the event rendered on March 30, 2026

## Expanded validation on March 15, 2026

- `go test ./...` still passes after:
  - file-backed event persistence
  - richer organizer authoring fields
  - social/share data in projections
  - public discovery redesign
- browser and HTTP checks confirmed:
  - homepage modules render featured pick, quick picks, organizer identity, and save/share signals
  - detail pages render organizer label, venue note, crowd fit, tags, related events, and copy-link control
  - organizer create form exposes neighborhood, organizer, cover image, tags, crowd label, and editorial blurb fields
- persistence validation:
  - created draft `Signals and Scenes Night` with organizer and cover-image metadata
  - restarted the app against the same `AS_DATA_FILE`
  - confirmed the draft still appeared in `/admin`
  - published it and confirmed the public detail page rendered persisted organizer/share metadata
