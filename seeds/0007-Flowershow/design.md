# Design

## Terminology: two meanings of "capability"

This codebase uses the word "capability" for two unrelated concepts. Both are spelled `capabilities` in YAML and Go, so context determines meaning.

| Sense | Where | Vocabulary | Validated by |
|---|---|---|---|
| Kernel primitive | `interaction_contract.yaml` (top-level `capabilities:` and per-command/projection/object lists) | Closed allowlist of ~23 store kinds: `state_transitions`, `uploads`, `sessions`, `messages`, ... | `kernel/internal/realizations/contracts.go` `allowedCapabilities` |
| RBAC permission | `artifacts/flowershow-app/authority.go` `authorityBundleDef.Capabilities`; kernel column `runtime_authority_bundles.capabilities` | Open-ended seed-defined permission strings: `entries.manage`, `shows.workspace.read`, `awards.manage`, ... | Seed authorization layer; kernel does not enforce membership |

When reading or writing contract YAML, "capability" always means the kernel-primitive sense. When reading or writing authority/role/bundle code, "capability" always means the RBAC sense.

## Core Structure

### 1. Organization Hierarchy
Organizations form a hierarchy:

Society → District → Region → Province → Country → Global

Each Show is hosted by an Organization.

This hierarchy is structural at this stage.
It supports categorization, reporting, browsing, and lookup context.

Parentage does not itself confer control.

Examples:

- a club may belong to a district
- a district may belong to a region
- that does not mean the district can edit the club
- that does not mean the region can edit the district or club

Any cross-organization control must come from explicit accepted grants, not
from ancestry alone.

---

### 2. Show

A Show represents a single competition event:

- organization_id
- name
- location
- datetime
- season

---

### 3. Show Schedule & Class Hierarchy

Real fair books organize entries as Division → Section → Class.

#### show_schedule
- id
- show_id
- source_document_id
- effective_standard_edition_id (nullable)
- notes

#### division
- id
- show_schedule_id
- code (nullable)
- title
- domain (horticulture, design, special, other)
- sort_order

#### section
- id
- division_id
- code (nullable)
- title
- sort_order

#### show_class
- id
- section_id
- class_number
- title
- domain
- description
- specimen_count (nullable)
- unit (nullable)
- measurement_rule (nullable)
- naming_requirement (nullable)
- container_rule (nullable)
- eligibility_rule (nullable)
- schedule_notes (nullable)
- taxon_refs[]

Replaces the earlier generic `category_id` on entries.

#### class_split
- id
- show_class_id
- code (auto-assigned letter: a, b, c, ...)
- label (nullable, optional human label e.g. "Yellow", "Double")
- sort_order

A class may be divided into splits at any time during intake or judging. Splits are judged independently: every placement (1/2/3, Special, HM) is scoped to a single split, and splits never overlap within a class. An entry's `split_id` records membership; `null` means the entry is not in a split. When the last entry leaves a split, the split is auto-purged.

---

### 4. Standards & Editions

Governing rulebooks that local schedules depend on.

#### standard_document
- id
- name
- issuing_org_id
- domain_scope
- description

#### standard_edition
- id
- standard_document_id
- edition_label
- publication_year
- revision_date
- status (current, superseded, draft)
- source_url
- source_kind (official_pdf, print_only, excerpt_pdf, catalog_record)

A show references one or more standard editions via its schedule.

---

### 5. Source Documents & Citations

Every structured record traces back to a source.

#### source_document
- id
- organization_id
- show_id (nullable)
- title
- document_type (rulebook, schedule, fair_book, newsletter, results_sheet, catalog_record)
- publication_date
- source_url
- local_path (nullable)
- checksum (nullable)

#### source_citation
- id
- source_document_id
- target_type
- target_id
- page_from
- page_to
- quoted_text (nullable)
- extraction_confidence

For `citations.create`, callers send those fields directly or inside an `{ "input": ... }`
envelope with optional `runtime_context`. `page_from` and `page_to` may be either JSON
strings or numbers, but are normalized to stored string page references. Current valid
`target_type` values are the same object kinds exposed by the realization, including
`show_class`, `standard_rule`, `class_rule_override`, `show_schedule`, `division`, and
`section`.

---

### 6. Standard Rules & Local Overrides

Local schedules inherit from standards and can adjust.

