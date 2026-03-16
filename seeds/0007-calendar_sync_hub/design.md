# Design

This seed defines a calendar signal hub that sits between many calendars and
event systems.

The request is not for "another calendar UI."
It is for a control plane over event time data:

- what arrived from upstream sources
- how it was normalized
- which event version is considered production truth
- where that production truth was propagated
- which deliveries failed, drifted, or require review

## Evaluation Summary

The concept is strong, but the current seed notes are too broad to realize
directly.

Before implementation, four boundaries need to be made explicit:

1. The hub has one write authority: the internal production calendar.
   External sources are imported into the hub. External destinations receive
   propagated copies from the hub. V1 should not behave like a bidirectional
   multi-master sync engine.
2. Normalized event identity must be distinct from external source event IDs.
   A single event may exist across multiple source systems and multiple
   destination copies. The hub needs its own durable event identity plus
   revision lineage.
3. Ambiguous changes require operator-visible review states.
   Upstream edits, recurrence shifts, timezone drift, duplicate detection, and
   downstream push failures should not silently rewrite production truth.
4. The first implementation should compare two materially different operator
   surfaces rather than one blended UI:
   a topology-first network console and a production-calendar workbench.

## Early Design Check

Before implementation begins, confirm these points:

1. V1 is hub-and-spoke with one internal production authority, not general
   bidirectional calendar editing.
2. The first adapter set should stay narrow enough to validate the seams.
   Google Calendar and ICS are the best first pair; CalDAV and event-platform
   adapters can follow once the normalized event model is stable.
3. Propagation should be controlled and inspectable per destination edge rather
   than automatically pushing every inbound event everywhere.
4. Two realization variants should share the same seed and core contract but
   differ in workflow emphasis and operator information architecture.

If any of those are wrong, the seed should be redirected before code is
produced.

## Core Authority Model

The seed should use this flow:

sources -> normalized events -> production events -> destination copies

Where:

- `calendar_source` represents an upstream importer
- `normalized_event` represents the hub-owned interpretation of upstream data
- `production_event` represents the approved internal authority for outward
  publishing
- `calendar_destination` represents a downstream publication target
- `propagation_edge` records per-destination delivery state for a production
  event
- `event_revision` records how the normalized or production interpretation
  changed over time

External source records are evidence.
The production calendar is the operational truth for propagation.

## Scope

Included in this seed:

- source registration and sync scheduling
- normalized event ingestion with provenance and revision tracking
- explicit promotion or approval into a production calendar
- outbound propagation to selected destination calendars
- operator-visible sync health, drift, and retry workflows
- lineage and event-history inspection
- a machine-readable interaction contract shared by human and agent clients

Included in the first realizations:

- source and destination management
- poll-based sync runs and per-source freshness visibility
- normalized event review with clear provenance
- production-event publication controls
- per-destination propagation status and retry
- issue-focused operational filtering

Deferred to later realizations:

- broad adapter coverage beyond the first narrow adapter set
- real-time webhooks and push-trigger fan-out
- automatic duplicate merge heuristics beyond safe first rules
- richer recurrence editing and exception authoring
- advanced conflict-resolution tooling for multi-party negotiation

Out of scope for v1:

- personal calendar replacement
- arbitrary editing directly against every external calendar
- attendee RSVP, reminders, or consumer productivity features
- silent multi-master merges across independent authorities

## Approach Candidates

### A. Network Operations Hub

This realization should lead with the calendar network itself:

- source calendars, hub, and destinations as visible nodes
- sync health, queue depth, and propagation failures as first-class signals
- event lineage and per-edge delivery inspection
- incident queue and retry operations

This is the right realization if the primary user is an operator responsible
for reliability across many systems.

### B. Production Calendar Workbench

This realization should lead with the production calendar as the main
workspace:

- intake queue of normalized events awaiting review
- timeline and calendar views anchored on production truth
- clear approve, defer, publish, and republish workflows
- destination publication state attached to production events

This is the right realization if the primary user is a producer or program
manager curating one central calendar for outward distribution.

Both should remain valid realizations of the same seed rather than one
replacing the other.

## Affected Surfaces

The seed should ultimately expose a shared interaction model with commands such
as:

- registering and configuring calendar sources
- triggering sync runs
- promoting normalized events into production events
- configuring or retrying propagation edges

The seed should expose projections such as:

- source and destination topology overview
- normalized event intake queue
- production calendar timeline
- event lineage and propagation detail
- sync incident and retry queues

## Source Notes

The existing concept documents at the seed root remain valid input material:

- `SCHEMA.md` for initial entity naming
- `INGESTION.md` for adapter and polling direction
- `PROPAGATION.md` for delivery-state language
- `UI.md` for graph and timeline ideas
- `WORKERS.md` for background job decomposition
- `ROADMAP.md` for phased rollout
- `KERNEL_NOTES.md` for minimizing kernel impact

Those files describe ideas.
This `design.md` defines the realization boundary.
