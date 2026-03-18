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
