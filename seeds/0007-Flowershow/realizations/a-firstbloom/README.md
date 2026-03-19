# a-firstbloom

First-bloom realization of the 0007-Flowershow seed.

Standards-backed flower show management with schedule hierarchy, rubric scoring,
taxonomy, provenance tracking, and real-time collaborative show admin.

## Local Development

```bash
cd artifacts/flowershow-app
go run .
# Server starts at http://127.0.0.1:8097
# Admin login: password "admin" (or AS_ADMIN_PASSWORD env var)
```

Without `AS_RUNTIME_DATABASE_URL`, the app runs with an in-memory store seeded
with demo data.

## Testing

```bash
cd artifacts/flowershow-app
go test ./...
```

```bash
cd /Users/splash/as-flower-agent/autosoftware/tests
npm run test:flowershow
```

Validation planning and acceptance checklists live in
[`validation/`](./validation/README.md).

For the deployed Cognito/OTP admin flow, run the remote projects headed after
setting `FLOWERSHOW_REMOTE_E2E=1`, `PLAYWRIGHT_SKIP_WEBSERVER=1`, and
`FLOWERSHOW_BASE_URL` to the deployed flower-show URL:

```bash
cd /Users/splash/as-flower-agent/autosoftware/tests
npm run test:flowershow:remote
```
