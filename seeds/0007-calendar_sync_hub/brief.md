# Brief

Build a calendar sync hub that treats calendar data as an operational network
rather than a single end-user calendar.

Full seed vision:

- ingest events from multiple calendar systems into one normalized model
- preserve source identity, provenance, and revision history for every event
- designate one production calendar inside the hub as the write authority for
  outbound propagation
- push approved production events to downstream calendars with visible per-edge
  delivery state
- let operators inspect conflicts, upstream changes, and sync failures from one
  coherent operational surface
- compare two realization directions before implementation: an operations-first
  network console and a production-calendar workbench

Constraints:

- v1 should remain hub-and-spoke, not multi-master calendar editing
- the product is for calendar orchestration, not personal scheduling
- lineage and observability are first-class requirements, not optional admin
  details
- kernel changes should stay minimal; most behavior should live in the seed
- the same interaction contract should support both human UI flows and machine
  clients
