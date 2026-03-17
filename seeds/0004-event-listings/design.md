# Design

This seed defines an event publishing and discovery product, not a full
ticketing platform.

The canonical model for this seed is graph-first.
If events, venues, organizers, media, versions, or attendance records are real
domain truths, they should be modeled as nodes and edges, not hidden forever in
one flattened `event` record.

## Product Shape

The application should have:

- an organizer admin workspace
- a public-facing event directory
- a calendar view and event detail pages
- a registry-visible graph that alternate clients can traverse

The key job is making event publishing fast, public discovery pleasant, and
shared data reusable by a different team or agent.

## Actors and Access

- Organizer: authenticated staff member who creates and manages events.
- Public visitor: anonymous visitor who browses, searches, filters, and opens
  event pages.
- Alternate client: another UI, agent, or integration that consumes the same
  contracts and graph rather than reverse-engineering the first-party site.

## Canonical Graph

The core publishing and discovery graph for this seed should include these
stable object identities:

- `calendar`: a public publication namespace or calendar brand that publishes
  events
- `event`: one concrete event occurrence or edition with a stable machine ID and
  stable public handle
- `event_version`: immutable event content version carrying draft, accepted, or
  published truth over time
- `series`: a reusable line of connected events such as a weekly meetup or
  seasonal program
- `series_version`: immutable version history for the series' public or private
  editorial content
- `venue`: a stable place identity rather than only a repeated venue string
- `venue_version`: immutable venue content and provenance
- `organizer_profile`: a public-facing organizer identity used across many
  events
- `organizer_profile_version`: immutable organizer profile edits and provenance
- `media_asset`: a stable media identity that may be reused across multiple
  events or series
- `media_asset_version`: immutable uploaded or accepted media version
- `taxonomy_term`: stable category, topic, or tag identity used for filtering
- `actor`: the identity that created or edited versions, whether that actor is a
  human organizer, service user, or migration/import process

The core edges should include at least:

- `calendar_publishes_event`
- `event_current_version`
- `series_current_version`
- `venue_current_version`
- `organizer_profile_current_version`
- `media_asset_current_version`
- `event_in_series`
- `event_at_venue`
- `event_hosted_by`
- `event_uses_media`
- `event_tagged_with`
- `version_created_by`
- `version_supersedes`

This graph is the durable public project model.
Individual realizations may still expose event-centric screens, but they should
not treat related things as permanently second-class.

## Versions and Provenance

Content-bearing objects should not be silently overwritten.
They should have stable object IDs plus immutable versions.

Rules:

- the stable object is the durable identity used for cross-client linking
- meaningful edits create a new immutable version object
- a "current" read model is a materialized traversal, not the only truth
- published history should remain inspectable so changes are transparent
- versions should record provenance such as `created_at`, `created_by_actor_id`,
  optional `accepted_at`, and optional `change_summary`

This matters for:

- events whose public description or time changes
- venues that are renamed or moved
- organizer profiles that are updated
- media assets that are replaced with newer accepted versions

## Snapshot vs Reference Rules

References and snapshots serve different jobs.

Rules:

- use references for shared reuse: `venue_id`, `series_id`, `organizer_id`,
  `media_asset_id`, `taxonomy_term_id`
- use accepted public snapshots when historical event pages must stay stable
  even if related objects later change
- do not force alternate clients to guess whether an old event page should
  render the current venue profile or the accepted venue snapshot at the time of
  publication

An event detail page may therefore include both:

- references to the current related objects
- the accepted public snapshot that was part of the published event version

## Public / Private Boundary

The boundary vocabulary for this seed is defined in
[PUBLIC_PRIVATE_DATA.md](PUBLIC_PRIVATE_DATA.md).

The important point is that the seed must be explicit about:

- what is shared registry truth
- what is anonymously public through authoritative APIs as well as the UI
- what remains organizer-only
- which few fields are merely runtime conveniences and not canonical registry
  content

The public boundary should apply to nodes, edges, and projections.
Do not assume relation visibility is always the same as object visibility.

## Event Model

Each event should have:

- stable event object identity for machine clients and alternate UIs
- stable public URL that does not change when the event is edited
- start timestamp, end timestamp, authoritative timezone, and optional
  `all_day` behavior
