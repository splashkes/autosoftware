# Design

## Core Structure

### 1. Organization Hierarchy
Organizations form a hierarchy:

Society → District → Region → Province → Country → Global

Each Show is hosted by an Organization.

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
- Client sends optimized-but-still-large files
- Server transcodes if exceeding size threshold
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
- S3 for media, Cognito for identity (roles in-app), kernel-managed Postgres
- HTMX + SSE for real-time collaborative show admin
