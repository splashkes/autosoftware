# Production Calendar Workbench Realization

This realization is the production-calendar-first variant of the calendar sync
hub seed.

It should make the production calendar the main operator workspace while still
preserving provenance and downstream propagation visibility.

## Why This Exists

Some users do not primarily think in terms of connector graphs.
They think in terms of the calendar they are responsible for publishing.

This realization leads with:

- intake of normalized inbound events
- approval into production truth
- timeline and calendar views of approved events
- destination publication state on the same operational surface

## Boundary

This directory defines the draft contract and approach boundary for the
workbench-first realization.
It does not yet include runnable artifacts.
