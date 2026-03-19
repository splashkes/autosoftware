# Autosoftware Agent Principles

This seed-local overlay inherits the canonical doctrine in
[kernel/protocol/v1/AUTOSOFTWARE_AGENT_PRINCIPLES.md](../../kernel/protocol/v1/AUTOSOFTWARE_AGENT_PRINCIPLES.md).

For `0000-genesis`:

- agents should treat the seed docs and founding realization outputs as the
  primary source of truth
- direct database or runtime mutation is out of scope unless a later seed
  explicitly introduces it
- the registry and contract model should remain discoverable to agents even
  when the founding seed is mostly document-shaped
