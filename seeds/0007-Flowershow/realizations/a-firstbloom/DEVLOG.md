# a-firstbloom Development Log

## Session: 2026-05-13

### Work Completed

**Operator exports**
- Added `handlers_exports.go` with protected show export endpoints for admin sessions and service-token agents.
- Added normalized CSV exports for entries, schedule hierarchy, leaderboard, and scorecards.
- Added normalized `workbook.xls` export with entries, schedule, leaderboard, and scorecards sheets.
- Added operator-style `tally.xls` export modeled after existing show-night tally workbooks:
  - `Show Tally` grid with class numbers across columns and exhibitors down rows
  - repeated `#1s #2s #3s` tally columns
  - `Point Summary` using local operator scoring conventions
  - `Participation` and `Results` tabs
- Added admin show-page download controls: `Tally XLS`, `Data XLS`, and CSV export links.
- Added API projection export URLs under `/v1/projections/0007-Flowershow/shows/{id}/exports/{file}`.

**Documentation and contract updates**
- Added `exports` capability and export projections to `interaction_contract.yaml`.
- Updated seed design, acceptance criteria, seed README, realization README, and validation evidence for tally/data exports.

### Decisions Made

1. **Two export shapes** — Keep normalized exports for agents/data pipelines and a separate operator workbook for show-night workflows.
2. **Generated-on-request reports** — Export files are generated from current store state rather than persisted artifacts.
3. **Service-token access for agents** — Agent/API export URLs require `Authorization: Bearer $AS_SERVICE_TOKEN`; admin downloads use session auth.
4. **Operator scoring convention** — Tally workbook point summary follows the provided sheets: horticulture/flowers/vegetables use 4/3/2 and design/special use 12/9/6.

### Validation

- `go test ./...` in `artifacts/flowershow-app`
- `./scripts/ci-seed-go-tests.sh`
- Local smoke test confirmed admin export controls render and `tally.xls` returns an Excel attachment.

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

---

## Session: 2026-04-01 through 2026-04-07

### Context

An agent authoring a full show schedule via the Flowershow API took ~8 minutes
and ~12 discovery round-trips. Post-mortem feedback identified five concrete
gaps in the agent-facing surface. This session validated that feedback against
the codebase and implemented fixes across the seed and kernel.

### Work Completed

