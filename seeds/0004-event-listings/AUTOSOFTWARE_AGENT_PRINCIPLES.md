# Autosoftware Agent Principles

This seed-local overlay inherits the canonical doctrine in
[kernel/protocol/v1/AUTOSOFTWARE_AGENT_PRINCIPLES.md](../../kernel/protocol/v1/AUTOSOFTWARE_AGENT_PRINCIPLES.md).

For `0004-event-listings`:

- agents should discover event, version, venue, and organizer surfaces through
  the registry and the realization contract, not by scraping organizer pages
- organizer workspaces and publish flows should stay available through the same
  normalized commands and projections used by the first-party UI
- stable event and version identifiers must remain available even when public
  routes are slug-based
