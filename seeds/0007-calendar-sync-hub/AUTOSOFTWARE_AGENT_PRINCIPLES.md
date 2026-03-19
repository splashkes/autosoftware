# Autosoftware Agent Principles

This seed-local overlay inherits the canonical doctrine in
[kernel/protocol/v1/AUTOSOFTWARE_AGENT_PRINCIPLES.md](../../kernel/protocol/v1/AUTOSOFTWARE_AGENT_PRINCIPLES.md).

For `0007-calendar-sync-hub`:

- agents should use declared sync, normalization, promotion, and propagation
  commands rather than direct datastore edits across connector state
- job state, lineage, incidents, and production-truth transitions should remain
  inspectable through projections, not only through operator dashboards
- any connector secrets, credentials, or maintenance-only repair paths must be
  clearly separated from the normal operational contract
