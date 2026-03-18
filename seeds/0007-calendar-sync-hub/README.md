# Seed 0007 - Calendar Sync Hub

`0007-calendar-sync-hub` is a docs-first seed for a system that ingests events
from many calendar sources, normalizes them into one operational model, and
propagates approved updates outward from a central production calendar.

The core product idea is a calendar orchestration layer rather than a personal
productivity calendar. The hub acts as a time-network router across external
systems, venues, event feeds, and internal production calendars.

## Seed Map

- `brief.md` captures the requested product shape and scope.
- `design.md` explains the system boundary, operating model, and future
  realization direction.
- `decision_log.md` records the durable choices that shape this seed.
- `acceptance.md` defines what future realizations must satisfy.
- `notes.md` points to the detailed supplemental planning docs already in this
  seed.
- `approaches/` holds named approaches that future realizations can implement.
- `realizations/` is reserved for compiled outputs; none are committed yet.
- `seed.yaml` holds machine-readable seed metadata.

## Supplemental Planning Docs

The original planning notes remain in place as seed-local references:

- `PLAN.md`
- `INGESTION.md`
- `PROPAGATION.md`
- `SCHEMA.md`
- `UI.md`
- `WORKERS.md`
- `KERNEL_NOTES.md`
- `ROADMAP.md`

## v1 Intent

- ingest events from multiple upstream calendar sources
- normalize those events into one canonical event model
- designate one production calendar as the authoritative outbound source
- propagate approved changes to downstream calendars and feeds
- preserve provenance, delivery state, and operator visibility

## v1 Non-Goals

- arbitrary bidirectional editing across every connected external calendar
- replacing personal calendar tools
- hiding propagation or transformation state from operators