#### standard_rule
- id
- standard_edition_id
- domain
- rule_type (definition, presentation, measurement, eligibility, scale_of_points, naming)
- subject_label
- body
- page_ref (nullable)

#### class_rule_override
- id
- show_class_id
- base_standard_rule_id (nullable)
- override_type (replace, narrow, extend, local_only)
- body
- rationale (nullable)

---

### 7. People

A Person:
- can be an entrant
- can be a judge
- may belong to zero or more organizations

Identity:
- private (full name)
- public (initials)

---

### 7A. Authority, Membership, and Delegation

Authority for this seed should be system-native rather than delegated
permanently to Cognito or any other auth provider.

Identity comes from auth.
Control comes from accepted Autosoftware authority history.

The seed should distinguish three related but different ideas:

- membership
- office
- operational authority

Examples:

- `organization_member`
- `organization_executive`
- `organization_admin`
- `show_editor`
- `show_judge`
- `show_steward`
- `show_scoring_operator`
- `show_awards_operator`

Authority should be scoped.
Typical scopes here are:

- organization
- show
- class

The seed should support explicit grant lifecycle and history:

- proposed grant
- accepted grant
- revoked grant
- expired grant
- superseded grant

Delegation should be explicit.
For example:

- a club admin may be allowed to grant club-scoped executive or member access
- a show editor may be allowed to grant narrower show-scoped roles
- a show judge should usually not be allowed to delegate judge power

Current effective access should be materialized from accepted grant history.
Revocation should be represented as later accepted history, not by mutating old
role rows in place and not by inventing status strings like `REVOKED
executive`.

Organization ancestry is not authority inheritance by default.
If any future cross-org authority is introduced, it must be explicitly granted
and explicitly evaluated, not inferred from parent-child organization links.

#### Implementation status (current realization)

The grant record format (bundle, scope, grantor, status) and effective-access materialization are implemented in `realizations/a-firstbloom`. Today only `roles.assign` is exposed, and it always writes status `accepted`. The remaining lifecycle transitions (`proposed`, `revoked`, `expired`, `superseded`) and per-role delegation policy enforcement (e.g. judges may not delegate judge power) are designed but not yet exercised by any command. Operationally, the only grant in production is the bootstrap admin.

---

### 8. Entries

Entries are submissions into a class within a show.

Each Entry:
- belongs to a Show
- belongs to a show_class
- belongs to a Person (optional — see anonymous entries below)
- optionally belongs to a class_split via `split_id`
- has placement and points
- has media (multiple photos/videos)
- has taxonomy references

#### Anonymous entries

`person_id` may be empty. Anonymous entries are created during fast intake (typically the photo-first sequential intake flow) before the entrant has been identified, and they display as "Anonymous · N" in admin lists until matched to a person record. Naming an anonymous entry is a normal post-intake correction.

#### Soft-archive

Entries carry a nullable `archived_at` timestamp. Default `entriesByShow`, `entriesByClass`, and `entriesByPerson` projections exclude archived rows; `*IncludeArchived` variants exist for restore screens. Commands: `entries.archive` (set `archived_at` to now) and `entries.restore` (clear it). Archive is reversible and is distinct from `suppressed` (see §14).

#### Cover photo

Each entry's cover photo is the media row whose `is_cover` is true. If no media is explicitly marked, the read path falls back to the first chronological photo as an implicit cover. See §13.

---

### 9. Judging & Rubric Scoring

Beyond placement — criterion-level scoring.

When a class has splits, placements are scoped per `(show_class_id, split_id)`: each split runs its own 1/2/3/Special/HM independently, and the placement computation from scorecards groups entries by split before ranking. Entries with `split_id = null` in a class that also has splits are ranked among themselves as the unsplit residual.

#### judging_rubric
- id
- standard_edition_id (nullable)
- show_id (nullable)
- domain
- title

#### judging_criterion
- id
- judging_rubric_id
- name
- max_points
- sort_order

#### entry_scorecard
- id
- entry_id
- judge_id
- rubric_id
- total_score
- notes (nullable)

#### entry_criterion_score
- id
- entry_scorecard_id
- criterion_id
- score
- comment (nullable)

Placements and awards are computed from scorecards when present.

#### Special status

Entries can carry a special status (e.g. award winner, honorable mention) independently of their numeric placement, optionally linked to a special award. The canonical command for setting or clearing this is `entries.set_special_status`. The admin "results" form is a UI convenience that fires both `entries.set_placement` and `entries.set_special_status` against the same entry; the contract surface remains the two separate semantic commands.

