# Validation Evidence

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
- **Cross-link navigation** → `TestTaxonDetail` (implicit via handler), Playwright: "taxon detail"

### Awards
- **Award computation** → `TestStoreMemoryBasics` (computeAward returns results)
- **Taxonomy filters + scoring rules** → `computeAward` supports sum/max/count

### Leaderboard
- **Season leaderboard** → `TestLeaderboard`, Playwright: "leaderboard"

### Show Admin
- **Login/auth** → `TestAdminLoginFlow`, `TestAdminRequiresAuth`, Playwright: "admin login"
- **SSE real-time** → `sse.go` broker with per-show channels, SSE stream endpoint
- **HTMX partial updates** → Templates use HTMX attributes

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
