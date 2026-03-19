# Autosoftware Agent Principles

This seed-local overlay inherits the canonical doctrine in
[kernel/protocol/v1/AUTOSOFTWARE_AGENT_PRINCIPLES.md](../../kernel/protocol/v1/AUTOSOFTWARE_AGENT_PRINCIPLES.md).

For `0005-charity-auction-manager`:

- bidding, lot management, registration, and settlement flows should be
  contract-driven so agent operators are not weaker than admin users
- service-token callers should receive structured validation and permission
  errors for auction-critical operations
- direct ledger or database repairs remain maintenance paths unless a
  realization declares them explicitly
