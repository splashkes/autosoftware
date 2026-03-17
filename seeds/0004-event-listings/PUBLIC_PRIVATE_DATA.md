*Quick `PUBLIC_PRIVATE_DATA.md` map: top = canonical nodes and their shared/public/private/runtime slices; middle = edge visibility, auth examples, and attendance/version rules; bottom = public API, runtime-only exclusions, and extension rules.*

# Public / Private Data

This document defines the seed-local data boundary for `0004-event-listings`.

The canonical model is graph-first.
Current realizations may still expose mostly event-centric screens and
projections, but the shared boundary should be defined in terms of nodes,
versions, and edges so a different team can build another experience on the
same underlying data.

## Canonical Nodes

Core publishing and discovery nodes:

- `calendar`
- `event`
- `event_version`
- `series`
- `series_version`
- `venue`
- `venue_version`
- `organizer_profile`
- `organizer_profile_version`
- `media_asset`
- `media_asset_version`
- `taxonomy_term`
- `actor`

Optional or deferred extension subgraphs:

- `attendance_intent`
- `registration`
- `check_in`
- `contact_identity`

## Event

### Shared Metadata

Shared metadata is the stable slice another client should be able to trust even
before the full event is anonymously readable.

For `event` it includes:

- stable `event_id`
- stable `slug`
- stable `status`
- `calendar_id`
- `current_version_id`
- `title`
- `start`, `end`, `timezone`, `all_day`
- discovery-facing `category`
- discovery-facing `location`, `venue`, and `neighborhood`

This is the minimum public registry shape for an event record.

### Public Payload

Public payload is the accepted event content intentionally visible to anonymous
users when the event is in a public lifecycle state.

For `event` it includes:

- `summary`
- `description`
- `venue_note`
- public organizer snapshot fields such as `organizer_name`, `organizer_role`,
  and `organizer_url`
- public media snapshot fields such as `cover_image_url`
- `featured_blurb`
- `external_url`
- `tags`
- `crowd_label`
- public discovery counters or signals such as `share_count` and `save_count`

This is the slice another team should be able to use to build a public
directory, calendar, or event-detail site.

### Private Payload

Private payload is accepted event truth that exists in the shared model but is
visible only to organizer-authenticated or service-authenticated clients.

For `event` it includes:

- organizer-only draft visibility state
- the unpublished or draft current version when the event is not
  anonymously readable
- organizer-only editorial notes or release controls when a realization carries
  them

The important rule is that the same event can have a stable registry record
before its fuller content is anonymously visible.

### Runtime-Only

Runtime-only values may appear in realization responses but are not accepted
registry content for the event object itself.

For `event` they include:

- derived labels such as `range_label`, `location_label`, and `status_label`
- presentation hints such as `theme_class`
- route-discovery helpers such as `record_api_url`, `handle_api_url`, and
  `workspace_api_url`

These are conveniences, not shared truth.

## Version Objects

Version objects exist to make changes transparent.

For `event_version`, `series_version`, `venue_version`,
`organizer_profile_version`, and `media_asset_version` the typical split is:

- `shared_metadata`: stable `version_id`, parent object ID, timestamps,
  lifecycle, and provenance identifiers such as `created_by_actor_id`
- `public_payload`: the accepted public snapshot and public-facing change
  summary once that version is published
- `private_payload`: draft snapshot content, internal notes, and any
  organizer-only approval context
- `runtime_only`: preview helpers, comparison helpers, or other non-canonical
  conveniences

Version objects should not be flattened away into "last edited at" strings when
the seed cares about transparency or reuse.

## Related Objects

### Series

`series` should be a stable reusable identity for connected events.

- `shared_metadata`: `series_id`, stable `slug`, lifecycle `status`,
  `current_version_id`
- `public_payload`: public title, summary, cadence or recurrence label, and any
  public brand framing
- `private_payload`: unpublished series copy or organizer-only notes
- `runtime_only`: sequence labels, convenience filters, or UI grouping helpers

### Venue

`venue` is a place identity, not only a repeated venue string.

- `shared_metadata`: `venue_id`, stable `slug`, lifecycle `status`,
  `current_version_id`, locality or timezone anchors useful across clients
- `public_payload`: public name, address or locality display, URL, arrival
  guidance, and accessibility summary
- `private_payload`: organizer-only venue notes, unpublished venue edits, or
  operational access details
- `runtime_only`: pre-rendered location labels, map URLs, or styling helpers

### Organizer Profile

`organizer_profile` is the reusable host identity for one or more events.

- `shared_metadata`: `organizer_profile_id`, stable `slug`, lifecycle `status`,
  `current_version_id`
- `public_payload`: public display name, summary, URL, public avatar or hero
  media references
- `private_payload`: organizer-only contact routes, unpublished profile edits,
  or private membership context
- `runtime_only`: runtime display labels or current-session convenience flags

### Media Asset

`media_asset` is a reusable media identity, not only a copied URL.

- `shared_metadata`: `media_asset_id`, media kind, lifecycle `status`,
  `current_version_id`, content digest or checksum when relevant
- `public_payload`: accepted public URL, alt text, attribution, focal point, or
  public caption
- `private_payload`: internal source references, unpublished asset versions, or
  private licensing notes
