> **Before you start.** Read these first:
> [EVOLUTION_QUICK.md](../../EVOLUTION_QUICK.md) (one page) |
> [EVOLUTION_INSTRUCTIONS.md](../../EVOLUTION_INSTRUCTIONS.md) (full guide) |
> [Seed Execution Model](../../docs/seed_execution_model.md) (how seeds become running software)

# Calendar Sync Hub Seed

This seed defines a calendar orchestration product that aggregates events from
multiple sources into a normalized hub, establishes a production calendar as
the operational authority, and propagates approved events outward to
destination calendars.

Primary surfaces in this seed:

- source connector management and sync operations
- normalized event review and production-calendar control
- propagation status, retries, and downstream delivery visibility
- lineage, conflict, and sync-health inspection

Current planned realizations:

- `a-network-operations-hub` as a topology-first operator console centered on
  sync health, incidents, and propagation reliability
- `b-production-calendar-workbench` as a timeline-first production workspace
  centered on review, curation, and controlled publication

Supporting concept notes retained at the seed root:

- `PLAN.md`
- `SCHEMA.md`
- `INGESTION.md`
- `PROPAGATION.md`
- `UI.md`
- `WORKERS.md`
- `ROADMAP.md`
- `KERNEL_NOTES.md`
