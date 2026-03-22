# a-firstbloom

First-bloom realization of the 0007-Flowershow seed.

Standards-backed flower show management with schedule hierarchy, rubric scoring,
taxonomy, provenance tracking, and real-time collaborative show admin.

## Current Architecture

- Go server-rendered app with HTMX and SSE for operational surfaces
- Kernel runtime registry boundary for accepted Flowershow domain history
- Postgres-backed Flowershow projections rebuilt from replayed accepted history
- Kernel runtime authority materialization for effective access
- S3-backed entry media storage
- Cognito-backed identity with app-native scoped authority

## Current Workspace Model

Show admin currently has six operator tabs:

- `Setup`
- `Entries`
- `Corrections`
- `Scoring`
- `Board`
- `Governance`

Operational intent:

- `Entries` is the default show-night intake surface for existing shows
- `Corrections` is the lightweight fix-up surface with search and revealed correction controls
- connected operators receive SSE refreshes for intake, corrections, board, scoring, and governance panels

## Import / Agent Surface

The realization is intentionally agent-equal with the browser UI. The contract in
[`interaction_contract.yaml`](/Users/splash/as-flower-agent/autosoftware/seeds/0007-Flowershow/realizations/a-firstbloom/interaction_contract.yaml)
is the canonical source for available commands and projections.

Current import-relevant commands include:

- `organization.create`
- `shows.create`
- `shows.reset_schedule`
- `schedules.upsert`
- `divisions.create`
- `sections.create`
- `classes.create`
- `entries.create`
- `entries/{entryID}/media.upload`
- `persons.create`
- `persons.update`
- `judges.assign`
- `citations.create`

Command body rule:

- callers may send flat JSON command fields
- callers may send `{ "input": { ... }, "runtime_context": { ... } }`
- callers may also send flat command fields with sibling `runtime_context`

`runtime_context` is runtime-only guidance and is not canonical domain truth by itself.

## Media Upload Notes

- photos are normalized client-side up to 4096px max edge
- accepted image types: JPEG, PNG, WebP
- accepted video types: MP4, WebM, MOV
- HEIC/HEIF is rejected explicitly in the client flow
- admin intake keeps the modal open while media uploads and entry edits continue
- upload queue tiles show preparation, upload, save, and error states inline

## Local Development

```bash
cd artifacts/flowershow-app
go run .
# Server starts at http://127.0.0.1:8097
# Local admin login uses the test-session helpers in Playwright or the local auth flow
```

Without `AS_RUNTIME_DATABASE_URL`, the app runs with an in-memory store seeded
with demo data.

With `AS_RUNTIME_DATABASE_URL`, the realization runs in durable mode and should
be treated as Postgres-backed plus kernel-runtime-backed rather than ephemeral demo state.

Useful environment/config inputs in deployed mode include:

- `AS_RUNTIME_DATABASE_URL`
- Cognito settings from [`artifacts/runtime.yaml`](/Users/splash/as-flower-agent/autosoftware/seeds/0007-Flowershow/realizations/a-firstbloom/artifacts/runtime.yaml)
- Flowershow S3 credentials for media upload
- the normal production/runtime secret injection paths used by `webd`

## Testing

```bash
cd artifacts/flowershow-app
go test ./...
```

```bash
cd /Users/splash/as-flower-agent/autosoftware/tests
npm run test:flowershow
```

The current local baseline is:

- deterministic Go coverage for app/store/materialization behavior
- Playwright coverage for public, admin-local, API, and widget surfaces
- remote OTP/admin harnesses for deployed verification when credentials are available

Validation planning and acceptance checklists live in
[`validation/`](./validation/README.md).

For the deployed Cognito/OTP admin flow, run the remote projects headed after
setting `FLOWERSHOW_REMOTE_E2E=1`, `PLAYWRIGHT_SKIP_WEBSERVER=1`, and
`FLOWERSHOW_BASE_URL` to the deployed flower-show URL:

```bash
cd /Users/splash/as-flower-agent/autosoftware/tests
npm run test:flowershow:remote
```