- a current event version and a transparent version history
- references to related `series`, `venue`, `organizer_profile`, `media_asset`,
  and `taxonomy_term` nodes when those are known
- accepted public snapshots where historical stability matters

Public event content should be able to cover at least the common expectations of
modern event products such as Facebook or Meetup, including:

- title, summary, description, and cover media
- schedule, timezone, cancellation state, and multi-day display
- venue/address/display guidance
- organizer identity and public links
- topic/category/tag discovery
- public share or save signals when the product exposes them
- optional external URL for off-site destination or registration

## Attendance and Participation

The first release stays focused on publishing and discovery, but the model
should leave room for participation data without breaking the graph.

If attendance data is added later, it should use a related subgraph rather than
become a loose JSON blob on the event:

- `attendance_intent`: public or semi-public intent such as interested, going,
  or not-going
- `registration`: organizer-controlled enrollment truth with approval or
  waitlist state
- `check_in`: operational proof of attendance
- `contact_identity`: email-only or guest attendee identity when the attendee is
  not a shared account principal

Public counts, organizer rosters, and private registration answers should be
separate projections over that subgraph, not one mixed payload.

## State Model

- `draft`: editable and not public
- `published`: publicly discoverable in list, calendar, search, and filters
- `canceled`: still publicly visible with a clear canceled label so visitors do
  not assume the event disappeared by mistake
- `archived`: removed from default upcoming discovery surfaces, while the event
  detail page may still remain reachable at the same URL with an archived
  notice

Unpublishing an event returns it to `draft`.

## Core Workflows

### Organizer Workflow

Organizers should be able to:

- create an event draft
- add or edit event title, summary, description, start and end time, timezone,
  venue, category, and public links
- publish or unpublish an event
- edit an existing event without losing its public URL
- mark an event as canceled or archived

These organizer workflows must not be private implementation details of the
first-party UI.
A realization should expose the same semantic commands and private organizer
read models so another UI or agent can manage the same event objects.

### Public Discovery Workflow

Visitors should be able to:

- browse upcoming events as a list
- switch to a month-calendar view
- search by title or keyword
- filter by category, date range, and location
- open a dedicated event page with clear time, venue, and status information

Public discovery may expose only the metadata appropriate for public visitors,
while organizer-facing projections may expose fuller draft or editorial state.
That split should be explicit in the realization contract rather than hidden in
server-only code.

Each event instance should also produce a stable registry-layer record whose
public registration includes at least the seed's shared metadata, even when the
fuller event payload is private or draft-only.

## Extension Policy

This seed should evolve additively.

Rules:

- if multiple clients should rely on a new truth, make it canonical in the
  shared graph
- if only one realization needs extra structured truth today, prefer a
  namespaced extension or a new related node over silently stretching an old
  field
- if something exists only for rendering or transport convenience, keep it
  `runtime_only`
- do not silently repurpose an existing canonical field for a new meaning

Examples:

- seating maps or sponsor packages should become new graph nodes if they carry
  durable truth
- CSS theme classes, client-side sort toggles, and pagination cursors should
  stay runtime-only

## MVP Boundaries

Included in v1:

- one organization managing one public calendar
- single events and simple multi-day events
- list and calendar views
- keyword search plus filtering by date, category, and location
- stable event URLs and explicit status behavior
- graph-first identity for event, series, venue, organizer, media, and version
  concepts even if the first operational surfaces stay event-centric

Deferred beyond v1:

- ticket sales
- waitlists
- advanced attendee management
- sponsor packages
- advanced recurrence rules
- ICS sync and external calendar import

## Realization Guidance

Realizations should make public discovery trustworthy first: time display,
status behavior, URL stability, and filter semantics should be deterministic.

Realizations should also declare:

- a stable by-ID event projection for alternate clients
- a public handle-based detail projection for the public site
- a private organizer workspace projection for draft and editorial state
- the domain objects and relations that make up the canonical event graph
- a machine-readable event data layout and auth-mode visibility matrix in
  `interaction_contract.yaml`

The current MVP may keep most operational surfaces centered on `event`, but the
canonical model should stay graph-first so future realizations can expose richer
venue, organizer, series, media, version, or attendance traversals without
breaking shared identity.
