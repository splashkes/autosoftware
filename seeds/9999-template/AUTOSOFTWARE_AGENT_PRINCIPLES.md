# Autosoftware Agent Principles

This seed-local overlay inherits the canonical doctrine in
[kernel/protocol/v1/AUTOSOFTWARE_AGENT_PRINCIPLES.md](../../kernel/protocol/v1/AUTOSOFTWARE_AGENT_PRINCIPLES.md).

For `9999-template`:

- every new seed should describe how agents discover the contract, authenticate,
  and achieve parity with the first-party UI
- if the seed has meaningful authority semantics, it should document
  system-native subjects, scopes, bundles, and delegation rather than leaning
  on auth-provider roles as canonical truth
- runtime-only authoring context should be modeled explicitly and kept separate
  from canonical shared truth
- the template should bias authors toward stable object ids, semantic commands,
  useful authenticated errors, and in-flight conformance tests
