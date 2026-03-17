# Realization Notes

Implementation should prioritize public browsing quality alongside admin speed,
but it must also make the project model legible.

Recommended build order:

1. durable event object identity and append-only claim persistence
2. organizer commands and public projections backed by the same materialized state
3. human-visible ledger/history surfaces for each event
4. calendar rendering, search facets, and final browsing polish
