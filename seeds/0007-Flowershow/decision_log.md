# Decision Log

## 1. API-First Ingestion
All data entry is done via API and external agents.

## 2. Show Admin UI
Rich control panel for show-night operations: manage judges, classes, entries, photos, winners. Overrides the earlier "no admin UI" decision — real show operations require it.

## 3. Append-Only Model
No deletion; only suppression.

## 4. Flexible Taxonomy
Graph-like tagging system instead of rigid schema.

## 5. Separate Domains
Horticulture and Design modeled separately.

## 6. Organization Hierarchy
Support multi-level org structure (club → district → region → province → country → global).

## 7. Integer Scoring
All monetary values handled in cents (no floats).

## 8. Identity Privacy
Public = initials, private = full identity mapping.

## 9. Provenance Is First-Class
Every structured record can trace back to a source document, page range, and extraction confidence. Standards, editions, and citations are not metadata — they are core entities.

## 10. Schedules Are Hierarchical
Real fair books use Division → Section → Class structure. The old "category" concept is replaced by `show_class` within this hierarchy.

## 11. Rule Inheritance With Overrides
Local schedules inherit from a governing standard edition. Class-level rules can override, narrow, or extend standard rules with provenance.

## 12. Rubric-Capable Scoring
Scoring supports criterion-level rubrics (not just placement). Placements and awards are computed from scorecards when present.

## 13. Media via S3
Photos and videos stored in S3. Client sends optimized (but still large) files. Server transcodes if file exceeds size threshold.

## 14. Auth via Cognito
AWS Cognito for authentication only (identity, not roles). Roles (admin, judge, entrant, public) are managed in-app. Setup is a late-stage task.

## 16. SSE for Real-Time Concurrent Access
Show Admin uses Server-Sent Events for live updates — multiple operators see changes without reload. No WebSocket needed.

## 17. HTMX Frontend
UI built with HTMX for partial page updates. SSE push fits naturally with HTMX's SSE extension.

## 15. Kernel-Managed Postgres
Database connection via `AS_RUNTIME_DATABASE_URL` injected by the kernel. Append-only claim ledger + materialized views.

## 18. Schedule Hierarchy Commands in the API
`schedules.upsert`, `divisions.create`, and `sections.create` are promoted to `/v1/commands/` endpoints. Previously these were only reachable through admin HTML form handlers (`POST /admin/shows/{showID}/schedule`, etc.), which meant agents authoring a full show schedule had to mix JSON commands with form-encoded admin POSTs. The store layer already supported these operations — only the command routing and contract entries were missing.