**Phase 1 — New command endpoints (PR #156)**

The schedule hierarchy (show → schedule → division → section → class) previously
required mixing JSON command endpoints with admin HTML form POSTs. Three store
methods already existed but had no command routing:

- `schedules.upsert` — upsert semantics (create or update by show_id)
- `divisions.create` — takes show_schedule_id, title, domain, sort_order
- `sections.create` — takes division_id, title, sort_order

Added: handler cases in `handlers_api.go`, route registrations in `main.go`,
command + domain object entries in `interaction_contract.yaml`, authoring
workflow section 18 in `design.md`, decision log entry 18, and
`TestScheduleHierarchyAPI` covering the full chain plus auth/validation.

Also closed a pre-existing gap: `ingestions.import` was in code but missing
from the interaction contract.

Updated `kernel/protocol/v1/interactions.md` with cross-seed guidance that
multi-step authoring workflows must document the command dependency chain.

**Phase 2 — Kernel fix (PR #157)**

Shipped a pre-existing fix: Go 1.22 ServeMux only matches `POST /feedback/incidents`
for POST — GET fell through to the bootloader homepage (200 instead of 405).
Added `exactPathHandler` wrapper in `kernel/cmd/webd/main.go`.

**Phase 3 — Production incident (PR #158)**

Flowershow failed to launch after the #156 merge — `exit status 2` on every
attempt. Diagnosed via `kubectl -n as-system exec as-webd-... -c execd -- cat`
of the process log on the server. Root cause: squash merge left duplicate
`HandleFunc` registrations for `schedules.upsert`, `divisions.create`,
`sections.create`. Go 1.22+ ServeMux panics on duplicate patterns. Also had
duplicate command entries in `interaction_contract.yaml` causing kernel contract
validation failures.

Key debugging path: `doctl kubernetes cluster kubeconfig save as-prod` →
`kubectl get pods -n as-system` → exec into `execd` sidecar container (not
`webd`) to read `/tmp/as-executions/.../process.log`.

**Phase 4 — Agent widget improvements (PR #159)**

The agent access widget (`templates/partials/agent_access_widget.html`) had:
- Wrong displayed paths (`GET /v1/contracts` instead of mounted `GET /flowershow/v1/contracts`)
- No authoring workflow documentation
- No token scope guidance
- No command field names or dependency chain

Rewrote the "Agent + access" tab with:
- Correct mounted paths using `{{$.BasePath}}`
- Full 5-step authoring workflow block with command paths and field names
- Token scope guidance ("Manage shows" scope, permission_denied recovery)
- Consolidated API Surface, Contribution, Workflow, and Registry Docs

**Phase 5 — Second-round agent feedback fixes (PR #160)**

Three issues identified from re-testing:

1. Contract JSON at `/v1/contracts/0007-Flowershow/a-firstbloom` returned
   paths without mount prefix. Added `base_url` field to both contract list
   and detail responses in `contract_api.go` so agents can resolve relative
   paths programmatically.

2. Authoring workflow started at `shows.create` requiring `organization_id`
   with no hint about where to find it. Added discovery preamble with
   `GET .../organizations` and `GET .../shows` projections.

3. `sort_order` was missing from `classes.create` field list in the widget.

### PRs

| PR | Title | Status |
|----|-------|--------|
| [#156](https://github.com/splashkes/autosoftware/pull/156) | Add schedule hierarchy command endpoints to Flowershow API | Merged |
| [#157](https://github.com/splashkes/autosoftware/pull/157) | Fix GET /feedback/incidents falling through to homepage | Merged |
| [#158](https://github.com/splashkes/autosoftware/pull/158) | Fix duplicate route registrations that panic on startup | Merged |
| [#159](https://github.com/splashkes/autosoftware/pull/159) | Add authoring workflow and fix paths in agent access widget | Merged |
| [#160](https://github.com/splashkes/autosoftware/pull/160) | Add base_url to contract, discovery preamble, and sort_order | Merged |

### Lessons

- Squash merges of rebased branches can leave duplicate entries when upstream
  already added the same content. Always check for duplicates after merge
  resolution, not just conflicts.
- The `execd` sidecar container holds execution logs, not the `webd` container.
- Go 1.22+ ServeMux panics on duplicate route patterns at startup — this is a
  hard crash (exit 2), not a graceful error.
- The kernel-injected widget (`data-agent-widget-source="kernel"`) takes
  priority over the app widget via CSS, but both are visible to curl/fetch.
  Widget changes belong in the app template, not the kernel.

---

## Session: 2026-05-06

### Context

A real-event beta test of the Flowershow admin produced a list of UX, data-model, and pipeline gaps. This session worked through them across six PRs, building on top of the contract-gap closures already shipped earlier the same day (#161 — entries.set_special_status, account projection, capability-terminology table).

### Work Completed

**PR #162 — Image pipeline (EXIF + thumbnails + cover photo + class media)**

Beta feedback reported sideways photos. Root cause: zero EXIF handling. Injected
`disintegration/imaging` autorotate into `mediaStore.Store` for both local and
S3 backends; rotation happens once at write time so all downstream consumers
get oriented bytes. Server-side 512px JPEG thumbnails generated on the same
write path, served via `/media/{id}?thumb=1`. New `Media.IsCover` flag with
implicit-first-photo read fallback. New `Media.EntityKind` (entry|class) +
nullable `Media.ClassID` for class overview photos. New commands:
`media.set_cover`, `media.attach_to_class`. Also declared `media` as a
`domain_objects` kind — the kernel validator rejected the new commands without it.

**PR #163 — Class splits + soft-archive (data model)**

New `class_split` object kind with auto-letter-coding (a/b/c) plus optional
label. `Entry.SplitID` tracks membership. `computePlacementsFromScores` is now
split-aware, grouping by split and ranking within each group. Empty splits
auto-purge when the last entry moves out. Soft-archive: `Entry.ArchivedAt`
excludes from default reads; `*IncludeArchived` variants exist for restore
screens. New commands: `class_splits.create`, `class_splits.delete`,
`entries.move_to_split`, `entries.archive`, `entries.restore`.

**PR #164 — Show landing page redesign**

`/shows/{slug}` rebuilt phone-first. Hero band with status pill + actions,
"Winners by class" table covering every class with 1/2/3/Special/HM, four nav
tiles (entries / classes / clubs / exhibitors). Two new sub-pages:
`/shows/{slug}/entries` (filterable, mobile card grid) and
`/shows/{slug}/exhibitors` (per-show aggregate; anonymous entrants grouped
under one card). All CSS appended to existing `assets/app.css` using existing
palette variables — no Tailwind, no new fonts.

**PR #165 — Helper share-link onboarding**

Two new domain objects (`show_helper_invite`, `show_badge_session`). Admin
generates a share link from `/admin/shows/{showID}/helpers`; helper redeems on
`/shows/{slug}/help-redeem` with name + email and optional person match. Badge
session token in cookie `as_show_badge`, HttpOnly, SameSite=Lax, path-scoped to
`/shows/`. NO Cognito for helpers. Badge sessions grant the existing
`show_intake_operator` authority bundle. New `requireShowBadge` middleware.
New commands: `show_helper_invites.create`, `show_helper_invites.revoke`,
`show_badge_sessions.end`.

**PR #166 — Sequential photo intake + retry queue + anon entries + gallery**

New page `/admin/shows/{showID}/intake/photos` with sticky class bar (prev/next
arrows wrap the schedule), big tap-to-capture surface, optimistic upload with
in-memory retry queue. Every shutter creates a fresh anonymous Entry
(PersonID="") to skip the name-lookup blocker. Multi-photo gallery on entry
detail page with delete + star-to-set-cover. No new commands — uses existing
`entries.create`, `media.attach`, `media.delete`, `media.set_cover`. Lazy
template registration to avoid main.go conflicts with the parallel sibling PR.

**PR #167 — Class splits UI + Fast/Update mode toggle + collapsibles**

Drives the data model from #163. Split button in the top-right of each class
panel (Update mode only); multi-select activation flow; per-entry "Move to →"
dropdown for sibling moves; per-split judging surface. Top-of-workspace
`Fast add` / `Update entries` segmented toggle persisted in `localStorage`.
Native `<details>` collapsibles per entry section (Name match, Ranking,
Comment, Photo detail), open/closed state persisted PER section type (not per
entry) — flipping Comment open opens it for every entry. New fragment endpoint
`GET /admin/classes/{classID}/splits/fragment` for refresh-after-mutation.

### PRs

- #161 Close Flowershow contract gaps; declare account projection and entries.set_special_status
- #162 Image pipeline: EXIF auto-rotate, server thumbnails, cover photo, class-attached media
- #163 Class splits + soft-archive (data model)
- #164 Show landing redesign + /entries + /exhibitors sub-pages
- #165 Helper share-link onboarding (show-scoped badge sessions)
- #166 Sequential photo intake + retry queue + anonymous entries + gallery
- #167 Class splits UI + Fast/Update mode toggle + collapsibles

### Lessons

- **Parallel agents on shared files cost more than they save when ownership
  isn't truly disjoint.** The first parallel batch (PR 163/164/165 in
  worktrees) hit a `main_test.go` conflict between PR 163 and PR 164 because
  both inserted new test functions at the bottom of the file. Resolution was
  clean (both blocks kept) but cost a manual round-trip per PR.
- **Tests-in-a-new-file is the cheap fix.** The second parallel batch
  (PR 166/167) put tests in dedicated `intake_flow_test.go` and
  `class_splits_ui_test.go` files. Conflicts dropped to `main.go` and
  `app.css` only, both append-only with marker comments — three-way merge was
  trivial.
- **Worktree-isolated agents push their own branches but the post-merge
  `gh pr merge --delete-branch` cannot delete a worktree-held branch.** Need
  to `git worktree remove -f -f` first. Ran into this twice.
- **Git's three-way merge handles "we both added the same thing" gracefully.**
  PR 163 and PR 162 both added `kind: media` to the contract YAML
  `domain_objects`. GitHub flagged the PRs as MERGEABLE despite being BEHIND,
  and post-update merge produced no conflict on that addition.

### Post-batch fixes

After the six-PR batch landed, eyes-on-page review surfaced three follow-ups:

- **PR #169 — Show landing imagery.** The redesigned `/shows/{slug}` was technically rendering a class-attached cover via CSS `background-image`, but a darkening scrim over it made the page read as image-less. Replaced the background-image pattern with a real `<img>` element (CSS-grid hero on tablet+, stacked on phones) and added a "Highlights" horizontal strip plus 64-80px thumbnails next to every winner in the winners-by-class table. Used `?thumb=1` everywhere so the page doesn't pull multi-MB originals. The page is now image-led, as a flower show landing should be.
- **PR #170 — HM badge label.** The "HONORABLE MENTION" badge was forcing entry names to wrap awkwardly in winners-row cells. Shortened to "HM" to match the visual rhythm of 1ST / 2ND / 3RD / SPECIAL.
- **PR #171 — Intake panel fast-add CTA + optional entrant.** Two related issues: the new-entry modal still required the Entrant name as the FIRST field (contradicting #166's photo-first anonymous-by-default flow), and the new sequential photo intake page from #166 had no entry point from the show workspace. Added a "Fast add photos" CTA card above the intake grid linking to `/admin/shows/{showID}/intake/photos`, reordered the new-entry modal so "Add media" is first with Capture as the primary action, dropped `required` from Entrant and labelled it "(optional — add later if unknown)". The server-side handler already accepted empty PersonID — the client-side `required` attribute was the only blocker.

### Lesson — eyes-on-page beats green CI

All six batch PRs were green on CI, deployed cleanly, and verified live via curl-grepping the HTML. None of those automated checks caught: a hero that looked dark, a badge that wrapped, or a modal that asked for the wrong field first. A short visual pass on the production page surfaced all three. The cheap fix is to bake "open the live page on a phone before declaring done" into the post-deploy workflow, not just contract probes and pod health.
