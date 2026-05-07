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

## 19. Authority Lifecycle: Designed Surface vs Current Realization
design.md §7A describes a five-state grant lifecycle (proposed, accepted, revoked, expired, superseded) and per-role delegation policy. The kernel-side bundle/grant tables, scope tracking, grantor capture, and effective-access materialization are all implemented in `realizations/a-firstbloom`.

For now, only `roles.assign` is exposed and it always writes status `accepted`. The other four lifecycle states are SQL-recognized but unreachable from any command. Per-role delegation policy enforcement (e.g. "judges may not delegate judge power") is also deferred. The only grant currently in production is the bootstrap admin (`grant_restore_simon_admin`).

The delegation surface remains usable in single-admin operation. When operational need for multi-grant delegation arises, the work is to add `roles.revoke` / `roles.propose` / `roles.supersede` commands, the corresponding handler logic, and a delegation-policy gate in the assign path. Until then, the docs in design.md §7A "Implementation status (current realization)" record the current operational state so the docs do not read as if delegation is operationally live.

## 20. Class Splits as a Separate Domain Object
A class can be partitioned at any time during judging into a/b/c sub-groups (auto-letter-coded, optionally labelled "Yellow", "Double"). Splits are judged independently — each split has its own 1st/2nd/3rd/Special/HM. Modeled as a separate `class_split` object kind rather than a field on `show_class` because splits are dynamic runtime structures created mid-event, not part of the static schedule; they can be re-created with the same letter after deletion (free letter pool per class); per-split placements need their own scoping that a flat field on the class would not give; and an empty split auto-purges when the last entry moves out, so split lifecycle is decoupled from class lifecycle.

`Entry.SplitID` (nullable) records membership. `computePlacementsFromScores` groups entries by split when splits exist for the class, otherwise behaves as before.

## 21. Soft-Archive Entries (Not Hard Delete)
`entries.archive` sets `Entry.ArchivedAt`. Archived entries are excluded from all default `entriesByShow`/`entriesByClass`/`entriesByPerson` reads; `*IncludeArchived` variants exist for restore screens. `entries.restore` clears the timestamp. Soft archive instead of hard delete because operator mistakes on event day are easy to recover from, the ledger and projection history remain intact for audit and replay, and "remove this entry" is a frequent fast-tempo action — making it reversible removes anxiety.

Hard delete remains available via direct admin action if a record genuinely needs erasing (e.g. PII compliance), but is not the primary user-facing flow.

## 22. Image Pipeline: Normalize-On-Write
EXIF orientation is honored ONCE, at upload time, in the `mediaStore.Store` path (both local + S3). The byte stream written to durable storage is already rotated to orientation=1, so every downstream consumer (browser, API client, agent) gets the same correctly-oriented bytes without needing any client-side rotation logic. Server-side thumbnails (max 512px JPEG, quality 80) are generated on the same write, stored alongside the original, and served via `?thumb=1` on the existing `/media/{id}` route. The route falls back to the full image when no thumbnail exists, so the change is forward-compatible with any pre-existing media.

`Media.IsCover` is a boolean flag with read-path implicit fallback: the chronologically-first media on an entry is treated as cover until any media is explicitly marked. This avoids backfilling existing entries.

`Media.EntityKind` discriminates `entry` (default) vs `class`, with nullable `EntryID` and `ClassID`. Class-attached media powers class overview photos and the show-page hero background.

## 23. Helper Share-Links: Show-Scoped Badge Sessions, Not Cognito
Event-day helpers (volunteers, photographers, late-arriving stewards) need to add photos and create entries without going through Cognito email-OTP signup. The new `show_helper_invite` + `show_badge_session` pair gives them a frictionless path: an admin generates a one-time share link (token format `fshi_`, sha256 hash stored, 7-day default expiry); the helper redeems it on `/shows/{slug}/help-redeem`, supplies name + email (optionally matching an existing person record); they receive a runtime-only `as_show_badge` cookie (token format `fsbs_`, 24h default expiry, HttpOnly, SameSite=Lax, path-scoped to `/shows/`). The badge inherits the existing `show_intake_operator` authority bundle (`entries.manage` + `media.manage` + `persons.manage` + `show_credits.manage`) — no new authority bundle was created.

Badge-session creation does NOT emit a claim (high-volume, not auditable). Only `show_badge_session.ended` enters the ledger so the lifecycle remains traceable.

## 24. UI Mode Toggle (Fast / Update) Replaces a Helper Role List
Beta-test feedback suggested separate roles for "fast entry adder", "name matcher", "ranking setter", etc. Implementing those as authority bundles would require permission tuning, role management UI, and operator switching ceremony. Instead, the admin show workspace exposes a single client-side `Fast add` / `Update entries` segmented toggle persisted in `localStorage` under `as.flowershow.workspace.mode`. The same operator can self-switch contexts mid-event with one tap, and there is no permission proliferation.

In Update mode, four collapsible sub-sections appear under each entry: Name match, Ranking, Comment, Photo detail. The open/closed state of each section is persisted PER SECTION TYPE (not per entry) under `as.flowershow.section.{name}`. Operators tend to batch by task ("today I am doing rankings") rather than by entry, so flipping Comment open should open it for every entry at once.