- `runtime_only`: derived renditions, CDN helper URLs, upload-progress hints, or
  editor-local preview URLs

### Taxonomy Term

`taxonomy_term` exists so discovery facets can be shared instead of copied as
plain strings forever.

- `shared_metadata`: `term_id`, vocabulary or namespace, stable slug, lifecycle
  state
- `public_payload`: label, description, parent term reference when relevant
- `private_payload`: editorial-only aliases or unpublished taxonomy changes
- `runtime_only`: display ordering or UI-only facet decoration

### Actor

`actor` models authorship and edit provenance as a related identity.

- `shared_metadata`: `actor_id`, actor kind, stable linked principal or service
  identity when one exists
- `public_payload`: public display name or profile link when the seed allows it
- `private_payload`: private operator labels or restricted identity metadata
- `runtime_only`: current-session helpers or impersonation/debug state

## Edge Visibility Rules

Edges are first-class.
Their visibility should be declared explicitly rather than inferred from the
node payloads.

Core edge expectations for this seed:

- `calendar_publishes_event`: `public`
- `event_current_version`: `mixed`
- `series_current_version`: `mixed`
- `venue_current_version`: `mixed`
- `organizer_profile_current_version`: `mixed`
- `media_asset_current_version`: `mixed`
- `event_in_series`: `public`
- `event_at_venue`: `public`
- `event_hosted_by`: `public`
- `event_uses_media`: `mixed`
- `event_tagged_with`: `public`
- `version_created_by`: `mixed`
- `version_supersedes`: `mixed`

`mixed` means the edge exists in the canonical graph for all clients, but some
traversals or edge attributes are not anonymously visible in every lifecycle
state.

## Media Attachment Rule

Media attachment should be explicit.
Do not infer attachment only from an uploader query.

The model should distinguish:

- `asset_uploaded_by(actor)` or version provenance
- `event_uses_media(event, media_asset)`

That allows a useful traversal such as:

- "what media assets are attached to event X and were uploaded by actor Y?"

without confusing upload provenance with attachment truth.

The attachment edge itself may carry attributes such as:

- `role`
- `slot`
- `position`
- `visibility`

## Auth-Split Examples

### Anonymous Published Event Detail

Anonymous clients should get:

- `event.shared_metadata`
- `event.public_payload`
- any declared public snapshots of related venue, organizer, or media content
- any declared `runtime_only` helpers returned by that realization

They should not get:

- organizer-only draft visibility
- unpublished version content
- private organizer contact or operational notes

### Organizer Workspace

Organizer-authenticated clients should get:

- `event.shared_metadata`
- `event.public_payload`
- `event.private_payload`
- current draft or unpublished version content
- private organizer-side projections over related objects when the realization
  exposes them
- any declared `runtime_only` helpers returned by that realization

### Metadata-Only Public Registration

Even when an event is still draft-only, the seed target is that another client
can still discover a stable registry-layer event record with at least:

- `event_id`
- `slug`
- lifecycle status
- enough shared metadata to understand what exists

The fuller unpublished event content stays private.

## Attendance and Participation

Attendance should be modeled as a subgraph if it is added.

Rules:

- `attendance_intent` is not the same thing as `registration`
- `registration` is not the same thing as `check_in`
- public visitor views usually expose only aggregates or policy labels
- organizer views may expose participant identity or answers
- private registration content should not leak into public event payloads

This seed's first release does not promise full attendee-management.
The important point is that if attendance data appears later, it should use
explicit related objects and edges.

## Public Access Rule

If event data is classified as anonymously public in this seed, it should be
reachable through authoritative event APIs as well as through the first-party
UI.

Public must not mean:

- visible only in HTML
- visible only after reverse-engineering frontend calls
- visible only to the first-party site

Anonymous public access may still be protected by shared delivery controls:

- global rate limiting
- crawl-abuse controls
- scam-resistance or traffic-shaping protections
- other non-discretionary anonymous-client safety controls

Those controls do not make the data private.
They are delivery protections, not permission boundaries.

If a realization needs stronger anonymous-access constraints than the shared
platform defaults, it should declare them explicitly in seed or realization
docs rather than hiding them in implementation detail.

## Runtime-Only Exclusions

These are the kinds of things that should usually stay out of the shared
registry entirely:

- CSRF tokens and session secrets
- upload-progress percentages
- pagination cursors
- ephemeral recommendation scores
- client-local filter state
- transient preview URLs
- CSS classes or layout hints that are not part of accepted content

These can appear in realization APIs when useful, but they are not canonical
registry truth.

## Extension Rule

This seed should evolve additively.

Rules:

- if multiple clients should rely on a new truth, add it to the canonical graph
- if only one realization needs extra structured truth today, prefer a
  namespaced extension or a new related node over stretching an old field
- if something is rendering convenience only, keep it `runtime_only`
- do not silently repurpose an existing canonical field for a new meaning

## Contract Rule

This seed's `interaction_contract.yaml` should make the boundary
machine-readable:

- the primary objects should declare grouped `data_layout`
- the graph edges should be declared in `domain_relations`
- each projection should declare `data_views` by auth mode when visibility
  differs

The registry browser and registry APIs should show that same grouping directly
so humans and agents do not have to infer it from prose alone.
