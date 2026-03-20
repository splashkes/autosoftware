# Validation Evidence

This folder has four distinct validation artifacts:

- [`ACCEPTANCE_CHECKLIST.md`](./ACCEPTANCE_CHECKLIST.md) combines the seed
  acceptance criteria with the agent/API parity work that just landed.
- [`API_AND_PLAYWRIGHT_PLAN.md`](./API_AND_PLAYWRIGHT_PLAN.md) lays out the
  next pass for comprehensive contract, API, and browser coverage.
- [`COMMAND_DURABILITY_MATRIX.yaml`](./COMMAND_DURABILITY_MATRIX.yaml) is the
  Flowershow-only source of truth for published mutating commands, their
  current durability class, their claim types, and whether replay coverage is
  required in CI.
- The current startup path now includes a claim-backed projection rebuilder:
  when `as_flowershow_m_*` tables drift or are wiped, the replayed claim
  snapshot can repopulate them transactionally before reads resume.
- `README.md` maps the current implementation to the tests that already exist.

## Current Coverage Posture

- Deterministic app-level coverage lives in
  `artifacts/flowershow-app/main_test.go`.
- Browser coverage lives in
  `/Users/splash/as-flower-agent/autosoftware/tests/flowershow.public.spec.ts`,
  `/Users/splash/as-flower-agent/autosoftware/tests/flowershow.admin.local.spec.ts`,
  `/Users/splash/as-flower-agent/autosoftware/tests/flowershow.api.spec.ts`,
  and `/Users/splash/as-flower-agent/autosoftware/tests/flowershow.widget.spec.ts`.
- A headed remote OTP harness now exists in
  `/Users/splash/as-flower-agent/autosoftware/tests/flowershow.remote-auth.setup.ts`,
  `/Users/splash/as-flower-agent/autosoftware/tests/flowershow.remote-admin.spec.ts`,
  and `/Users/splash/as-flower-agent/autosoftware/tests/flowershow.remote-agent-api.spec.ts`.
- The recent agent/API parity work added contract discovery, structured errors,
  service-token workspace access, schedule upsert, and the shared agent-access
  widget. Some of that is covered now, but the full admin parity surface still
  needs broader API and browser tests.

## Acceptance Criteria → Test Evidence

### Core Functionality
- **Shows can be created and browsed** → `TestHomePageLoads`, `TestShowDetailBySlug`, `TestAPIShowsDirectory`, Playwright: "home page loads", "show detail page"
- **Entries are captured with media and placements** → `TestFullAPIFlow`, `TestStoreMemoryBasics`, Playwright: "admin CRUD"
- **Entries link to people and classes** → `TestEntryDetail`, `TestClassBrowse`

### Schedule Hierarchy
- **Division → Section → Class browsing** → `TestClassBrowse`, Playwright: "class browse shows schedule hierarchy"
- **Entry uses show_class_id** → `TestStoreMemoryBasics` (entries reference class_id)

### Governance & Provenance
- **Standard editions** → `TestEffectiveRulesForClass` (OJES standard + edition created)
- **Source documents & citations** → Store interface tested via `TestStoreMemoryBasics`
- **Extraction confidence** → Model includes `extraction_confidence` field

### Rule Inheritance
- **Standard rules per edition** → `TestEffectiveRulesForClass` (creates standard rule)
- **Local class overrides** → `TestEffectiveRulesForClass` (override + local_only verified)

### Rubric Scoring
- **Rubrics with criteria** → `TestScorecardAndPlacement`
- **Entry scorecards with per-criterion scores** → `TestScorecardAndPlacement` (total 85 verified)
- **Computed placements** → `TestScorecardAndPlacement` (entry_01 = 1st, entry_02 = 2nd)

### Taxonomy
- **Taxon browsing** → `TestTaxonomyBrowse`, Playwright: "taxonomy browse"
- **Cross-link navigation** → Playwright: "taxon detail shows related entries"

### Awards
- **Award computation** → `TestStoreMemoryBasics` (computeAward returns results)
- **Taxonomy filters + scoring rules** → `computeAward` supports sum/max/count

### Leaderboard
- **Season leaderboard** → `TestLeaderboard`, Playwright: "leaderboard"

### Show Admin
- **Login/auth** → `TestAdminLoginFlow`, `TestAdminRequiresAuth`, Playwright: "admin login"
- **SSE real-time** → `sse.go` broker with per-show channels, SSE stream endpoint
- **HTMX partial updates** → Templates use HTMX attributes
- **Agent access widget** → `TestHomePageLoads`
- **Contract discovery from the realization** → `TestContractsEndpointsReturnLocalContract`
- **Authenticated show workspace projection** → `TestShowWorkspaceProjectionAcceptsServiceToken`
- **Schedule upsert parity command** → `TestScheduleUpsertCommandCreatesSchedule`

### Privacy
- **Initials only in public view** → `TestEntryDetail` (checks for "MC"), Playwright: "entry detail shows initials only"

### Authentication
- **Session auth** → Cookie-based admin auth
- **Service token auth** → `TestCommandEndpointsAcceptServiceToken`
- **Commands require auth** → `TestCommandEndpointsRequireAuth`

### API Surface
- **Projections return JSON** → `TestProjectionsReturnJSON`
- **Ledger projection (auth required)** → `TestLedgerProjection`
- **Full CRUD via API** → `TestFullAPIFlow`
- **Structured authenticated errors** → `TestCommandEndpointsRequireAuth`, `TestCommandEndpointsReturnUsefulStructuredErrorsForAuthenticatedCallers`
