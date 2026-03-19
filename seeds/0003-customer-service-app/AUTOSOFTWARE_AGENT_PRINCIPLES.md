# Autosoftware Agent Principles

This seed-local overlay inherits the canonical doctrine in
[kernel/protocol/v1/AUTOSOFTWARE_AGENT_PRINCIPLES.md](../../kernel/protocol/v1/AUTOSOFTWARE_AGENT_PRINCIPLES.md).

For `0003-customer-service-app`:

- agents should use the declared ticket, thread, inbox, and article commands
  rather than hidden admin mutations
- service-token access should be treated as the normal remote-agent path for
  support automation and operator tooling
- private ticket state, participant identity, and reply permissions must remain
  explicit in the contract rather than implied by the UI
