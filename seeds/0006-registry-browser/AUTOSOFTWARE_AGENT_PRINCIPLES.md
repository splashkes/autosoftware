# Autosoftware Agent Principles

This seed-local overlay inherits the canonical doctrine in
[kernel/protocol/v1/AUTOSOFTWARE_AGENT_PRINCIPLES.md](../../kernel/protocol/v1/AUTOSOFTWARE_AGENT_PRINCIPLES.md).

For `0006-registry-browser`:

- agents should be able to inspect the registry through authoritative API and
  contract surfaces without scraping HTML
- the browser remains read-only, but it should still expose enough path and
  schema detail for alternate agent clients to reuse the same information
- any convenience summaries should point back to stable objects, claims,
  schemas, and change records
