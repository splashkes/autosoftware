# a-firstbloom Development Log

## Session: 2026-03-17

### Work Completed

**Phase 0 — Scaffolding**
- Created realization directory structure under `seeds/0007-Flowershow/realizations/a-firstbloom/`
- Wrote `realization.yaml`, `interaction_contract.yaml`, `artifacts/runtime.yaml`, `README.md`, `validation/README.md`
- Initialized `go.mod` with `pgx/v5` dependency
- `go mod tidy` succeeded

**Phase 1 — Models & Store**
- `models.go`: 30+ domain structs covering all entities from design.md — Organization, Show, Person, schedule hierarchy (ShowSchedule, Division, Section, ShowClass), Entry, Media, Taxon, TaxonRelation, AwardDefinition, StandardDocument, StandardEdition, SourceDocument, SourceCitation, StandardRule, ClassRuleOverride, JudgingRubric, JudgingCriterion, EntryScorecard, EntryCriterionScore, FlowershowObject, FlowershowClaim, LeaderboardEntry
- `store.go`: `flowershowStore` interface with 50+ methods. Full in-memory implementation (`memoryStore`) with thread-safe `sync.RWMutex`. Postgres store (`postgresFlowershowStore`) with complete schema migration (20+ tables using `as_flowershow_` prefix), three-table ledger pattern (objects, claims, materialized). Postgres delegates to in-memory for now — schema is ready for future SQL implementation.
- Demo seed data: 1 org (Metro Rose Society), 2 shows, 3 persons, 1 schedule with 3 divisions / 6 sections / 12 classes, 8 entries with placements, 5 taxons, 2 awards, 1 rubric with 6 criteria
- ID prefixes: `org_`, `show_`, `person_`, `entry_`, `class_`, `div_`, `sec_`, `sched_`, `taxon_`, `award_`, `claim_`, `std_`, `edition_`, `source_`, `cite_`, `rule_`, `override_`, `rubric_`, `crit_`, `scorecard_`, `crit_score_`

**Phase 2 — Server & Public Pages**
- `main.go`: App struct, `main()`, 50+ routes registered on `http.ServeMux`, `//go:embed` for templates and assets, health check, unix socket support, request logging, `envOrDefault()`, admin auth middleware, service token auth
- `handlers_public.go`: Home page, show detail (with schedule tree + entries), class browse, class detail, entry detail (privacy: initials only), taxonomy browse, taxon detail (with child taxons + related entries), leaderboard, standards listing, show rules (effective rules merged from standard + overrides)
- 15 HTML templates with HTMX 2.0.4 + SSE extension, warm green/pink/gold aesthetic
- Default port 8097, falls back to in-memory store if no `AS_RUNTIME_DATABASE_URL`

**Phase 3 — Show Admin**
- `handlers_admin.go`: Login/logout with cookie auth, admin dashboard with stats, show CRUD, schedule/division/section/class creation, entry CRUD, placement setting, person management, award creation + computation trigger, standards/editions/sources/citations/rules/overrides admin, rubric/criterion creation, scorecard submission, compute-placements-from-scores
- `templates/show_admin.html`: Tabbed layout (Info, Schedule, Entries, Winners, Scoring) with inline add forms using `<details>` elements

**Phase 5 — SSE Real-Time**
- `sse.go`: `sseBroker` with per-show subscriber channels, subscribe/unsubscribe/publish. Admin mutations publish toast notifications via SSE to all connected tabs.
- Show admin template wired with `hx-ext="sse" sse-connect="/admin/shows/{id}/stream"`

**Phases 4, 6, 7, 8 — Class Browse, Taxonomy, Standards, Scoring**
- All implemented inline with phases 2-3 (handlers + templates + store methods)
- `effectiveRulesForClass()` merges standard rules with class overrides
- `computePlacementsFromScores()` averages scorecards and assigns 1st/2nd/3rd
- `computeAward()` supports sum/max/count scoring with taxon filters

**Phase 9 — JSON API**
- `handlers_api.go`: 20 command endpoints (`POST /v1/commands/0007-Flowershow/*`) and 8 projection endpoints (`GET /v1/projections/0007-Flowershow/*`)
- Commands require session cookie or service token auth
- Public projections are anonymous; ledger and admin dashboard require auth

**Phase 12 — Tests**
- `main_test.go`: 18 Go tests — health, home page, show detail, class browse, entry detail (privacy), taxonomy, leaderboard, admin login flow, admin auth required, API projections, API commands auth, service token, ledger projection, full API flow, store basics, effective rules, scorecard + placement computation
- `tests/flowershow.spec.ts`: 13 Playwright tests — home page, show detail, class browse, entry privacy, taxonomy, taxon detail, leaderboard, admin login, admin CRUD, API shows, API auth, service token, health check
- Updated `tests/playwright.config.ts` to multi-project config (customer-service + flowershow)
- Updated `scripts/ci-seed-go-tests.sh` to include flowershow-app

**Phase 13 — Validation**
- `validation/README.md` maps each acceptance criterion to specific test evidence

### Decisions Made

1. **In-memory store as primary, Postgres schema ready** — Postgres store creates all tables on migration but delegates to in-memory for all operations. This lets the app run without a database for dev/CI while having the schema ready for a future SQL implementation pass.

2. **Per-page template cloning** — Go's `template.ParseFS` puts all templates in one namespace, causing `{{define "content"}}` collisions. Solved by parsing base.html once, then cloning it per page template. `render()` calls `ExecuteTemplate(w, "base", data)` on the page-specific clone.

3. **Flat handler files over nested packages** — Following the 0004-event-listings pattern: single package `main` with `handlers_public.go`, `handlers_admin.go`, `handlers_api.go`. Clean separation without import complexity.

4. **SSE via channel-per-show** — Simple broker pattern: `map[showID]map[chan string]struct{}`. Mutations publish rendered toast HTML. No message persistence — SSE is for live operator coordination only.

5. **Points map for placements** — Default points: 1st=6, 2nd=4, 3rd=2. Admin can override per-entry.

### Problems Overcome

1. **Template `"content"` collision** — All page templates defined `{{define "content"}}` which conflicted when parsed together. Fixed by switching from single `*template.Template` to `map[string]*template.Template` with per-page cloning from base.

2. **Float64 vs int comparison in templates** — `{{if gt .Entry.Points 0}}` failed because Go templates can't compare `float64` with `int` literal. Changed to `{{if .Entry.Points}}` which is truthy for non-zero.

3. **interaction_contract.yaml schema mismatch** — Kernel expects commands/projections as structured objects (name, summary, path, auth_modes, object_kinds) not plain strings. CI failed on `TestRepositoryRealizationsDeclareInteractionContracts`. Rewrote contract to match 0003's format.

### Current State

- **Branch**: `feat/0007-flowershow-a-firstbloom`
- **PR**: https://github.com/splashkes/autosoftware/pull/64
- **Commits**: 2 (initial realization + contract fix)
- **CI status**: Pushed contract fix, waiting for checks to pass before squash merge
- **Go tests**: All 18 pass locally
- **`kernel-go-tests` failure**: Was caused by malformed `interaction_contract.yaml` — fixed in second commit
- **`kernel-stack-smoke` failure**: Likely unrelated to our changes (infrastructure check), needs investigation if it persists after contract fix
