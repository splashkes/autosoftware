Now I have thoroughly explored all the reference implementations, patterns, infrastructure, CI, and seed documents. Let me construct the comprehensive implementation plan.

---

# Flowershow `a-firstbloom` Implementation Plan

## Table of Contents

1. [Overview and Conventions](#overview-and-conventions)
2. [Branch Strategy](#branch-strategy)
3. [Phase 0: Realization Metadata and Scaffolding](#phase-0)
4. [Phase 1: Core Schema and Store Layer](#phase-1)
5. [Phase 2: Minimal Server and Public Pages](#phase-2)
6. [Phase 3: Show Admin MVP (Operational Heart)](#phase-3)
7. [Phase 4: Schedule Hierarchy and Class Browse](#phase-4)
8. [Phase 5: SSE Real-Time for Show Admin](#phase-5)
9. [Phase 6: Taxonomy, Awards, and Leaderboard](#phase-6)
10. [Phase 7: Standards, Rules, and Provenance](#phase-7)
11. [Phase 8: Rubric Scoring and Judging](#phase-8)
12. [Phase 9: JSON API Surface (Commands and Projections)](#phase-9)
13. [Phase 10: S3 Media Uploads](#phase-10)
14. [Phase 11: Cognito Authentication and Roles](#phase-11)
15. [Phase 12: Playwright Integration Tests](#phase-12)
16. [Phase 13: Polish, Seed Data, and Final PR](#phase-13)

---

## Overview and Conventions

### Repository Location

All files live under:
```
seeds/0007-Flowershow/realizations/a-firstbloom/
```

### Patterns Carried Forward from Reference Implementations

**From `/Users/splash/AS/seeds/0004-event-listings/realizations/a-ledger-web/`:**
- Three-table Postgres schema: `as_flowershow_objects`, `as_flowershow_claims`, `as_flowershow_materialized_*` (one materialized table per entity type that needs fast reads)
- `eventStore` interface pattern with Postgres and in-memory implementations
- `newLedgerID(prefix)` for generating IDs with `crypto/rand`
- Transactional `allocateSlug()` for unique human-readable URLs
- `pgxpool.New()` connection pooling with `AS_RUNTIME_DATABASE_URL`
- Migration via `CREATE TABLE IF NOT EXISTS` in `migrate()` method
- Admin auth via `adminCookieName` cookie + `AS_ADMIN_PASSWORD` env var
- Service token via `X-AS-Service-Token` header
- Commands at `POST /v1/commands/0007-Flowershow/...`
- Projections at `GET /v1/projections/0007-Flowershow/...`
- `httptest` based unit tests with `newTestApp()` / `newTestMux()`

**From `/Users/splash/AS/seeds/0003-customer-service-app/realizations/a-web-mvp/`:**
- HTMX loaded from CDN: `https://unpkg.com/htmx.org@2.0.4`
- SSE extension: `https://unpkg.com/htmx-ext-sse@2.2.2/sse.js`
- SSE handler pattern: `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`, `http.Flusher`
- Channel-based pub/sub for live streaming with `r.Context().Done()` for cleanup
- HTMX attributes: `hx-ext="sse"`, `sse-connect="/path"`, `sse-swap="message"`, `hx-swap="beforeend"`

**From kernel infrastructure:**
- `runtime.yaml`: `kind: runtime`, `version: 1`, `runtime: go`, `run.command: prebuilt`
- `AS_ADDR` injected as `127.0.0.1:<port>` by execd
- Go module name: `as/realization/flowershow`
- Go version: `1.26.1`
- Dependency: `github.com/jackc/pgx/v5`

**From CI pipeline (`/Users/splash/AS/scripts/ci-seed-go-tests.sh`):**
- Add the flowershow app directory to the `module_dirs` array for automated test runs
- Tests must pass with `go test ./...`

**From Playwright tests (`/Users/splash/AS/tests/`):**
- New spec file: `tests/flowershow.spec.ts`
- Update `playwright.config.ts` to include the flowershow webServer configuration
- Test pattern: create data through the UI, verify it renders, verify admin operations

### Naming Conventions

All Postgres tables prefixed with `as_flowershow_` to avoid collisions in the shared kernel database. This matches the `as_event_listings_` prefix used by 0004.

All ledger IDs use the format `{prefix}_{random_hex}` via `newLedgerID()`.

---

## Branch Strategy

1. Create feature branch: `git checkout -b feat/0007-flowershow-a-firstbloom`
2. Work through phases, committing after each phase
3. Each phase commit message: `0007-Flowershow: Phase N — <description>`
4. When complete, open a PR to `main` with a squash merge
5. Merging to `main` triggers auto-deploy via GitHub Actions

---

## Phase 0: Realization Metadata and Scaffolding

**Goal:** Establish the realization directory structure and metadata files so the kernel knows this realization exists.

### Files to Create

**`seeds/0007-Flowershow/realizations/a-firstbloom/realization.yaml`**

Follow the pattern from `/Users/splash/AS/seeds/0004-event-listings/realizations/a-ledger-web/realization.yaml`:

```yaml
realization_id: a-firstbloom
seed_id: 0007-Flowershow
approach_id: api-first-taxonomy
summary: Federated flower show registry with show admin, schedule hierarchy, rubric scoring, and real-time collaboration.
status: draft
subdomain: flowershow
path_prefix: /flowershow/
artifacts:
  - artifacts/runtime.yaml
  - artifacts/flowershow-app/go.mod
  - artifacts/flowershow-app/go.sum
  - artifacts/flowershow-app/main.go
  - artifacts/flowershow-app/models.go
  - artifacts/flowershow-app/store.go
  - artifacts/flowershow-app/store_memory_test.go
  - artifacts/flowershow-app/handlers_public.go
  - artifacts/flowershow-app/handlers_admin.go
  - artifacts/flowershow-app/handlers_api.go
  - artifacts/flowershow-app/sse.go
  - artifacts/flowershow-app/main_test.go
  - artifacts/flowershow-app/README.md
  - artifacts/flowershow-app/assets/app.css
  - artifacts/flowershow-app/assets/app.js
  - artifacts/flowershow-app/templates/base.html
```

**`seeds/0007-Flowershow/realizations/a-firstbloom/interaction_contract.yaml`**

Model after the event-listings contract but with Flowershow domain objects. Include:
- `auth_modes`: anonymous, session, service_token
- `domain_objects`: organization, show, person, entry, show_class, division, section, taxon, award_definition, media, standard_document, standard_edition, source_document, source_citation, judging_rubric, entry_scorecard, user_role
- `commands`: shows.create, shows.update, entries.create, entries.update, entries.set_placement, classes.create, schedule.create, persons.create, awards.compute, media.attach, rubrics.create, scorecards.submit
- `projections`: shows.directory, shows.detail, shows.admin, classes.browse, entries.detail, leaderboard, taxonomy.browse

**`seeds/0007-Flowershow/realizations/a-firstbloom/artifacts/runtime.yaml`**

```yaml
kind: runtime
version: 1
runtime: go
entrypoint: artifacts/flowershow-app/main.go
working_directory: artifacts/flowershow-app
run:
  command: prebuilt
  args: []
environment:
  AS_ADMIN_PASSWORD: admin
notes:
  - AS_ADDR is injected by the kernel execution backend at launch time.
  - AS_RUNTIME_DATABASE_URL is injected by the kernel execution backend at launch time.
  - Show admin uses AS_ADMIN_PASSWORD and defaults to admin for local development.
```

**`seeds/0007-Flowershow/realizations/a-firstbloom/README.md`**

Brief description of the realization, linking to seed docs.

**`seeds/0007-Flowershow/realizations/a-firstbloom/validation/README.md`**

Placeholder for acceptance evidence. Will be populated as phases complete.

**`seeds/0007-Flowershow/realizations/a-firstbloom/artifacts/flowershow-app/go.mod`**

```
module as/realization/flowershow

go 1.26.1

require github.com/jackc/pgx/v5 v5.8.0
```

Then run `go mod tidy` to generate `go.sum`.

**`seeds/0007-Flowershow/realizations/a-firstbloom/artifacts/flowershow-app/README.md`**

Brief app-level README.

### What's Testable

- `go mod tidy` succeeds
- Directory structure matches the canonical layout from EVOLUTION_QUICK.md

---

## Phase 1: Core Schema and Store Layer

**Goal:** Create the Postgres schema, the Go data models, the store interface, and the in-memory test implementation. Everything needed to read/write data, but no HTTP handlers yet.

### Files to Create

**`artifacts/flowershow-app/models.go`**

Define all Go structs for the domain. The key principle: models are plain Go structs, not ORM models. JSON tags for API serialization.

Start with the **core entity set** needed for the Show Admin MVP (Phases 1-3):

```go
// Core entities for Phase 1
type Organization struct { ... }
type Show struct { ... }
type Person struct { ... }
type PersonOrganization struct { ... }
type ShowSchedule struct { ... }
type Division struct { ... }
type Section struct { ... }
type ShowClass struct { ... }
type Entry struct { ... }
type Media struct { ... }
type Taxon struct { ... }
type TaxonRelation struct { ... }
type AwardDefinition struct { ... }
type Vote struct { ... }

// Ledger types
type FlowershowObject struct { ... }  // stable identity
type FlowershowClaim struct { ... }   // append-only change record

// Input types for form/API submissions
type ShowInput struct { ... }
type EntryInput struct { ... }
type ClassInput struct { ... }
type PersonInput struct { ... }
```

**Entity ID prefixes** (following the `newLedgerID` pattern):
- Organizations: `org_`
- Shows: `show_`
- Persons: `person_`
- Entries: `entry_`
- Classes: `class_`
- Divisions: `div_`
- Sections: `sec_`
- Schedules: `sched_`
- Taxons: `taxon_`
- Awards: `award_`
- Media: `media_`
- Claims: `claim_`

**`artifacts/flowershow-app/store.go`**

Define the store interface and the Postgres implementation.

The store interface should cover operations needed for Phase 1-3:

```go
type flowershowStore interface {
    // Organizations
    createOrganization(OrganizationInput) (*Organization, error)
    organizationByID(string) (*Organization, bool)
    allOrganizations() []*Organization

    // Shows
    createShow(ShowInput) (*Show, error)
    updateShow(string, ShowInput) (*Show, error)
    showByID(string) (*Show, bool)
    showBySlug(string) (*Show, bool)
    showsByOrganization(string) []*Show
    allShows() []*Show

    // Persons
    createPerson(PersonInput) (*Person, error)
    personByID(string) (*Person, bool)
    allPersons() []*Person

    // Schedule hierarchy
    createSchedule(ScheduleInput) (*ShowSchedule, error)
    createDivision(DivisionInput) (*Division, error)
    createSection(SectionInput) (*Section, error)
    createClass(ClassInput) (*ShowClass, error)
    scheduleByShowID(string) (*ShowSchedule, error)
    divisionsBySchedule(string) []*Division
    sectionsByDivision(string) []*Section
    classesBySection(string) []*ShowClass
    classByID(string) (*ShowClass, bool)

    // Entries
    createEntry(EntryInput) (*Entry, error)
    updateEntry(string, EntryInput) (*Entry, error)
    setPlacement(entryID string, placement int, points int) (*Entry, error)
    entriesByShow(string) []*Entry
    entriesByClass(string) []*Entry
    entryByID(string) (*Entry, bool)

    // Taxons
    createTaxon(TaxonInput) (*Taxon, error)
    taxonByID(string) (*Taxon, bool)
    allTaxons() []*Taxon
    taxonsByType(string) []*Taxon

    // Awards
    createAward(AwardInput) (*AwardDefinition, error)
    awardsByOrganization(string) []*AwardDefinition
    computeAward(awardID string, showID string) ([]AwardResult, error)

    // Leaderboard
    leaderboard(organizationID string, season string) []LeaderboardEntry

    // Ledger
    ledgerByObjectID(string) ([]FlowershowClaim, error)

    Close()
}
```

**Postgres schema** (inside `migrate()` method):

The three-table ledger pattern adapted for Flowershow. Because Flowershow has many entity types (not just events), the objects/claims tables are polymorphic:

```sql
-- Objects table: stable identity for any Flowershow entity
CREATE TABLE IF NOT EXISTS as_flowershow_objects (
    object_id TEXT PRIMARY KEY,
    object_type TEXT NOT NULL,  -- 'organization', 'show', 'person', 'entry', etc.
    slug TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by TEXT NOT NULL DEFAULT 'system'
);
CREATE UNIQUE INDEX IF NOT EXISTS as_flowershow_objects_slug_type_idx
    ON as_flowershow_objects (slug, object_type) WHERE slug IS NOT NULL;

-- Claims table: append-only ledger
CREATE TABLE IF NOT EXISTS as_flowershow_claims (
    claim_id TEXT PRIMARY KEY,
    object_id TEXT NOT NULL REFERENCES as_flowershow_objects(object_id),
    claim_seq BIGINT GENERATED ALWAYS AS IDENTITY UNIQUE,
    claim_type TEXT NOT NULL,
    accepted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accepted_by TEXT NOT NULL,
    supersedes_claim_id TEXT REFERENCES as_flowershow_claims(claim_id),
    payload JSONB NOT NULL DEFAULT '{}'::JSONB
);
CREATE INDEX IF NOT EXISTS as_flowershow_claims_object_idx
    ON as_flowershow_claims (object_id, claim_seq DESC);
```

Then **materialized tables** for each entity type (denormalized for fast reads):

```sql
-- Materialized: Organizations
CREATE TABLE IF NOT EXISTS as_flowershow_m_organizations (
    organization_id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    org_type TEXT NOT NULL DEFAULT 'club',
    parent_organization_id TEXT,
    location TEXT NOT NULL DEFAULT '',
    slug TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

-- Materialized: Shows
CREATE TABLE IF NOT EXISTS as_flowershow_m_shows (
    show_id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    location TEXT NOT NULL DEFAULT '',
    show_datetime TIMESTAMPTZ,
    season TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

-- Materialized: Persons
CREATE TABLE IF NOT EXISTS as_flowershow_m_persons (
    person_id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    initials TEXT NOT NULL DEFAULT '',
    privacy_flag BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

-- Materialized: Schedule hierarchy (4 tables)
CREATE TABLE IF NOT EXISTS as_flowershow_m_schedules ( ... );
CREATE TABLE IF NOT EXISTS as_flowershow_m_divisions ( ... );
CREATE TABLE IF NOT EXISTS as_flowershow_m_sections ( ... );
CREATE TABLE IF NOT EXISTS as_flowershow_m_classes ( ... );

-- Materialized: Entries
CREATE TABLE IF NOT EXISTS as_flowershow_m_entries (
    entry_id TEXT PRIMARY KEY,
    show_id TEXT NOT NULL,
    show_class_id TEXT NOT NULL,
    person_id TEXT NOT NULL,
    placement INTEGER,
    points INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

-- Materialized: Taxons
CREATE TABLE IF NOT EXISTS as_flowershow_m_taxons ( ... );
CREATE TABLE IF NOT EXISTS as_flowershow_m_taxon_relations ( ... );

-- Materialized: Awards
CREATE TABLE IF NOT EXISTS as_flowershow_m_award_definitions ( ... );

-- Materialized: Person-Organization links
CREATE TABLE IF NOT EXISTS as_flowershow_m_person_orgs ( ... );
```

**Key design decision:** Use a single polymorphic objects/claims pair (not per-entity-type tables). This simplifies the ledger and makes provenance queries uniform. The materialized tables are per-entity-type for query performance.

**`artifacts/flowershow-app/store_memory_test.go`**

In-memory implementation of `flowershowStore` for unit testing. Follows the same pattern as `/Users/splash/AS/seeds/0004-event-listings/realizations/a-ledger-web/artifacts/event-listings-app/store_memory_test.go`:

- `memoryFlowershowStore` struct with maps and `sync.RWMutex`
- `newMemoryFlowershowStore()` returns a seeded store with:
  - 1 organization ("Thornhill Garden & Horticultural Society")
  - 1 show ("Spring Flower Show 2026") with schedule, 3 divisions, 6 sections, 12 classes
  - 3 persons
  - 8 entries with placements
  - 5 taxons (rose, dahlia, crescent-design, novice, miniature)
  - 2 award definitions

### What's Testable

- `go test ./...` passes
- `store_memory_test.go` tests: create/read organizations, shows, persons, entries, schedule hierarchy, taxons
- Ledger queries return correct claim history
- Slug uniqueness is enforced

---

## Phase 2: Minimal Server and Public Pages

**Goal:** A running HTTP server with health check, home page listing shows, and show detail page. Produces the first visible web surface.

### Files to Create/Modify

**`artifacts/flowershow-app/main.go`**

Following the pattern from `/Users/splash/AS/seeds/0004-event-listings/realizations/a-ledger-web/artifacts/event-listings-app/main.go` (lines 1-175):

```go
package main

import (
    "embed"
    "html/template"
    "io/fs"
    "log"
    "net"
    "net/http"
    "os"
    // ...
)

const adminCookieName = "as_flowershow_admin"

//go:embed assets/*
var assets embed.FS

//go:embed templates/*.html templates/partials/*.html
var templateFS embed.FS

type app struct {
    store         flowershowStore
    templates     *template.Template
    adminPassword string
    serviceToken  string
    sseBroker     *sseBroker  // Phase 5
}

func main() {
    addr := envOrDefault("AS_ADDR", "127.0.0.1:8097")
    store, err := newFlowershowStore(envOrDefault("AS_RUNTIME_DATABASE_URL", ""))
    if err != nil {
        log.Fatal(err)
    }
    defer store.Close()

    a := &app{
        store:         store,
        templates:     parseTemplates(),
        adminPassword: envOrDefault("AS_ADMIN_PASSWORD", "admin"),
        serviceToken:  strings.TrimSpace(os.Getenv("AS_SERVICE_TOKEN")),
    }

    mux := http.NewServeMux()
    // Assets
    mux.Handle("GET /assets/", a.assetHandler())
    mux.HandleFunc("GET /healthz", a.handleHealth)

    // Public routes (Phase 2)
    mux.HandleFunc("GET /", a.handleHome)
    mux.HandleFunc("GET /shows/{slug}", a.handleShowDetail)

    // ... (more routes added in later phases)

    log.Printf("flowershow listening on http://%s", addr)
    if err := http.ListenAndServe(addr, requestLog(mux)); err != nil {
        log.Fatal(err)
    }
}
```

Note the port: `8097` (unique, not conflicting with 8095 for customer-service or 8096 for event-listings).

**`artifacts/flowershow-app/handlers_public.go`**

Public-facing handlers:

- `handleHome` -- lists published shows, featured show, upcoming shows
- `handleShowDetail` -- show detail page with schedule hierarchy, entries, awards

**`artifacts/flowershow-app/templates/base.html`**

Following the pattern from `/Users/splash/AS/seeds/0003-customer-service-app/realizations/a-web-mvp/artifacts/service-app/templates/base.html` but with the "flower-magazine aesthetic" -- bright colors, centered layout, floral accent colors:

```html
{{define "base"}}<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}} — Flowershow</title>
<script src="https://unpkg.com/htmx.org@2.0.4"></script>
<script src="https://unpkg.com/htmx-ext-sse@2.2.2/sse.js"></script>
<link rel="stylesheet" href="/assets/app.css">
</head>
<body>
<nav>...</nav>
<main class="container">{{template "content" .}}</main>
</body>
</html>
{{end}}
```

**`artifacts/flowershow-app/templates/home.html`**

Shows listing: upcoming shows, organization info, featured entries.

**`artifacts/flowershow-app/templates/show_detail.html`**

Show detail page: show info, schedule hierarchy (divisions/sections/classes), entries per class, awards.

**`artifacts/flowershow-app/assets/app.css`**

Bright, centered, flower-magazine aesthetic. Warm accent colors (greens, pinks, golds). Clean card layouts. Responsive.

**`artifacts/flowershow-app/assets/app.js`**

Minimal JS for now -- HTMX handles most interactivity.

**`artifacts/flowershow-app/main_test.go`**

HTTP tests following the pattern from the event-listings test file:

```go
func newTestApp(t *testing.T) *app { ... }
func TestHealthEndpoint(t *testing.T) { ... }
func TestHomePageLoads(t *testing.T) { ... }
func TestShowDetailBySlug(t *testing.T) { ... }
```

### What's Testable

- `go test ./...` passes
- `go run .` starts the server (with in-memory store if no DATABASE_URL)
- Browser: `http://127.0.0.1:8097/` shows the home page with seeded shows
- Browser: `http://127.0.0.1:8097/shows/spring-flower-show-2026` shows show detail
- `GET /healthz` returns `{"status":"ok","seed":"0007-Flowershow"}`

---

## Phase 3: Show Admin MVP (Operational Heart)

**Goal:** The Show Admin control panel -- the key early deliverable per the requirements. Admin can manage shows, classes, entries, persons, and set winners.

### Files to Create/Modify

**`artifacts/flowershow-app/handlers_admin.go`**

Admin handlers with `requireAdmin()` middleware (cookie-based auth identical to event-listings):

Routes to register in `main.go`:
```go
// Admin auth
mux.HandleFunc("GET /admin/login", a.handleAdminLogin)
mux.HandleFunc("POST /admin/login", a.handleAdminLoginPost)
mux.HandleFunc("POST /admin/logout", a.handleAdminLogoutPost)

// Admin dashboard
mux.HandleFunc("GET /admin", a.requireAdmin(a.handleAdminDashboard))

// Show management
mux.HandleFunc("GET /admin/shows/new", a.requireAdmin(a.handleAdminShowNew))
mux.HandleFunc("POST /admin/shows", a.requireAdmin(a.handleAdminShowCreate))
mux.HandleFunc("GET /admin/shows/{showID}", a.requireAdmin(a.handleAdminShowDetail))
mux.HandleFunc("POST /admin/shows/{showID}", a.requireAdmin(a.handleAdminShowUpdate))

// Schedule management (within a show)
mux.HandleFunc("POST /admin/shows/{showID}/schedule", a.requireAdmin(a.handleAdminScheduleCreate))
mux.HandleFunc("POST /admin/shows/{showID}/divisions", a.requireAdmin(a.handleAdminDivisionCreate))
mux.HandleFunc("POST /admin/shows/{showID}/sections", a.requireAdmin(a.handleAdminSectionCreate))
mux.HandleFunc("POST /admin/shows/{showID}/classes", a.requireAdmin(a.handleAdminClassCreate))

// Entry management
mux.HandleFunc("POST /admin/shows/{showID}/entries", a.requireAdmin(a.handleAdminEntryCreate))
mux.HandleFunc("POST /admin/entries/{entryID}/placement", a.requireAdmin(a.handleAdminSetPlacement))

// Person management
mux.HandleFunc("GET /admin/persons", a.requireAdmin(a.handleAdminPersons))
mux.HandleFunc("POST /admin/persons", a.requireAdmin(a.handleAdminPersonCreate))
```

**Key handler design:** The Show Admin page (`/admin/shows/{showID}`) is a single-page experience using HTMX partial updates. The page layout has tabs/sections:

1. **Show Info** -- edit show name, date, location, season
2. **Schedule** -- manage divisions, sections, classes (tree view)
3. **Entries** -- add entries, assign persons, set class
4. **Judges** -- assign judges to classes (person + class relationship)
5. **Winners** -- set placement and points per entry per class

Each section uses `hx-post` and `hx-target` to swap partials without full page reload.

**`artifacts/flowershow-app/templates/show_admin.html`**

The operational control panel. Tabbed layout with HTMX-powered sections:

```html
{{define "content"}}
<h1>Show Admin: {{.Show.Name}}</h1>
<div class="admin-tabs">
    <button hx-get="/admin/shows/{{.Show.ID}}/tab/info" hx-target="#admin-panel">Info</button>
    <button hx-get="/admin/shows/{{.Show.ID}}/tab/schedule" hx-target="#admin-panel">Schedule</button>
    <button hx-get="/admin/shows/{{.Show.ID}}/tab/entries" hx-target="#admin-panel">Entries</button>
    <button hx-get="/admin/shows/{{.Show.ID}}/tab/winners" hx-target="#admin-panel">Winners</button>
</div>
<div id="admin-panel">
    {{template "admin_info_tab" .}}
</div>
{{end}}
```

**`artifacts/flowershow-app/templates/partials/`** directory with:
- `admin_info_tab.html` -- show info edit form
- `admin_schedule_tab.html` -- schedule tree with add forms
- `admin_entries_tab.html` -- entry list with inline add
- `admin_winners_tab.html` -- class-by-class winner assignment
- `admin_class_row.html` -- single class row (for HTMX swap)
- `admin_entry_row.html` -- single entry row (for HTMX swap)

### What's Testable

- Admin login at `/admin/login` with password "admin"
- `/admin` dashboard shows list of shows
- `/admin/shows/{showID}` loads the Show Admin panel
- Can create a new show via form
- Can add divisions/sections/classes to a show
- Can add entries and set placements
- Can add persons
- All changes persist (via Postgres or in-memory store)
- Unit tests: admin routes require auth, CRUD operations work

---

## Phase 4: Schedule Hierarchy and Class Browse

**Goal:** Public-facing browsing of the schedule hierarchy. Users can navigate Division > Section > Class and see entries within each class.

### Files to Modify

**`handlers_public.go`** -- add:
- `handleClassBrowse` -- `GET /shows/{slug}/classes` -- browsable tree
- `handleClassDetail` -- `GET /shows/{slug}/classes/{classID}` -- entries in a class
- `handleEntryDetail` -- `GET /entries/{entryID}` -- single entry with media, person (initials), taxons

**New templates:**
- `templates/class_browse.html` -- expandable tree: Division > Section > Class
- `templates/entry_detail.html` -- entry detail page

**Register routes in `main.go`:**
```go
mux.HandleFunc("GET /shows/{slug}/classes", a.handleClassBrowse)
mux.HandleFunc("GET /shows/{slug}/classes/{classID}", a.handleClassDetail)
mux.HandleFunc("GET /entries/{entryID}", a.handleEntryDetail)
```

### What's Testable

- Public browsing of schedule hierarchy works
- Class detail shows entries sorted by placement
- Entry detail shows person initials (not full name -- privacy)
- Unit test: class browse returns seeded data

---

## Phase 5: SSE Real-Time for Show Admin

**Goal:** Multiple admin operators see live updates without page reload. When one admin adds an entry or sets a placement, other connected admins see the change instantly.

### Files to Create

**`artifacts/flowershow-app/sse.go`**

SSE broker following the pattern from `/Users/splash/AS/seeds/0003-customer-service-app/realizations/a-web-mvp/artifacts/service-app/handlers.go` (lines 264-317):

```go
type sseBroker struct {
    mu          sync.RWMutex
    subscribers map[string]map[chan string]struct{} // showID -> set of channels
}

func newSSEBroker() *sseBroker { ... }
func (b *sseBroker) Subscribe(showID string) chan string { ... }
func (b *sseBroker) Unsubscribe(showID string, ch chan string) { ... }
func (b *sseBroker) Publish(showID string, html string) { ... }

func (a *app) handleShowAdminStream(w http.ResponseWriter, r *http.Request) {
    showID := r.PathValue("showID")
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    flusher, ok := w.(http.Flusher)
    if !ok { ... }
    ch := a.sseBroker.Subscribe(showID)
    defer a.sseBroker.Unsubscribe(showID, ch)
    for {
        select {
        case <-r.Context().Done():
            return
        case html, ok := <-ch:
            if !ok { return }
            // Send as SSE data
            for _, line := range strings.Split(html, "\n") {
                fmt.Fprintf(w, "data: %s\n", line)
            }
            fmt.Fprintf(w, "\n")
            flusher.Flush()
        }
    }
}
```

### Modifications

**`handlers_admin.go`** -- After every mutation (create entry, set placement, add class, etc.), call `a.sseBroker.Publish(showID, renderedPartialHTML)`.

**`templates/show_admin.html`** -- Add SSE connection:
```html
<div id="live-updates" hx-ext="sse" sse-connect="/admin/shows/{{.Show.ID}}/stream" sse-swap="message" hx-swap="beforeend">
</div>
```

**`main.go`** -- Add route:
```go
mux.HandleFunc("GET /admin/shows/{showID}/stream", a.requireAdmin(a.handleShowAdminStream))
```

Initialize sseBroker in `main()`:
```go
a.sseBroker = newSSEBroker()
```

### What's Testable

- Open two browser tabs to the same Show Admin page
- Add an entry in tab 1 -- it appears in tab 2 without reload
- Set a winner in tab 1 -- tab 2 updates
- SSE connection shows in browser DevTools Network tab
- Unit test: SSE broker subscribe/publish/unsubscribe

---

## Phase 6: Taxonomy, Awards, and Leaderboard

**Goal:** Taxonomy browsing, award computation, and leaderboard display.

### Files to Modify

**`handlers_public.go`** -- add:
- `handleTaxonomyBrowse` -- `GET /taxonomy` -- browse taxons by type
- `handleTaxonDetail` -- `GET /taxonomy/{taxonID}` -- entries/classes referencing this taxon
- `handleLeaderboard` -- `GET /leaderboard` -- season leaderboard per organization

**`store.go`** -- implement award computation logic:
```go
func (s *postgresFlowershowStore) computeAward(awardID, showID string) ([]AwardResult, error) {
    // 1. Load award definition (includes_taxons, excludes_taxons, scoring rule)
    // 2. Query entries matching taxon filters
    // 3. Aggregate points per person using scoring rule (sum, max, custom)
    // 4. Return sorted results
}

func (s *postgresFlowershowStore) leaderboard(orgID, season string) []LeaderboardEntry {
    // Aggregate points per person across all shows in org+season
}
```

**`handlers_admin.go`** -- add award management:
- `POST /admin/awards` -- create award definition
- `POST /admin/awards/{awardID}/compute` -- trigger computation
- `GET /admin/shows/{showID}/awards` -- view computed awards for a show

**New templates:**
- `templates/taxonomy_browse.html`
- `templates/leaderboard.html`

**Routes in `main.go`:**
```go
mux.HandleFunc("GET /taxonomy", a.handleTaxonomyBrowse)
mux.HandleFunc("GET /taxonomy/{taxonID}", a.handleTaxonDetail)
mux.HandleFunc("GET /leaderboard", a.handleLeaderboard)
```

### What's Testable

- Browse taxons, click a taxon to see related entries
- Award computation produces correct results from seeded data
- Leaderboard shows person rankings for a season
- Cross-link: clicking "rose" on an entry navigates to the rose taxon page

---

## Phase 7: Standards, Rules, and Provenance

**Goal:** Standards, editions, source documents, citations, and rule inheritance.

### Schema Additions (in `migrate()`)

```sql
CREATE TABLE IF NOT EXISTS as_flowershow_m_standard_documents ( ... );
CREATE TABLE IF NOT EXISTS as_flowershow_m_standard_editions ( ... );
CREATE TABLE IF NOT EXISTS as_flowershow_m_source_documents ( ... );
CREATE TABLE IF NOT EXISTS as_flowershow_m_source_citations ( ... );
CREATE TABLE IF NOT EXISTS as_flowershow_m_standard_rules ( ... );
CREATE TABLE IF NOT EXISTS as_flowershow_m_class_rule_overrides ( ... );
```

### Store Interface Additions

```go
// Standards
createStandardDocument(StandardDocumentInput) (*StandardDocument, error)
createStandardEdition(StandardEditionInput) (*StandardEdition, error)

// Source provenance
createSourceDocument(SourceDocumentInput) (*SourceDocument, error)
createSourceCitation(SourceCitationInput) (*SourceCitation, error)
citationsByTarget(targetType string, targetID string) []*SourceCitation

// Rules
createStandardRule(StandardRuleInput) (*StandardRule, error)
rulesByEdition(editionID string) []*StandardRule
createClassRuleOverride(ClassRuleOverrideInput) (*ClassRuleOverride, error)
effectiveRulesForClass(classID string) []*EffectiveRule  // merged standard + overrides
```

### Handler Additions

**Admin routes:**
- `POST /admin/standards` -- create standard document
- `POST /admin/standards/{id}/editions` -- create edition
- `POST /admin/sources` -- register source document
- `POST /admin/citations` -- create source citation
- `POST /admin/rules` -- create standard rule
- `POST /admin/classes/{classID}/overrides` -- create class rule override

**Public routes:**
- `GET /standards` -- browse standards and editions
- `GET /shows/{slug}/rules` -- effective rules for a show's classes (merged view)

### What's Testable

- Create a standard document (e.g., "OJES") with an edition ("2019")
- Create standard rules for the edition
- Create a class rule override that narrows a standard rule
- `effectiveRulesForClass()` returns the merged rule set
- Source citations link to entries, classes, etc. with page ranges
- Public rules page shows merged rules

---

## Phase 8: Rubric Scoring and Judging

**Goal:** Rubric-based scoring with criterion-level scores from judges.

### Schema Additions

```sql
CREATE TABLE IF NOT EXISTS as_flowershow_m_judging_rubrics ( ... );
CREATE TABLE IF NOT EXISTS as_flowershow_m_judging_criteria ( ... );
CREATE TABLE IF NOT EXISTS as_flowershow_m_entry_scorecards ( ... );
CREATE TABLE IF NOT EXISTS as_flowershow_m_entry_criterion_scores ( ... );
```

### Store Interface Additions

```go
// Rubrics
createRubric(RubricInput) (*JudgingRubric, error)
createCriterion(CriterionInput) (*JudgingCriterion, error)
criteriaByRubric(rubricID string) []*JudgingCriterion

// Scoring
submitScorecard(ScorecardInput) (*EntryScorecard, error)
scorecardsByEntry(entryID string) []*EntryScorecard
criterionScoresByScorecard(scorecardID string) []*EntryCriterionScore

// Computed placement from scores
computePlacementsFromScores(classID string) error  // updates entry placements
```

### Admin Routes

```go
// Rubric management
mux.HandleFunc("POST /admin/rubrics", a.requireAdmin(a.handleAdminRubricCreate))
mux.HandleFunc("POST /admin/rubrics/{rubricID}/criteria", a.requireAdmin(a.handleAdminCriterionCreate))

// Scoring (judge-facing within show admin)
mux.HandleFunc("GET /admin/shows/{showID}/scoring", a.requireAdmin(a.handleAdminScoring))
mux.HandleFunc("POST /admin/entries/{entryID}/scorecard", a.requireAdmin(a.handleAdminScorecardSubmit))
mux.HandleFunc("POST /admin/classes/{classID}/compute-placements", a.requireAdmin(a.handleAdminComputePlacements))
```

### Scoring UI

The scoring UI is a form within Show Admin that shows each criterion for the rubric, lets the judge enter a score (0 to max_points), and submits the entire scorecard at once. HTMX form submission with partial swap.

### What's Testable

- Create a rubric with 3 criteria (e.g., "Form" max 25, "Color" max 25, "Condition" max 50)
- Submit a scorecard for an entry (score each criterion)
- `computePlacementsFromScores` correctly ranks entries by total score
- Scorecard shows audit trail (judge, timestamp, per-criterion scores)

---

## Phase 9: JSON API Surface (Commands and Projections)

**Goal:** Formal API endpoints matching the interaction contract for alternate clients and ingestion agents.

### Files to Create/Modify

**`artifacts/flowershow-app/handlers_api.go`**

JSON API endpoints following the event-listings pattern:

**Commands (POST, require auth):**
```go
mux.HandleFunc("POST /v1/commands/0007-Flowershow/shows.create", a.handleShowCreateCommand)
mux.HandleFunc("POST /v1/commands/0007-Flowershow/shows.update", a.handleShowUpdateCommand)
mux.HandleFunc("POST /v1/commands/0007-Flowershow/entries.create", a.handleEntryCreateCommand)
mux.HandleFunc("POST /v1/commands/0007-Flowershow/entries.update", a.handleEntryUpdateCommand)
mux.HandleFunc("POST /v1/commands/0007-Flowershow/entries.set_placement", a.handleSetPlacementCommand)
mux.HandleFunc("POST /v1/commands/0007-Flowershow/classes.create", a.handleClassCreateCommand)
mux.HandleFunc("POST /v1/commands/0007-Flowershow/schedule.create", a.handleScheduleCreateCommand)
mux.HandleFunc("POST /v1/commands/0007-Flowershow/persons.create", a.handlePersonCreateCommand)
mux.HandleFunc("POST /v1/commands/0007-Flowershow/awards.compute", a.handleAwardComputeCommand)
mux.HandleFunc("POST /v1/commands/0007-Flowershow/rubrics.create", a.handleRubricCreateCommand)
mux.HandleFunc("POST /v1/commands/0007-Flowershow/scorecards.submit", a.handleScorecardSubmitCommand)
```

**Projections (GET, anonymous for public data):**
```go
mux.HandleFunc("GET /v1/projections/0007-Flowershow/shows", a.handleShowsDirectoryProjection)
mux.HandleFunc("GET /v1/projections/0007-Flowershow/shows/{slug}", a.handleShowDetailProjection)
mux.HandleFunc("GET /v1/projections/0007-Flowershow/shows/by-id/{showID}", a.handleShowRecordProjection)
mux.HandleFunc("GET /v1/projections/0007-Flowershow/shows/by-id/{showID}/ledger", a.handleShowLedgerProjection)
mux.HandleFunc("GET /v1/projections/0007-Flowershow/entries/by-id/{entryID}", a.handleEntryRecordProjection)
mux.HandleFunc("GET /v1/projections/0007-Flowershow/classes/by-id/{classID}", a.handleClassRecordProjection)
mux.HandleFunc("GET /v1/projections/0007-Flowershow/taxonomy", a.handleTaxonomyProjection)
mux.HandleFunc("GET /v1/projections/0007-Flowershow/leaderboard", a.handleLeaderboardProjection)
mux.HandleFunc("GET /v1/projections/0007-Flowershow/admin/shows", a.handleAdminShowsProjection)
```

Auth pattern (matching event-listings):
- Anonymous: can read published show data, public projections
- Session (admin cookie): can read all data, execute commands
- Service token (`X-AS-Service-Token`): can read all data, execute commands

### What's Testable

- `TestCommandEndpointsRequireAuth` -- commands return 401 without auth
- `TestCommandEndpointsAcceptServiceToken` -- create show via API with service token
- `TestProjectionsReturnJSON` -- directory, detail, record projections return well-formed JSON
- `TestLedgerProjection` -- returns claim history
- Full API flow: create show -> create schedule -> create classes -> create entries -> set placements -> verify via projections

---

## Phase 10: S3 Media Uploads

**Goal:** Photo and video uploads to S3 with client-side optimization hints and server-side transcode for oversized files.

### Files to Create

**`artifacts/flowershow-app/media.go`**

```go
type mediaStore interface {
    Upload(ctx context.Context, file io.Reader, metadata MediaMetadata) (*Media, error)
    GetURL(mediaID string) (string, error)
    Delete(mediaID string) error
}

type s3MediaStore struct {
    bucket string
    region string
    client *s3.Client
}

type localMediaStore struct {
    // For development/testing -- stores files on disk
    dir string
}
```

### Dependencies

Add `github.com/aws/aws-sdk-go-v2` to `go.mod` (only when this phase is implemented).

### Routes

```go
mux.HandleFunc("POST /admin/entries/{entryID}/media", a.requireAdmin(a.handleMediaUpload))
mux.HandleFunc("DELETE /admin/media/{mediaID}", a.requireAdmin(a.handleMediaDelete))
mux.HandleFunc("POST /v1/commands/0007-Flowershow/media.attach", a.handleMediaAttachCommand)
```

### Client-Side Optimization

In `app.js`, add image resizing before upload using Canvas API:
- Photos: resize to max 2048px on longest edge, JPEG quality 85
- Client-side preview before upload
- Progress indicator via HTMX `hx-indicator`

### Server-Side Transcode

If uploaded file exceeds threshold (5MB for photos, 50MB for videos), queue a transcode job. For MVP: reject oversized files with a helpful error message. Full transcode can be a future iteration.

### Environment Variables

- `AWS_REGION` -- S3 region
- `AS_S3_BUCKET` -- bucket name
- `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` -- credentials (or use IAM role in production)

### What's Testable

- Upload a photo via admin UI, see it attached to entry
- Media URL renders in entry detail page
- Multiple media per entry
- File size validation
- In tests: use localMediaStore (filesystem) instead of S3

---

## Phase 11: Cognito Authentication and Roles

**Goal:** Replace the simple admin password with AWS Cognito for identity and in-app role management.

### Files to Create

**`artifacts/flowershow-app/auth.go`**

```go
type authProvider interface {
    ValidateToken(token string) (*UserIdentity, error)
    GetLoginURL() string
    GetCallbackURL() string
}

type cognitoAuth struct {
    userPoolID string
    clientID   string
    region     string
}

type simpleAuth struct {
    // Fallback for development (existing admin password approach)
    adminPassword string
}

type UserIdentity struct {
    CognitoSub string
    Email       string
    Name        string
}
```

### Role Management Schema

```sql
CREATE TABLE IF NOT EXISTS as_flowershow_m_user_roles (
    id TEXT PRIMARY KEY,
    cognito_sub TEXT NOT NULL,
    organization_id TEXT,
    show_id TEXT,
    role TEXT NOT NULL,  -- 'admin', 'judge', 'entrant', 'public'
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Routes

```go
mux.HandleFunc("GET /auth/login", a.handleCognitoLogin)
mux.HandleFunc("GET /auth/callback", a.handleCognitoCallback)
mux.HandleFunc("POST /auth/logout", a.handleCognitoLogout)
mux.HandleFunc("GET /admin/roles", a.requireAdmin(a.handleRoleManagement))
mux.HandleFunc("POST /admin/roles", a.requireAdmin(a.handleRoleAssign))
```

### Design Decision

Keep the simple admin password (`AS_ADMIN_PASSWORD`) as a fallback for local development and CI. Cognito is only active when `AS_COGNITO_USER_POOL_ID` is set.

### What's Testable

- Without Cognito env vars: falls back to simple password auth (existing behavior)
- With Cognito env vars: redirects to Cognito hosted UI, handles callback
- Role assignment: admin assigns "judge" role to a user for a specific show
- Role-gated access: only judges can submit scorecards for their assigned shows

---

## Phase 12: Playwright Integration Tests

**Goal:** End-to-end browser tests proving the acceptance criteria.

### Files to Create

**`tests/flowershow.spec.ts`**

Following the pattern from `/Users/splash/AS/tests/customer-service.spec.ts`:

```typescript
import { test, expect } from '@playwright/test';

// -- Core Functionality --
test('home page loads with show listings', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('h1')).toContainText('Flowershow');
    await expect(page.locator('text=Spring Flower Show')).toBeVisible();
});

test('show detail page displays schedule hierarchy', async ({ page }) => {
    await page.goto('/shows/spring-flower-show-2026');
    await expect(page.locator('h1')).toContainText('Spring Flower Show');
    await expect(page.locator('text=Division')).toBeVisible();
});

// -- Schedule Hierarchy --
test('class browse shows division > section > class tree', async ({ page }) => {
    await page.goto('/shows/spring-flower-show-2026/classes');
    await expect(page.locator('.division')).toBeVisible();
});

// -- Show Admin --
test('admin login and dashboard', async ({ page }) => {
    await page.goto('/admin/login');
    await page.fill('#password', 'admin');
    await page.click('button[type="submit"]');
    await expect(page).toHaveURL(/\/admin/);
});

test('admin can create a show', async ({ page }) => { ... });
test('admin can add classes to a show', async ({ page }) => { ... });
test('admin can add entries and set placements', async ({ page }) => { ... });

// -- Taxonomy --
test('taxonomy browse shows taxon types', async ({ page }) => {
    await page.goto('/taxonomy');
    await expect(page.locator('text=botanical')).toBeVisible();
});

// -- Leaderboard --
test('leaderboard shows person rankings', async ({ page }) => {
    await page.goto('/leaderboard');
    await expect(page.locator('table')).toBeVisible();
});

// -- Privacy --
test('public view shows initials, not full names', async ({ page }) => {
    await page.goto('/shows/spring-flower-show-2026');
    // Should NOT contain full names, only initials
    await expect(page.locator('text=J.S.')).toBeVisible();
});

// -- Entry Detail --
test('entry detail shows media and taxonomy tags', async ({ page }) => {
    // Navigate to a seeded entry
    ...
});

// -- API --
test('API: create show via service token', async ({ request }) => {
    const response = await request.post('/v1/commands/0007-Flowershow/shows.create', {
        headers: { 'X-AS-Service-Token': 'test-token' },
        data: { name: 'API Test Show', organization_id: 'org_...' }
    });
    expect(response.status()).toBe(201);
});
```

### Modifications to `tests/playwright.config.ts`

Add a second project for flowershow:

```typescript
const flowershowAppDir = path.resolve(
    __dirname,
    '../seeds/0007-Flowershow/realizations/a-firstbloom/artifacts/flowershow-app',
);

export default defineConfig({
    // ... existing config ...
    projects: [
        {
            name: 'customer-service',
            testMatch: 'customer-service.spec.ts',
            use: { baseURL: 'http://127.0.0.1:38090' },
        },
        {
            name: 'flowershow',
            testMatch: 'flowershow.spec.ts',
            use: { baseURL: 'http://127.0.0.1:38097' },
        },
    ],
    webServer: [
        // existing customer-service server config
        {
            command: 'go run .',
            cwd: flowershowAppDir,
            reuseExistingServer: false,
            timeout: 120000,
            url: 'http://127.0.0.1:38097',
            env: {
                ...process.env,
                AS_ADDR: '127.0.0.1:38097',
            },
        },
    ],
});
```

### Modifications to `scripts/ci-seed-go-tests.sh`

Add the flowershow app to the module_dirs array:

```bash
declare -a module_dirs=(
  "$repo_root/seeds/0003-customer-service-app/realizations/a-web-mvp/artifacts/service-app"
  "$repo_root/seeds/0004-event-listings/realizations/a-web-mvp/artifacts/event-listings-app"
  "$repo_root/seeds/0006-registry-browser/realizations/a-authoritative-browser/artifacts/registry-browser"
  "$repo_root/seeds/0007-Flowershow/realizations/a-firstbloom/artifacts/flowershow-app"
)
```

### What's Testable

- `npx playwright test flowershow.spec.ts` passes
- CI pipeline runs the tests automatically

---

## Phase 13: Polish, Seed Data, and Final PR

**Goal:** Production-quality seed data, visual polish, validation evidence, and PR submission.

### Seed Data

Update `seedIfEmpty()` in `store.go` to create a rich demonstration dataset:

- **1 organization**: "Thornhill Garden & Horticultural Society"
- **2 shows**: "Spring Flower Show 2026" (published, detailed), "Fall Chrysanthemum Show 2026" (draft)
- **1 standard document**: "OJES" with edition "2019"
- **3 source documents**: schedule PDF, rulebook PDF, results sheet
- **8 divisions** across both domains (horticulture and design)
- **20+ classes** with varying specimen counts and rules
- **5 persons** with realistic initials
- **30+ entries** with placements, some with rubric scores
- **10+ taxons** covering botanical names, design types, and skill levels
- **3 award definitions**: High Points, Best Rose, High Points Novice
- **5+ source citations** linking entries and classes to source documents

### Visual Polish

- Finalize the flower-magazine CSS: warm greens (#2d6a4f), petal pinks (#e8a598), gold accents (#c9a227)
- Centered layout, generous whitespace, card-based components
- Responsive: works well on mobile for show-night usage
- Print stylesheet for schedule/results

### Validation Evidence

Update `validation/README.md` with acceptance criteria mapping:

```markdown
# Validation Evidence

## Core Functionality
- [x] Shows can be created and browsed — See Playwright test `home page loads`
- [x] Entries captured with media and placements — See test `admin can add entries`
...

## Schedule Hierarchy
- [x] Division > Section > Class navigation — See test `class browse`
...
```

### CI Integration

Verify:
1. `go test ./...` passes in the flowershow-app directory
2. Playwright tests pass
3. `ci-realization-rendered-url-guard.sh` passes (no baked environment URLs)
4. Docker build succeeds with the new realization

### PR Strategy

```
Title: Add 0007-Flowershow a-firstbloom realization

Body:
## Summary
- New seed realization for federated flower show competition registry
- Go HTTP server with Postgres persistence (three-table ledger pattern)
- Show Admin with HTMX + SSE for real-time collaborative operations
- Schedule hierarchy, taxonomy, awards, leaderboard
- Standards, rules, provenance tracking
- Rubric-based scoring
- JSON API surface for alternate clients

## Files
- seeds/0007-Flowershow/realizations/a-firstbloom/ — full realization
- tests/flowershow.spec.ts — Playwright integration tests
- scripts/ci-seed-go-tests.sh — added to CI test matrix
- tests/playwright.config.ts — added flowershow project

## Test Plan
- [ ] `go test ./...` passes
- [ ] Playwright tests pass locally
- [ ] Docker build succeeds
- [ ] Manual: browse shows, use show admin, verify SSE updates
```

---

## Dependency Graph Between Phases

```
Phase 0 (metadata) ─→ Phase 1 (schema/store) ─→ Phase 2 (server/public)
                                                        │
                                                        ├─→ Phase 3 (show admin) ─→ Phase 5 (SSE)
                                                        │
                                                        ├─→ Phase 4 (class browse)
                                                        │
                                                        └─→ Phase 6 (taxonomy/awards/leaderboard)
                                                        
Phase 1 ─→ Phase 7 (standards/rules/provenance)
Phase 1 ─→ Phase 8 (rubric scoring)
Phase 3 ─→ Phase 9 (JSON API)
Phase 3 ─→ Phase 10 (S3 media)
Phase 3 ─→ Phase 11 (Cognito auth)

Phases 2-11 ─→ Phase 12 (Playwright tests)
All phases ─→ Phase 13 (polish and PR)
```

Phases 4, 5, 6, 7, 8, 9, 10, 11 can largely be worked in parallel once Phase 3 is complete, though 7+8 have a logical dependency (rubrics may reference standard editions).

---

## Estimated File Count

| Phase | New Files | Modified Files |
|-------|-----------|---------------|
| 0 | 6 | 0 |
| 1 | 3 | 0 |
| 2 | 6 | 0 |
| 3 | 7+ | 1 (main.go) |
| 4 | 2 | 2 |
| 5 | 1 | 3 |
| 6 | 2 | 3 |
| 7 | 0 | 3 |
| 8 | 0 | 3 |
| 9 | 1 | 1 |
| 10 | 1 | 3 |
| 11 | 1 | 3 |
| 12 | 1 | 2 |
| 13 | 0 | 5+ |

**Total**: ~31 new files, ~29 file modifications across all phases.

---

### Critical Files for Implementation

- `/Users/splash/AS/seeds/0004-event-listings/realizations/a-ledger-web/artifacts/event-listings-app/store.go` - Primary reference for the three-table ledger pattern, Postgres migration, pgx pool setup, transactional operations, and slug allocation
- `/Users/splash/AS/seeds/0004-event-listings/realizations/a-ledger-web/artifacts/event-listings-app/main.go` - Primary reference for the app struct, route registration, admin auth middleware, asset embedding, template parsing, and server bootstrap
- `/Users/splash/AS/seeds/0003-customer-service-app/realizations/a-web-mvp/artifacts/service-app/handlers.go` - SSE streaming pattern (lines 264-317): Content-Type headers, http.Flusher, channel-based loop, context cancellation
- `/Users/splash/AS/seeds/0003-customer-service-app/realizations/a-web-mvp/artifacts/service-app/templates/base.html` - HTMX CDN loading pattern, SSE extension inclusion, base template structure with nav/main layout
- `/Users/splash/AS/seeds/0007-Flowershow/design.md` - The authoritative entity model specification: all table schemas, field definitions, relationships, and domain rules that the store layer must implement