---

### 10. Domains

#### Horticulture
Describes physical specimens:
- common name
- scientific name (genus, species, subspecies, cultivar)
- characteristics
- presentation rules

#### Design

---

## 11. Interface And Agent Access

The operational surface for this seed should be equally usable by:

- the HTMX admin UI
- session-authenticated human operators
- remote service-token agents

Normal flower-show authoring should happen through semantic commands and
projections, not through hidden admin-only store mutations and not through raw
database access.

Required interface expectations:

- all normal admin mutations have matching semantic commands
- important admin workspaces have matching projections, including private
  workspace views where needed
- stable by-id projections exist for primary objects even when a public route
  also exposes a slug
- authenticated agents receive structured validation and permission errors
  useful enough to recover without guessing
- command authorization should resolve through system-native authority over the
  relevant organization, show, or narrower scope, not through hidden
  seed-local-only checks that alternate clients cannot inspect
- authority changes are ledger-visible history and effective-access
  projections via the kernel-native authority model (see §7A "Implementation status (current realization)" for the subset of the lifecycle that is currently exercised)

Runtime-only assistant instructions may accompany authoring requests, for
example guidance about how to interpret a cited schedule or which standard
should be treated as authoritative. That runtime context must not be stored as
canonical show truth unless it is converted into explicit structured records or
citations.

Current command-body rule for this realization:

- callers may send a flat JSON object containing the command fields
- callers may send `{ "input": { ... }, "runtime_context": { ... } }`
- callers may also send flat command fields with `runtime_context` as a sibling
  top-level property

In all three cases, `runtime_context` is runtime-only guidance and must not be
persisted into canonical show data.
Describes compositions:
- type (crescent design, etc)
- title
- criteria
- specifics (taxonomic references)

---

### 11. Taxonomy (Core System)

A flexible graph-like system:

- Taxons represent concepts (rose, crescent design, novice)
- Entries, classes, and awards reference taxons
- Taxons can relate to each other

Taxon types: botanical, scientific_name, cultivar, characteristic, design_type, presentation_rule, award_dimension, free_tag

Enables:
- cross-linking
- filtering
- discovery

---

### 12. Awards

Awards are defined per organization and season.

Each award:
- defines filters (taxonomy-based)
- defines scoring rules (sum, max, custom)

Examples:
- High Points
- Best Rose
- High Points Novice
- Memorial awards

---

### 13. Operator Reports & Exports

Flower show operators often run the event from print-style tally sheets rather
than normalized tables. The system therefore supports both structured data
exports and operator workbook exports.

#### Normalized exports
- entries with show, class, person, placement, points, notes, and taxon refs
- schedule hierarchy from division → section → class
- leaderboard by organization and season
- scorecards with per-criterion scores

#### Operator tally workbook
- class numbers across columns
- exhibitors down rows
- placement markers in the class grid
- repeated `#1s #2s #3s` tally columns
- point summary using local show-night scoring conventions:
  - horticulture / flowers / vegetables: 1st = 4, 2nd = 3, 3rd = 2
  - design / special exhibits: 1st = 12, 2nd = 9, 3rd = 6
- participation and results tabs for season rollups and show-night reporting

Reports are generated on request from current show, entry, class, and person
state. Admin UI downloads use session auth; agent/API downloads use service
token auth.

---

### 14. Media

Media is a first-class declared `domain_objects` kind. Each row carries `id`, `entity_kind`, the relevant entity foreign key, MIME type, dimensions, duration (for video), original filename, and `is_cover`.

#### Storage

Originals go to S3 in production and to local disk in dev. Both write paths apply EXIF orientation normalization at upload time: the stored file is rotated to orientation 1 and the EXIF tag is cleared, so downstream code never has to rotate. HEIC/HEIF is still rejected in the client flow; the client also normalizes large photos up to 4096px max edge before upload. Ingress/body limits must be sized for normalized photos and short videos.

#### Thumbnails

A 512px-max-edge JPEG thumbnail is generated server-side at upload. The public read path is `GET /media/{id}` for the original and `GET /media/{id}?thumb=1` for the thumbnail; if no thumbnail exists for a row, `?thumb=1` falls back to the original. Multi-photo galleries and admin grids read `?thumb=1`.

#### Cover photo

