# Decisions

## The Product Is An Orchestration Hub

The system is framed as calendar orchestration rather than a personal calendar
replacement.

Reason:

- the value is coordination across fragmented systems
- the hard part is lineage, normalization, and propagation
- personal productivity features would expand scope without solving the core
  problem

## Production Calendar Is The Outbound Authority

The hub should treat one production calendar as the authoritative source for
propagation to downstream calendars.

Reason:

- not every connected calendar should be able to fan changes outward
- operators need a clear authority model
- propagation rules become easier to explain and audit

## Provenance Must Survive Normalization

Normalization should not erase origin information.

Reason:

- operators need to debug upstream changes and conflicts
- downstream consumers need confidence about where an event came from
- event history is part of the product, not just implementation metadata

## Keep Kernel Changes Minimal

The first implementation should prefer seed-local behavior and only ask the
kernel for clearly reusable capabilities.

Reason:

- most of the problem is product logic, adapter work, and observability
- broad kernel work would slow down a still-evolving seed
- the seed can prove concrete kernel gaps before asking for shared changes
