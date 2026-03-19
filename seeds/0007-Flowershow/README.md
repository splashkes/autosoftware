# 0007-Flowershow

Federated, append-only registry for horticulture and design competitions.

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

## Related Documents

- [Brief](brief.md)
- [Acceptance Criteria](acceptance.md)
- [Design](design.md)
- [Autosoftware Agent Principles](AUTOSOFTWARE_AGENT_PRINCIPLES.md)
- [Taxonomy Model](flower_show_taxonomy.md)
- [Decision Log](decision_log.md)
- [Approach](approaches/default.yaml)