`is_cover` is a boolean per media row. The canonical command is `media.set_cover` (it un-sets any other cover for the same entity in the same call). When nothing is explicitly marked, the read path treats the first chronological photo as an implicit cover, so newly created entries always have something to render.

#### Entity discriminator

`entity_kind` is either `entry` (with `entry_id` set, the default) or `class` (with `class_id` set). Class-attached media is what powers class overview photos and the show landing page hero. The command to attach to a class is `media.attach_to_class`.

---

### 15. Privacy & Suppression

- System is append-only
- Content can be suppressed (hidden)
- Identity mapping is private

`suppressed` and `archived_at` are different concepts. A suppressed entry is hidden from public views (board, public results, exhibitor pages) but remains active for show admin: it still has a place in its class, still appears in the workspace grid, can still receive scoring or corrections. An archived entry (`archived_at` non-null on `entries`) is soft-deleted: it is excluded from default `entriesByShow`/`entriesByClass`/`entriesByPerson` projections and from any UI that does not opt in via the `*IncludeArchived` variants. Restoring it via `entries.restore` returns it to the show.

---

### 16. Show Admin

Rich control panel for show-night operations:
- Set judge info per class
- Add and manage classes
- Assign people to entries
- Upload photos/videos
- Set winners per class
- Download operator tally workbooks and normalized data exports
- Multiple operators work simultaneously via SSE push (no reload)

Current working workspace model:

- `Setup` handles show profile, judges, credits, and schedule governance
- `Entries` is the intake grid rendered directly from class order
- `Corrections` is the post-intake correction surface with search, in-place results, and revealed correction controls
- `Scoring` handles rubrics and scorecards
- `Board` shows the live show board
- `Governance` exposes standards, rules, citations, and sources

#### Sequential photo intake

`/admin/shows/{showID}/intake/photos` is a phone-first capture surface. A sticky class bar at the top has prev/next arrows and wraps around the schedule. The body is a large tap-to-capture area; every shutter click creates an anonymous Entry (`person_id = ""`) in the active class, attaches the photo, and renders a tile. Uploads are optimistic with an in-memory retry queue: failed tiles outline red and expose per-tile retry plus a global "Retry all failed". No new commands are needed — the page composes existing `entries.create`, `media.attach`, `media.delete`, and `media.set_cover`.

#### Multi-photo gallery

The entry detail page renders all attached media as `?thumb=1` thumbnails in a gallery. Each thumbnail supports click-to-enlarge, delete, and a star-as-cover affordance bound to `media.set_cover`.

#### Fast / Update workspace mode and persistent collapsibles

A segmented Fast / Update toggle sits at the top of the workspace and is persisted in `localStorage` under `as.flowershow.workspace.mode`. Update mode reveals controls (split actions, move-to dropdowns, correction widgets) that Fast mode hides. Per-entry sections (Name match, Ranking, Comment, Photo detail) are native `<details>` collapsibles whose open/closed state is persisted *per section type, not per entry*, under `as.flowershow.section.{name}` — opening Comment on one entry opens it for every entry at once.

#### Class splits UI

In Update mode, each class panel shows a Split button top-right. Activating it opens a multi-select flow with a floating "Split N entries" bar that creates a new auto-coded split (a, b, c, ...) and moves the selection into it. A per-entry "Move to →" dropdown lets operators reassign an entry to a sibling split. Each split renders its own judging surface with its own placement slots. New commands: `class_splits.create`, `class_splits.delete`, `entries.move_to_split`. New fragment endpoint: `GET /admin/classes/{classID}/splits/fragment`.

---

## Show Landing Page

`/shows/{slug}` is the public-facing show page and is phone-first by design — event-day usage is heavily mobile, so layout, tap targets, and image sizing are tuned for small screens before any desktop polish.

The page renders:

- a hero band with show name, date/location, a status pill (upcoming / live / closed), and primary actions
- a "Winners by class" table that lists every class with its 1/2/3, Special, and HM placements (split-aware: each split is its own row)
- four navigation tiles: entries, classes, clubs, exhibitors

Two sub-pages hang off it:

- `/shows/{slug}/entries` is a filterable per-show entry directory
- `/shows/{slug}/exhibitors` is a per-show exhibitor aggregate; anonymous entrants are grouped together rather than listed as individuals

Class- and entry-level cover photos (see §13) drive the imagery on the landing page and tiles.

---

