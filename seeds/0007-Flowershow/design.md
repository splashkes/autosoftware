# Design

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

---

### 8. Entries

Entries are submissions into a class within a show.

Each Entry:
- belongs to a Show
- belongs to a show_class
- belongs to a Person
- has placement and points
- has media (multiple photos/videos)
- has taxonomy references

---

### 9. Judging & Rubric Scoring

Beyond placement — criterion-level scoring.

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
- authority changes should become ledger-visible history and effective-access
  projections once the seed adopts the kernel-native authority model

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

### 13. Media

- Multiple photos and videos per entry
- Client normalizes images before upload
- Current realization normalizes photos up to 4096px max edge before upload
- HEIC/HEIF is rejected explicitly in the client flow
- Ingress/body limits must be sized for normalized photos and short videos
- Stored in S3
- Metadata: type, dimensions, duration, original filename

---

### 14. Privacy & Suppression

- System is append-only
- Content can be suppressed (hidden)
- Identity mapping is private

---

### 15. Show Admin

Rich control panel for show-night operations:
- Set judge info per class
- Add and manage classes
- Assign people to entries
- Upload photos/videos
- Set winners per class
- Multiple operators work simultaneously via SSE push (no reload)

Current working workspace model:

- `Setup` handles show profile, judges, credits, and schedule governance
- `Entries` is the intake grid rendered directly from class order
- `Corrections` is the post-intake correction surface with search, in-place results, and revealed correction controls
- `Scoring` handles rubrics and scorecards
- `Board` shows the live show board
- `Governance` exposes standards, rules, citations, and sources

---

### 16. Real-Time & Frontend

- HTMX for partial page updates and SSE integration
- Server-Sent Events push live changes to all connected operators
- No full-page reloads during show-night operations

---

### 17. Authentication

- Cognito handles identity only (login, signup, token validation)
- Roles (admin, judge, entrant, public) are managed in-app, not in Cognito
- App-level role assignment per organization/show
- Authority grants and effective access are materialized from kernel runtime authority history

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
- Graph-like taxonomy over rigid schema
- Separate structured domains (horticulture vs design)
- Organization-scoped everything
- S3 for media, Cognito for identity, kernel-managed Postgres/runtime registry
- HTMX + SSE for real-time collaborative show admin
