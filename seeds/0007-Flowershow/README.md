# 0007-Flowershow

Federated, append-only registry for horticulture and design competitions.

## Current Canonical State

The current canonical implementation is the `a-firstbloom` realization.

- Domain facts for Flowershow now append through the kernel runtime registry boundary.
- `as_flowershow_m_*` tables are materialized projections, not the only durable source of truth.
- Effective authority is materialized from kernel runtime authority history rather than trusted as seed-local mutable state.
- Public identity defaults to initials unless a person explicitly chooses a broader public display mode.
- The interactive contract in
  [`realizations/a-firstbloom/interaction_contract.yaml`](/Users/splash/as-flower-agent/autosoftware/seeds/0007-Flowershow/realizations/a-firstbloom/interaction_contract.yaml)
  is the canonical API surface for authoring, imports, and projections.

## Canonical Operator Surfaces

- Public browsing: shows, clubs, classes, entries, people, taxonomy, leaderboard
- Profile shell: overview, shows, clubs, entries, access tokens, admin workspace
- Club admin: overview, invites, scoped permissions, membership context
- Show admin: `Setup`, `Entries`, `Corrections`, `Scoring`, `Board`, `Governance`
- Show-night operations: class-tile intake, in-place entry editing, media capture/upload, live SSE refresh across connected operators

## Key Concepts

- **Organizations** form a hierarchy (Club → District → Region → Province → Country → Global)
- **Shows** are competition events hosted by an organization
- **Schedule Hierarchy** — Division → Section → Class — models real fair-book structure
- **Standards & Editions** — governing rulebooks (OJES, Publication 34) with edition tracking
- **Source Provenance** — every structured record traces back to a source document and page
- **Rule Inheritance** — local schedules inherit from standards and can override class rules
- **Entries** are submissions into a class within a show, with media, placements, and taxonomy
- **Rubric Scoring** — criterion-level judging with score provenance, not just placement
- **Taxonomy** — flexible graph-like tagging (botanical names, design types, skill levels)
- **Awards** — organization-scoped, taxonomy-filtered, computed from scores
- **Media** — multiple photos/videos per entry, client-optimized uploads to S3
- **Privacy** — public display of initials; private identity mapping; append-only suppression
- **Authority & Delegation** — system-native club and show control with
  delegated grants, revocations, and ledger-visible authority history

## Design Principles

- API-first authoring and review from humans and remote agents carrying cited
  source material
- Standards and provenance as first-class structural layers
- Graph-like taxonomy over rigid schemas
- Append-only with suppression (no deletion)
- Organization-scoped computation
- Agent-equal or agent-better access for normal authenticated workflows
- External auth for identity, internal registry-backed authority for control

## Domains

- Horticulture (botanical specimens)
- Design (compositions)
- Special
- Other

## Realization

- **a-firstbloom** — Go web app with Show Admin UI, Postgres persistence, S3 media, Cognito auth

## Import And API Posture

Flowershow is now explicitly API-first for both humans and agents.

Important current commands and projections include:

- `organization.create`
- `organizations.directory`
- `shows.create`
- `shows.reset_schedule`
- `schedules.upsert`
- `divisions.create`
- `sections.create`
- `classes.create`
- `entries.create`
- `entries/{entryID}/media.upload`
- `persons.create`
- `persons.update`
- `judges.assign`
- `citations.create`

The recommended import order is:

1. create or discover the organization
2. create or discover the show
3. upsert the schedule
4. create divisions, sections, and classes
5. create or update people
6. assign judges
7. create entries
8. upload media
9. add citations and credits

If a resumable import pollutes a show hierarchy, `shows.reset_schedule` is the
canonical cleanup path instead of creating a duplicate show record.

## Related Documents

- [Brief](brief.md)
- [Acceptance Criteria](acceptance.md)
- [Design](design.md)
- [Realization README](realizations/a-firstbloom/README.md)
- [Validation Evidence](realizations/a-firstbloom/validation/README.md)
- [Interaction Contract](realizations/a-firstbloom/interaction_contract.yaml)
- [Autosoftware Agent Principles](AUTOSOFTWARE_AGENT_PRINCIPLES.md)
- [Taxonomy Model](flower_show_taxonomy.md)
- [Decision Log](decision_log.md)
- [Approach](approaches/default.yaml)