### 17. Real-Time & Frontend

- HTMX for partial page updates and SSE integration
- Server-Sent Events push live changes to all connected operators
- No full-page reloads during show-night operations

---

### 18. Authentication

- Cognito handles identity only (login, signup, token validation)
- Roles (admin, judge, entrant, public) are managed in-app, not in Cognito
- App-level role assignment per organization/show
- Authority grants and effective access are materialized from kernel runtime authority history

#### 17A. Show helper invites and badge sessions

Show admins frequently need to bring on a one-day helper (a club member taking photos, a friend running a tablet at intake) without putting them through Cognito signup. Two domain objects support this:

- `show_helper_invite` — a share-link record with token prefix `fshi_`, a sha256 hash of the secret, an `expires_at` (default 7 days), and the show it scopes to. Created via `show_helper_invites.create` and revoked via `show_helper_invites.revoke`.
- `show_badge_session` — a runtime-only badge record with token prefix `fsbs_` and a sha256 hash of the secret, tied to a show, with an `expires_at` (default 24h) and the helper's claimed name + email. Ended via `show_badge_sessions.end`.

A helper redeems an invite at `/shows/{slug}/help-redeem` by submitting name + email. If the email matches an existing person record the badge is linked; otherwise the badge stands alone. On success the server sets cookie `as_show_badge` (HttpOnly, SameSite=Lax, path=`/shows/`) and the helper is now authorized within that show.

Badge sessions inherit the existing `show_intake_operator` authority bundle — that is, `entries.manage` and `media.manage` scoped to the badge's show. No new authority bundle was introduced.

Public routes:

- `GET /shows/{slug}/help` — landing page for an invite token
- `GET /shows/{slug}/help-redeem` and `POST /shows/{slug}/help-redeem` — redemption form
- `POST /shows/{slug}/help/end` — voluntary session end

Admin routes:

- `GET /admin/shows/{showID}/helpers` — list active invites and live badges
- `POST` to the same path to create an invite; revoke via the per-row revoke action

---

### 18. Durability And Materialization

The current realization is no longer allowed to treat in-memory state as
durable truth in Postgres-backed mode.

Current operating rule:

- accepted Flowershow domain facts append through the kernel runtime registry boundary
- Flowershow materializes `as_flowershow_m_*` projection tables from replayed registry history
- startup can rebuild projections when those tables drift or are wiped
- authority grants are also materialized from registry-backed runtime history

This means the authoritative write path is:

1. accept semantic command
2. append accepted history through the kernel boundary
3. materialize effective projection/runtime state
4. serve public/admin/API reads from those projections

Memory may still be used as a cache or working snapshot, but not as the sole
authoritative store of accepted domain facts.

---

### 18. Schedule Authoring Workflow (API)

The canonical command chain for building a show schedule:

1. **`shows.create`** `{ organization_id, name, location, date, season }` → returns show with `id`
2. **`schedules.upsert`** `{ show_id }` → returns schedule with `id` (this is the `show_schedule_id` for divisions)
3. **`divisions.create`** `{ show_schedule_id, title, domain, sort_order, code? }` → returns division with `id`
   - `domain` must be one of: `horticulture`, `design`, `special`, `other`
4. **`sections.create`** `{ division_id, title, sort_order, code? }` → returns section with `id`
5. **`classes.create`** `{ section_id, class_number, title, domain, description?, specimen_count?, unit?, schedule_notes?, taxon_refs?[] }` → returns class with `id`

Each step requires the `id` returned by the previous step. After authoring, read the result back via `GET /v1/projections/0007-Flowershow/shows/{id}`.

The `schedules.upsert` command uses upsert semantics: if a schedule already exists for the given `show_id`, it updates it; otherwise it creates one. Optional fields on schedule: `source_document_id`, `effective_standard_edition_id`, `notes`.

---

## Key Design Decisions

- API-first ingestion with show admin UI for live operations
- Standards and provenance as first-class structural layers
- Schedule hierarchy (division → section → class) over flat categories
- Rule inheritance with local overrides
- Rubric-capable scoring, not just placement
- Operator tally workbooks alongside normalized CSV/XLS exports
- Graph-like taxonomy over rigid schema
- Separate structured domains (horticulture vs design)
- Organization-scoped everything
- S3 for media, Cognito for identity, kernel-managed Postgres/runtime registry
- HTMX + SSE for real-time collaborative show admin
