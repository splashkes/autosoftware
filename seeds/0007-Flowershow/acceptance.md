# Acceptance

The system is complete when:

## Core Functionality
- Shows can be created and browsed
- Entries are captured with media and placements
- Entries link to people and classes
- Judges can be associated with shows

## Schedule Hierarchy
- A show has a schedule with divisions, sections, and classes
- Classes can be browsed within schedule structure
- Entry `category_id` is replaced by `show_class_id`
- Classes can be explicitly reordered within sections

## Governance & Provenance
- A show can cite a standard edition (e.g. OJES 2019)
- A source document can be registered with type, URL, and checksum
- Any structured record can carry source citations with page ranges
- Extraction confidence is recorded per citation

## Rule Inheritance
- Standard rules are stored per edition with domain, type, and body
- A local class can override a standard rule with type (replace, narrow, extend, local_only)
- Override provenance is preserved (base rule + rationale)

## Rubric Scoring
- Judging rubrics can be defined with criteria and max points
- Entry scorecards store per-criterion scores from judges
- Placements and awards can be computed from scorecards

## Taxonomy
- Entries can be tagged with taxons
- Cross-link navigation works (e.g. click "rose")

## Awards
- Awards can be defined per organization
- Awards correctly compute results from taxonomy filters and scoring rules

## Leaderboards
- Current season leaderboard visible per organization

## Show Admin
- Control panel for live show operations
- Manage judges, classes, entries, media, and winners per class
- The same operator workspace is used for show setup and live event admin
- Operators can move an entry to a different class when judging corrects it
- Operators can change the assigned entrant on an entry
- Operators can delete a mistaken entry when it has not progressed into scoring
- Entries can be added before photos and completed later by another operator
- Entry intake can quickly look up club members and guests by name from the
  show workspace without leaving the event page
- Free-form show credits (host, scribe, designer, etc) are managed separately
  from permissions
- A scoped onsite support role can perform live floor corrections without broad
  organization authority
- A show-scoped judge support role can assign judges and make live floor
  corrections without receiving global admin access
- A full-show thumbnail or board view can show all classes and entries at once
- Multiple operators see live updates via SSE (no reload)
- HTMX-driven partial updates
- The same normal operations are reachable through declared semantic API
  commands and projections for authenticated agents
- Operator-facing pages expose the active contract, stable ids, and related
  agent-facing command/projection paths so humans can inspect the same surface
  authenticated agents use

## Media
- Multiple photos and/or videos per entry
- Client-side optimization before upload
- Server-side transcode if file exceeds threshold
- Cross-browser, low-bandwidth, high-performance upload UI

## Agent Authoring
- Contract discovery is available through `GET /v1/contracts` and a
  realization-specific self path
- The declared contract exposes auth modes, UI-surface mappings, runtime-only
  command context, and structured error codes
- Data can be authored through API by remote agents using `service_token`
  authentication
- Authenticated agents can read private workspace and by-id projections using
  stable ids rather than only public slugs
- Runtime-only authoring context can be sent with commands without becoming
  canonical truth
- Cited source material is preserved as source documents and citations rather
  than as opaque prompt text
- Authenticated API errors expose stable codes, request ids, and actionable
  hints

## Privacy
- Public view shows initials only
- Private identity mapping retained
- Suppression hides entries without deletion

## Authentication
- Cognito for identity (login/signup/tokens)
- Roles (admin, judge, entrant, public) managed in-app per organization/show
- Long-term authority is modeled as system-native accepted history rather than
  as auth-provider role truth
- Grant, revoke, and delegated-control behavior is inspectable through ledger
  or effective-access projections once the kernel-native authority model lands

---

## Nice-to-Have (not required for acceptance)
- Cross-organization leaderboards
- Advanced filtering UI
- Historical season browsing
- Browsing by flower, category, scientific name, judge, club
- Individual history for a person including all associated media
- Show summary with real-time winner display
