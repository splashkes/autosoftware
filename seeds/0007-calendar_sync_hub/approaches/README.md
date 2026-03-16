# Approaches

`approaches/` holds named, machine-readable approaches for the calendar sync
hub seed.

Current approaches:

- `a-network-operations-hub` for a topology-first operator console centered on
  sync health, incidents, and propagation reliability
- `b-production-calendar-workbench` for a production-calendar-first workspace
  centered on review, curation, and controlled publication

Each realization should point back to one approach through
`realization.yaml:approach_id`.
