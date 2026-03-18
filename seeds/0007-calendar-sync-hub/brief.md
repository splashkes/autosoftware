# Brief

Define a calendar sync hub that aggregates events from many external calendar
systems into one operational model and allows a central production calendar to
propagate approved updates outward.

Desired initial outcome:

- multiple source calendars can be connected and polled incrementally
- incoming events are normalized into one canonical event model
- operators can inspect source lineage and propagation state
- one production calendar acts as the authoritative outbound source
- downstream calendars and feeds can receive controlled pushes from that
  production model

Primary users:

- operations staff coordinating event schedules across many systems
- producers maintaining the authoritative production calendar
- integrators configuring source and destination adapters

Scope:

- source ingestion from APIs and feed formats such as Google Calendar, CalDAV,
  and ICS
- event normalization into one shared schema
- provenance and revision history for imported and propagated events
- destination propagation with explicit delivery status
- operational visibility into conflicts, failures, and unsynced changes

Constraints:

- keep the seed focused on orchestration rather than personal scheduling
- preserve source lineage instead of flattening away origin details
- keep kernel changes minimal unless a seed-local implementation proves a
  concrete gap
- treat propagation state and operator observability as core, not optional
