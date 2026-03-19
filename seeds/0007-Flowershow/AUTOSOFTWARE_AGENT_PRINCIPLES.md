# Autosoftware Agent Principles

This seed-local overlay inherits the canonical doctrine in
[kernel/protocol/v1/AUTOSOFTWARE_AGENT_PRINCIPLES.md](../../kernel/protocol/v1/AUTOSOFTWARE_AGENT_PRINCIPLES.md).

For `0007-Flowershow`:

- remote agents should author shows, schedules, classes, rules, citations, and
  related flower-show data through semantic API commands, not through hidden
  admin handlers and not through direct database writes
- the normal remote-agent path is `service_token`; session users and service
  agents should reach equivalent operational capabilities and comparable or
  better observability
- flower-show control should evolve toward system-native authority over
  organizations and shows, with delegated grants and revocations visible in the
  ledger, rather than relying permanently on auth-provider role claims
- cited source content and runtime prompt context must stay separate: citations
  become canonical flower-show truth, while assistant instructions remain
  runtime-only
- public views may hide private identity data, but authenticated agents should
  still receive stable object ids, workspace projections, and useful structured
  errors
